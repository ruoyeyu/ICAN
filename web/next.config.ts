import type { NextConfig } from "next";

const config: NextConfig = {
    output: "standalone",
    poweredByHeader: false,
    images: { unoptimized: true, remotePatterns: [], localPatterns: [] },
    // Image optimization is disabled; do not ship its unused native decoders.
    outputFileTracingExcludes: {
        "/*": ["node_modules/sharp/**/*", "node_modules/@img/**/*"],
        // Next traces the server entry separately from application routes.
        "next-server": ["**/node_modules/sharp/**/*", "**/node_modules/@img/**/*"],
    },
    async headers() {
        return [{ source: "/:path*", headers: [
            { key: "X-Content-Type-Options", value: "nosniff" },
            { key: "Referrer-Policy", value: "no-referrer" },
            { key: "X-Frame-Options", value: "DENY" },
            { key: "Content-Security-Policy", value: [
                "default-src 'self'",
                "script-src 'self' 'unsafe-inline' 'unsafe-eval'",
                "style-src 'self' 'unsafe-inline'",
                "font-src 'self' data:",
                "img-src 'self' data: blob: https: http:",
                "media-src 'self' data: blob: https: http:",
                `connect-src 'self' https: http: blob:${process.env.NODE_ENV === "development" ? " ws: wss:" : ""}`,
                "worker-src 'self' blob:", "object-src 'none'", "frame-src 'none'",
                "base-uri 'self'", "form-action 'self'", "frame-ancestors 'none'",
            ].join("; ") },
        ] }];
    },
};
export default config;
