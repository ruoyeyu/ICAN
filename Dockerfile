# Pinned upstream manifests; update only with dependency and image review.
FROM oven/bun:1.4.4@sha256:1d22548f177f4ff38e0c15baa3c2c46c6c2060ba2d1bc573bb4720b407548f1d AS bun
FROM library/node:24.22.1-bookworm-slim@sha256:55fb5f5a527ea2c5c4a95f963e48ef90aa38c7b346054fdc8ee9957a2b55d205 AS web-build
COPY --from=bun /usr/local/bin/bun /usr/local/bin/bun
WORKDIR /app/web
ENV NEXT_TELEMETRY_DISABLED=1 DO_NOT_TRACK=1
COPY web/package.json web/bun.lock ./
RUN bun install --frozen-lockfile --ignore-scripts
COPY web ./
COPY director /app/director
WORKDIR /app/director
RUN bun install --frozen-lockfile --ignore-scripts && bun audit && bun run test && bun run build
RUN mkdir -p /app/web/public/director && cp dist/* /app/web/public/director/
WORKDIR /app/web
RUN bun audit && bun test src && bun run build

FROM library/golang:1.27.3-bookworm@sha256:972b4110ba0f47539a975c31e52c64765e1ff19f2d74469ae4a595c6676e80a5 AS api-build
WORKDIR /app
ENV GOTOOLCHAIN=local
COPY go.mod go.sum ./
RUN go telemetry off && go mod download && go mod verify
COPY config ./config
COPY handler ./handler
COPY middleware ./middleware
COPY model ./model
COPY repository ./repository
COPY router ./router
COPY service ./service
COPY integration ./integration
COPY internal ./internal
COPY main.go ./
RUN CGO_ENABLED=0 go test ./... && CGO_ENABLED=0 go build -trimpath -o /server .
RUN mkdir -p /runtime-data /runtime-cache

# Candidate runtime: signed Wolfi packages, pinned to the reviewed image digest.
FROM cgr.dev/chainguard/glibc-dynamic@sha256:2827063c9efb4ba971d8621eb75f4e61b8d19850a6293a797df37358cc6f687b
ARG SOURCE_REVISION
LABEL org.opencontainers.image.source="https://github.com/ruoyeyu/ICAN" \
      org.opencontainers.image.revision=$SOURCE_REVISION \
      org.opencontainers.image.version="0.1.1"
WORKDIR /app
ENV NODE_ENV=production NEXT_TELEMETRY_DISABLED=1 HOSTNAME=0.0.0.0
COPY --from=web-build /usr/local/bin/node /usr/local/bin/node
COPY --from=web-build /usr/local/LICENSE /usr/share/licenses/node/LICENSE
COPY --from=api-build /server /app/server
COPY --chmod=0444 docker-entrypoint.mjs /app/docker-entrypoint.mjs
COPY --from=web-build /app/web/public /app/web/public
COPY --from=web-build /app/web/.next/standalone /app/web
COPY --from=web-build /app/web/.next/static /app/web/.next/static
COPY --from=api-build --chown=10001:10001 /runtime-data /app/data
COPY --from=api-build --chown=10001:10001 /runtime-cache /app/web/.next/cache
USER 10001:10001
EXPOSE 3000
CMD ["/usr/local/bin/node", "/app/docker-entrypoint.mjs"]
