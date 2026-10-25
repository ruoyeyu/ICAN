"use client";

import { App, Button, Form, Input, Modal, Select, Space } from "antd";
import { useRef, useState } from "react";
import { compileTemplate, importChannelTemplate, newTemplateModel } from "@/lib/channel-template";
import type { LocalModelChannel } from "@/stores/use-config-store";

export function ChannelTemplateDialog({ onCancel, onSave }: { onCancel: () => void; onSave: (channel: LocalModelChannel, source: string) => Promise<void> }) {
    const { message } = App.useApp();
    const [preset, setPreset] = useState<"blank" | "openai" | "video">("blank");
    const [name, setName] = useState("");
    const [baseUrl, setBaseUrl] = useState("");
    const [apiKey, setApiKey] = useState("");
    const [models, setModels] = useState<string[]>([]);
    const [imported, setImported] = useState<ReturnType<typeof importChannelTemplate>>();
    const [saving, setSaving] = useState(false);
    const file = useRef<HTMLInputElement>(null);
    const save = async () => {
        setSaving(true);
        try {
            const url = new URL(baseUrl);
            if (!name.trim() || !models.length || !["http:", "https:"].includes(url.protocol) || url.username || url.password || url.search || url.hash) throw new Error("请填写名称、无凭据的基础地址和真实模型名称");
            const source = imported ? compileTemplate(imported.authoring) : preset === "video" ? compileTemplate({ version: 1, models: models.map(model => newTemplateModel(model, "video")) }) : "";
            await onSave({ id: `local-${crypto.randomUUID()}`, name: name.trim(), baseUrl, apiKey, models, protocol: imported?.protocol || (preset === "openai" ? "openai" : "custom"), modelCapabilities: imported ? { ...imported.modelCapabilities, ...Object.fromEntries(imported.authoring.models.map(m => [m.name, m.kind])) } : preset === "video" ? Object.fromEntries(models.map(m => [m, "video"])) : {} }, source);
            onCancel();
        } catch (error) { message.error(error instanceof Error ? error.message : "配置无效"); } finally { setSaving(false); }
    };
    return <Modal open title="新增自定义 API" onCancel={onCancel} onOk={() => void save()} confirmLoading={saving} okText="添加渠道">
        <Form layout="vertical">
            <Form.Item label="初始化预设"><Select aria-label="初始化预设" disabled={!!imported} value={preset} onChange={setPreset} options={[{ value: "blank", label: "空白自定义" }, { value: "openai", label: "OpenAI 兼容" }, { value: "video", label: "通用异步视频" }]} /></Form.Item>
            <Form.Item label="名称"><Input aria-label="渠道名称" value={name} onChange={e => setName(e.target.value)} /></Form.Item>
            <Form.Item label="基础地址"><Input aria-label="基础地址" value={baseUrl} onChange={e => setBaseUrl(e.target.value)} placeholder="https://api.example.com" /></Form.Item>
            <Form.Item label="个人 API Key"><Input.Password aria-label="个人 API Key" autoComplete="new-password" value={apiKey} onChange={e => setApiKey(e.target.value)} placeholder="由当前用户填写，不随模板导入导出" /></Form.Item>
            <Form.Item label="模型（输入名称后回车）"><Select aria-label="模型名称" disabled={!!imported} mode="tags" value={models} onChange={setModels} /></Form.Item>
            <Space><Button onClick={() => file.current?.click()}>导入配置文件</Button>{imported && <span>已校验，导入为新渠道</span>}</Space>
            <input ref={file} type="file" accept="application/json,.json" hidden onChange={async e => {
                const chosen = e.target.files?.[0]; e.target.value = ""; if (!chosen) return;
                try { const value = importChannelTemplate(await chosen.text()); setImported(value); setName(value.name); setBaseUrl(value.baseUrl); setModels(value.models); setApiKey(""); } catch(error) { message.error(error instanceof Error ? error.message : "导入失败"); }
            }} />
            <p className="mt-4 text-[var(--ant-color-text-secondary)]">添加后在“接口配置”中编辑各模型的字段、参数、素材和任务查询。点击设置页“完成”同步到账号。</p>
        </Form>
    </Modal>;
}
