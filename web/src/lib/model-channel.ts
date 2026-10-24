export const modelChannelProtocols = [
    { value: "custom", label: "自定义 API（原样参数）", baseUrl: "" },
    { value: "openai", label: "OpenAI 兼容 API", baseUrl: "" },
    { value: "gemini", label: "Gemini", baseUrl: "" },
    { value: "grok2api", label: "Grok2API", baseUrl: "" },
    { value: "minimax", label: "MiniMax 协议", baseUrl: "" },
    { value: "apimart", label: "APIMart", baseUrl: "", directRequestPlan: true },
    { value: "88api", label: "88API", baseUrl: "" },
    { value: "tokendance", label: "TokenDance", baseUrl: "", directRequestPlan: true },
    { value: "runninghub", label: "RunningHub", baseUrl: "" },
    { value: "comfyui", label: "ComfyUI", baseUrl: "" },
    { value: "autodl", label: "AutoDL", baseUrl: "", directRequestPlan: true },
    { value: "ark", label: "火山方舟", baseUrl: "", directRequestPlan: true },
    { value: "kie", label: "KIE", baseUrl: "", directRequestPlan: true },
    { value: "mimo", label: "MiMo", baseUrl: "" },
] as const;

export type ModelChannelProtocol = (typeof modelChannelProtocols)[number]["value"];
export type DirectAIProvider = Extract<(typeof modelChannelProtocols)[number], { directRequestPlan: true }>["value"];
export const modelChannelProtocolOptions = modelChannelProtocols.filter(({value}) => ["custom","openai","gemini"].includes(value)).map(({ value, label }) => ({ label, value }));
export const modelChannelDefaultBaseUrls = Object.fromEntries(modelChannelProtocols.map(({ value, baseUrl }) => [value, baseUrl])) as Record<ModelChannelProtocol, string>;
export const modelChannelApiKeyUrls: Partial<Record<ModelChannelProtocol,string>> = {};

export const tokenDanceAppUrl = "";

export function modelChannelAttributionHeaders(protocol: string) {
    return {};
}

const directRequestProviders: ReadonlySet<string> = new Set(modelChannelProtocols.flatMap((protocol) => "directRequestPlan" in protocol && protocol.directRequestPlan === true ? [protocol.value] : []));

export function directAIProviderForProtocol(protocol: string): DirectAIProvider | null {
    return directRequestProviders.has(protocol) ? protocol as DirectAIProvider : null;
}

export function isWorkflowProtocol(protocol: string): protocol is "runninghub" | "comfyui" {
    return protocol === "runninghub" || protocol === "comfyui";
}
