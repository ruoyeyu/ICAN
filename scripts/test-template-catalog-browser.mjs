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
async function session(username,password){const ctx=await browser.newContext({viewport:{width:1500,height:1000}});const page=await ctx.newPage();page.on('pageerror',e=>errors.push(e.message));await page.goto(origin+'/login');await page.getByLabel('用户名',{exact:true}).fill(username);await page.getByLabel('密码',{exact:true}).fill(password);await page.locator('button[type=submit]').click();await page.waitForURL(u=>!u.pathname.startsWith('/login'));return {ctx,page};}
const {ctx:adminCtx,page:adminPage}=await session('fixture-admin','fixture-admin-password-only');await adminPage.goto(origin+'/admin/settings');
const card=adminPage.locator('.ant-card').filter({has:adminPage.getByText('供应商模板',{exact:true})});
await card.locator('input[type=file]').setInputFiles(repo+'/web/scripts/fixtures/channel-template-v1.json');
await adminPage.getByLabel('模板名称',{exact:true}).fill('Site fixture');await adminPage.getByLabel('模板使用说明',{exact:true}).fill('Select supplier. <b>Plain text</b>.');
const created=adminPage.waitForResponse(r=>r.url().endsWith('/api/admin/channel-templates')&&r.request().method()==='POST');await adminPage.getByRole('button',{name:'保存模板',exact:true}).click();assert.equal((await(await created).json()).code,0);
await card.getByRole('button',{name:/^编\s*辑$/}).click();await adminPage.getByLabel('创建请求路径（追加到渠道基础地址）',{exact:true}).fill('/catalog-jobs');const edited=adminPage.waitForResponse(r=>r.url().includes('/api/admin/channel-templates/')&&r.request().method()==='POST');await adminPage.getByRole('button',{name:'保存模板',exact:true}).click();assert.equal((await(await edited).json()).code,0);
const downloadEvent=adminPage.waitForEvent('download');await card.getByRole('button',{name:/^导\s*出$/}).click();const download=await downloadEvent;const stream=await download.createReadStream();const chunks=[];for await(const chunk of stream)chunks.push(chunk);const exported=JSON.parse(Buffer.concat(chunks).toString());assert.equal(exported.authoring.models[0].path,'/catalog-jobs');assert.equal(exported.instructions,'Select supplier. <b>Plain text</b>.');
const {ctx:aliceCtx,page}=await session('alice','alice-password');await page.goto(origin+'/canvas');await page.getByRole('button',{name:'配置',exact:true}).click();await page.getByRole('button',{name:'新增自定义 API',exact:true}).click();await page.getByLabel('初始化预设',{exact:true}).click();await page.locator('.ant-select-dropdown:visible .ant-select-item-option').filter({hasText:/^Site fixture$/}).click();await page.getByText('Select supplier. <b>Plain text</b>.',{exact:true}).waitFor();assert.equal(await page.getByLabel('个人 API Key',{exact:true}).inputValue(),'');await page.getByLabel('个人 API Key',{exact:true}).fill('alice-only-key');await page.getByRole('button',{name:'添加渠道',exact:true}).click();const saved=page.waitForResponse(r=>r.url().endsWith('/api/v1/user-config/model')&&r.request().method()==='POST');await page.getByRole('button',{name:/^完\s*成$/}).click();assert.equal((await(await saved).json()).code,0);
const config=await call('/api/v1/user-config',undefined,alice.token);const selectedChannel=config.modelConfig.localChannels.find(item=>item.name==='Site fixture');assert.ok(selectedChannel);assert.equal(selectedChannel.apiKey,'alice-only-key');assert.ok(config.modelConfig.channelTranslations.find(item=>item.channelId===selectedChannel.id).parameterTranslation.includes('/catalog-jobs'));
const manual={...exported,name:'Manual file fixture'};fs.writeFileSync(data+'/api-templates/manual.json.tmp',JSON.stringify(manual));fs.renameSync(data+'/api-templates/manual.json.tmp',data+'/api-templates/manual.json');
await page.getByRole('button',{name:'配置',exact:true}).click();await page.getByRole('button',{name:'新增自定义 API',exact:true}).click();await page.getByLabel('初始化预设',{exact:true}).click();await page.locator('.ant-select-dropdown:visible .ant-select-item-option').filter({hasText:/^Manual file fixture$/}).waitFor();await page.getByLabel('初始化预设',{exact:true}).press('Escape');
await card.getByRole('button',{name:/^删\s*除$/}).click();const removed=adminPage.waitForResponse(r=>r.url().includes('/api/admin/channel-templates/')&&r.request().method()==='DELETE');await adminPage.getByRole('button',{name:/^确\s*定$/}).click();assert.equal((await(await removed).json()).code,0);
await page.getByLabel('初始化预设',{exact:true}).click();await page.locator('.ant-select-dropdown:visible .ant-select-item-option').filter({hasText:/^Site fixture$/}).waitFor({state:'detached'});await page.locator('.ant-select-dropdown:visible .ant-select-item-option').filter({hasText:/^Manual file fixture$/}).waitFor();assert.deepEqual((await call('/api/v1/user-config',undefined,alice.token)).modelConfig,config.modelConfig);assert.deepEqual(fs.readdirSync(data+'/api-templates'),['manual.json']);
assert.deepEqual(errors,[]);console.log(JSON.stringify({catalog:'admin import/edit/export/delete; ordinary user selects without importing',discovery:'manual file found without restart; deleted file disappears',credentials:'user key not in catalog; existing account unchanged after deletion',instructions:'HTML shown as plain text',pageErrors:errors}));await aliceCtx.close();await adminCtx.close();
}finally{if(browser)await browser.close();next.kill();api.kill();fs.rmSync(data,{recursive:true,force:true});}
