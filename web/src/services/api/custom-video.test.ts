import test from "node:test";
import assert from "node:assert/strict";
import { createVideoRequestBody, readVideoContentResponse, VideoRequestError } from "./video";
import { defaultConfig, localChannelForActiveModel, resolveModelForCapability } from "@/stores/use-config-store";
import { geminiActionUrl } from "@/lib/gemini";

test("Gemini protocol never supplies a default vendor address", () => {
    assert.throws(() => geminiActionUrl(" ", "own-model", "generateContent"), /API 地址/);
    assert.equal(geminiActionUrl("https://provider.invalid/v1beta", "own-model", "generateContent"), "https://provider.invalid/v1beta/models/own-model:generateContent");
});

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

test("video download keeps sanitized server diagnostics including HTTP 200 error envelopes", async () => {
    for (const status of [400, 401, 403, 429]) {
        const upstream = { status, stage: "下载", requestId: "fixture-request", detail: "permission denied", body: JSON.stringify({message:"permission denied", detail:"x".repeat(6000), key:"[已脱敏]"}), truncated:false };
        const payload = {code:1, msg:`上游下载失败（HTTP ${status}；request ID: fixture-request）：permission denied`, data:{upstream}};
        for (const httpStatus of [200, status]) {
            await assert.rejects(() => readVideoContentResponse(new Response(JSON.stringify(payload), {status:httpStatus,headers:{"Content-Type":"application/json"}})), (error: unknown) => {
                assert.ok(error instanceof VideoRequestError);
                assert.equal(error.message,payload.msg);
                assert.deepEqual(JSON.parse(error.detail || ""),upstream);
                return true;
            });
        }
    }
    await assert.rejects(() => readVideoContentResponse(new Response("upstream unavailable", {status:502,headers:{"Content-Type":"text/plain"}})), /upstream unavailable/);
    const media = await readVideoContentResponse(new Response(new Uint8Array([0,1,2,3]),{headers:{"Content-Type":"video/mp4"}}));
    assert.equal(media.type,"video/mp4");
    assert.deepEqual([...new Uint8Array(await media.arrayBuffer())],[0,1,2,3]);
});
