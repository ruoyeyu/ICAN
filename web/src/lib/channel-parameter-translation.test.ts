import assert from "node:assert/strict";
import test from "node:test";
import { spawnSync } from "node:child_process";
import { translationPrograms, translationReferenceInput } from "./channel-parameter-translation";

test("reference metadata is static and legacy templates retain their defaults", async () => {
 const old='{models:{sample:{POST:{url:"/generate",body:{model,prompt}}}}}';
 assert.ok((await translationPrograms(old)).sample);
 assert.equal(await translationReferenceInput(old,"sample"),undefined);
 const source='{models:{sample:{references:{format:"url",max:6},POST:{url:"/generate"}}}}';
 assert.deepEqual(await translationReferenceInput(source,"sample"),{format:"url",max:6});
 for(const refs of ['{format:(()=>"url")()}','{format:"url",max:-1}','{format:"invalid"}','{format:"url",unknown:1}']) {
  await assert.rejects(translationPrograms(`{models:{sample:{references:${refs},POST:{url:"/generate"}}}}`));
 }
});

test("reference preparation uses owned share links and genuine local image bytes", () => {
 const result=spawnSync(process.execPath,["run","scripts/verify-translation-images.ts"],{cwd:process.cwd(),encoding:"utf8"});
 assert.equal(result.status,0,result.stdout+result.stderr);
});
