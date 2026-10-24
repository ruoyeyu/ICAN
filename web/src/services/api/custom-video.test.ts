import test from "node:test";
import assert from "node:assert/strict";
import { createVideoRequestBody } from "./video";
import { defaultConfig, localChannelForActiveModel, resolveModelForCapability } from "@/stores/use-config-store";

test("custom video preserves explicit model, prompt, aspect ratio, resolution and duration", async () => {
    for (const resolution of ["720p", "custom-resolution"]) {
        const config = { ...defaultConfig, model: "seedance-custom", videoModel: "seedance-custom", activeChannelId: "own", videoChannelId: "own", size: "9:16", vquality: resolution, videoSeconds: "27.5", localChannels: [{ id: "own", name: "own", protocol: "custom" as const, baseUrl: "https://provider.invalid", apiKey: "own-key", models: ["seedance-custom"] }] };
        const body = await createVideoRequestBody(config, config.model, "  exact prompt\n", { references: [], videoReferences: [], audioReferences: [], firstFrame: null, lastFrame: null });
        assert.ok(body instanceof FormData);
        assert.deepEqual(Object.fromEntries((body as FormData).entries()), { model: "seedance-custom", prompt: "  exact prompt\n", aspect_ratio: "9:16", resolution, seconds: "27.5" });
        assert.equal(localChannelForActiveModel({ ...config, activeChannelId: "missing", videoChannelId: "missing" }), undefined);
        assert.equal(resolveModelForCapability(config, "explicit-unavailable", "video"), "explicit-unavailable");
    }
});
