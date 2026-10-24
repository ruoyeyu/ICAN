import { apiDelete, apiGet, apiPost } from "@/services/api/request";
import type { UserWebDAVStorageProvider } from "@/services/image-storage";

export type RegisteredStorageObject = {
    url: string;
    storageKey: string;
    bytes: number;
    mimeType: string;
};

export type StorageObjectInfo = {
    id: string;
    objectKey: string;
    publicUrl: string;
    mimeType: string;
    bytes: number;
    direct: boolean;
};

export function getStorageObjectInfo(id: string) {
    return apiGet<StorageObjectInfo>(`/api/files/${encodeURIComponent(id)}`);
}

export function registerDirectStorageObject(
    token: string,
    payload: { provider: UserWebDAVStorageProvider; objectKey: string; mimeType: string; bytes: number },
) {
    return apiPost<RegisteredStorageObject>("/api/v1/files/direct", payload, token);
}

export function deleteDirectStorageObjectRecord(token: string, id: string) {
    return apiDelete<boolean>(`/api/v1/files/${encodeURIComponent(id)}/record`, token);
}

// Supplier links grant access to one owned media object only. Login tokens are
// never appended to a URL or sent to the supplier.
export async function authorizeReferenceURL(value: string): Promise<string> {
    if (typeof window === "undefined") return value;
    const parsed = new URL(value, window.location.origin);
    if (parsed.origin !== window.location.origin) return value;
    const match = parsed.pathname.match(/^\/api\/files\/([^/]+)\/content$/);
    if (!match) return value;
    const result = await apiPost<{ url: string }>(`/api/v1/files/${encodeURIComponent(decodeURIComponent(match[1]))}/share`);
    return result.url;
}
