"use client";
import { useEffect, useState } from "react";
import { translationModelInputs, translationPrograms } from "@/lib/channel-parameter-translation";
import type { TranslationInputs } from "@/lib/translation-inputs";
import { channelIdForActiveModel, localChannelForActiveModel, type AiConfig } from "@/stores/use-config-store";
import { useChannelTranslationStore } from "@/stores/use-channel-translation-store";
import { useUserStore } from "@/stores/use-user-store";

export function useTranslationInputs(config: AiConfig, model: string) {
    const account = useUserStore(s => s.user?.id);
    const channel = config.channelMode === "remote" ? channelIdForActiveModel({ ...config, model }) : localChannelForActiveModel({ ...config, model })?.id;
    const source = useChannelTranslationStore(s => s.accounts[account || ""]?.[channel || ""]);
    const [result, setResult] = useState<{ key: string; configured: boolean; inputs?: TranslationInputs; error?: string }>();
    const key = JSON.stringify([account, channel, model, source]);
    useEffect(() => {
        if (!account || config.channelMode === "remote") return;
        let active = true;
        void (async () => {
            try {
                const text = (await useChannelTranslationStore.getState().load(account))[channel || ""] || "";
                const configured = !!(await translationPrograms(text))[model];
                const inputs = await translationModelInputs(text, model);
                if (active) setResult({ key, configured, inputs });
            } catch(error) { if (active) setResult({ key, configured: true, error: error instanceof Error ? error.message : "配置无效" }); }
        })();
        return () => { active = false; };
    }, [account, channel, model, source, key, config.channelMode]);
    if (config.channelMode === "remote") {
        const remote = config.publicChannels.find(c => c.id === channel);
        return { configured: !!remote?.parameterTranslationModels?.includes(model), inputs: remote?.parameterTranslationInputs?.[model] };
    }
    return result?.key === key ? result : { configured: !!source };
}
