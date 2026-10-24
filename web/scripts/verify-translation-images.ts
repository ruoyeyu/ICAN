// Separate process keeps module mocks out of the rest of the test suite.
import assert from "node:assert/strict";
import { mock } from "bun:test";
const png="iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jGZkAAAAASUVORK5CYII=";
const dataUrl="data:image/png;base64,"+png;
let state={token:"fixture-session",user:{id:"fixture-user"}};
let uploadFailure=false, switchDuringRead=false;
const reads: string[]=[], shares: string[]=[], uploads: RequestInit[]=[];
mock.module("../src/stores/use-user-store",()=>({useUserStore:{getState:()=>state}}));
mock.module("../src/services/image-storage",()=>({
 resolveImageUrl:async (key:string)=>key==="server:owned"?"/api/files/owned/content":"",
 imageToDataUrl:async (ref:{url?:string,dataUrl?:string})=>{
  reads.push(ref.url||ref.dataUrl||""); if(switchDuringRead)state={token:"different-session",user:{id:"other"}};
  return ref.dataUrl?.startsWith("data:")?ref.dataUrl:ref.url?.startsWith("blob:")?"":dataUrl;
 }
}));
// Exercise the real URL authorization helper without any network.
mock.module("../src/services/api/request",()=>({
 apiGet:async()=>{throw Error("unexpected GET");},apiDelete:async()=>{throw Error("unexpected DELETE");},
 apiPost:async(path:string)=>{shares.push(path);return {url:"https://ican.invalid/share/owned?signature=fixture"};}
}));
Object.assign(globalThis,{window:{location:{origin:"https://ican.invalid"}},FileReader:class {
 result="";onload=()=>{};onerror=()=>{};
 async readAsDataURL(blob:Blob){this.result=`data:${blob.type};base64,${Buffer.from(await blob.arrayBuffer()).toString("base64")}`;this.onload();}
}});
globalThis.fetch=async(input,init)=>{
 if(String(input)==="blob:fixture")return new Response(Buffer.from(png,"base64"),{headers:{"Content-Type":"image/png"}});
 assert.equal(input,"/api/v1/files"); uploads.push(init!);
 assert.equal((init!.headers as Record<string,string>).Authorization,"Bearer fixture-session");
 const file=(init!.body as FormData).get("file") as File;
 assert.equal(Buffer.from(await file.arrayBuffer()).toString("base64"),png);
 return Response.json(uploadFailure?{code:1,msg:"fixture upload denied"}:{code:0,data:{url:"https://ican.invalid/api/files/owned/content"}},{status:uploadFailure?403:200});
};
const {prepareTranslationImages}=await import("../src/services/api/translation-images");
const ref=(url:string)=>({id:"fixture",name:"ref.png",type:"image/png",dataUrl:url.startsWith("data:")?url:"",url});
assert.deepEqual(await prepareTranslationImages([],{format:"url"}),[]);
assert.deepEqual(await prepareTranslationImages([ref("https://external.invalid/a.png")],{format:"url"}),["https://external.invalid/a.png"]);
assert.equal(shares.length,0);assert.equal(uploads.length,0);
assert.deepEqual(await prepareTranslationImages([ref("/api/files/owned/content")],{format:"url"}),["https://ican.invalid/share/owned?signature=fixture"]);
assert.equal(shares[0],"/api/v1/files/owned/share");
assert.deepEqual(await prepareTranslationImages([ref(dataUrl),ref("blob:fixture")],{format:"url"}),Array(2).fill("https://ican.invalid/share/owned?signature=fixture"));
assert.equal(uploads.length,2);
assert.deepEqual(await prepareTranslationImages([ref(dataUrl),ref("blob:fixture")],{format:"dataUrl"}),[dataUrl,dataUrl]);
const count=uploads.length;
await assert.rejects(prepareTranslationImages([ref(dataUrl),ref(dataUrl)],{format:"url",max:1}),/最多/);
assert.equal(uploads.length,count);
uploadFailure=true;await assert.rejects(prepareTranslationImages([ref(dataUrl)],{format:"url"}),/denied/);uploadFailure=false;
await assert.rejects(prepareTranslationImages([ref("https://user@external.invalid/a")],{format:"url"}),/地址无效/);
switchDuringRead=true;await assert.rejects(prepareTranslationImages([ref(dataUrl)],{format:"dataUrl"}),/登录状态已变化/);
state={token:"",user:null as never};await assert.rejects(prepareTranslationImages([],{format:"url"}),/登录/);
console.log("reference encoding: URL, private share, Data URL, blob, count, failure, session checks passed");
