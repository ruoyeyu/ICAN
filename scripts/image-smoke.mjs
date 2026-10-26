import assert from 'node:assert/strict';
import { setTimeout } from 'node:timers/promises';
import { readFileSync, existsSync, writeFileSync } from 'node:fs';
assert.equal(process.getuid(),10001);
assert.throws(()=>writeFileSync('/app/root-write-probe','fixture'),/EROFS|EACCES/);
assert.match(readFileSync('/proc/self/status','utf8'),/CapEff:\s+0+\n/);
assert.match(readFileSync('/proc/self/status','utf8'),/NoNewPrivs:\s+1\n/);
assert.match(readFileSync('/proc/self/status','utf8'),/Seccomp:\s+2\n/);
for(const path of ['/bin/sh','/usr/bin/apt','/usr/local/bin/npm','/usr/bin/mount']) assert.equal(existsSync(path),false,path);
const origin='http://127.0.0.1:3000';
async function api(path,body,token,method=body===undefined?'GET':'POST') {
 const r=await fetch(origin+path,{method,headers:{'Content-Type':'application/json',Origin:origin,...(token?{Authorization:`Bearer ${token}`}:{})},body:body===undefined?undefined:JSON.stringify(body)});
 const value=await r.json();assert.equal(r.status,200,path);assert.equal(value.code,0,path);return value.data;
}
for(let i=0;i<60;i++){try{if((await fetch(origin+'/api/health')).ok)break;}catch{}await setTimeout(500);}
const loginPage=await fetch(origin+'/login');assert.equal(loginPage.status,200);
assert.match(loginPage.headers.get('content-security-policy'),/script-src 'self' 'unsafe-inline' 'unsafe-eval';/);
assert.ok([400,404].includes((await fetch(origin+'/_next/image?url=https%3A%2F%2Fsupplier.invalid%2Ffixture.png&w=64&q=75')).status));
assert.equal(existsSync('/app/web/node_modules/sharp'),false);
assert.equal(existsSync('/app/web/node_modules/@img'),false);
for (const path of ['/canvas','/image','/video','/asset-library','/director/index.html']) {
 const response=await fetch(origin+path,{redirect:'manual'});
 assert.equal(response.status,307,'anonymous page '+path);
 assert.match(response.headers.get('location'),/\/login\?redirect=/);
}
assert.equal((await fetch(origin+'/api/v1/files',{method:'POST',body:'fixture'})).status,401,'anonymous upload rejected');
assert.equal((await fetch(origin+'/api/anonymous/files',{method:'POST',body:'fixture'})).status,404,'legacy anonymous upload disabled');
const admin=await api('/api/auth/login',{username:process.env.ADMIN_USERNAME,password:process.env.ADMIN_PASSWORD});
const code=(await api('/api/admin/invitations',{},admin.token)).code;
const user=await api('/api/auth/register',{username:'image-fixture-user',password:'fixture-password-only',invitationCode:code});
assert.equal((await fetch(origin+'/api/v1/channel-templates')).status,401,'anonymous template catalog');
const template={format:'ican-channel',version:1,name:'Runtime fixture',baseUrl:'https://api.example.invalid',protocol:'openai',models:['fixture-model'],modelCapabilities:{},authoring:{version:1,models:[]}};
const catalog=await api('/api/admin/channel-templates',{template},admin.token);
assert.equal((await api('/api/v1/channel-templates/'+catalog.id,undefined,user.token)).template.name,template.name);
const common={name:'image',kind:'image',method:'POST',path:'/pictures',format:'json',fields:[{target:'model',source:'model',value:'null',string:false,optional:false,when:'always',location:'body'}],auth:{type:'bearer',header:'Authorization',prefix:'Bearer '},poll:{enabled:false,method:'GET',path:'',taskId:'',status:'',progress:'',success:[],failure:[]},result:'data',content:'',inputs:{version:1,parameters:{},media:{}}};
const sharedTemplate={...template,name:'Shared runtime fixture',protocol:'custom',models:['new-image'],modelCapabilities:{'new-image':'image'},authoring:{version:2,models:[],defaults:[common]}};
common.fields.push({...common.fields[0],target:'resolution',source:'resolution',optional:true,omitValues:['auto']});
const sharedCatalog=await api('/api/admin/channel-templates',{template:sharedTemplate},admin.token);
const translation='{models:{},defaults:{image:{POST:{url:"/pictures",body:{model,resolution:resolution===undefined||resolution===null||resolution===""||resolution==="auto"?undefined:resolution}},result:"data"}}}';
await api('/api/v1/user-config/model',{config:{channelTemplates:[{id:'personal-fixture',template},{id:'personal-shared',template:sharedTemplate}],localChannels:[{id:'runtime-shared',name:'Shared runtime fixture',baseUrl:template.baseUrl,apiKey:'runtime-fixture-private-key',protocol:'custom',models:['new-image']}],channelTranslations:[{channelId:'runtime-shared',parameterTranslation:translation}]}},user.token);
const restartFixture={id:catalog.id,revision:catalog.revision,sharedId:sharedCatalog.id,sharedRevision:sharedCatalog.revision,authoring:sharedTemplate.authoring,translation};
writeFileSync('/app/data/template-restart-fixture.json',JSON.stringify(restartFixture));
const page=await fetch(origin+'/canvas',{headers:{Cookie:`ican_session=${user.token}`},redirect:'manual'});assert.equal(page.status,200,'authenticated canvas');
const png=Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jGZkAAAAASUVORK5CYII=','base64');
const form=new FormData();form.append('file',new Blob([png],{type:'image/png'}),'fixture.png');
const upload=await fetch(origin+'/api/v1/files',{method:'POST',body:form,headers:{Origin:origin,Authorization:`Bearer ${user.token}`}});
const item=await upload.json();assert.equal(item.code,0,'upload');
restartFixture.mediaURL=item.data.url;restartFixture.mediaBase64=png.toString('base64');writeFileSync('/app/data/template-restart-fixture.json',JSON.stringify(restartFixture));
assert.equal((await fetch(origin+item.data.url)).status,404,'private media');
const owned=await fetch(origin+item.data.url,{headers:{Authorization:`Bearer ${user.token}`}});assert.deepEqual(Buffer.from(await owned.arrayBuffer()),png);
const link=(await api('/api/v1/files/'+item.data.id+'/share',{},user.token)).url;
assert.equal((await fetch(link)).status,200,'supplier grant');
await api('/api/v1/files/'+item.data.id+'/share',undefined,user.token,'DELETE');assert.equal((await fetch(link)).status,404,'revoked grant');
await api('/api/auth/logout',{},user.token);assert.equal((await api('/api/auth/me',undefined,user.token)).role,'guest');
assert.equal((await fetch(origin+'/canvas',{headers:{Cookie:`ican_session=${user.token}`},redirect:'manual'})).status,307,'revoked page session');
console.log(JSON.stringify({productionImageSmoke:'passed',user:process.getuid(),seccompFilter:true,readOnlyRoot:true}));
