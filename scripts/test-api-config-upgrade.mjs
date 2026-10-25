// Disposable database/media copies only. Never point these binaries at a server.
import assert from 'node:assert/strict';
import {spawn} from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import crypto from 'node:crypto';
const oldBinary=process.env.ICAN_TEST_OLD_API_BINARY,newBinary=process.env.ICAN_TEST_API_BINARY;
assert.ok(oldBinary&&newBinary,'Provide both source-built fixture API binaries');
const root=fs.mkdtempSync(os.tmpdir()+'/ican-api-upgrade-'),current=root+'/current',origin='http://127.0.0.1:3438';fs.mkdirSync(current);
const env={...process.env,PUBLIC_ORIGIN:origin,PORT:'3438',ADMIN_USERNAME:'fixture-admin',ADMIN_PASSWORD:'fixture-admin-password-only',JWT_SECRET:crypto.randomBytes(48).toString('hex'),DATA_ENCRYPTION_KEY:crypto.randomBytes(32).toString('base64'),DATABASE_DSN:current+'/ican.db',MEDIA_DIR:current+'/media',AI_LOG_DIR:current+'/logs',AUTH_ATTEMPTS_PER_MINUTE:'1000',MEDIA_LINK_TTL_SECONDS:'60'};
let api;
async function start(binary){api=spawn(binary,[],{cwd:current,env,stdio:'ignore'});for(let i=0;i<80;i++){try{if((await fetch(origin+'/api/health')).ok)return;}catch{} await new Promise(r=>setTimeout(r,100));}throw Error('API fixture failed to start');}
async function stop(){if(!api)return;const stopped=new Promise(r=>api.once('exit',r));api.kill();await stopped;api=undefined;}
async function call(path,body,token){const r=await fetch(origin+path,{method:body===undefined?'GET':'POST',headers:{'Content-Type':'application/json',...(token?{Authorization:'Bearer '+token}:{})},body:body===undefined?undefined:JSON.stringify(body)});const j=await r.json();assert.equal(j.code,0,path+': '+j.msg);return j.data;}
try{
await start(oldBinary);
const admin=await call('/api/auth/login',{username:'fixture-admin',password:'fixture-admin-password-only'});
const invite=await call('/api/admin/invitations',{},admin.token);
const alice=await call('/api/auth/register',{username:'upgrade-user',password:'upgrade-password',invitationCode:invite.code});
const settings=await call('/api/admin/settings',undefined,admin.token);settings.public.modelChannel.allowCustomChannel=true;await call('/api/admin/settings',settings,admin.token);
const legacy={localChannels:[{id:'stable-channel',name:'Existing',protocol:'custom',baseUrl:'https://old.example.invalid',apiKey:'retained-fixture-key',models:['old-model']}],channelTranslations:[{channelId:'stable-channel',parameterTranslation:'{models:{"old-model":{POST:{url:"/generate",body:{model,prompt}},result:"url"}}}'}]};
await call('/api/v1/user-config/model',{config:legacy},alice.token);
const png=Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jGZkAAAAASUVORK5CYII=','base64');const form=new FormData();form.append('file',new Blob([png],{type:'image/png'}),'owned.png');
const upload=await(await fetch(origin+'/api/v1/files',{method:'POST',headers:{Authorization:'Bearer '+alice.token},body:form})).json();assert.equal(upload.code,0,JSON.stringify(upload));const mediaURL=new URL(upload.data.url,origin);
const before=await call('/api/v1/user-config',undefined,alice.token);
await stop();fs.cpSync(current,root+'/before-upgrade',{recursive:true});
await start(newBinary);
assert.deepEqual((await call('/api/v1/user-config',undefined,alice.token)).modelConfig,before.modelConfig);
assert.deepEqual(Buffer.from(await(await fetch(mediaURL,{headers:{Authorization:'Bearer '+alice.token}})).arrayBuffer()),png);
assert.notEqual((await fetch(mediaURL)).status,200);
const next=structuredClone(before.modelConfig);next.localChannels.push({id:'new-channel',name:'Configured',protocol:'custom',baseUrl:'https://new.example.invalid',apiKey:'new-fixture-key',models:['fixture-video']});next.channelTranslations.push({channelId:'new-channel',parameterTranslation:fs.readFileSync(process.cwd()+'/service/testdata/channel-template-v1.js','utf8')});
await call('/api/v1/user-config/model',{config:next},alice.token);await stop();await start(newBinary);
const restored=(await call('/api/v1/user-config',undefined,alice.token)).modelConfig;assert.deepEqual(restored,next);
await stop();fs.cpSync(current,root+'/after-upgrade',{recursive:true});fs.rmSync(current,{recursive:true});fs.cpSync(root+'/before-upgrade',current,{recursive:true});
await start(oldBinary);assert.deepEqual((await call('/api/v1/user-config',undefined,alice.token)).modelConfig,before.modelConfig);
assert.deepEqual(Buffer.from(await(await fetch(mediaURL,{headers:{Authorization:'Bearer '+alice.token}})).arrayBuffer()),png);
console.log(JSON.stringify({baseline:'v0.1.2 source 5080380520a61db3c49dd34543b87350bfd10d8a',upgrade:'existing account/session/encrypted API key/channel ID/legacy translation/media retained',newConfig:'persisted over restart',rollback:'stopped copy restored with old binary and original keys',productionData:'not accessed'}));
}finally{await stop();fs.rmSync(root,{recursive:true,force:true});}
