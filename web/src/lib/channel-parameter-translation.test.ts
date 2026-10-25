import assert from "node:assert/strict";
import test from "node:test";
import { spawnSync } from "node:child_process";
import { translationPrograms, translationReferenceInput } from "./channel-parameter-translation";

test("reference metadata is static and legacy templates retain their defaults", async () => {
 const old='{models:{sample:{POST:{url:"/generate",body:{model,prompt}}}}}';
 assert.ok((await translationPrograms(old)).sample);
 assert.equal(await (await import("./channel-parameter-translation")).translationProgram(old,"sample","text"),undefined);
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

test("shared and model rules require explicit types and never evaluate unrelated requests", async()=>{
 const {translationProgram}=await import('./channel-parameter-translation');
 const source='{models:{special:{kind:"video",POST:{url:"/special"}}},defaults:{video:{POST:{url:"/video",body:{model}}},image:{POST:{url:(()=>{throw new Error("unused")})()}}}}';
 const program=await translationProgram(source,'new','video');assert.ok(program);
 assert.equal(new Function('model',`return ${program}`)('new').rule.POST.body.model,'new');
 assert.ok((await translationProgram(source,'special','video'))?.includes('/special'));
 assert.equal(await translationProgram(source,'new','text'),undefined);
 for(const invalid of ['{models:{},defaults:{video:{}}}','{models:{},defaults:{video:{kind:"image",POST:{url:"/bad"}}}}','{models:{a:{POST:{url:"/a"}}},defaults:{video:{POST:{url:"/video"}}}}','{models:{},defaults:{other:{POST:{url:"/a"}}}}'])await assert.rejects(translationPrograms(invalid));
});

test("typed advanced rules accept static JavaScript string literals", async()=>{
 const {translationProgram}=await import("./channel-parameter-translation");
 assert.ok(await translationProgram("{models:{sample:{kind:'video',POST:{url:'/jobs'}}}}","sample","video"));
});
