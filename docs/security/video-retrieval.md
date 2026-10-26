# Video completion and delivery

Generation and delivery have separate states. A completed upstream task can be `fetching`, `saving`, `failed` or `ready` locally. The canvas shows completion/retrieval progress and a **继续取回** action on delivery failure. That action never submits a new generation request.

## Cause and scope

The previous protected-download handler wrote success headers before reading the entire upstream stream and discarded copy errors. A truncated response could become an indexed media file. Browser synchronization also awaited media metadata and suppressed polling errors, leaving stale generation progress. Operator evidence supports an HTTP deadline interrupting a transfer; the original transport error and exact browser wait were not captured. This change does not claim either was directly reproduced in production.

The checked download is staged privately on disk, with bounded copy buffers, before success headers or a media record are committed. Read errors, declared-length mismatches, non-video content, incomplete MP4 top-level boxes and missing MP4 `moov`/`mdat` are rejected. MP4 box headers use pinned MIT `github.com/abema/go-mp4 v1.7.4`; no media decoder binary is added. This is transport/container validation, not a full codec decode. Browser playback errors remain visible.

For account-managed video tasks, the first completion query starts one server-side delivery job via the existing task API. Closing the page after that request does not cancel delivery. Subsequent queries and refreshed pages reuse the persisted state and stored media, without going through browser download/upload or the auto-sync Promise cache. Retrieving the completed file is explicit after a failed attempt; no automatic retries, parameter changes or model substitutions occur. Existing channel HTTP timeouts and storage permissions are retained.

The original upstream ID stays on the task. A separate row keyed by task/account holds delivery status, attempt, object ID and sanitized diagnostics. Conditional database updates prevent duplicate claims and stale responses from replacing a completed result. Existing private file endpoints and storage providers are reused; temporary files are not indexed or publicly addressable. Local storage commits use an atomic rename; S3/WebDAV receive a file stream and the media index is committed only after upload succeeds.

Completed task records now remain until explicit task deletion, instead of being removed after ten minutes without a delivery acknowledgement. Their media follows the existing media-deletion flow. Operators should include these records in existing backups. No new numeric capacity, retry, concurrency or timeout limits are introduced.

## Upgrade and rollback

`repository.migrateVideoRetrievalV1` is an additive, idempotent migration creating only `video_retrievals`. It does not change existing account, channel, key, canvas, media or task fields. On startup, an interrupted delivery is marked failed and requires **继续取回**; ready results remain ready. Recognized private staging files from the previous API process are cleaned before accepting requests. Deployment remains the existing single API process; multiple API replicas sharing this database are not a supported recovery mode.

Before updating, stop writes and make a consistent backup of the database, media directory, templates, configuration and the original encryption key; record the old image digest. Pull and verify the new digest, then recreate with the existing Compose/environment and data mounts using `--no-build`. Do not initialize a new database or generate new keys.

Rollback to the previous pinned image leaves the additive table in place. Existing tables and media remain readable. The old image ignores delivery state and retains the old download defect, so do not retry downloads through it. Keep the data and key; a full snapshot restore, if required, must be planned separately because it would discard later writes. No automatic rollback or deletion of existing material is performed.

Already saved truncated files are not silently repaired or deleted. For a task whose original record still exists, use the new delivery path to fetch its existing result. Tasks already removed by an older image cannot be reconstructed from a screenshot; recovery requires their original IDs/configuration. No automatic generation is used as a substitute.

## Verification

Free fixtures cover complete/slow/interrupted streams, short/long declared lengths, missing MP4 index, error redaction, account ownership, concurrent initial claims/retries, refresh reuse, zero generation calls, stale poll protection and migration on a database backup. Browser verification deliberately leaves the media response pending so no metadata event can arrive: the canvas still persists and restores the ready result. API/template browser regressions, TypeScript, production builds and the unchanged image CI gates are required before publication.

Local evidence is recorded in `evidence/video-retrieval-local.json`. Final image results and immutable digest will be recorded separately after CI. No paid generation or production-server operation is performed.
