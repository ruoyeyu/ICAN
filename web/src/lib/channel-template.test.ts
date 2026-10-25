import assert from "node:assert/strict";
import test from "node:test";
import { readFileSync } from "node:fs";
import { compileTemplate, exportChannelTemplate, importChannelTemplate, newTemplateField, newTemplateModel, readTemplate } from "./channel-template";
import { translationModelInputs, translationPrograms } from "./channel-parameter-translation";
const model = newTemplateModel("fixture-video", "video");
const authoring = { version: 1 as const, models: [model] };
const channel = { name: "Fixture", baseUrl: "https://api.example.invalid", apiKey: "private-fixture-key", models: [model.name], protocol: "custom" };
test("form roundtrip uses existing translator and excludes personal credentials", async () => {
 const source = compileTemplate(authoring);
 assert.deepEqual(readTemplate(source), authoring);
 assert.ok((await translationPrograms(source))[model.name]);
 assert.deepEqual(await translationModelInputs(source, model.name), {...model.inputs,authentication:"key"});
 assert.equal(readTemplate(source.replace('"/v1/videos"', '"/changed"')), null);
 const exported = exportChannelTemplate(channel, source);
 assert.ok(!exported.includes(channel.apiKey)); assert.ok(!exported.includes('"apiKey"'));
 assert.equal(compileTemplate(importChannelTemplate(exported).authoring), source);
 const openai = exportChannelTemplate({...channel,protocol:"openai",models:["chat-a","image-b"]}, "");
 assert.deepEqual(importChannelTemplate(openai).models,["chat-a","image-b"]);
});
test("reject credentials, unsupported versions, critical omissions and prototype mappings", () => {
 const valid = JSON.parse(exportChannelTemplate(channel, compileTemplate(authoring)));
 for (const mutate of [
  (v:any)=>v.apiKey="secret", (v:any)=>v.version=2, (v:any)=>delete v.baseUrl,
  (v:any)=>v.baseUrl="https://api.invalid/?key=secret", (v:any)=>delete v.authoring.models[0].poll.taskId,
  (v:any)=>v.authoring.models[0].poll.success=[],
  (v:any)=>v.authoring.models[0].inputs.parameters.seconds={options:["2"],allowCustom:false,default:"2"},
  (v:any)=>v.authoring.models[0].fields=[newTemplateField("__proto__.evil","model")],
  (v:any)=>v.authoring.models[0].fields=[{...newTemplateField("token"),value:'{"apiKey":"secret"}'}],
  (v:any)=>v.authoring.models[0].path="https://other.invalid/jobs",
 ]) { const next=structuredClone(valid); mutate(next); assert.throws(()=>importChannelTemplate(JSON.stringify(next))); }
 assert.throws(()=>exportChannelTemplate(channel,'{models:{x:{POST:{url:"/a"}}}}'),/高级代码/);
 assert.throws(()=>exportChannelTemplate({...channel,name:channel.apiKey},compileTemplate(authoring)),/API Key/);
});
test("shared fixture compiles identically for Go and preserves exact field values", async () => {
 const fixture=JSON.parse(readFileSync(new URL("../../scripts/fixtures/channel-template-v1.json",import.meta.url),"utf8"));
 const imported=importChannelTemplate(JSON.stringify(fixture));
 const source=compileTemplate(imported.authoring);
 assert.equal(source,readFileSync(new URL("../../../service/testdata/channel-template-v1.js",import.meta.url),"utf8"));
 const program=(await translationPrograms(source))[model.name];
 const names=["model","rawPrompt","rawSize","resolution","rawSeconds","images","firstFrame","lastFrame","videos","audios","mode"];
 const execute=new Function(...names,`return ${program};`);
 const input:any[]=[model.name,"  literal {channelKey}\n","9:16","custom-res","2",["https://media.invalid/a"],"","",[],[],"first_last_frame"];
 const body=execute(...input).rule.POST.body;
 assert.equal(body.prompt,input[1]);assert.equal(body.aspect_ratio,"9:16");assert.equal(body.resolution,"custom-res");assert.equal(body.seconds,"2");assert.equal(body.image_url,input[5][0]);
 input[5].push("https://media.invalid/b");assert.deepEqual(execute(...input).rule.POST.body.reference_image_urls,input[5]);
 input[6]="https://media.invalid/frame";assert.equal(execute(...input).rule.POST.body.reference_image_urls,undefined);assert.equal(execute(...input).rule.POST.body.video_config.reference_mode,"first_last_frame");
});

test("declared model behavior is scoped to channel/model and keyless auth is explicit", async () => {
 const {defaultConfig,usesCustomVideoInterface,useConfigStore}=await import("@/stores/use-config-store");
 const {useUserStore}=await import("@/stores/use-user-store");
 const {useChannelTranslationStore}=await import("@/stores/use-channel-translation-store");
 const oldUser=useUserStore.getState(),oldAccounts=useChannelTranslationStore.getState().accounts;
 try {
  useUserStore.setState({token:"fixture",user:{id:"fixture-user"} as never});
  const m=newTemplateModel("fixture-video","video");m.auth.type="none";
  useChannelTranslationStore.setState({accounts:{"fixture-user":{one:compileTemplate({version:1,models:[m]})}}});
  const config={...defaultConfig,activeChannelId:"one",videoChannelId:"one",localChannels:[{id:"one",name:"one",protocol:"openai" as const,baseUrl:"https://api.invalid",apiKey:"",models:["fixture-video","seedance-2"]}]};
  assert.equal(usesCustomVideoInterface(config,"fixture-video"),true);
  assert.equal(usesCustomVideoInterface(config,"seedance-2"),false);
  assert.equal(useConfigStore.getState().isAiConfigReady(config,"fixture-video"),true);
  assert.equal(useConfigStore.getState().isAiConfigReady(config,"seedance-2"),false);
  assert.equal(usesCustomVideoInterface({...config,activeChannelId:"missing",videoChannelId:"missing"},"fixture-video"),false);
 } finally {useUserStore.setState(oldUser);useChannelTranslationStore.setState({accounts:oldAccounts});}
});


test("supplier instructions remain data and personal presets remain credential free", () => {
 const template=importChannelTemplate(exportChannelTemplate(channel,compileTemplate(authoring)));
 template.instructions='<img src=x onerror=alert(1)> is displayed as plain text';
 assert.equal(importChannelTemplate(JSON.stringify(template)).instructions,template.instructions);
 assert.throws(()=>importChannelTemplate(JSON.stringify({...template,source:'https://remote.invalid/auto-update.js'})));
 const copy=structuredClone(template);copy.name="Other supplier";copy.baseUrl="https://other.example.invalid";
 assert.notEqual(copy.baseUrl,template.baseUrl);
 assert.equal(compileTemplate(copy.authoring),compileTemplate(template.authoring));
});
