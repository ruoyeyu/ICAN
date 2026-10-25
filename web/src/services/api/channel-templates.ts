import { apiDelete, apiGet, apiPost } from "./request";
import { importChannelTemplate, type ChannelTemplate } from "@/lib/channel-template";

export type ChannelTemplateEntry = { id: string; name: string; models: string[]; revision: string; template?: ChannelTemplate };
export type ChannelTemplateCatalog = { items: ChannelTemplateEntry[]; errors?: string[] };
const path = (admin: boolean) => `/api/${admin ? "admin" : "v1"}/channel-templates`;
export const fetchChannelTemplates = (token: string, admin = false) => apiGet<ChannelTemplateCatalog>(path(admin), undefined, token);
export async function fetchChannelTemplate(token: string, id: string, admin = false) {
    const entry = await apiGet<ChannelTemplateEntry>(`${path(admin)}/${encodeURIComponent(id)}`, undefined, token);
    return { ...entry, template: importChannelTemplate(JSON.stringify(entry.template)) };
}
export const saveChannelTemplate = (token: string, template: ChannelTemplate, id = "", revision = "") => apiPost<ChannelTemplateEntry>(`${path(true)}${id ? `/${encodeURIComponent(id)}` : ""}`, { template, revision }, token);
export const deleteChannelTemplate = (token: string, entry: ChannelTemplateEntry) => apiDelete(`${path(true)}/${encodeURIComponent(entry.id)}`, token, { "If-Match": entry.revision });
