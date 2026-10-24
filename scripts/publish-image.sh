#!/usr/bin/env bash
# Only called after the same-run checked archive has been loaded and validated.
set -euo pipefail
: "${GHCR_TOKEN:?missing ephemeral workflow token}"
: "${GITHUB_SHA:?missing source commit}"
[[ "$GITHUB_SHA" =~ ^[0-9a-f]{40}$ ]]
test "$GITHUB_REPOSITORY" = 'ruoyeyu/ICAN'
test "$GITHUB_REF" = 'refs/heads/main'
image='ghcr.io/ruoyeyu/ican'
tag="$image:sha-$GITHUB_SHA-run-$GITHUB_RUN_ID-$GITHUB_RUN_ATTEMPT"
expected_id=$(cat verified-image/image-id.txt)
printf '%s' "$GHCR_TOKEN" | docker login ghcr.io -u "$GITHUB_ACTOR" --password-stdin
unset GHCR_TOKEN
docker tag "$expected_id" "$tag"
docker push "$tag"
reference=$(docker image inspect "$tag" --format '{{range .RepoDigests}}{{println .}}{{end}}' | awk '/^ghcr.io\/ruoyeyu\/ican@sha256:[0-9a-f]+$/ {print}')
[[ "$reference" =~ ^ghcr.io/ruoyeyu/ican@sha256:[0-9a-f]{64}$ ]]
# Verify registry content, not just a mutable commit tag.
docker pull "$reference"
test "$(docker image inspect "$reference" --format '{{.Id}}')" = "$expected_id"
python3 - "$reference" "$tag" "$expected_id" <<'PY'
import json, os, pathlib, sys
reference, tag, image_id = sys.argv[1:]
record = {
    'sourceCommit': os.environ['GITHUB_SHA'], 'image': reference, 'tag': tag,
    'imageId': image_id, 'platform': 'linux/amd64',
    'checks': 'All prerequisite tests, runtime, Compose, inventory and scan steps passed',
    'scanPolicy': 'Block HIGH/CRITICAL matches; preserve complete raw scan including other severities',
    'workflowRun': f"https://github.com/{os.environ['GITHUB_REPOSITORY']}/actions/runs/{os.environ['GITHUB_RUN_ID']}",
    'runAttempt': os.environ['GITHUB_RUN_ATTEMPT'], 'productionApproved': False,
}
pathlib.Path('publication.json').write_text(json.dumps(record, indent=2) + '\n')
with open(os.environ['GITHUB_STEP_SUMMARY'], 'a') as out:
    out.write(f"## Published checked image\n\nSource: `{record['sourceCommit']}`\n\n"
              f"Platform: `linux/amd64`\n\nImmutable reference:\n```text\n{reference}\n```\n\n"
              f"Checks: [{record['checks']}]({record['workflowRun']}). "
              "Full scan and SBOM: `image-security-evidence`; manifest: `published-image`.\n\n"
              "This is a verified build, not production deployment approval. See `docs/security/ghcr-deployment.md`.\n")
PY
