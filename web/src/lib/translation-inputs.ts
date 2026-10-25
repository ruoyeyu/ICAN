import { z } from "zod";

const option = z.object({ options: z.array(z.string().min(1)), allowCustom: z.boolean() }).strict()
    .refine(value => value.allowCustom || value.options.length > 0, "禁止自定义时必须列出选项")
    .refine(value => new Set(value.options).size === value.options.length, "选项不能重复");
const image = z.object({ format: z.enum(["auto", "url", "dataUrl"]), max: z.number().int().nonnegative().optional() }).strict();
const media = z.object({ format: z.enum(["auto", "url"]), max: z.number().int().nonnegative().optional() }).strict();
export const translationInputsSchema = z.object({
    version: z.literal(1), authentication: z.enum(["key", "none"]).optional(),
    parameters: z.object({ seconds: option.optional(), resolution: option.optional(), aspectRatio: option.optional(), mode: option.optional() }).strict(),
    media: z.object({ images: image.optional(), firstFrame: image.optional(), lastFrame: image.optional(), videos: media.optional(), audios: media.optional() }).strict(),
}).strict();
export type TranslationInputs = z.infer<typeof translationInputsSchema>;
export const emptyTranslationInputs = (): TranslationInputs => ({ version: 1, parameters: {}, media: {} });
