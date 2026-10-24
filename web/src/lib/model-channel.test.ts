import assert from "node:assert/strict";
import test from "node:test";
import { modelChannelApiKeyUrls, modelChannelDefaultBaseUrls, modelChannelProtocolOptions } from "./model-channel";

test("configuration starts without vendor destinations or purchase links", () => {
    assert.ok(Object.values(modelChannelDefaultBaseUrls).every(url => url === ""));
    assert.deepEqual(modelChannelApiKeyUrls, {});
    assert.deepEqual(modelChannelProtocolOptions.map(item => item.value), ["custom", "openai", "gemini"]);
});
