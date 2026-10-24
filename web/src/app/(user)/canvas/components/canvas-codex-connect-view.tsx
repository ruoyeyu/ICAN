"use client";

import type { useCodexAgent } from "../agent/use-codex-agent";

export function CanvasCodexConnectView(_props: {
    agent: Pick<ReturnType<typeof useCodexAgent>, "connection" | "status" | "error" | "connect" | "disconnect">;
    onChat: () => void;
}) {
    return <p>此部署未启用本地 Agent。</p>;
}
