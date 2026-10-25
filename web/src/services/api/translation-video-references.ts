import type { TranslationInputs } from "@/lib/translation-inputs";
import { resolveMediaUrl } from "@/services/file-storage";
import { useUserStore } from "@/stores/use-user-store";
import type { ReferenceAudio, ReferenceVideo } from "@/types/media";
import type { ReferenceImage } from "@/types/image";
import { authorizeReferenceURL } from "./storage";
import { prepareTranslationImages } from "./translation-images";

function checkAddress(value: string) {
    const url = new URL(value, window.location.origin);
    if (!["http:", "https:"].includes(url.protocol) || url.username || url.password) throw new Error("素材必须使用 HTTP(S) URL");
    if (url.origin === window.location.origin && !/^\/api\/files\/[^/]+\/content$/.test(url.pathname)) throw new Error("站内素材需要受控文件地址，不能提交需浏览器登录的路径");
    return url.href;
}
export async function prepareVideoImageInput(refs: ReferenceImage[], policy: NonNullable<TranslationInputs["media"]["images"]>) {
    if (policy.format === "url") for (const ref of refs) for (const value of [ref.url, ref.dataUrl]) {
        if (value && !value.startsWith("data:") && !value.startsWith("blob:")) checkAddress(value);
    }
    return prepareTranslationImages(refs, policy);
}
export async function prepareVideoMediaInput(refs: (ReferenceVideo | ReferenceAudio)[], policy: NonNullable<TranslationInputs["media"]["videos"]>) {
    if (policy.max !== undefined && refs.length > policy.max) throw new Error("参考素材数量超过此模型声明的范围");
    const token = useUserStore.getState().token;
    if (!token || !useUserStore.getState().user) throw new Error("请先登录");
    const checkSession = () => { if (useUserStore.getState().token !== token) throw new Error("登录状态已变化"); };
    const result: string[] = [];
    for (const ref of refs) {
        checkSession();
        let address = await resolveMediaUrl(ref.storageKey, ref.url);
        if (address.startsWith("blob:") || address.startsWith("data:")) {
            const response = await fetch(address);
            if (!response.ok) throw new Error("无法读取参考素材");
            const blob = await response.blob();
            const body = new FormData(); body.append("file", new File([blob], ref.name || "reference", { type: blob.type || ref.type }));
            checkSession();
            const uploaded = await fetch("/api/v1/files", { method: "POST", headers: { Authorization: `Bearer ${token}` }, body });
            const payload = await uploaded.json();
            if (!uploaded.ok || payload.code !== 0 || !payload.data?.url) throw new Error(payload.msg || "素材上传失败");
            address = payload.data.url;
        }
        checkSession();
        result.push(await authorizeReferenceURL(checkAddress(address)));
    }
    checkSession(); return result;
}
