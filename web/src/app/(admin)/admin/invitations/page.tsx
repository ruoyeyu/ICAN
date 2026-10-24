"use client";

import { useEffect, useState } from "react";
import { App, Button, DatePicker, Flex, Modal, Table, Typography } from "antd";
import type { Dayjs } from "dayjs";
import { apiDelete, apiGet, apiPost } from "@/services/api/request";
import { useUserStore } from "@/stores/use-user-store";

type Invitation = { id: string; createdAt: string; expiresAt: number; usedBy: string; revoked: boolean };

export default function InvitationsPage() {
    const { message } = App.useApp();
    const token = useUserStore((state) => state.token);
    const [items, setItems] = useState<Invitation[]>([]);
    const [expiresAt, setExpiresAt] = useState<Dayjs | null>(null);
    const [code, setCode] = useState("");
    const [busy, setBusy] = useState(false);
    const load = async () => setItems((await apiGet<Invitation[]>("/api/admin/invitations", undefined, token)) || []);
    useEffect(() => { if (token) void load().catch((error) => message.error(error.message)); }, [token]);
    const create = async () => {
        setBusy(true);
        try {
            const result = await apiPost<{ code: string }>("/api/admin/invitations", { expiresAt: expiresAt?.unix() || 0 }, token);
            setCode(result.code);
            await load();
        } catch (error) { message.error(error instanceof Error ? error.message : "创建失败"); }
        finally { setBusy(false); }
    };
    const revoke = async (id: string) => {
        try { await apiDelete(`/api/admin/invitations/${encodeURIComponent(id)}`, token); await load(); }
        catch (error) { message.error(error instanceof Error ? error.message : "撤销失败"); }
    };
    return <Flex vertical gap={20} style={{ padding: 24 }}>
        <Typography.Title level={3}>邀请码</Typography.Title>
        <Typography.Text type="secondary">每个邀请码只能注册一个账号。邀请码仅在创建时显示，可设置过期时间或随时撤销。</Typography.Text>
        <Flex gap={12} wrap>
            <DatePicker showTime value={expiresAt} onChange={setExpiresAt} placeholder="可选：过期时间" />
            <Button type="primary" loading={busy} onClick={() => void create()}>创建一次性邀请码</Button>
        </Flex>
        <Table rowKey="id" dataSource={items} columns={[
            { title: "创建时间", dataIndex: "createdAt" },
            { title: "过期时间", render: (_, item) => item.expiresAt ? new Date(item.expiresAt * 1000).toLocaleString() : "未设置" },
            { title: "状态", render: (_, item) => item.revoked ? "已撤销" : item.usedBy ? "已使用" : item.expiresAt > 0 && item.expiresAt * 1000 <= Date.now() ? "已过期" : "可使用" },
            { title: "操作", render: (_, item) => <Button danger disabled={item.revoked || !!item.usedBy} onClick={() => void revoke(item.id)}>撤销</Button> },
        ]} />
        <Modal open={!!code} title="保存邀请码" onCancel={() => setCode("")} footer={<Button onClick={() => setCode("")}>已保存</Button>}>
            <Typography.Paragraph>关闭后不会再次显示完整邀请码，请通过可信渠道交给受邀人。</Typography.Paragraph>
            <Typography.Paragraph copyable code>{code}</Typography.Paragraph>
        </Modal>
    </Flex>;
}
