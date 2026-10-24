# Login-required workspace

The inherited client allowed guests to use the canvas and keep projects and imported images in browser storage. The user layout protected the asset library, but not the canvas or image/video workspace. Shared canvas caches could also be reconciled into the next signed-in account. Seeing an imported image in the canvas did not establish that it had been uploaded to the server: the client supported local blob storage.

The existing Go `/api/v1/files` upload route already required an authenticated account. This review did not inspect the production host or its old proxy/upload routes.

## Changes

- Next.js verifies the revocable server session before serving workspace pages, including the Director HTML. Missing, forged or revoked sessions redirect to login. Auth service failures also deny access. Login, static application scripts and API routing stay reachable; the Go API independently checks authorization.
- The HttpOnly session cookie now covers `/`, with the old `/api` cookie explicitly expired. Existing installations need one fresh login. Secure and SameSite settings remain in force.
- The client hides workspace content until account validation finishes. Logout resets the client and navigates to login even when the logout request fails; server-side revocation still requires a successful request. Login starts a fresh document, focus rechecks the account, and another tab's session changes reload the page.
- Image/media imports reject missing login before reading or fetching the material. Asset and prompt catalog endpoints now require login as well.
- Canvas, personal asset, API configuration and image/media blob caches are scoped by account ID. Pending responses cannot apply a previous account's state after a switch. Legacy keys remain untouched and are never automatically displayed, assigned or uploaded to another account.
- Provider reference links retain their separate, controlled authorization; login gating does not remove the provider's ability to read an explicitly shared reference.

## Existing data and update

There is no database schema change and no deletion of database, media, API settings or encryption keys. Account data already synced to the server remains available after login. Preserve the existing data directory and original encryption key when replacing the container.

Old browser-only projects and local blobs without a recorded account owner remain in the old browser storage, but are no longer automatically loaded. Recover them through an explicit, owner-approved export/import; do not clear browser storage or silently claim them for whichever account signs in next. A generic automatic legacy importer is not included in this patch.

The old v0.1.0 release image is immutable and does not gain this fix merely because `main` changes. Use the successful build's new source commit and digest. Production rollout and rollback follow `ghcr-deployment.md`; no server deployment is performed by this change.

## Verification

- Go tests cover anonymous/forged upload and canvas writes, protected catalogs, disabled anonymous upload and cookie scope, alongside existing two-account ownership/session checks.
- Frontend tests cover missing/invalid sessions, authentication failure, account-separated caches, preservation of unowned data and an account switch during asynchronous hydration.
- Final-image smoke tests exercise anonymous page redirects/upload refusal, authenticated page access and revoked-session rejection in the actual production image.
- Local production browser test passed: anonymous redirects/upload refusal, real login, authenticated Director capture/send-to-canvas, logout revocation, Alice/Bob separation and unchanged unowned legacy data; zero page errors. Go suite, 24 frontend tests and production web build passed. See `evidence/login-required-local.json`.
- `scripts/test-login-browser.mjs` reproduces that browser check with disposable data. Run from the repository after building the API and Next standalone output and copying `public`/`.next/static` into it. Set `ICAN_TEST_API_BINARY`, `ICAN_TEST_CHROMIUM`, and optionally `ICAN_PLAYWRIGHT_MODULE` to existing local test tools, then run the script with the project Node version. It uses only fixture accounts, ports 3417/3418, and no paid API.
- Final-image checks run in GitHub CI before GHCR publication; local browser success alone does not publish an image.

## Deployment acceptance still required

Use a fresh private browser window against the real public URL. Confirm every workspace requires login, anonymous upload returns 401, old anonymous routes return 404, and logging out prevents returning to a usable workspace. Check the host's reverse proxy and any legacy public upload routes independently. No conclusion about those external routes is inferred from container tests.
