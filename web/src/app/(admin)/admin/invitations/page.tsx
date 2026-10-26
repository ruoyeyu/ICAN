"use client";

import { useEffect, useState } from "react";
import { App, Button, DatePicker, Flex, Table, Typography } from "antd";
import type { Dayjs } from "dayjs";
import { createInvitation, listInvitations, revokeInvitation, type Invitation } from "@/services/api/invitations";
import { useUserStore } from "@/stores/use-user-store";

export default function InvitationsPage() {
    const { message } = App.useApp();
    const token = useUserStore((state) => state.token);
    const [items, setItems] = useState<Invitation[]>([]);
    const [expiresAt, setExpiresAt] = useState<Dayjs | null>(null);
    const [busy, setBusy] = useState(false);
    const load = async () => setItems((await listInvitations(token)) || []);
    useEffect(() => { if (token) void load().catch((error) => message.error(error.message)); }, [token]);
    const create = async () => {
        setBusy(true);
        try {
            await createInvitation(expiresAt?.unix() || 0, token);
            await load();
        } catch (error) { message.error(error instanceof Error ? error.message : "创建失败"); }
        finally { setBusy(false); }
    };
    const revoke = async (id: string) => {
        try { await revokeInvitation(id, token); await load(); }
        catch (error) { message.error(error instanceof Error ? error.message : "撤销失败"); }
    };
    return <Flex vertical gap={20} style={{ padding: 24 }}>
        <Typography.Title level={3}>邀请码</Typography.Title>
        <Flex gap={12} wrap>
            <DatePicker showTime value={expiresAt} onChange={setExpiresAt} placeholder="可选：过期时间" />
            <Button type="primary" loading={busy} onClick={() => void create()}>创建一次性邀请码</Button>
        </Flex>
        <Table rowKey="id" dataSource={items} columns={[
            { title: "邀请码", render: (_, item) => item.code ? <Typography.Text copyable code>{item.code}</Typography.Text> : "旧版邀请码（未保存原文）" },
            { title: "创建时间", dataIndex: "createdAt" },
            { title: "过期时间", render: (_, item) => item.expiresAt ? new Date(item.expiresAt * 1000).toLocaleString() : "未设置" },
            { title: "状态", render: (_, item) => item.usedBy ? "已使用" : item.expiresAt > 0 && item.expiresAt * 1000 <= Date.now() ? "已过期" : "可使用" },
            { title: "操作", render: (_, item) => item.usedBy ? null : <Button danger onClick={() => void revoke(item.id)}>撤销</Button> },
        ]} />
    </Flex>;
}
