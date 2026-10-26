import assert from 'node:assert/strict';
import {spawn} from 'node:child_process';
import {once} from 'node:events';
import fs from 'node:fs';
import os from 'node:os';
import crypto from 'node:crypto';
const {chromium}=await import(process.env.ICAN_PLAYWRIGHT_MODULE || 'playwright-core');
const repo=process.cwd(), origin='http://127.0.0.1:3417';
assert.ok(process.env.ICAN_TEST_API_BINARY, 'Set ICAN_TEST_API_BINARY to the locally built API binary');
assert.ok(process.env.ICAN_TEST_CHROMIUM, 'Set ICAN_TEST_CHROMIUM to an installed test Chromium');
const data=fs.mkdtempSync(os.tmpdir()+'/ican-invitations-');
const env={...process.env,PUBLIC_ORIGIN:origin,PORT:'3418',ADMIN_USERNAME:'fixture-admin',ADMIN_PASSWORD:'fixture-admin-password-only',JWT_SECRET:crypto.randomBytes(48).toString('hex'),DATA_ENCRYPTION_KEY:crypto.randomBytes(32).toString('base64'),API_TEMPLATE_DIR:data+'/api-templates',DATABASE_DSN:data+'/ican.db',MEDIA_DIR:data+'/media',AI_LOG_DIR:data+'/logs',AUTH_ATTEMPTS_PER_MINUTE:'1000',MEDIA_LINK_TTL_SECONDS:'60'};
const startAPI=()=>spawn(process.env.ICAN_TEST_API_BINARY,[],{cwd:repo,env,stdio:'ignore'});
let api=startAPI();
const next=spawn(process.execPath,['server.js'],{cwd:repo+'/web/.next/standalone',env:{...process.env,PORT:'3417',HOSTNAME:'127.0.0.1',NODE_ENV:'production',NEXT_TELEMETRY_DISABLED:'1',API_BASE_URL:'http://127.0.0.1:3418'},stdio:'ignore'});
let browser;
async function ready(){for(let i=0;i<80;i++){try{if((await fetch(origin+'/api/health')).ok)return;}catch{}await new Promise(r=>setTimeout(r,250));}throw Error('Fixture app did not start');}
async function call(path,method='GET',body,token){const r=await fetch(origin+path,{method,headers:{'Content-Type':'application/json',...(token?{Authorization:'Bearer '+token}:{})},body:body===undefined?undefined:JSON.stringify(body)});return {response:r,value:await r.json()};}
try {
await ready();
const login=await call('/api/auth/login','POST',{username:'fixture-admin',password:'fixture-admin-password-only'});assert.equal(login.value.code,0);const token=login.value.data.token;
browser=await chromium.launch({executablePath:process.env.ICAN_TEST_CHROMIUM,headless:true,args:['--no-sandbox','--disable-dev-shm-usage','--no-zygote'],env:process.env});
const context=await browser.newContext({viewport:{width:1500,height:1000},permissions:['clipboard-read','clipboard-write']}),page=await context.newPage();
const errors=[];page.on('pageerror',e=>errors.push(e.message));
await page.goto(origin+'/login');await page.getByLabel('用户名',{exact:true}).fill('fixture-admin');await page.getByLabel('密码',{exact:true}).fill('fixture-admin-password-only');await page.locator('button[type=submit]').click();await page.waitForURL(u=>!u.pathname.startsWith('/login'));
await page.goto(origin+'/admin/invitations');
const create=async()=>{const response=page.waitForResponse(r=>r.url().endsWith('/api/admin/invitations')&&r.request().method()==='POST');await page.getByRole('button',{name:'创建一次性邀请码',exact:true}).click();const {data,code}=await (await response).json();assert.equal(code,0);await page.getByText(data.code,{exact:true}).waitFor();return data;};
const first=await create();
assert.equal(await page.getByRole('dialog').count(),0);
let row=page.getByRole('row').filter({hasText:first.code});
await row.locator('.ant-typography-copy').click();assert.equal(await page.evaluate(()=>navigator.clipboard.readText()),first.code);
await page.reload();await page.getByText(first.code,{exact:true}).waitFor();
// Restart the API with the same database and original encryption key.
const exited=once(api,'exit');api.kill();await exited;api=startAPI();await ready();
await page.reload();await page.getByText(first.code,{exact:true}).waitFor();
row=page.getByRole('row').filter({hasText:first.code});
await row.locator('.ant-typography-copy').click();assert.equal(await page.evaluate(()=>navigator.clipboard.readText()),first.code);
const registered=await call('/api/auth/register','POST',{username:'fixture-invited',password:'fixture-password-only',invitationCode:first.code});assert.equal(registered.value.code,0);
await page.reload();await page.getByText('已使用',{exact:true}).waitFor();
assert.equal(await page.getByRole('row').filter({hasText:first.code}).getByRole('button',{name:/^撤\s*销$/}).count(),0);
assert.notEqual((await call('/api/admin/invitations/'+first.invitation.id,'DELETE',undefined,token)).value.code,0);
const second=await create();
await page.getByRole('row').filter({hasText:second.code}).getByRole('button',{name:/^撤\s*销$/}).click();
await page.getByText(second.code,{exact:true}).waitFor({state:'detached'});
await page.reload();await page.getByText(first.code,{exact:true}).waitFor();assert.equal(await page.getByText(second.code,{exact:true}).count(),0);
const listing=await call('/api/admin/invitations','GET',undefined,token);assert.equal(listing.response.headers.get('cache-control'),'no-store');assert.equal(listing.value.data.length,1);
const reused=await call('/api/auth/register','POST',{username:'fixture-reused',password:'fixture-password-only',invitationCode:second.code});assert.notEqual(reused.value.code,0);
for(const auth of [undefined,registered.value.data.token]){const denied=await call('/api/admin/invitations','GET',undefined,auth);assert.notEqual(denied.value.code,0);assert.ok(!JSON.stringify(denied.value).includes(first.code));}
assert.deepEqual(errors,[]);
console.log(JSON.stringify({create:'code in existing table, no one-time modal',copy:'clipboard matches before and after refresh/restart',persistence:'original key and database restore displayed code',revoke:'unused row disappears and code cannot register',used:'record retained, no revoke action, backend deletion rejected',permissions:'admin only; list is no-store',paidCalls:0,pageErrors:errors}));
await context.close();
}finally{if(browser)await browser.close();next.kill();api.kill();fs.rmSync(data,{recursive:true,force:true});}
