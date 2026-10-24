# ICAN 0.1.0 repository cleanup

This cleanup retains the canvas, account/configuration sync, media storage, custom API templates and the source-built Director. It does not change database models, tables, data directories, encryption keys or request limits.

Removed:

- Unreachable Director panels, GLTF import wrappers, skeleton adapters and their obsolete tests. The active ViewportToolbar import and CameraPanel capture flows remain.
- Unused web release/environment/text-animation helpers and a disabled credit-log hook.
- HTTP registrations and handlers for disabled Bridge, workflow, anonymous storage, LinuxDo OAuth and credit-management endpoints. The security middleware still denies these paths.
- Unreferenced workflow/Agent services and embedded default skill templates. Active AI call-log handlers moved to `handler/ai_call_log.go`.

Retained deliberately:

- Shared protocol helpers used by parameter translation, task cancellation, polling and history. Disabled providers were not re-enabled.
- Models, repository functions and persisted fields needed to read existing data; no schema/table removal.
- Director rig math and scene import/export validation utilities used by current code or security tests.
- Tests, locks, licenses, audit evidence, screenshots and migration utilities.

Validation: Go suite passed; Director 294 tests across 29 files passed; web 18 tests passed. Removed 13 tests covered only removed, unreachable components. Director and Next.js production builds passed. Browser fixture smoke passed: add camera, capture/send image to canvas, reload and restore the scene; no external requests were observed. This smoke uses local fixtures, not real-account or paid-provider verification. Bun audit found no advisory matches in 270 Director and 467 web packages. See `evidence/cleanup-local.json` for log hashes. Final Docker verification runs in GitHub CI; no local Docker daemon is available.

Rollback: revert this cleanup commit and rebuild the image. No reverse data migration is required for this change because persistence schemas are unchanged. Preserve the original data directory and encryption key across image updates. Real-account cross-device and existing-data migration acceptance remain open in `../progress/todo.md`.
