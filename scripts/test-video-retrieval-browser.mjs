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


await call('/api/v1/user-config/model',{config:{channelMode:'local',model:'fixture-video',videoModel:'fixture-video',activeChannelId:'fixture',videoChannelId:'fixture',localChannels:[{id:'fixture',name:'Fixture',protocol:'openai',baseUrl:'https://provider.invalid',apiKey:'fixture-key',models:['fixture-video']}]}},alice.token);
const taskId='client_video_task_browser_fixture',projectId='retrieval-fixture';
const timestamp=new Date().toISOString();
await call('/api/v1/canvas/projects',{data:{id:projectId,title:'Video recovery fixture',createdAt:timestamp,updatedAt:timestamp,nodes:[{id:'video-fixture',type:'video',title:'Video recovery fixture',position:{x:200,y:120},width:400,height:230,metadata:{status:'loading',startedAt:Date.now()-45*60*1000,progress:30,clientTaskId:taskId,videoTaskId:taskId,taskBackend:'account',model:'fixture-video',channelId:'fixture',prompt:'Fixture'}}],connections:[],chatSessions:[],activeChatId:null,agentConfig:null,autoTitlePending:false,backgroundMode:'lines',showImageInfo:false,viewport:{x:0,y:0,k:1},sidePanel:{open:false,width:280},agentPanel:{open:false,width:464}}},alice.token);
browser=await chromium.launch({executablePath:process.env.ICAN_TEST_CHROMIUM,headless:true,args:['--no-sandbox','--disable-dev-shm-usage','--no-zygote'],env:process.env});
const ctx=await browser.newContext({viewport:{width:1400,height:1000}}),page=await ctx.newPage();
const errors=[];page.on('pageerror',e=>errors.push(e.message));
let retrieves=0,creates=0,polls=0;
let task={id:taskId,task_id:'original-fixture-id',model:'fixture-video',server_task:true,status:'completed',progress:100};
await page.route('**/api/v1/videos',route=>{creates++;return route.fulfill({status:500,body:'Paid creates forbidden'});});
await page.route('**/api/v1/videos/*',route=>{polls++;return route.fulfill({contentType:'application/json',body:JSON.stringify({code:0,data:task})});});
await page.route('**/api/v1/video-tasks/*/retrieve',route=>{
 retrieves++;const body=route.request().postDataJSON();
 if(retrieves===2)assert.equal(body.retryAttempt,1);
 task={...task,retrieval:{status:'fetching',attempt:retrieves}};
 return route.fulfill({contentType:'application/json',body:JSON.stringify({code:0,data:task})});
});
// Hold media responses indefinitely: no loadedmetadata or error can unblock delivery.
const held=[];await page.route('**/api/files/fixture-result/content',route=>{held.push(route);});
await page.goto(origin+'/login');await page.getByLabel('用户名',{exact:true}).fill('alice');await page.getByLabel('密码',{exact:true}).fill('alice-password');await page.locator('button[type=submit]').click();await page.waitForURL(u=>!u.pathname.startsWith('/login'));
await page.goto(origin+'/canvas/'+projectId);
await page.getByText('生成完成，正在取回',{exact:true}).waitFor({timeout:20000});assert.equal(retrieves,1);
await page.reload();await page.getByText('生成完成，正在取回',{exact:true}).waitFor({timeout:15000});assert.equal(retrieves,1);
task={...task,retrieval:{status:'failed',attempt:1,errorDetail:'Fixture: download length mismatch'}};
await page.getByRole('button',{name:'继续取回',exact:true}).waitFor({timeout:15000});
assert.ok((await page.locator('body').innerText()).includes('取回失败'));assert.equal(retrieves,1);
await page.getByRole('button',{name:'继续取回',exact:true}).click();await page.getByText('生成完成，正在取回',{exact:true}).waitFor();assert.equal(retrieves,2);
task={...task,retrieval:{status:'ready',attempt:2},url:'/api/files/fixture-result/content',video_url:'/api/files/fixture-result/content',storageKey:'server:fixture-result'};
await page.locator('video[src="/api/files/fixture-result/content"]').waitFor({state:'attached',timeout:15000});
assert.equal(await page.locator('video[src="/api/files/fixture-result/content"]').evaluate(v=>v.readyState),0);
for(let i=0;i<30;i++){const projects=await call('/api/v1/canvas/projects',undefined,alice.token);if(projects.find(p=>p.id===projectId)?.nodes[0].metadata.storageKey==='server:fixture-result')break;await new Promise(r=>setTimeout(r,100));}
const stored=(await call('/api/v1/canvas/projects',undefined,alice.token)).find(p=>p.id===projectId).nodes[0].metadata;
assert.equal(stored.status,'success');assert.equal(stored.videoTaskId,'original-fixture-id');assert.equal(stored.videoRetrieval.status,'ready');
await page.reload({waitUntil:'domcontentloaded'});await page.locator('video[src="/api/files/fixture-result/content"]').waitFor({state:'attached'});
assert.equal(retrieves,2);assert.equal(creates,0);assert.deepEqual(errors,[]);
console.log(JSON.stringify({completion:'generation and retrieval shown separately',refresh:'fetching and ready restored without duplicate retrieval',failure:'visible; same-task manual retrieval only',metadata:'response held; successful canvas persisted before metadata',polls,retrieves,paidCreates:creates,pageErrors:errors}));
await ctx.close();
}finally{if(browser)await browser.close();next.kill();api.kill();fs.rmSync(data,{recursive:true,force:true});}
