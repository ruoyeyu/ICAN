"use client";

import { Alert, App, Button, Form, Input, Modal, Select, Space } from "antd";
import { useCallback, useEffect, useRef, useState } from "react";
import { compileTemplate, importChannelTemplate, newTemplateModel } from "@/lib/channel-template";
import { useConfigStore, type LocalModelChannel } from "@/stores/use-config-store";
import { useUserStore } from "@/stores/use-user-store";
import { fetchChannelTemplates, fetchChannelTemplate, type ChannelTemplateEntry } from "@/services/api/channel-templates";

export function ChannelTemplateDialog({ onCancel, onSave }: { onCancel: () => void; onSave: (channel: LocalModelChannel, source: string) => Promise<void> }) {
    const { message, modal } = App.useApp();
    const token = useUserStore(state => state.token);
    const personal = useConfigStore(state => state.config.channelTemplates) || [];
    const [catalog, setCatalog] = useState<ChannelTemplateEntry[]>([]);
    const [catalogError, setCatalogError] = useState("");
    const [choice, setChoice] = useState("blank");
    const [loading, setLoading] = useState(false);
    const selection = useRef(0);
    const catalogRequest = useRef(0);
    const alive = useRef(true);
    useEffect(() => { alive.current = true; return () => { alive.current = false; }; }, []);
    const refresh = useCallback(async () => {
        const revision = ++catalogRequest.current;
        try {
            const data = await fetchChannelTemplates(token);
            if (alive.current && revision === catalogRequest.current && useUserStore.getState().token === token) { setCatalog(data.items); setCatalogError(""); }
        } catch (error) { if (alive.current && revision === catalogRequest.current && useUserStore.getState().token === token) setCatalogError(error instanceof Error ? error.message : "站点模板读取失败"); }
    }, [token]);
    useEffect(() => { void refresh(); }, [refresh]);
    const apply = (value: ReturnType<typeof importChannelTemplate>) => { setImported(value); setName(value.name); setBaseUrl(value.baseUrl); setModels(value.models); setApiKey(""); };
    const choose = async (value: string) => {
        if (value === "import") return;
        const revision = ++selection.current;
        setChoice(value); setLoading(true); setImported(undefined); setApiKey("");
        try {
            if (value.startsWith("site:")) {
                const entry = await fetchChannelTemplate(token, value.slice(5));
                if (alive.current && revision === selection.current && useUserStore.getState().token === token) apply(entry.template);
            } else if (value.startsWith("personal:")) {
                const entry = personal.find(item => item.id === value.slice(9));
                if (!entry) throw new Error("个人模板已删除");
                apply(importChannelTemplate(JSON.stringify(entry.template)));
            } else { setPreset(value as "blank" | "openai" | "video"); setName(""); setBaseUrl(""); setModels([]); }
        } catch (error) { if (alive.current && revision === selection.current && useUserStore.getState().token === token) { setChoice(""); message.error(error instanceof Error ? error.message : "模板不可用"); } }
        finally { if (alive.current && revision === selection.current) setLoading(false); }
    };
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
            if (!choice || loading || useUserStore.getState().token !== token) throw new Error("请先选择有效模板");
            const url = new URL(baseUrl);
            if (!name.trim() || !models.length || !["http:", "https:"].includes(url.protocol) || url.username || url.password || url.search || url.hash) throw new Error("请填写名称、无凭据的基础地址和真实模型名称");
            const source = imported ? compileTemplate(imported.authoring) : preset === "video" ? compileTemplate({ version: 1, models: models.map(model => newTemplateModel(model, "video")) }) : "";
            await onSave({ id: `local-${crypto.randomUUID()}`, name: name.trim(), baseUrl, apiKey, models, protocol: imported?.protocol || (preset === "openai" ? "openai" : "custom"), modelCapabilities: imported ? { ...imported.modelCapabilities, ...Object.fromEntries(imported.authoring.models.map(m => [m.name, m.kind])) } : preset === "video" ? Object.fromEntries(models.map(m => [m, "video"])) : {} }, source);
            if (choice === "import" && imported && useUserStore.getState().token === token) {
                const state = useConfigStore.getState();
                state.updateConfig("channelTemplates", [...(state.config.channelTemplates || []), { id: crypto.randomUUID(), template: imported }]);
            }
            onCancel();
        } catch (error) { message.error(error instanceof Error ? error.message : "配置无效"); } finally { setSaving(false); }
    };
    return <Modal open title="新增自定义 API" onCancel={onCancel} onOk={() => void save()} confirmLoading={saving} okButtonProps={{ disabled: loading || !choice }} okText="添加渠道">
        <Form layout="vertical">
            <Form.Item label="供应商模板"><Select aria-label="初始化预设" loading={loading} value={choice || undefined} onOpenChange={open => { if (open) void refresh(); }} onChange={value => void choose(value)} options={[
                { label: "通用预设", options: [{ value: "blank", label: "空白自定义" }, { value: "openai", label: "OpenAI 兼容" }, { value: "video", label: "通用异步视频" }] },
                { label: "站点模板", options: catalog.map(item => ({ value: `site:${item.id}`, label: item.name })) },
                { label: "个人模板", options: personal.map(item => ({ value: `personal:${item.id}`, label: item.template.name })) },
                ...(choice === "import" ? [{ label: "本次导入", options: [{ value: "import", label: imported?.name || "导入配置" }] }] : []),
            ]} /></Form.Item>
            {catalogError && <Alert type="warning" title={catalogError} action={<Button size="small" onClick={() => void refresh()}>重试</Button>} />}
            {imported?.instructions && <p className="mb-4 whitespace-pre-wrap">{imported.instructions}</p>}
            <Form.Item label="名称"><Input aria-label="渠道名称" value={name} onChange={e => setName(e.target.value)} /></Form.Item>
            <Form.Item label="基础地址"><Input aria-label="基础地址" value={baseUrl} onChange={e => setBaseUrl(e.target.value)} placeholder="https://api.example.com" /></Form.Item>
            <Form.Item label="个人 API Key"><Input.Password aria-label="个人 API Key" autoComplete="new-password" value={apiKey} onChange={e => setApiKey(e.target.value)} placeholder="由当前用户填写，不随模板导入导出" /></Form.Item>
            <Form.Item label="模型（输入名称后回车）"><Select aria-label="模型名称" disabled={!!imported} mode="tags" value={models} onChange={setModels} /></Form.Item>
            <Space wrap><Button onClick={() => file.current?.click()}>导入配置文件</Button>{choice.startsWith("personal:") && <Button danger onClick={() => modal.confirm({title:"删除此个人模板？",content:"已保存渠道不受影响；点击设置页完成后同步删除。",onOk: () => { const state = useConfigStore.getState(); state.updateConfig("channelTemplates", (state.config.channelTemplates || []).filter(item => item.id !== choice.slice(9))); void choose("blank"); }})}>删除个人模板</Button>}{imported && <span>已校验，添加为独立渠道</span>}</Space>
            <input ref={file} type="file" accept="application/json,.json" hidden onChange={async e => {
                const chosen = e.target.files?.[0]; e.target.value = ""; if (!chosen) return;
                try { const value = importChannelTemplate(await chosen.text()); if (!alive.current || useUserStore.getState().token !== token) return; ++selection.current; setLoading(false); setChoice("import"); apply(value); } catch(error) { message.error(error instanceof Error ? error.message : "导入失败"); }
            }} />
            <p className="mt-4 text-[var(--ant-color-text-secondary)]">添加后在“接口配置”中编辑各模型的字段、参数、素材和任务查询。导入文件同时保留为个人模板。点击设置页“完成”将渠道、Key 和个人模板同步到账号。</p>
        </Form>
    </Modal>;
}
