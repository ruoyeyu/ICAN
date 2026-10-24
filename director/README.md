# ICAN Director

A browser-side 3D composition editor embedded in the ICAN canvas. Characters,
poses, primitives, cameras, local model imports and screenshots run on the user's
GPU. It does not need ComfyUI, RunningHub, a model server or a provider API key.

Source: [storyai-3d-director-desk](https://github.com/jiguang132/storyai-3d-director-desk),
commit `8c8bd361790be4d37158a7430365e65546e358fe`, MIT; see [LICENSE](LICENSE).
The adopted archive's 104 files were checked against the pinned Git blob hashes.
This is not the unreproducible precompiled bundle previously included by tigerowo.
Timeline and video export from that bundle are not restored by this implementation.

From the repository root, install `web` dependencies first, then:

```sh
cd director
bun install --frozen-lockfile --ignore-scripts
bun audit
bun run test
bun run build
```

The Docker build compiles these sources and copies `dist` into `/director/`.
Generated files and node_modules are not committed. A standalone preview does
not have the production security headers; use the embedded application for tests.

The iframe has an opaque origin, restrictive CSP and no network access. Parent
messages must match the parent window and origin. The parent accepts messages
only from its iframe. Scenes pass a shared Zod schema; only embedded raster pixels
and local model data can cross the boundary. Account credentials are never sent.
Scenes are stored in the existing canvas metadata; embedded localStorage is disabled.

The UE mannequin binary is omitted because its redistribution rights were not
established. Characters use the upstream procedural implementation. The external
model library is empty; users can import their own FBX/OBJ files. No runtime CDN,
remote fonts, automatic plugin updates or external model downloads are added.

See [integration review](../docs/security/director-integration.md) for evidence,
limitations and test adjustments.
