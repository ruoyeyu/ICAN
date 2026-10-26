import assert from "node:assert/strict";
import test from "node:test";
import { readFileSync } from "node:fs";
import { compileTemplate, exportChannelTemplate, importChannelTemplate, newTemplateField, newTemplateModel, readTemplate } from "./channel-template";
import { translationModelInputs, translationPrograms, translationProgram } from "./channel-parameter-translation";
const model = newTemplateModel("fixture-video", "video");
const authoring = { version: 1 as const, models: [model] };
const channel = { name: "Fixture", baseUrl: "https://api.example.invalid", apiKey: "private-fixture-key", models: [model.name], protocol: "custom" };
test("field omission is opt-in, typed and roundtrips through the existing format", async () => {
 const fixture=importChannelTemplate(readFileSync(new URL('../../scripts/fixtures/channel-template-omission.json',import.meta.url),'utf8'));
 const source=compileTemplate(fixture.authoring);
 assert.equal(source,readFileSync(new URL('../../../service/testdata/channel-template-omission.js',import.meta.url),'utf8'));
 assert.deepEqual(readTemplate(source),fixture.authoring);
 assert.deepEqual(importChannelTemplate(exportChannelTemplate({...fixture,apiKey:'private-fixture-key'},source)).authoring,fixture.authoring);
 const program=await translationProgram(source,'new-video','video');assert.ok(program);
 const execute=new Function('resolution','count','generateAudio','quality','model','rawPrompt',`return ${program}`);
 for(const resolution of [undefined,null,'','auto','720p',' AUTO ',' auto ',0,false]) {
  const request=JSON.parse(JSON.stringify(execute(resolution,0,false,'auto','new-video',' exact {channelKey} ').rule.POST));
  const omitted=resolution===undefined||resolution===null||resolution===''||resolution==='auto';
  assert.equal(Object.hasOwn(request.body,'resolution'),!omitted);
  assert.equal(Object.hasOwn(request.params,'resolution'),!omitted);
  if(!omitted){assert.equal(request.body.resolution,resolution);assert.equal(request.params.resolution,resolution);}
  for(const key of ['fixed_auto','explicit_zero','explicit_false','empty_list','empty_string','empty_null'])assert.equal(Object.hasOwn(request.body,key),false,key);
  assert.equal(request.body.zero,0);assert.equal(request.body.disabled,false);assert.equal(request.body.fixed_zero,0);assert.equal(request.body.fixed_false,false);
  assert.equal(request.body.string_zero,'0');assert.equal(request.body.legacy,'auto');assert.equal(request.body.model,'new-video');assert.equal(request.body.prompt,' exact {channelKey} ');
 }
 for(const invalid of ['auto',null,[null],[{}],[[]]]) {
  const copy=structuredClone(fixture);(copy.authoring.defaults![0].fields[0] as any).omitValues=invalid;
  assert.throws(()=>importChannelTemplate(JSON.stringify(copy)));
 }
 const legacy=structuredClone(authoring);legacy.models[0].fields=[{...newTemplateField('resolution','resolution'),omitValues:[]}];
 const code=(await translationPrograms(compileTemplate(legacy)))[model.name];
 const run=new Function('resolution',`return ${code}`);
 assert.throws(()=>run(undefined),/缺少/);assert.equal(run('auto').rule.POST.body.resolution,'auto');
});
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

test("typed supplier defaults survive export and resolve new models without copying rules", async () => {
 const {translationProgram}=await import('./channel-parameter-translation');
 const fixture=importChannelTemplate(readFileSync(new URL('../../scripts/fixtures/channel-template-v2.json',import.meta.url),'utf8'));
 const source=compileTemplate(fixture.authoring);
 assert.equal(source,readFileSync(new URL('../../../service/testdata/channel-template-v2.js',import.meta.url),'utf8'));
 assert.deepEqual(readTemplate(source),fixture.authoring);
 assert.deepEqual(importChannelTemplate(exportChannelTemplate({...fixture,apiKey:'secret-fixture-key'},source)).authoring,fixture.authoring);
 for(const [kind,path] of [['text','/text'],['image','/pictures'],['video','/jobs']] as const){
  const code=await translationProgram(source,'brand-new-model',kind);assert.ok(code);
  const rule=new Function('model','rawPrompt','rawSize','rawSeconds','resolution','images','messages',`return ${code}`)('brand-new-model','raw prompt','9:16','3','custom-resolution',[],[{role:'user',content:'hello'}]).rule;
  assert.equal(rule.POST.url,path);assert.equal(rule.POST.body.model,'brand-new-model');
 }
 assert.equal(await translationProgram(source,'brand-new-model','audio'),undefined);
 assert.equal((await translationModelInputs(source,'fixture-special','video'))?.parameters.seconds?.allowCustom,false);
 assert.equal((await translationModelInputs(source,'brand-new-model','video'))?.parameters.seconds?.allowCustom,true);
 const own=await translationProgram(source,'fixture-special','video');assert.ok(own?.includes('/special-jobs'));
 const other=await translationProgram(source,'fixture-special','image');assert.ok(other?.includes('/pictures'));assert.ok(!other?.includes('/special-jobs'));
 const duplicate=structuredClone(fixture);duplicate.authoring.defaults!.push(duplicate.authoring.defaults![0]);assert.throws(()=>importChannelTemplate(JSON.stringify(duplicate)));
});

test("missing custom rules fail before requests while native and legacy channels retain behavior", async () => {
 const {findParameterTranslation}=await import('@/services/api/channel-parameter-translation');
 const {defaultConfig}=await import('@/stores/use-config-store');
 const {useUserStore}=await import('@/stores/use-user-store');
 const {useChannelTranslationStore}=await import('@/stores/use-channel-translation-store');
 const oldUser=useUserStore.getState(),oldTranslations=useChannelTranslationStore.getState();
 const fixture=importChannelTemplate(readFileSync(new URL('../../scripts/fixtures/channel-template-v2.json',import.meta.url),'utf8'));
 const source=compileTemplate(fixture.authoring);
 try{
  useUserStore.setState({token:'test-token',user:{id:'shared-fixture-user'} as never});
  useChannelTranslationStore.setState({load:async()=>({one:source})});
  const config={...defaultConfig,channelMode:'local' as const,activeChannelId:'one',model:'new-model',localChannels:[{id:'one',name:'Fixture',baseUrl:'https://api.example.invalid',apiKey:'fixture-key',protocol:'custom' as const,models:['new-model']}]};
  assert.ok(await findParameterTranslation(config,'video'));
  await assert.rejects(findParameterTranslation(config,'audio'),/调用规则/);
  useChannelTranslationStore.setState({load:async()=>({})});
  await assert.rejects(findParameterTranslation(config,'video'),/调用规则/);
  assert.equal(await findParameterTranslation({...config,localChannels:[{...config.localChannels[0],protocol:'openai'}]},'video'),null);
  const remote={...config,channelMode:'remote' as const,publicChannels:[{id:'one',protocol:'custom' as const,models:['new-model'],parameterTranslationRules:{video:{'new-model':{}}}}]};
  assert.ok(await findParameterTranslation(remote,'video'));
  await assert.rejects(findParameterTranslation(remote,'image'),/调用规则/);
 }finally{useUserStore.setState(oldUser);useChannelTranslationStore.setState(oldTranslations);}
});
