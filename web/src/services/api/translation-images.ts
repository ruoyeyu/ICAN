import type { TranslationReferences } from "@/lib/channel-parameter-translation";
import { dataUrlToFile, readFileAsDataUrl } from "@/lib/image-utils";
import { imageToDataUrl, resolveImageUrl } from "@/services/image-storage";
import { useUserStore } from "@/stores/use-user-store";
import type { ReferenceImage } from "@/types/image";
import { authorizeReferenceURL } from "./storage";

async function referenceDataUrl(reference: ReferenceImage) {
    const value = await imageToDataUrl(reference);
    if (value) return value;
    const blob = [reference.url, reference.dataUrl].find((url) => url?.startsWith("blob:"));
    if (!blob) throw new Error("无法读取参考图片");
    const response = await fetch(blob);
    if (!response.ok) throw new Error("读取参考图片失败");
    const file = await response.blob();
    return readFileAsDataUrl(new File([file], reference.name || "reference.png", { type: file.type }));
}

export async function prepareTranslationImages(references: ReferenceImage[], policy?: TranslationReferences): Promise<string[]> {
    const token = useUserStore.getState().token;
    if (!token || !useUserStore.getState().user) throw new Error("请先登录后再使用参考素材");
    if (policy?.max !== undefined && references.length > policy.max) throw new Error(`该模型最多接受 ${policy.max} 张参考图片`);
    const result = [];
    for (const reference of references) {
        if (useUserStore.getState().token !== token) throw new Error("登录状态已变化");
        if (policy?.format === "dataUrl") {
            result.push(await referenceDataUrl(reference));
            continue;
        }
        let address = "";
        const resolved = reference.storageKey ? await resolveImageUrl(reference.storageKey, "") : "";
        for (const value of [reference.url, reference.dataUrl, resolved]) {
            if (!value || value.startsWith("blob:") || value.startsWith("data:")) continue;
            const url = new URL(value, window.location.origin);
            if (!["http:", "https:"].includes(url.protocol) || url.username || url.password) throw new Error("参考图片地址无效");
            address = url.href;
            break;
        }
        if (!address && policy?.format === "url") {
            const file = dataUrlToFile({ ...reference, dataUrl: await referenceDataUrl(reference) });
            if (useUserStore.getState().token !== token) throw new Error("登录状态已变化");
            const body = new FormData(); body.append("file", file);
            const response = await fetch("/api/v1/files", { method: "POST", headers: { Authorization: `Bearer ${token}` }, body });
            const payload = await response.json();
            if (!response.ok || payload.code !== 0 || !payload.data?.url) throw new Error(payload.msg || "参考图片上传失败");
            address = payload.data.url;
        }
        if (useUserStore.getState().token !== token) throw new Error("登录状态已变化");
        result.push(address ? await authorizeReferenceURL(address) : await referenceDataUrl(reference));
    }
    if (useUserStore.getState().token !== token) throw new Error("登录状态已变化");
    return result;
}
