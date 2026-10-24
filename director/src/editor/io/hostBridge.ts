import { directorProjectSchema } from "../../../../web/src/lib/director-project";
import { useDirectorStore } from "../store/directorStore";

interface HostPanoramaPayload {
  edgeId?: unknown;
  sourceNodeId?: unknown;
  imageUrl?: unknown;
  fileName?: unknown;
}

interface HostSessionPayload {
  instanceId?: unknown;
  theme?: unknown;
  project?: unknown;
}

export interface HostCaptureItemPayload {
  dataUrl?: unknown;
  fileName?: unknown;
}

export interface HostCaptureBatchPayload {
  captures?: HostCaptureItemPayload[];
}

interface HostConnectedPanorama {
  edgeId: string;
  sourceNodeId: string;
}

let initialized = false;
let sessionOpen = false;
let projectUnsubscribe: (() => void) | null = null;
let hostConnectedPanorama: HostConnectedPanorama | null = null;
let removeUnsubscribe: (() => void) | null = null;
let suppressNextPanoramaRemovalNotice = false;

function normalizeString(value: unknown) {
  return typeof value === "string" ? value.trim() : "";
}

function getHostOrigin() {
  return window.location.origin;
}

function normalizeTheme(value: unknown): "dark" | "light" | null {
  return value === "light" || value === "dark" ? value : null;
}

function applyDirectorDeskTheme(theme: "dark" | "light") {
  document.documentElement.dataset.theme = theme;
  document.documentElement.classList.toggle("dark", theme === "dark");
}

function getInitialHostTheme() {
  try {
    return normalizeTheme(new URLSearchParams(window.location.search).get("theme"));
  } catch {
    return null;
  }
}

function notifyPanoramaRemoved() {
  if (!hostConnectedPanorama) {
    return;
  }

  window.parent?.postMessage(
    {
      type: "storyai:director-panorama-removed",
      payload: hostConnectedPanorama,
    },
    getHostOrigin()
  );
  hostConnectedPanorama = null;
}

function subscribeToPanoramaRemoval() {
  if (removeUnsubscribe) {
    return;
  }

  let previousPanoramaAssetId = useDirectorStore.getState().project.panoramaAssetId;
  removeUnsubscribe = useDirectorStore.subscribe((state) => {
    const nextPanoramaAssetId = state.project.panoramaAssetId;

    if (previousPanoramaAssetId && !nextPanoramaAssetId) {
      if (suppressNextPanoramaRemovalNotice) {
        suppressNextPanoramaRemovalNotice = false;
        hostConnectedPanorama = null;
      } else {
        notifyPanoramaRemoved();
      }
    }

    previousPanoramaAssetId = nextPanoramaAssetId;
  });
}

function importHostPanorama(payload: HostPanoramaPayload) {
  const imageUrl = normalizeString(payload.imageUrl);
  if (!/^data:image\/(?:png|jpeg|webp);base64,[A-Za-z0-9+/]+={0,2}$/.test(imageUrl)) {
    return;
  }

  const fileName = normalizeString(payload.fileName) || "画布全景图.png";
  const edgeId = normalizeString(payload.edgeId);
  const sourceNodeId = normalizeString(payload.sourceNodeId);

  const state = useDirectorStore.getState();
  const current = state.project.assets.find(asset => asset.id === state.project.panoramaAssetId);
  if (current?.url === imageUrl && hostConnectedPanorama?.edgeId === edgeId) return;
  hostConnectedPanorama = edgeId && sourceNodeId ? { edgeId, sourceNodeId } : null;
  useDirectorStore.getState().addImportedAsset({
    kind: "panorama",
    name: fileName,
    fileName,
    url: imageUrl,
    projectionMode: "backdrop",
  });
}

function openHostSession(payload: HostSessionPayload) {
  sessionOpen = false;
  const instanceId = normalizeString(payload.instanceId);
  const theme = normalizeTheme(payload.theme);
  if (theme) {
    applyDirectorDeskTheme(theme);
  }
  suppressNextPanoramaRemovalNotice = Boolean(useDirectorStore.getState().project.panoramaAssetId);
  useDirectorStore.getState().openScopedScene(instanceId || null);
  const parsed = directorProjectSchema.safeParse(payload.project);
  if (parsed.success) useDirectorStore.getState().replaceProject(parsed.data);
  else if (payload.project != null) {
    window.parent.postMessage({ type: "storyai:director-error", payload: { message: "旧场景格式不受支持，请先保留原始数据。" } }, getHostOrigin());
    return;
  }
  suppressNextPanoramaRemovalNotice = false;
  hostConnectedPanorama = null;
  sessionOpen = true;
}

export function postDirectorDeskCapturesToHost(
  captures: Array<{
    dataUrl: string;
    fileName?: string;
  }>
) {
  const normalizedCaptures = captures
    .map((capture, index) => {
      const dataUrl = normalizeString(capture.dataUrl);
      if (!dataUrl) {
        return null;
      }

      return {
        dataUrl,
        fileName: normalizeString(capture.fileName) || `director-desk-capture-${index + 1}.png`,
      };
    })
    .filter((capture): capture is { dataUrl: string; fileName: string } => Boolean(capture));

  if (normalizedCaptures.length === 0) {
    return;
  }

  window.parent?.postMessage(
    {
      type: "storyai:director-captures-sent",
      payload: {
        captures: normalizedCaptures,
      },
    },
    getHostOrigin()
  );
}

function handleHostMessage(event: MessageEvent) {
  if (event.source !== window.parent || event.origin !== getHostOrigin()) {
    return;
  }

  if (event.data?.type === "storyai:director-session") {
    openHostSession((event.data.payload || {}) as HostSessionPayload);
    return;
  }

  if (sessionOpen && event.data?.type === "storyai:director-panoramas") {
    const panoramas = event.data?.payload?.panoramas;
    if (!Array.isArray(panoramas)) return;
    if (panoramas.length) importHostPanorama(panoramas[0]);
    else if (hostConnectedPanorama) {
      suppressNextPanoramaRemovalNotice = true;
      useDirectorStore.getState().removePanoramaAsset();
      hostConnectedPanorama = null;
    }
  }
}

export function initDirectorDeskHostBridge() {
  if (initialized) {
    return;
  }

  initialized = true;
  applyDirectorDeskTheme(getInitialHostTheme() ?? "dark");
  window.addEventListener("message", handleHostMessage);
  subscribeToPanoramaRemoval();
  projectUnsubscribe = useDirectorStore.subscribe((state, previous) => {
    if (!sessionOpen || state.project === previous.project) return;
    const parsed = directorProjectSchema.safeParse(state.project);
    if (parsed.success) window.parent.postMessage({ type: "storyai:director-project-changed", payload: { project: parsed.data } }, getHostOrigin());
  });
}

export function clearDirectorDeskHostBridge() {
  if (!initialized) {
    return;
  }

  initialized = false;
  sessionOpen = false;
  projectUnsubscribe?.();
  projectUnsubscribe = null;
  hostConnectedPanorama = null;
  suppressNextPanoramaRemovalNotice = false;
  window.removeEventListener("message", handleHostMessage);
  removeUnsubscribe?.();
  removeUnsubscribe = null;
}
