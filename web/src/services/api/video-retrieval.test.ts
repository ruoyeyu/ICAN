import test from "node:test";
import assert from "node:assert/strict";
import axios from "axios";
import { pollVideoGenerationTaskStatus, retrieveVideoGenerationTask, readVideoContentResponse, type VideoResponse } from "./video";
import { uploadMediaFile } from "../file-storage";
import { useUserStore } from "@/stores/use-user-store";
import { defaultConfig } from "@/stores/use-config-store";

test("server delivery survives refreshed callers without browser download or create", async () => {
    const previousUser=useUserStore.getState(), adapter=axios.defaults.adapter;
    const calls:string[]=[];
    let state:VideoResponse={id:"client_video_task_fixture",task_id:"original-id",status:"completed",progress:100,server_task:true};
    const config={...defaultConfig,channelMode:"remote" as const,model:"fixture-video",videoModel:"fixture-video"};
    useUserStore.setState({token:"fixture-token",user:{id:"fixture-owner",role:"admin"} as never});
    axios.defaults.adapter=async request=>{
        const path=request.url || "";calls.push(`${request.method} ${path}`);
        assert.ok(path.startsWith("/api/v1/videos/") || path.endsWith("/retrieve"));
        if(path.endsWith("/retrieve")) state={...state,retrieval:{status:"fetching",attempt:1}};
        return {data:{code:0,data:state},status:200,statusText:"OK",headers:{},config:request};
    };
    try{
        let task=await pollVideoGenerationTaskStatus(config,{id:state.id});
        assert.equal(task.status,"completed");assert.equal(task.retrieval?.status,"fetching");assert.equal(task.url,undefined);
        task=await pollVideoGenerationTaskStatus(config,{id:state.id});
        assert.equal(calls.filter(x=>x.endsWith("/retrieve")).length,1);
        state={...state,retrieval:{status:"ready",attempt:1},url:"/api/files/fixture/content",storageKey:"server:fixture"};
        task=await pollVideoGenerationTaskStatus(config,{id:state.id});
        assert.equal(task.storageKey,"server:fixture");assert.equal(task.retrieval?.status,"ready");
        assert.ok(!calls.some(x=>x==="post /api/v1/videos"||x.includes("/content")));
        state={...state,url:undefined,storageKey:undefined,retrieval:{status:"failed",attempt:1,errorDetail:"fixture failure"}};
        task=await pollVideoGenerationTaskStatus(config,{id:state.id});
        assert.equal(task.retrieval?.status,"failed");assert.equal(calls.filter(x=>x.endsWith("/retrieve")).length,1);
        await retrieveVideoGenerationTask(config,task,true);
        assert.equal(calls.filter(x=>x.endsWith("/retrieve")).length,2);
    }finally{axios.defaults.adapter=adapter;useUserStore.setState(previousUser)}
});

test("media delivery never waits for a video metadata DOM event", async()=>{
    const previousUser=useUserStore.getState(),adapter=axios.defaults.adapter,originalFetch=globalThis.fetch;
    const descriptor=Object.getOwnPropertyDescriptor(globalThis,"document");
    let uploads=0;
    useUserStore.setState({token:"metadata-token",user:{id:"metadata-owner",role:"admin"} as never});
    Object.defineProperty(globalThis,"document",{configurable:true,value:{createElement(){throw new Error("metadata events deliberately unavailable")}}});
    axios.defaults.adapter=async config=>({data:{code:0,data:{mode:"server_sqlite_s3",autoSyncAllAssets:true,allowUserGlobalProvider:true}},status:200,statusText:"OK",headers:{},config});
    globalThis.fetch=(async (url)=>{assert.equal(url,"/api/v1/files");uploads++;return Response.json({code:0,data:{storageKey:"server:metadata",url:"/api/files/metadata/content",mimeType:"video/mp4",bytes:4}})}) as typeof fetch;
    try{
        const video=new Blob(["data"],{type:"video/mp4"});
        const results=await Promise.all([uploadMediaFile(video,"fixture","metadata-fixture"),uploadMediaFile(video,"fixture","metadata-fixture")]);
        assert.equal(uploads,1);assert.ok(results.every(result=>result.storageKey==="server:metadata"));
        assert.equal(results[0].width,undefined);assert.equal(results[0].height,undefined);
    }finally{globalThis.fetch=originalFetch;axios.defaults.adapter=adapter;useUserStore.setState(previousUser);if(descriptor)Object.defineProperty(globalThis,"document",descriptor);else Reflect.deleteProperty(globalThis,"document")}
});

test("browser rejects a truncated checked download before upload",async()=>{
    await assert.rejects(readVideoContentResponse(new Response("short",{headers:{"Content-Type":"video/mp4","X-ICAN-Content-Length":"100"}})),/长度不符/);
});
