import assert from 'node:assert/strict';
import {spawn} from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import crypto from 'node:crypto';
const {chromium}=await import(process.env.ICAN_PLAYWRIGHT_MODULE || 'playwright-core');
const repo=process.cwd(), origin='http://127.0.0.1:3417';
assert.ok(process.env.ICAN_TEST_API_BINARY, 'Set ICAN_TEST_API_BINARY to the locally built API binary');
assert.ok(process.env.ICAN_TEST_CHROMIUM, 'Set ICAN_TEST_CHROMIUM to an installed test Chromium');
const data=fs.mkdtempSync(os.tmpdir()+'/ican-login-');
const api=spawn(process.env.ICAN_TEST_API_BINARY,[],{cwd:repo,env:{...process.env,PUBLIC_ORIGIN:origin,PORT:'3418',ADMIN_USERNAME:'fixture-admin',ADMIN_PASSWORD:'fixture-admin-password-only',JWT_SECRET:crypto.randomBytes(48).toString('hex'),DATA_ENCRYPTION_KEY:crypto.randomBytes(32).toString('base64'),API_TEMPLATE_DIR:data+'/api-templates',DATABASE_DSN:data+'/ican.db',MEDIA_DIR:data+'/media',AI_LOG_DIR:data+'/logs',AUTH_ATTEMPTS_PER_MINUTE:'1000',MEDIA_LINK_TTL_SECONDS:'60'},stdio:'ignore'});
const next=spawn(process.execPath,['server.js'],{cwd:repo+'/web/.next/standalone',env:{...process.env,PORT:'3417',HOSTNAME:'127.0.0.1',NODE_ENV:'production',NEXT_TELEMETRY_DISABLED:'1',API_BASE_URL:'http://127.0.0.1:3418'},stdio:'ignore'});
let browser;
async function call(path,body,token){const r=await fetch(origin+path,{method:body===undefined?'GET':'POST',headers:{'Content-Type':'application/json',...(token?{Authorization:'Bearer '+token}:{})},body:body===undefined?undefined:JSON.stringify(body)});const j=await r.json();assert.equal(j.code,0,path);return j.data;}
try {
for(let i=0;i<80;i++){try{if((await fetch(origin+'/api/health')).ok)break;}catch{}await new Promise(r=>setTimeout(r,250));}
const admin=await call('/api/auth/login',{username:'fixture-admin',password:'fixture-admin-password-only'});
const invite=async()=> (await call('/api/admin/invitations',{},admin.token)).code;
const alice=await call('/api/auth/register',{username:'alice',password:'alice-password',invitationCode:await invite()});
const bob=await call('/api/auth/register',{username:'bob',password:'bob-password',invitationCode:await invite()});

const settings=await call('/api/admin/settings',undefined,admin.token);
settings.public.modelChannel.allowCustomChannel=true;
settings.public.modelChannel.allowUserRemoteChannel=true;
await call('/api/admin/settings',settings,admin.token);
browser=await chromium.launch({executablePath:process.env.ICAN_TEST_CHROMIUM,headless:true,args:['--no-sandbox','--disable-dev-shm-usage','--no-zygote'],env:process.env});
const errors=[];
async function settleModals(page){await page.locator('.ant-modal:visible').evaluateAll(elements=>Promise.all(elements.flatMap(element=>element.getAnimations()).map(animation=>animation.finished.catch(()=>{}))));}
async function session(username,password){const ctx=await browser.newContext({viewport:{width:1500,height:1000}});const page=await ctx.newPage();page.on('pageerror',e=>errors.push(e.message));await page.goto(origin+'/login');await page.getByLabel('用户名',{exact:true}).fill(username);await page.getByLabel('密码',{exact:true}).fill(password);await page.locator('button[type=submit]').click();await page.waitForURL(u=>!u.pathname.startsWith('/login'));return {ctx,page};}
const {ctx:adminCtx,page:adminPage}=await session('fixture-admin','fixture-admin-password-only');await adminPage.goto(origin+'/admin/settings');
// Local storage has no mounted type Form.Item; watch the preserved form value.
const savedSettings=await call('/api/admin/settings',undefined,admin.token);
assert.ok(savedSettings.private.storage.providers.some(provider=>provider.type==='local'));
for(let i=0;i<3;i++){
    await adminPage.getByRole('tab',{name:'私有配置（不会对外暴露）',exact:true}).click();
    await adminPage.getByText('服务器本地媒体存储',{exact:true}).waitFor();
    await adminPage.evaluate(async()=>{
        let changed=false;
        const observer=new MutationObserver(()=>{if(!document.body.textContent.includes('服务器本地媒体存储')||document.querySelector('input[placeholder="https://<account>.r2.cloudflarestorage.com"]'))changed=true;});
        observer.observe(document.body,{childList:true,subtree:true});
        for(let frame=0;frame<20;frame++)await new Promise(requestAnimationFrame);
        observer.disconnect();
        if(changed)throw new Error('Local storage form switched to S3');
    });
    await adminPage.getByRole('tab',{name:'公开配置（对外暴露）',exact:true}).click();
}
await adminPage.getByRole('tab',{name:'私有配置（不会对外暴露）',exact:true}).click();
const settingsSaved=adminPage.waitForResponse(r=>r.url().endsWith('/api/admin/settings')&&r.request().method()==='POST');
await adminPage.getByRole('button',{name:/保存设置/}).click();assert.equal((await(await settingsSaved).json()).code,0);
assert.deepEqual((await call('/api/admin/settings',undefined,admin.token)).private.storage.providers,savedSettings.private.storage.providers);
const temporaryKey='temporary-catalog-fixture-key';
let discoveryMode='ok', discoveryCalls=0, unexpectedKeyWrites=0, generationCalls=0, finishDelayed;
adminPage.on('request',request=>{
    if(request.postData()?.includes(temporaryKey)&&!request.url().endsWith('/api/admin/settings/channel-models'))unexpectedKeyWrites++;
    if(/channel-test|images\/generations|videos|chat\/completions/.test(new URL(request.url()).pathname))generationCalls++;
});
await adminPage.route('**/api/admin/settings/channel-models',async route=>{
    discoveryCalls++;
    const payload=route.request().postDataJSON();
    assert.equal(payload.channel.apiKey,temporaryKey);assert.equal(payload.channel.baseUrl,'https://api.example.invalid');
    assert.equal(payload.index,undefined);assert.ok(!payload.channel.parameterTranslation);
    if(discoveryMode==='delayed')await new Promise(resolve=>{finishDelayed=resolve;});
    await route.fulfill({contentType:'application/json',body:JSON.stringify(discoveryMode==='fail'?{code:1,msg:'Upstream echoed '+temporaryKey}:{code:0,data:['fixture-video','candidate-alpha','candidate-beta','candidate-alpha']})});
});
const card=adminPage.locator('.ant-card').filter({has:adminPage.getByText('供应商模板',{exact:true})});
await card.locator('input[type=file]').setInputFiles(repo+'/web/scripts/fixtures/channel-template-v1.json');
await adminPage.getByLabel('模板名称',{exact:true}).fill('Site fixture');await adminPage.getByLabel('模板使用说明',{exact:true}).fill('Select supplier. <b>Plain text</b>.');
const editor=adminPage.getByRole('dialog',{name:'新增供应商模板',exact:true});
const selector=adminPage.getByRole('dialog',{name:/选择渠道模型/});
await editor.getByRole('button',{name:/拉取模型$/}).click();assert.equal(discoveryCalls,0);
async function fetchCandidates(dialog){await dialog.getByLabel('模板临时 API Key',{exact:true}).fill(temporaryKey);await dialog.getByRole('button',{name:/拉取模型$/}).click();await selector.waitFor();await settleModals(adminPage);}
await fetchCandidates(editor);
assert.equal(await selector.getByRole('checkbox',{name:'candidate-alpha',exact:true}).isChecked(),false);
assert.equal(await selector.getByRole('checkbox',{name:'candidate-beta',exact:true}).isChecked(),false);
await selector.getByRole('tab',{name:/已有的模型/}).click();assert.equal(await selector.getByRole('checkbox',{name:'fixture-video',exact:true}).isChecked(),true);
await selector.getByRole('tab',{name:/新获取的模型/}).click();
await selector.getByPlaceholder('搜索模型',{exact:true}).fill('alpha');
await selector.getByRole('button',{name:'全选当前列表',exact:true}).click();assert.equal(await selector.getByRole('checkbox',{name:'candidate-alpha',exact:true}).isChecked(),true);
await selector.getByRole('button',{name:'取消当前列表',exact:true}).click();assert.equal(await selector.getByRole('checkbox',{name:'candidate-alpha',exact:true}).isChecked(),false);
await selector.getByRole('checkbox',{name:'candidate-alpha',exact:true}).check();
await selector.getByRole('button',{name:/^取\s*消$/}).click();
assert.equal(await editor.getByLabel('模板临时 API Key',{exact:true}).inputValue(),'');
assert.equal(await editor.locator('.ant-select-selection-item').filter({hasText:/^candidate-alpha$/}).count(),0);
await fetchCandidates(editor);
assert.equal(await selector.getByRole('checkbox',{name:'candidate-alpha',exact:true}).isChecked(),false);
await selector.getByRole('checkbox',{name:'candidate-alpha',exact:true}).check();
discoveryMode='fail';await selector.getByRole('button',{name:/拉取模型列表/}).click();
await adminPage.getByText('拉取模型失败，请检查基础地址、临时 Key 和模型列表接口；原配置未修改',{exact:true}).last().waitFor();
assert.equal(await selector.getByRole('checkbox',{name:'candidate-alpha',exact:true}).isChecked(),true);
assert.equal(await adminPage.getByText('Upstream echoed '+temporaryKey,{exact:true}).count(),0);
discoveryMode='ok';await selector.getByRole('button',{name:/拉取模型列表/}).click();
await selector.getByRole('tab',{name:/已有的模型/}).click();await selector.getByRole('checkbox',{name:'candidate-alpha',exact:true}).waitFor();
assert.equal(await selector.getByRole('checkbox',{name:'candidate-alpha',exact:true}).isChecked(),true);
await selector.getByRole('tab',{name:/新获取的模型/}).click();assert.equal(await selector.getByRole('checkbox',{name:'candidate-beta',exact:true}).isChecked(),false);
await selector.getByRole('button',{name:/^确\s*定$/}).click();assert.equal(await editor.getByLabel('模板临时 API Key',{exact:true}).inputValue(),'');
await editor.getByLabel('模板临时 API Key',{exact:true}).fill(temporaryKey);
const omissionInput=editor.getByLabel('指定值时省略 4',{exact:true});
for(const value of ['auto','0','false']){await omissionInput.fill(value);await omissionInput.press('Enter');}
await omissionInput.press('Escape');
await editor.getByRole('checkbox',{name:'为空时省略',exact:true}).nth(3).check();
await editor.getByRole('button',{name:'复制为公共规则',exact:true}).click();
await editor.getByLabel('创建请求路径（追加到渠道基础地址）',{exact:true}).fill('/shared-jobs');
const created=adminPage.waitForResponse(r=>r.url().endsWith('/api/admin/channel-templates')&&r.request().method()==='POST');await adminPage.getByRole('button',{name:'保存模板',exact:true}).click();assert.equal((await(await created).json()).code,0);
await card.getByRole('button',{name:/^编\s*辑$/}).click();await adminPage.getByLabel('创建请求路径（追加到渠道基础地址）',{exact:true}).fill('/catalog-jobs');const edited=adminPage.waitForResponse(r=>r.url().includes('/api/admin/channel-templates/')&&r.request().method()==='POST');await adminPage.getByRole('button',{name:'保存模板',exact:true}).click();assert.equal((await(await edited).json()).code,0);
const downloadEvent=adminPage.waitForEvent('download');await card.getByRole('button',{name:/^导\s*出$/}).click();const download=await downloadEvent;const stream=await download.createReadStream();const chunks=[];for await(const chunk of stream)chunks.push(chunk);const exported=JSON.parse(Buffer.concat(chunks).toString());assert.equal(exported.authoring.models[0].path,'/catalog-jobs');assert.equal(exported.instructions,'Select supplier. <b>Plain text</b>.');
assert.deepEqual(exported.models,['fixture-video','candidate-alpha']);assert.equal(exported.authoring.version,2);assert.equal(exported.authoring.defaults[0].path,'/shared-jobs');assert.equal(exported.authoring.models.length,1);assert.equal(JSON.stringify(exported).includes(temporaryKey),false);
for(const rule of [exported.authoring.models[0],exported.authoring.defaults[0]]){assert.deepEqual(rule.fields[3].omitValues,['auto',0,false]);assert.equal(rule.fields[3].optional,true);}
// Failed discovery and a response arriving after cancel must not overwrite the template.
const beforeCancel=fs.readFileSync(data+'/api-templates/'+fs.readdirSync(data+'/api-templates')[0],'utf8');
await card.getByRole('button',{name:/^编\s*辑$/}).click();
const editDialog=adminPage.getByRole('dialog',{name:'编辑供应商模板',exact:true});
assert.equal(await editDialog.getByLabel('模板临时 API Key',{exact:true}).inputValue(),'');
discoveryMode='fail';await editDialog.getByLabel('模板临时 API Key',{exact:true}).fill(temporaryKey);await editDialog.getByRole('button',{name:/拉取模型$/}).click();
await adminPage.getByText('拉取模型失败，请检查基础地址、临时 Key 和模型列表接口；原配置未修改',{exact:true}).last().waitFor();assert.equal(await selector.count(),0);
discoveryMode='delayed';await editDialog.getByRole('button',{name:/拉取模型$/}).click();
for(let i=0;i<100&&!finishDelayed;i++)await new Promise(resolve=>setTimeout(resolve,10));assert.ok(finishDelayed);
await editDialog.getByRole('button',{name:/^取\s*消$/}).click();
const delayed=adminPage.waitForResponse(r=>r.url().endsWith('/api/admin/settings/channel-models'));finishDelayed();await delayed;
await card.getByRole('button',{name:/^编\s*辑$/}).click();assert.equal(await selector.count(),0);assert.equal(await editDialog.getByLabel('模板临时 API Key',{exact:true}).inputValue(),'');
await editDialog.getByRole('button',{name:/^取\s*消$/}).click();
assert.equal(fs.readFileSync(data+'/api-templates/'+fs.readdirSync(data+'/api-templates')[0],'utf8'),beforeCancel);
assert.equal(unexpectedKeyWrites,0);assert.equal(generationCalls,0);
assert.equal(JSON.stringify(await call('/api/admin/settings',undefined,admin.token)).includes(temporaryKey),false);
assert.equal(JSON.stringify(await call('/api/v1/user-config',undefined,admin.token)).includes(temporaryKey),false);
assert.equal(await adminPage.evaluate(async key=>{
    if(JSON.stringify(localStorage).includes(key)||JSON.stringify(sessionStorage).includes(key))return true;
    for(const info of await indexedDB.databases()){
        const db=await new Promise((resolve,reject)=>{const request=indexedDB.open(info.name);request.onsuccess=()=>resolve(request.result);request.onerror=()=>reject(request.error);});
        for(const name of db.objectStoreNames){
            const values=await new Promise((resolve,reject)=>{const request=db.transaction(name).objectStore(name).getAll();request.onsuccess=()=>resolve(request.result);request.onerror=()=>reject(request.error);});
            if(JSON.stringify(values).includes(key)){db.close();return true;}
        }
        db.close();
    }
    return false;
},temporaryKey),false);

const {ctx:aliceCtx,page}=await session('alice','alice-password');await page.goto(origin+'/canvas');await page.getByRole('button',{name:'配置',exact:true}).click();await page.getByRole('button',{name:'新增自定义 API',exact:true}).click();await settleModals(page);await page.getByLabel('初始化预设',{exact:true}).click();await page.locator('.ant-select-dropdown:visible .ant-select-item-option').filter({hasText:/^Site fixture$/}).click();await page.getByText('Select supplier. <b>Plain text</b>.',{exact:true}).waitFor();assert.equal(await page.getByLabel('个人 API Key',{exact:true}).inputValue(),'');await page.getByLabel('个人 API Key',{exact:true}).fill('alice-only-key');await page.getByRole('button',{name:'添加渠道',exact:true}).click();const saved=page.waitForResponse(r=>r.url().endsWith('/api/v1/user-config/model')&&r.request().method()==='POST');await page.getByRole('button',{name:/^完\s*成$/}).click();assert.equal((await(await saved).json()).code,0);
const config=await call('/api/v1/user-config',undefined,alice.token);const selectedChannel=config.modelConfig.localChannels.find(item=>item.name==='Site fixture');assert.ok(selectedChannel);assert.equal(selectedChannel.apiKey,'alice-only-key');assert.ok(config.modelConfig.channelTranslations.find(item=>item.channelId===selectedChannel.id).parameterTranslation.includes('/catalog-jobs'));
assert.ok(config.modelConfig.channelTranslations.find(item=>item.channelId===selectedChannel.id).parameterTranslation.includes('defaults:'));
const channelSource=config.modelConfig.channelTranslations.find(item=>item.channelId===selectedChannel.id).parameterTranslation;
const channelAuthoring=JSON.parse(channelSource.split('\n')[0].replace('// ICAN_FORM_V2 ',''));
for(const rule of [channelAuthoring.models[0],channelAuthoring.defaults[0]])assert.deepEqual(rule.fields[3].omitValues,['auto',0,false]);
const manual={...exported,name:'Manual file fixture'};fs.writeFileSync(data+'/api-templates/manual.json.tmp',JSON.stringify(manual));fs.renameSync(data+'/api-templates/manual.json.tmp',data+'/api-templates/manual.json');
await page.getByRole('button',{name:'配置',exact:true}).click();await page.getByRole('button',{name:'新增自定义 API',exact:true}).click();await settleModals(page);await page.getByLabel('初始化预设',{exact:true}).click();await page.locator('.ant-select-dropdown:visible .ant-select-item-option').filter({hasText:/^Manual file fixture$/}).waitFor();await page.getByLabel('初始化预设',{exact:true}).press('Escape');
await card.getByRole('button',{name:/^删\s*除$/}).click();const removed=adminPage.waitForResponse(r=>r.url().includes('/api/admin/channel-templates/')&&r.request().method()==='DELETE');await adminPage.getByRole('button',{name:/^确\s*定$/}).click();assert.equal((await(await removed).json()).code,0);
await page.getByLabel('初始化预设',{exact:true}).click();await page.locator('.ant-select-dropdown:visible .ant-select-item-option').filter({hasText:/^Site fixture$/}).waitFor({state:'detached'});await page.locator('.ant-select-dropdown:visible .ant-select-item-option').filter({hasText:/^Manual file fixture$/}).waitFor();assert.deepEqual((await call('/api/v1/user-config',undefined,alice.token)).modelConfig,config.modelConfig);assert.deepEqual(fs.readdirSync(data+'/api-templates'),['manual.json']);
// Other consumers keep the selector's existing fetch-and-select behavior.
discoveryMode='ok';
await adminPage.getByRole('button',{name:/新增渠道/}).click();
const channelDrawer=adminPage.getByRole('dialog',{name:'新增渠道',exact:true});
await channelDrawer.getByLabel('接口地址',{exact:true}).fill('https://api.example.invalid');
await channelDrawer.getByLabel('API Key',{exact:true}).fill(temporaryKey);
await channelDrawer.getByRole('button',{name:'选择模型',exact:true}).click();
await selector.getByRole('button',{name:/拉取模型列表/}).click();
await selector.getByRole('checkbox',{name:'candidate-beta',exact:true}).waitFor();
assert.equal(await selector.getByRole('checkbox',{name:'candidate-beta',exact:true}).isChecked(),true);
await selector.getByRole('button',{name:/^取\s*消$/}).click();
assert.deepEqual(errors,[]);console.log(JSON.stringify({catalog:'admin import/edit/export/delete; ordinary user selects without importing',discovery:'manual file found without restart; deleted file disappears',credentials:'user key not in catalog; existing account unchanged after deletion',instructions:'HTML shown as plain text',modelDiscovery:'existing selector; search/multiple/all/clear; candidates opt in; cancel/failure/late response preserve configuration; temporary key absent from writes and export; no generation',privateStorage:'local type remains stable across tabs and save',fieldOmission:'typed values edited, copied to shared rules, saved, exported and synced to account channel',discoveryCalls,pageErrors:errors}));await aliceCtx.close();await adminCtx.close();
}catch(error){if(browser){for(const context of browser.contexts())for(const page of context.pages())console.error('Dialog buttons on failure:',await page.locator('[role=dialog] button').allTextContents());}throw error;}finally{if(browser)await browser.close();next.kill();api.kill();fs.rmSync(data,{recursive:true,force:true});}
