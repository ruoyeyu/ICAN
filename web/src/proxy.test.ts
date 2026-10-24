import test from "node:test";
import assert from "node:assert/strict";
import { NextRequest } from "next/server";
import { proxy } from "./proxy";

test("workspace pages require a server-validated session", async () => {
    for (const path of ["/canvas", "/canvas/old-local", "/image", "/video", "/asset-library", "/director/index.html"]) {
        const response = await proxy(new NextRequest(`https://ican.invalid${path}`));
        assert.equal(response.status, 307);
        assert.equal(new URL(response.headers.get("location")!).pathname, "/login");
    }
});

test("login, scripts and backend API remain reachable", async () => {
    for (const path of ["/login", "/api/auth/login", "/api/v1/files", "/_next/static/app.js", "/director/director.js", "/logo.svg"]) {
        assert.equal((await proxy(new NextRequest(`https://ican.invalid${path}`))).headers.get("x-middleware-next"), "1");
    }
});

test("forged/revoked cookies and unavailable auth fail closed; valid sessions pass", async () => {
    const original = globalThis.fetch;
    try {
        for (const role of ["guest", "user", "admin"]) {
            globalThis.fetch = (async (_input, init) => {
                assert.equal(init?.cache, "no-store");
                assert.equal(new Headers(init?.headers).get("cookie"), "ican_session=fixture");
                return Response.json({ code: 0, data: { role } });
            }) as typeof fetch;
            const response = await proxy(new NextRequest("https://ican.invalid/canvas", { headers: { cookie: "ican_session=fixture" } }));
            assert.equal(response.status, role === "guest" ? 307 : 200);
        }
        globalThis.fetch = (async () => { throw new Error("unavailable"); }) as typeof fetch;
        assert.equal((await proxy(new NextRequest("https://ican.invalid/canvas", { headers: { cookie: "ican_session=fixture" } }))).status, 307);
    } finally { globalThis.fetch = original; }
});
