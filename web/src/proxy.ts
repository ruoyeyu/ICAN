import { NextRequest, NextResponse } from "next/server";

// Page authorization is verified against the revocable server session, never
// inferred from a browser-stored user object. API authorization stays in Go.
export async function proxy(request: NextRequest) {
    const path = request.nextUrl.pathname;
    if (path === "/login" || path === "/admin/login" || path.startsWith("/api/") ||
        path.startsWith("/_next/") || path === "/logo.svg" || path === "/favicon.ico" || path.startsWith("/icons/") ||
        path === "/director/director.js" || path === "/director/director.css") {
        return NextResponse.next();
    }
    const cookie = request.cookies.get("ican_session")?.value;
    if (cookie) {
        try {
            const base = process.env.API_BASE_URL || "http://127.0.0.1:8080";
            const response = await fetch(`${base.replace(/\/$/, "")}/api/auth/me`, {
                headers: { Cookie: `ican_session=${cookie}` }, cache: "no-store", redirect: "error",
            });
            const result = await response.json();
            if (response.ok && result.code === 0 && ["user", "admin"].includes(result.data?.role)) {
                const next = NextResponse.next();
                next.headers.set("Cache-Control", "private, no-store");
                return next;
            }
        } catch { /* An unavailable auth service must not expose the workspace. */ }
    }
    const login = new URL("/login", request.url);
    login.searchParams.set("redirect", path);
    const response = NextResponse.redirect(login);
    response.headers.set("Cache-Control", "no-store");
    return response;
}

export const config = { matcher: ["/((?!_next/static|_next/image).*)"] };
