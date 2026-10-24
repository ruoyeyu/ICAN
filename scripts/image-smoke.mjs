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
const admin=await api('/api/auth/login',{username:process.env.ADMIN_USERNAME,password:process.env.ADMIN_PASSWORD});
const code=(await api('/api/admin/invitations',{},admin.token)).code;
const user=await api('/api/auth/register',{username:'image-fixture-user',password:'fixture-password-only',invitationCode:code});
const png=Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jGZkAAAAASUVORK5CYII=','base64');
const form=new FormData();form.append('file',new Blob([png],{type:'image/png'}),'fixture.png');
const upload=await fetch(origin+'/api/v1/files',{method:'POST',body:form,headers:{Origin:origin,Authorization:`Bearer ${user.token}`}});
const item=await upload.json();assert.equal(item.code,0,'upload');
assert.equal((await fetch(origin+item.data.url)).status,404,'private media');
const owned=await fetch(origin+item.data.url,{headers:{Authorization:`Bearer ${user.token}`}});assert.deepEqual(Buffer.from(await owned.arrayBuffer()),png);
const link=(await api('/api/v1/files/'+item.data.id+'/share',{},user.token)).url;
assert.equal((await fetch(link)).status,200,'supplier grant');
await api('/api/v1/files/'+item.data.id+'/share',undefined,user.token,'DELETE');assert.equal((await fetch(link)).status,404,'revoked grant');
await api('/api/auth/logout',{},user.token);assert.equal((await api('/api/auth/me',undefined,user.token)).role,'guest');
console.log(JSON.stringify({productionImageSmoke:'passed',user:process.getuid(),seccompFilter:true,readOnlyRoot:true}));
