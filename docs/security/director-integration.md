# Director integration review

## Source and enabled scope

Source: jiguang132/storyai-3d-director-desk, commit
`8c8bd361790be4d37158a7430365e65546e358fe` (MIT, copyright YZ).
The 104 archive files were checked against Git blob hashes in tree
`9d3746ca7794cb208480cb102dc4efb96de5333a` before editing.
The previous tigerowo precompiled bundle is not reused.

Retained: procedural characters/body types/poses, primitives, local FBX/OBJ,
raster panoramas, cameras, screenshots and canvas scene persistence.
Not restored: timeline/video export. ComfyUI, RunningHub and Agent/Bridge
remain server-disabled. The third-party UE mannequin binary and external model
catalog are omitted because redistribution rights were not established.

## Security changes

- Frozen source build, installation scripts disabled, no runtime CDN/font/model
  downloads or automatic plugin updates. Docker audits/tests the director tree.
- Opaque-origin iframe, with no allow-same-origin. A route-specific CSP also
  enforces sandbox on direct navigation, disables network access and permits
  only bundled scripts/styles plus data/blob media. No account credentials enter it.
- Both message receivers check the exact source window and expected origin.
  The host uses `*` only when addressing its own opaque-origin iframe.
- Shared Zod scene validation rejects remote/private URLs, SVG/HTML,
  non-finite transforms and unsupported versions; unknown fields are stripped.
  Incompatible existing scenes report an error and are not overwritten.
- Host panoramas are decoded to raster pixels. Remote images must first be
  saved as canvas media. Repeated panorama messages are deduplicated.
- Screenshot messages require a PNG signature and pass through the existing
  image decoder and account-aware storage. Guests retain local storage behavior.
- Embedded scenes use existing canvas metadata, not iframe localStorage.
  Parent CSP permits data fetches required by local image import, and same-origin
  frames; only the director route allows itself to be framed.

## Dependencies and tests

Vite 6.4.3 → 6.4.5; Vitest 4.1.8 → 4.1.12. Patched transitive versions:
fflate 0.8.3, nanoid 3.3.20, undici 7.30.1, postcss 8.5.23,
source-map-js 1.2.2, browserslist 4.28.7, baseline-browser-mapping 2.11.0.
Removed the macOS-only direct Rollup binary dependency. The initial audit
matched 10 package names, not 10 demonstrated exploits. Patched `bun audit
--json` returned `{}`; build/test packages remain included, without ignore rules.

- Director: 34 files / 307 tests passed; TypeScript and production build passed.
- ICAN: 18 frontend tests passed and Next production build passed.
- Real WebGL browser: procedural character rendering, camera creation, PNG
  capture to host and scene restoration after iframe reload passed.
- Browser sandbox: parent DOM, cookies, localStorage and same-origin API fetch
  were blocked. Network restrictions were not relaxed for the test.
- Full canvas integration and final image CI are tracked in the acceptance
  record below. These checks do not establish production or cross-device readiness.

Test changes are explicit: bridge tests follow the new message and persistence
contract, with new rejection cases for foreign messages, malformed scenes and
unsafe URLs. The unused UE loader and its four binary-specific tests were removed
together. Library UI tests now use synthetic model entries. Stale cosmetic
assertions were reconciled with upstream implementation: one aspect mask, 0.35
camera helper scale, current gizmo geometry and phone pose; jsdom is no longer
expected to resolve CSS variables into a hardcoded color. Character tests now
require procedural rendering. No tests were marked skipped to obtain a pass.

## Data, release and rollback

No database tables, account configuration or encryption keys change. Scenes live
in `metadata.directorProject`; captures use existing media storage. Keep the same
persistent data directory and DATA_ENCRYPTION_KEY when replacing a container.
Back up database and media together; retain copies of old director scenes before
any separate format migration. Unsupported prior binary scene formats are not
silently converted or reset.

Rollback uses the preceding image with unchanged data and keys. Preserve new
scene metadata and media even if the older UI cannot display them. No main merge,
production deployment or paid model call is included here. Browser/GPU bugs and
large-scene memory exhaustion remain possible; resource boundaries, actual host
isolation, backup recovery and cross-device/account checks remain release gates.

## Local acceptance record

The production Next build was exercised with a real Chromium/WebGL browser and
local API fixtures. Opening the Director node, creating a camera, capturing PNG,
sending it to a new canvas image node, reloading the canvas and reopening the saved
scene passed. No external requests or paid model calls occurred. This was a guest
fixture; authenticated cross-device synchronization remains unverified.
The first integration attempt exposed blocked `data:` image reads in the parent
CSP; the policy was corrected and the complete flow rerun successfully.
