import { tokenDanceAppUrl } from "@/lib/model-channel";

export type TokenDanceOAuthContext =
    | {
        target: "local";
        channelId: string;
    }
    | {
        target: "admin";
        draft: Record<string, unknown>;
        editingChannelIndex: number | null;
    };

export type TokenDanceOAuthState = TokenDanceOAuthContext & {
    verifier: string;
    returnTo: string;
};

export async function startTokenDanceOAuth(..._args: unknown[]): Promise<void> {
    throw new Error("此部署仅支持自行配置 API 地址与密钥");
}
