import { apiDelete, apiGet, apiPost } from "./request";

export type Invitation = { id: string; code?: string; createdAt: string; expiresAt: number; usedBy: string };

export const listInvitations = (token: string) => apiGet<Invitation[]>("/api/admin/invitations", undefined, token);
export const createInvitation = (expiresAt: number, token: string) => apiPost<{ invitation: Invitation; code: string }>("/api/admin/invitations", { expiresAt }, token);
export const revokeInvitation = (id: string, token: string) => apiDelete(`/api/admin/invitations/${encodeURIComponent(id)}`, token);
