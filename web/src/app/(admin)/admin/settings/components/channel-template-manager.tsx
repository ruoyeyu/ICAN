"use client";

import { Alert, App, Button, Card, Form, Input, Modal, Select, Space, Table, Typography } from "antd";
import { saveAs } from "file-saver";
import { useCallback, useEffect, useRef, useState } from "react";
import { ChannelTemplateForm } from "@/components/channel-template-form";
import { importChannelTemplate, type ChannelTemplate } from "@/lib/channel-template";
import { deleteChannelTemplate, fetchChannelTemplate, fetchChannelTemplates, saveChannelTemplate, type ChannelTemplateEntry, type ChannelTemplateCatalog } from "@/services/api/channel-templates";
import { useUserStore } from "@/stores/use-user-store";

const blank = (): ChannelTemplate => ({ format: "ican-channel", version: 1, name: "", instructions: "", baseUrl: "", protocol: "custom", models: [], modelCapabilities: {}, authoring: { version: 1, models: [] } });

export function ChannelTemplateManager() {
    const { message, modal } = App.useApp();
    const token = useUserStore(state => state.token);
    const [catalog, setCatalog] = useState<ChannelTemplateCatalog>({ items: [] });
    const [error, setError] = useState("");
    const [draft, setDraft] = useState<ChannelTemplate>();
    const [editing, setEditing] = useState<ChannelTemplateEntry>();
    const [saving, setSaving] = useState(false);
    const file = useRef<HTMLInputElement>(null);
    const request = useRef(0);
    const alive = useRef(true);
    useEffect(() => { alive.current = true; return () => { alive.current = false; }; }, []);
    const current = useCallback(() => alive.current && useUserStore.getState().token === token, [token]);
    const refresh = useCallback(async () => {
        const revision = ++request.current;
        try { const data = await fetchChannelTemplates(token, true); if (current() && revision === request.current) { setCatalog(data); setError(""); } }
        catch (error) { if (current() && revision === request.current) setError(error instanceof Error ? error.message : "读取失败"); }
    }, [token, current]);
    useEffect(() => { void refresh(); }, [refresh]);
    const report = (error: unknown) => { if (current()) message.error(error instanceof Error ? error.message : "操作失败"); };
    const load = async (entry: ChannelTemplateEntry, download = false) => {
        try {
            const loaded = await fetchChannelTemplate(token, entry.id, true);
            if (!current()) return;
            if (download) saveAs(new Blob([JSON.stringify(loaded.template, null, 2)], { type: "application/json" }), `${entry.id}.json`);
            else { setEditing(loaded); setDraft(loaded.template); }
        } catch (error) { report(error); }
    };
    const save = async () => {
        setSaving(true);
        try {
            const value = importChannelTemplate(JSON.stringify(draft));
            await saveChannelTemplate(token, value, editing?.id, editing?.revision);
            if (current()) { setDraft(undefined); message.success("模板已保存，用户重新打开选择器即可使用"); await refresh(); }
        } catch (error) { report(error); } finally { if (current()) setSaving(false); }
    };
    return <Card title="供应商模板" variant="borderless" extra={<Space><Button onClick={() => void refresh()}>刷新模板</Button><Button onClick={() => file.current?.click()}>导入模板</Button><Button type="primary" onClick={() => { setEditing(undefined); setDraft(blank()); }}>新增模板</Button></Space>}>
        <Typography.Paragraph type="secondary">仅当前站点登录用户可见。模板不保存 Key，用户选择后填写自己的 Key；修改或删除模板不影响已保存渠道。</Typography.Paragraph>
        {error && <Alert type="error" title={error} />}
        {catalog.errors?.map(item => <Alert key={item} type="warning" title={item} />)}
        <Table rowKey="id" dataSource={catalog.items} pagination={false} columns={[
            { title: "供应商", dataIndex: "name" },
            { title: "模型", render: (_, entry) => entry.models.join("、") },
            { title: "操作", render: (_, entry) => <Space><Button onClick={() => void load(entry)}>编辑</Button><Button onClick={() => void load(entry, true)}>导出</Button><Button danger onClick={() => modal.confirm({ title: `删除模板“${entry.name}”？`, content: "将删除方法文件并从站点选择器移除。用户已有渠道、Key 和生成记录保留。", onOk: async () => { try { await deleteChannelTemplate(token, entry); if (current()) await refresh(); } catch (error) { report(error); throw error; } } })}>删除</Button></Space> },
        ]} />
        <input ref={file} type="file" accept="application/json,.json" hidden onChange={async e => {
            const chosen = e.target.files?.[0]; e.target.value = ""; if (!chosen) return;
            try { const value = importChannelTemplate(await chosen.text()); if (current()) { setEditing(undefined); setDraft(value); } } catch (error) { report(error); }
        }} />
        {draft && <Modal open title={editing ? "编辑供应商模板" : "新增供应商模板"} width={960} styles={{ body: { maxHeight: "70vh", overflowY: "auto" } }} onCancel={() => setDraft(undefined)} onOk={() => void save()} confirmLoading={saving} okText="保存模板">
            <Form layout="vertical">
                <Form.Item label="供应商名称"><Input aria-label="模板名称" value={draft.name} onChange={e => setDraft({ ...draft, name: e.target.value })} /></Form.Item>
                <Form.Item label="基础地址"><Input aria-label="模板基础地址" value={draft.baseUrl} onChange={e => setDraft({ ...draft, baseUrl: e.target.value })} placeholder="https://api.example.com" /></Form.Item>
                <Form.Item label="基础协议"><Select value={draft.protocol} onChange={protocol => setDraft({ ...draft, protocol })} options={[{ value: "custom", label: "自定义接口" }, { value: "openai", label: "OpenAI 兼容" }]} /></Form.Item>
                <Form.Item label="模型名称"><Select aria-label="模板模型名称" mode="tags" value={draft.models} onChange={models => setDraft({ ...draft, models })} /></Form.Item>
                <Form.Item label="使用说明"><Input.TextArea aria-label="模板使用说明" value={draft.instructions || ""} onChange={e => setDraft({ ...draft, instructions: e.target.value })} placeholder="说明支持的参数和素材；不要填写密钥" /></Form.Item>
            </Form>
            <ChannelTemplateForm key={editing?.id || "new"} models={draft.models} value={draft.authoring} onChange={authoring => setDraft({ ...draft, authoring })} />
        </Modal>}
    </Card>;
}
