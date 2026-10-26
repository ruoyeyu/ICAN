"use client";

import { App, Button, Checkbox, Collapse, Form, Input, Select, Space, Switch, Typography } from "antd";
import { useState } from "react";
import { fieldSources, newTemplateField, newTemplateModel, type TemplateAuthoring, type TemplateModel } from "@/lib/channel-template";

export function ChannelTemplateForm({ value, onChange, models }: { value: TemplateAuthoring; onChange: (value: TemplateAuthoring) => void; models: string[] }) {
    const { message } = App.useApp();
    const [selected, setSelected] = useState(value.models[0]?.name || "");
    const [scope, setScope] = useState<"models" | "defaults">(!value.models.length && value.defaults?.length ? "defaults" : "models");
    const [kind, setKind] = useState<TemplateModel["kind"]>(value.defaults?.[0]?.kind || "video");
    const rules = scope === "defaults" ? value.defaults || [] : value.models;
    const [name, setName] = useState("");
    const [preset, setPreset] = useState<"blank" | "openai" | "video">("blank");
    const model = rules.find(m => (scope === "defaults" ? m.kind === kind : m.name === selected));
    const patch = (next: Partial<TemplateModel>) => onChange({ ...value, ...((scope === "defaults" || next.kind === "text") ? {version:2 as const} : {}), [scope]: rules.map(m => m === model ? { ...m, ...next } : m) });
    const text = (label: string, val: string, change: (v: string) => void, placeholder = "") => <Form.Item label={label}><Input aria-label={label} value={val} onChange={e => change(e.target.value)} placeholder={placeholder} /></Form.Item>;
    return <div className="min-h-0 flex-1 overflow-auto p-5"><Form layout="vertical">
        <Select aria-label="规则范围" className="mb-5" value={scope} onChange={setScope} options={[{value:"models",label:"模型专属规则"},{value:"defaults",label:"供应商公共规则"}]} />
        {scope === "defaults" ? <Space wrap className="mb-5">
            <Select aria-label="公共规则类型" value={kind} onChange={setKind} options={[{value:"text",label:"文本"},{value:"image",label:"图片"},{value:"video",label:"视频"},{value:"audio",label:"音频"}]} />
            <Button disabled={!!model} onClick={() => onChange({...value,version:2,defaults:[...rules,{...newTemplateModel(kind),kind}]})}>新增公共规则</Button>
        </Space> : <Space wrap className="mb-5">
            <Select aria-label="初始化预设" value={preset} onChange={setPreset} options={[{ value: "blank", label: "空白自定义" }, { value: "openai", label: "OpenAI 兼容图片" }, { value: "video", label: "通用异步视频" }]} />
            <Select aria-label="待配置模型" style={{ minWidth: 200 }} showSearch value={name || undefined} onChange={setName} options={models.filter(n => !value.models.some(m => m.name === n)).map(n => ({ value: n, label: n }))} placeholder="选择已添加的模型" />
            <Button onClick={() => { if (!name || value.models.some(m => m.name === name)) { message.error("请选择尚未配置的模型"); return; } onChange({ ...value, models: [...value.models, newTemplateModel(name, preset)] }); setSelected(name); setName(""); }}>初始化模型配置</Button>
        </Space>}
        {scope === "models" && <Select aria-label="当前配置模型" className="mb-5 w-full" value={model?.name} onChange={setSelected} options={value.models.map(m => ({ value: m.name, label: m.name }))} placeholder="尚未配置模型" />}
        {model && <>
            <div className="grid gap-x-4 md:grid-cols-3">
                <Form.Item label="任务类型"><Select disabled={scope === "defaults"} value={model.kind} onChange={kind => patch({ kind })} options={[{ value: "text", label: "文本" }, { value: "image", label: "图片" }, { value: "video", label: "视频" }, { value: "audio", label: "音频" }]} /></Form.Item>
                <Form.Item label="请求方法"><Select value={model.method} onChange={method => patch({ method })} options={["POST", "PUT", "PATCH", "GET"].map(value => ({ value }))} /></Form.Item>
                <Form.Item label="编码"><Select value={model.format} onChange={format => patch({ format })} options={["json", "formData", "urlencoded"].map(value => ({ value }))} /></Form.Item>
            </div>
            {text("创建请求路径（追加到渠道基础地址）", model.path, path => patch({ path }), "/v1/videos")}
            <div className="grid gap-x-4 md:grid-cols-3">
                <Form.Item label="鉴权方式"><Select value={model.auth.type} onChange={type => patch({ auth: { ...model.auth, type } })} options={[{ value: "bearer", label: "Bearer" }, { value: "header", label: "自定义请求头" }, { value: "none", label: "无需鉴权" }]} /></Form.Item>
                {model.auth.type === "header" && <>{text("鉴权头名称", model.auth.header, header => patch({ auth: { ...model.auth, header } }))}{text("Key 前缀（可空）", model.auth.prefix, prefix => patch({ auth: { ...model.auth, prefix } }))}</>}
            </div>
            <Typography.Title level={5}>请求字段映射</Typography.Title>
            {model.fields.map((field, index) => {
                const update = (next: Partial<typeof field>) => patch({ fields: model.fields.map((f, i) => i === index ? { ...f, ...next } : f) });
                return <div key={index} className="mb-3 space-y-2 rounded border border-[var(--ant-color-border-secondary)] p-3">
                    <div className="grid gap-2 md:grid-cols-3">
                        <Input aria-label={`目标字段 ${index + 1}`} value={field.target} placeholder="目标字段，例如 video_config.reference_mode" onChange={e => update({ target: e.target.value })} />
                        <Select aria-label={`字段来源 ${index + 1}`} value={field.source} onChange={source => update({ source })} options={fieldSources.map(value => ({ value, label: value === "literal" ? "固定 JSON 值" : value }))} />
                        <Select value={field.location} onChange={location => update({ location })} options={[{ value: "body", label: "请求体" }, { value: "query", label: "查询参数" }]} />
                    </div>
                    {field.source === "literal" && <Input.TextArea value={field.value} onChange={e => update({ value: e.target.value })} placeholder={'固定 JSON，例如 "first_last_frame"'} autoSize />}
                    <Space wrap>
                        <Checkbox checked={field.string} onChange={e => update({ string: e.target.checked })}>转字符串</Checkbox>
                        <Checkbox checked={field.optional} onChange={e => update({ optional: e.target.checked })}>为空时省略</Checkbox>
                        <Select mode="tags" aria-label={`指定值时省略 ${index + 1}`} style={{ minWidth: 220 }} placeholder='指定值时省略，如 "auto"' value={(field.omitValues || []).map(v => JSON.stringify(v))} onChange={(values: string[]) => update({ omitValues: values.map(value => {
                            try { const parsed: unknown = JSON.parse(value); if (typeof parsed === "string" || typeof parsed === "boolean" || (typeof parsed === "number" && Number.isFinite(parsed))) return parsed; } catch {}
                            return value;
                        }) })} />
                        <Select value={field.when} style={{ minWidth: 160 }} onChange={when => update({ when })} options={[{ value: "always", label: "始终发送" }, { value: "singleImage", label: "单图且无首尾帧" }, { value: "multipleImages", label: "多图且无首尾帧" }, { value: "frames", label: "有首尾帧" }, { value: "noFrames", label: "无首尾帧" }]} />
                        <Button danger onClick={() => patch({ fields: model.fields.filter((_, i) => i !== index) })}>删除字段</Button>
                    </Space>
                </div>;
            })}
            <Button onClick={() => patch({ fields: [...model.fields, newTemplateField()] })}>添加字段</Button>
            <Collapse className="my-5" items={[
                { key: "inputs", label: "参数选项与素材编码", children: <>
                    {(["seconds", "resolution", "aspectRatio", "mode"] as const).map(key => {
                        const option = model.inputs.parameters[key];
                        const update = (next: typeof option) => patch({ inputs: { ...model.inputs, parameters: { ...model.inputs.parameters, [key]: next } } });
                        return <Form.Item key={key} label={<Space><Checkbox checked={!!option} onChange={e => update(e.target.checked ? { options: [], allowCustom: true } : undefined)}>{key}</Checkbox></Space>}>
                            {option && <Space direction="vertical" className="w-full"><Select mode="tags" className="w-full" value={option.options} onChange={options => update({ ...option, options })} placeholder="输入供应商明确支持的值，回车添加" /><Checkbox checked={option.allowCustom} onChange={e => update({ ...option, allowCustom: e.target.checked })}>允许用户输入自定义值</Checkbox></Space>}
                        </Form.Item>;
                    })}
                    {(["images", "firstFrame", "lastFrame", "videos", "audios"] as const).map(key => <Form.Item key={key} label={key}><Select value={model.inputs.media[key]?.format || "legacy"} onChange={format => patch({ inputs: { ...model.inputs, media: { ...model.inputs.media, [key]: format === "legacy" ? undefined : { format } } } })} options={[{ value: "legacy", label: "未声明此类素材（有输入时报错）" }, { value: "auto", label: "使用此类素材，保持原有编码" }, { value: "url", label: "HTTP(S) URL（私有素材签发限时链接）" }, ...(["videos", "audios"].includes(key) ? [] : [{ value: "dataUrl", label: "图片 Data URL" }])]} /></Form.Item>)}
                </> },
                { key: "poll", label: "异步任务查询", children: <>
                    <Form.Item label="启用轮询"><Switch checked={model.poll.enabled} onChange={enabled => patch({ poll: { ...model.poll, enabled } })} /></Form.Item>
                    {model.poll.enabled && <>
                        <Form.Item label="查询方法"><Select value={model.poll.method} onChange={method => patch({ poll: { ...model.poll, method } })} options={[{ value: "GET" }, { value: "POST" }]} /></Form.Item>
                        {([['path', '查询路径（含 {taskId}）'], ['taskId', '创建响应中的任务 ID 路径'], ['status', '状态路径'], ['progress', '进度路径（可空）']] as const).map(([key, label]) => <div key={key}>{text(label, model.poll[key], val => patch({ poll: { ...model.poll, [key]: val } }))}</div>)}
                        {([['success', '成功状态'], ['failure', '失败状态']] as const).map(([key, label]) => <Form.Item key={key} label={label}><Select mode="tags" value={model.poll[key]} onChange={values => patch({ poll: { ...model.poll, [key]: values } })} /></Form.Item>)}
                    </>}
                </> },
            ]} />
            {text("结果提取路径（如 data.0.url；使用鉴权下载可空）", model.result, result => patch({ result }))}
            {text("鉴权下载路径（可空；含 {taskId}，与查询同源）", model.content, content => patch({ content }))}
            {scope === "models" && <Button className="mr-3" onClick={() => {
                if (value.defaults?.some(rule => rule.kind === model.kind)) { message.error("该任务类型已有公共规则"); return; }
                onChange({...value, version:2, defaults:[...(value.defaults || []), {...model,name:model.kind}]});
                setKind(model.kind); setScope("defaults");
            }}>复制为公共规则</Button>}
            <Button danger onClick={() => { onChange({ ...value, [scope]: rules.filter(m => m !== model) }); setSelected(""); }}>{scope === "defaults" ? "删除公共规则" : "删除此模型的转译配置"}</Button>
        </>}
    </Form></div>;
}
