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
const api=spawn(process.env.ICAN_TEST_API_BINARY,[],{cwd:repo,env:{...process.env,PUBLIC_ORIGIN:origin,PORT:'3418',ADMIN_USERNAME:'fixture-admin',ADMIN_PASSWORD:'fixture-admin-password-only',JWT_SECRET:crypto.randomBytes(48).toString('hex'),DATA_ENCRYPTION_KEY:crypto.randomBytes(32).toString('base64'),DATABASE_DSN:data+'/ican.db',MEDIA_DIR:data+'/media',AI_LOG_DIR:data+'/logs',AUTH_ATTEMPTS_PER_MINUTE:'1000',MEDIA_LINK_TTL_SECONDS:'60'},stdio:'ignore'});
const next=spawn(process.execPath,['server.js'],{cwd:repo+'/web/.next/standalone',env:{...process.env,PORT:'3417',HOSTNAME:'127.0.0.1',NODE_ENV:'production',NEXT_TELEMETRY_DISABLED:'1',API_BASE_URL:'http://127.0.0.1:3418'},stdio:'ignore'});
let browser;
async function call(path,body,token){const r=await fetch(origin+path,{method:body===undefined?'GET':'POST',headers:{'Content-Type':'application/json',...(token?{Authorization:'Bearer '+token}:{})},body:body===undefined?undefined:JSON.stringify(body)});const j=await r.json();assert.equal(j.code,0,path);return j.data;}
try {
for(let i=0;i<80;i++){try{if((await fetch(origin+'/api/health')).ok)break;}catch{}await new Promise(r=>setTimeout(r,250));}
const admin=await call('/api/auth/login',{username:'fixture-admin',password:'fixture-admin-password-only'});
const invite=async()=> (await call('/api/admin/invitations',{},admin.token)).code;
const alice=await call('/api/auth/register',{username:'alice',password:'alice-password',invitationCode:await invite()});
const bob=await call('/api/auth/register',{username:'bob',password:'bob-password',invitationCode:await invite()});
const project={id:'alice-project',title:'Alice private project',createdAt:new Date().toISOString(),updatedAt:new Date().toISOString(),nodes:[{id:'director',type:'director',title:'导演台',position:{x:100,y:100},width:480,height:350,metadata:{}}],connections:[],chatSessions:[],activeChatId:null,agentConfig:null,backgroundMode:'dots',viewport:{x:0,y:0,k:1},sidePanel:{open:false,width:280},agentPanel:{open:false,width:464}};
await call('/api/v1/canvas/projects',{data:project},alice.token);
browser=await chromium.launch({executablePath:process.env.ICAN_TEST_CHROMIUM,headless:true,args:['--no-sandbox','--disable-dev-shm-usage','--no-zygote','--use-gl=angle','--use-angle=swiftshader','--enable-unsafe-swiftshader'],env:process.env});
const ctx=await browser.newContext({viewport:{width:1500,height:950}});const page=await ctx.newPage();const errors=[];page.on('pageerror',e=>errors.push(e.message));
await page.goto(origin+'/canvas');assert.equal(new URL(page.url()).pathname,'/login');
await page.addScriptTag({content:fs.readFileSync(repo+'/web/node_modules/localforage/dist/localforage.min.js','utf8')});
await page.evaluate(async p=>{window.localforage.config({name:'infinite-canvas',storeName:'app_state'});await window.localforage.setItem('infinite-canvas:canvas_store',JSON.stringify({state:{projects:[{...p,id:'legacy',title:'UNOWNED LEGACY'}]},version:0}));},project);
for(const path of ['/canvas','/image','/video','/director/index.html']){await page.goto(origin+path);assert.equal(new URL(page.url()).pathname,'/login');assert.ok(!(await page.locator('body').innerText()).includes('UNOWNED LEGACY'));}
const form=new FormData();form.append('file',new Blob(['fixture'],{type:'image/png'}),'guest.png');assert.equal((await fetch(origin+'/api/v1/files',{method:'POST',body:form})).status,401);
async function uiLogin(name){await page.getByLabel('用户名',{exact:true}).fill(name);await page.getByLabel('密码',{exact:true}).fill(name+'-password');await page.locator('button[type=submit]').click();await page.waitForURL(u=>!u.pathname.startsWith('/login'));await page.goto(origin+'/canvas');}
await uiLogin('alice');await page.getByText('Alice private project',{exact:true}).waitFor();assert.ok(!(await page.locator('body').innerText()).includes('UNOWNED LEGACY'));
assert.equal((await call('/api/v1/canvas/projects',undefined,alice.token)).length,1,'legacy not auto-uploaded');
await page.goto(origin+'/canvas/alice-project');await page.getByRole('button',{name:'打开导演台',exact:true}).click();
await page.locator('iframe').waitFor(); let frame=page.frameLocator('iframe');await frame.getByRole('button',{name:'添加机位',exact:true}).click();await frame.getByRole('button',{name:'当前视角截图',exact:true}).click();await frame.locator('.camera-capture-thumb').first().hover();await frame.getByRole('button',{name:/发送到画布/}).first().click();
await page.waitForTimeout(1200);await frame.getByRole('button',{name:/关闭/}).first().click();
assert.ok((await page.locator('body').innerText()).includes('截图'));
// Revoke on the server, then exercise navigation and browser history with stale state.
await ctx.request.post(origin+'/api/auth/logout');await page.goto(origin+'/canvas');assert.equal(new URL(page.url()).pathname,'/login');
await uiLogin('bob');await page.waitForTimeout(700);let text=await page.locator('body').innerText();assert.ok(!text.includes('Alice private project'));assert.ok(!text.includes('UNOWNED LEGACY'));assert.equal((await call('/api/v1/canvas/projects',undefined,bob.token)).length,0);
await page.addScriptTag({content:fs.readFileSync(repo+'/web/node_modules/localforage/dist/localforage.min.js','utf8')});
assert.ok(await page.evaluate(async()=>{window.localforage.config({name:'infinite-canvas',storeName:'app_state'});return (await window.localforage.getItem('infinite-canvas:canvas_store')).includes('UNOWNED LEGACY');}),'legacy data preserved');
assert.deepEqual(errors,[]);
console.log(JSON.stringify({anonymousPages:'blocked',anonymousUpload:401,login:'passed',directorCapture:'passed',revokedSession:'blocked',accountIsolation:'passed',unownedLegacyCache:'preserved/not shown/not uploaded',pageErrors:errors}));
}finally{if(browser)await browser.close();next.kill();api.kill();fs.rmSync(data,{recursive:true,force:true});}
