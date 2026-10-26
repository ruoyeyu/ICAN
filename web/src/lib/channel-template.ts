import { z } from "zod";
import { emptyTranslationInputs, translationInputsSchema } from "./translation-inputs";

// Authoring data only: compiled into the existing parameter-translation engine.
// The versioned comment allows lossless form editing; modified advanced code is
// never replaced by stale form data. Imports do not execute JavaScript.
const path = z.string().regex(/^[\w-]+(?:\.[\w-]+)*$/, "字段路径仅支持点分隔名称").refine(s => !s.split(".").some(k => ["__proto__", "prototype", "constructor"].includes(k)), "禁止原型字段");
const endpoint = z.string().min(1).refine(s => s.startsWith("/") && !s.startsWith("//") && !/[?#\\\r\n]/.test(s) && !s.includes("{channelKey}"), "使用相对于基础地址的路径；查询字段单独映射");
export const fieldSources = ["model", "rawPrompt", "rawSize", "rawSeconds", "resolution", "quality", "count", "mode", "negativePrompt", "generateAudio", "watermark", "images", "firstImage", "firstFrame", "lastFrame", "videos", "audios", "voice", "audioFormat", "speed", "instructions", "messages", "request", "literal"] as const;
const field = z.object({ target: path, source: z.enum(fieldSources), value: z.string(), string: z.boolean(), optional: z.boolean(), omitValues: z.array(z.union([z.string(), z.number(), z.boolean()])).optional(), when: z.enum(["always", "singleImage", "multipleImages", "frames", "noFrames"]), location: z.enum(["body", "query"]) }).strict();
const auth = z.object({ type: z.enum(["bearer", "header", "none"]), header: z.string().regex(/^[A-Za-z0-9-]+$/), prefix: z.string().refine(s => !/[\r\n]/.test(s)) }).strict();
export const templateModelSchema = z.object({
    name: z.string().min(1), kind: z.enum(["image", "video", "audio", "text"]),
    method: z.enum(["POST", "PUT", "PATCH", "GET"]), path: endpoint,
    format: z.enum(["json", "urlencoded", "formData"]), fields: z.array(field), auth,
    poll: z.object({ enabled: z.boolean(), method: z.enum(["GET", "POST"]), path: z.string(), taskId: z.string(), status: z.string(), progress: z.string(), success: z.array(z.string().min(1)), failure: z.array(z.string().min(1)) }).strict(),
    result: z.string(), content: z.string(), inputs: translationInputsSchema,
}).strict().superRefine((m, ctx) => {
    const fail = (message: string) => ctx.addIssue({ code: "custom", message });
    if (m.kind === "text" && (m.poll.enabled || m.content)) fail("自定义文本规则暂不支持异步查询或媒体下载");
    if (m.poll.enabled) {
        for (const v of [m.poll.taskId, m.poll.status, m.poll.progress].filter(Boolean)) if (!path.safeParse(v).success) fail("查询提取路径无效");
        if (!m.poll.taskId || !m.poll.status || !m.poll.success.length || !endpoint.safeParse(m.poll.path).success || !m.poll.path.includes("{taskId}")) fail("异步查询需要任务 ID、查询路径（含 {taskId}）、状态路径和成功状态");
        if (m.poll.success.some(s => m.poll.failure.includes(s))) fail("成功与失败状态不能重叠");
    } else if (m.content) fail("鉴权下载需要异步查询");
    if (m.content && (!endpoint.safeParse(m.content).success || !m.content.includes("{taskId}"))) fail("下载路径需要 {taskId}");
    if (m.result && !path.safeParse(m.result).success) fail("结果提取路径无效");
    if (!m.result && !m.content) fail("请填写结果提取路径或鉴权下载路径");
    for (const f of m.fields) {
        if (f.source === "literal") { try { const v = JSON.parse(f.value); assertNoCredentials(v); } catch { fail(`字段 ${f.target} 的固定值需要不含凭据的 JSON`); } }
        if (/(?:^|\.)(?:api_?key|authorization|password|secret|access_?token|cookie)$/i.test(f.target)) fail("凭据必须使用鉴权设置，不能写入字段映射");
        if (f.location === "query" && f.target.includes(".")) fail("查询字段不支持嵌套对象");
        if (m.method === "GET" && f.location === "body") fail("GET 字段需要放在查询参数中");
    }
    for (const location of ["body", "query"]) {
        const names = m.fields.filter(f => f.location === location).map(f => f.target);
        if (names.some((a, i) => names.some((b, j) => i !== j && (a === b || a.startsWith(b + "."))))) fail("字段路径重复或相互覆盖");
    }
});
export type TemplateModel = z.infer<typeof templateModelSchema>;
export type TemplateField = z.infer<typeof field>;
const authoringSchema = z.object({ version: z.union([z.literal(1), z.literal(2)]), models: z.array(templateModelSchema), defaults: z.array(templateModelSchema).optional() }).strict().superRefine((v, ctx) => {
    if (new Set(v.models.map(m => m.name)).size !== v.models.length) ctx.addIssue({code:"custom",message:"模型名称不能重复"});
    if (v.version === 1 && v.models.some(m => m.kind === "text")) ctx.addIssue({code:"custom",message:"文本规则需要版本 2"});
    if ((v.version === 1 && v.defaults !== undefined) || new Set(v.defaults?.map(m => m.kind)).size !== (v.defaults?.length || 0)) ctx.addIssue({code:"custom",message:"公共规则需要版本 2，每种任务类型只能配置一次"});
});
export type TemplateAuthoring = z.infer<typeof authoringSchema>;
const baseURL = z.string().url().refine(s => { const u = new URL(s); return ["https:", "http:"].includes(u.protocol) && !u.username && !u.password && !u.search && !u.hash; }, "基础地址不能含凭据、查询参数或片段");
const exportSchema = z.object({ format: z.literal("ican-channel"), version: z.literal(1), name: z.string().min(1), instructions: z.string().optional(), baseUrl: baseURL, protocol: z.enum(["custom", "openai"]), models: z.array(z.string().min(1)).min(1), modelCapabilities: z.record(z.string(), z.enum(["image", "video", "audio", "text"])), authoring: authoringSchema }).strict().refine(v => new Set(v.models).size === v.models.length && v.authoring.models.every(m => v.models.includes(m.name)) && Object.keys(v.modelCapabilities).every(m => v.models.includes(m)), "模型列表重复或遗漏了转译模型");
export type ChannelTemplate = z.infer<typeof exportSchema>;

function assertNoCredentials(value: unknown): void {
    if (Array.isArray(value)) { value.forEach(assertNoCredentials); return; }
    if (value && typeof value === "object") for (const [key, v] of Object.entries(value)) {
        if (/^(api_?key|authorization|password|secret|access_?token|cookie)$/i.test(key) || ["__proto__", "prototype", "constructor"].includes(key)) throw new Error("模板不能包含凭据或原型字段");
        assertNoCredentials(v);
    }
    if (typeof value === "string" && /^(?:https?:)?\/\//i.test(value)) {
        const u = new URL(value); if (u.username || u.password || u.search || u.hash) throw new Error("固定 URL 不允许嵌入凭据或查询参数");
    }
}
export function newTemplateField(target = "", source: TemplateField["source"] = "literal"): TemplateField { return { target, source, value: "null", string: false, optional: false, when: "always", location: "body" }; }
export function newTemplateModel(name: string, preset: "blank" | "video" | "openai" = "blank"): TemplateModel {
    return { name, kind: preset === "video" ? "video" : "image", method: "POST", path: preset === "video" ? "/v1/videos" : preset === "openai" ? "/v1/images/generations" : "", format: "json", auth: { type: "bearer", header: "Authorization", prefix: "Bearer " },
        fields: preset === "blank" ? [] : [newTemplateField("model", "model"), newTemplateField("prompt", "rawPrompt"), ...(preset === "video" ? [newTemplateField("aspect_ratio", "rawSize"), newTemplateField("resolution", "resolution"), { ...newTemplateField("seconds", "rawSeconds"), string: true }] : [])],
        poll: { enabled: preset === "video", method: "GET", path: preset === "video" ? "/v1/videos/{taskId}" : "", taskId: preset === "video" ? "id" : "", status: preset === "video" ? "status" : "", progress: "", success: preset === "video" ? ["completed"] : [], failure: preset === "video" ? ["failed"] : [] },
        result: preset === "openai" ? "data" : "", content: preset === "video" ? "/v1/videos/{taskId}/content" : "", inputs: emptyTranslationInputs() };
}
function expression(f: TemplateField): string {
    const raw = f.source === "literal" ? JSON.stringify(JSON.parse(f.value)) : f.source === "firstImage" ? "images[0]" : f.source;
    let value = f.string ? `String(${raw})` : raw;
    if (f.optional) value = `((${raw}) === undefined || (${raw}) === null || (${raw}).length === 0 ? undefined : (${value}))`;
    else if (f.source !== "literal") value = `(() => {if ((${raw}) === undefined || (${raw}) === null) throw new Error(${JSON.stringify(`字段 ${f.target} 缺少 ${f.source}`)}); return (${value});})()`;
    if (f.omitValues?.length) value = `((${f.omitValues.map(v => `(${raw}) === ${JSON.stringify(v)}`).join(" || ")}) ? undefined : (${value}))`;
    const conditions = { always: "", singleImage: "images.length === 1 && !firstFrame && !lastFrame", multipleImages: "images.length > 1 && !firstFrame && !lastFrame", frames: "firstFrame || lastFrame", noFrames: "!firstFrame && !lastFrame" };
    return conditions[f.when] ? `((${conditions[f.when]}) ? (${value}) : undefined)` : value;
}
function mappedObject(fields: TemplateField[]): string {
    const root: Record<string, unknown> = Object.create(null);
    for (const f of fields) {
        const keys = f.target.split("."); let parent = root;
        for (const key of keys.slice(0, -1)) parent = (parent[key] ||= Object.create(null)) as typeof root;
        parent[keys.at(-1)!] = expression(f);
    }
    const render = (v: Record<string, unknown>): string => `{${Object.entries(v).map(([k, x]) => `${JSON.stringify(k)}:${typeof x === "string" ? x : render(x as typeof root)}`).join(",")}}`;
    return render(root);
}
const marker = "// ICAN_FORM_V1 ";
const markerV2 = "// ICAN_FORM_V2 ";
export function compileTemplate(input: TemplateAuthoring): string {
    const data = authoringSchema.parse(input);
    const compileRule = (m: TemplateModel, name: string) => {
        const headers = m.auth.type === "none" ? { Authorization: "" } : m.auth.type === "bearer" ? { Authorization: "Bearer {channelKey}" } : { [m.auth.header]: `${m.auth.prefix}{channelKey}`, ...(m.auth.header.toLowerCase() === "authorization" ? {} : { Authorization: "" }) };
        const request = `{url:${JSON.stringify(m.path)},headers:${JSON.stringify(headers)},format:${JSON.stringify(m.format)},${m.fields.some(f => f.location === "body") ? `body:${mappedObject(m.fields.filter(f => f.location === "body"))},` : ""}params:${mappedObject(m.fields.filter(f => f.location === "query"))}}`;
        const poll = m.poll.enabled ? `poll:{taskId:${JSON.stringify(m.poll.taskId)},${m.poll.method}:{url:${JSON.stringify(m.poll.path)},headers:${JSON.stringify(headers)}},status:${JSON.stringify(m.poll.status)},success:${JSON.stringify(m.poll.success)},failure:${JSON.stringify(m.poll.failure)}${m.poll.progress ? `,progress:${JSON.stringify(m.poll.progress)}` : ""}},` : "";
        return `${JSON.stringify(name)}:{${data.version === 2 ? `kind:${JSON.stringify(m.kind)},` : ""}inputs:${JSON.stringify({...m.inputs, authentication: m.auth.type === "none" ? "none" : "key"})},${m.method}:${request},${poll}${m.result ? `result:${JSON.stringify(m.result)},` : ""}${m.content ? `content:{url:${JSON.stringify(m.content)}},` : ""}}`;
    };
    const rules = data.models.map(m => compileRule(m, m.name));
    const defaults = data.version === 2 ? `,defaults:{${(data.defaults || []).map(m => compileRule(m, m.kind)).join(",\n")}}` : "";
    return `${data.version === 2 ? markerV2 : marker}${JSON.stringify(data).replaceAll("\u2028", "\\u2028").replaceAll("\u2029", "\\u2029")}\n{models:{${rules.join(",\n")}}${defaults}}`;
}
const formReadCache = new Map<string, TemplateAuthoring | null>();
export function readTemplate(source: string): TemplateAuthoring | null {
    if (!source.trim()) return { version: 1, models: [] };
    const prefix = source.startsWith(markerV2) ? markerV2 : marker;
    if (!source.startsWith(prefix)) return null;
    if (formReadCache.has(source)) return formReadCache.get(source)!;
    if (formReadCache.size >= 32) formReadCache.delete(formReadCache.keys().next().value!);
    try { const data = authoringSchema.parse(JSON.parse(source.slice(prefix.length).split("\n")[0])); const result = compileTemplate(data) === source.trim() ? data : null; formReadCache.set(source,result); return result; } catch { return null; }
}
export function importChannelTemplate(text: string): ChannelTemplate { const value = JSON.parse(text); assertNoCredentials(value); const data = exportSchema.parse(value); compileTemplate(data.authoring); return data; }
export function exportChannelTemplate(channel: {name?: string; baseUrl?: string; protocol?: string; apiKey?: string; models: string[]; modelCapabilities?: Record<string, string | undefined>}, source: string): string {
    const authoring = readTemplate(source); if (!authoring) throw new Error("此配置含高级代码；请在高级编辑器审查后复制，不自动导出可能含凭据的代码");
    const data = exportSchema.parse({ format: "ican-channel", version: 1, name: channel.name, baseUrl: channel.baseUrl, protocol: channel.protocol, models: channel.models, modelCapabilities: channel.modelCapabilities || {}, authoring });
    assertNoCredentials(data);
    const text = JSON.stringify(data, null, 2);
    if (channel.apiKey && text.includes(channel.apiKey)) throw new Error("配置内容包含当前 API Key，请先移除");
    return text;
}
