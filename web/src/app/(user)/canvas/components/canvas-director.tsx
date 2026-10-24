"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { directorProjectSchema } from "@/lib/director-project";
import type { CanvasDirectorCapture, CanvasDirectorPanorama, CanvasDirectorVideo } from "../types";

type PanoramaRemoval = Pick<CanvasDirectorPanorama, "edgeId" | "sourceNodeId">;
type DirectorCapturePayload = { dataUrl?: unknown; fileName?: unknown } | null | undefined;

export function CanvasDirector({
    nodeId,
    project,
    panoramas,
    theme,
    onClose,
    onProjectChange,
    onPanoramaRemoved,
    onCapturesSent,
}: {
    nodeId: string;
    project: unknown;
    panoramas: CanvasDirectorPanorama[];
    theme: "light" | "dark";
    onClose: () => void;
    onProjectChange: (project: unknown) => void;
    onPanoramaRemoved: (payload: PanoramaRemoval) => void;
    onCapturesSent: (nodeId: string, captures: CanvasDirectorCapture[]) => void;
    onVideoSent: (nodeId: string, video: CanvasDirectorVideo) => void;
}) {
    const iframeRef = useRef<HTMLIFrameElement>(null);
    const projectRef = useRef(project);
    const themeRef = useRef(theme);
    const sessionSentRef = useRef(false);
    const [ready, setReady] = useState(false);
    const [error, setError] = useState("");

    useEffect(() => {
        projectRef.current = project;
    }, [project]);

    useEffect(() => {
        themeRef.current = theme;
    }, [theme]);

    const postToDesk = useCallback((type: string, payload: unknown) => {
        iframeRef.current?.contentWindow?.postMessage({ type, payload }, "*");
    }, []);

    useEffect(() => {
        function handleMessage(event: MessageEvent) {
            if (event.origin !== "null" || event.source !== iframeRef.current?.contentWindow) return;

            const type = event.data?.type;
            if (type === "storyai:director-error") { setError("场景格式无法读取，原始数据已保留。"); return; }
            if (type === "storyai:director-ready") {
                setReady(true);
                return;
            }

            if (type === "storyai:director-close") {
                onClose();
                return;
            }

            if (type === "storyai:director-project-changed") {
                const nextProject = event.data?.payload?.project;
                const parsed = directorProjectSchema.safeParse(nextProject);
                if (parsed.success) onProjectChange(parsed.data);
                return;
            }

            if (type === "storyai:director-panorama-removed") {
                const edgeId = typeof event.data?.payload?.edgeId === "string" ? event.data.payload.edgeId.trim() : "";
                const sourceNodeId = typeof event.data?.payload?.sourceNodeId === "string" ? event.data.payload.sourceNodeId.trim() : "";
                if (edgeId && sourceNodeId) onPanoramaRemoved({ edgeId, sourceNodeId });
                return;
            }

            if (type === "storyai:director-captures-sent") {
                const captures = Array.isArray(event.data?.payload?.captures)
                    ? event.data.payload.captures
                        .filter((capture: DirectorCapturePayload): capture is { dataUrl: string; fileName?: unknown } => typeof capture?.dataUrl === "string" && /^data:image\/png;base64,iVBORw0KGgo[A-Za-z0-9+/]*={0,2}$/.test(capture.dataUrl))
                        .map((capture: { dataUrl: string; fileName?: unknown }, index: number) => ({ dataUrl: capture.dataUrl, fileName: typeof capture.fileName === "string" && capture.fileName.trim() ? capture.fileName.trim() : "导演台截图-" + (index + 1) + ".png" }))
                    : [];
                if (captures.length) void onCapturesSent(nodeId, captures);
                return;
            }


        }

        window.addEventListener("message", handleMessage);
        return () => window.removeEventListener("message", handleMessage);
    }, [nodeId, onCapturesSent, onClose, onPanoramaRemoved, onProjectChange]);

    useEffect(() => {
        if (!ready || sessionSentRef.current) return;
        sessionSentRef.current = true;
        postToDesk("storyai:director-session", {
            instanceId: nodeId,
            theme: themeRef.current,
            project: projectRef.current,
        });
    }, [nodeId, postToDesk, ready]);

    useEffect(() => {
        if (!ready || !sessionSentRef.current) return;
        let cancelled = false;
        void Promise.all(panoramas.map(async panorama => {
            const url = new URL(panorama.imageUrl, window.location.href);
            if (!["data:", "blob:"].includes(url.protocol) && url.origin !== window.location.origin) throw new Error("请先保存外部全景图");
            const response = await fetch(url, { redirect: "error" });
            if (!response.ok) throw new Error("全景图读取失败");
            const bitmap = await createImageBitmap(await response.blob());
            try {
                const canvas = document.createElement("canvas");
                canvas.width = bitmap.width; canvas.height = bitmap.height;
                const context = canvas.getContext("2d");
                if (!context) throw new Error("全景图转换失败");
                context.drawImage(bitmap, 0, 0);
                return { ...panorama, imageUrl: canvas.toDataURL("image/png") };
            } finally { bitmap.close(); }
        })).then(items => { if (!cancelled) postToDesk("storyai:director-panoramas", { panoramas: items }); })
            .catch(() => { if (!cancelled) setError("全景图读取失败，请先将图片保存为画布素材。"); });
        return () => { cancelled = true; };
    }, [panoramas, postToDesk, ready]);

    return (
        <div className="fixed inset-0 z-[2000]">
            {error ? <div role="alert" className="absolute inset-x-0 top-0 z-10 p-4" style={{ background: "Canvas", color: "CanvasText" }}>{error}<button onClick={onClose} className="ml-4 underline">关闭</button></div> : null}
            <iframe sandbox="allow-scripts allow-downloads" referrerPolicy="no-referrer" ref={iframeRef} title="3D导演台" src="/director/index.html" className="block h-full w-full border-0" />
        </div>
    );
}
