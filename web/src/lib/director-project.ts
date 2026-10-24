import { z } from "zod";

// Data-only interchange. Remote URLs, SVG/HTML and non-finite transforms are rejected.
const number = z.number().finite();
const vector = z.tuple([number, number, number]);
const image = z.string().regex(/^data:image\/(?:png|jpeg|webp);base64,[A-Za-z0-9+/]+={0,2}$/);
const transform = z.object({ position: vector, rotation: vector, scale: vector });
const capture = z.object({ id: z.string(), index: number, name: z.string(), dataUrl: image });
export const directorProjectSchema = z.object({
    version: z.literal(1),
    scene: z.object({ scale: number, position: vector, rotation: vector, backgroundColor: z.string(), panoramaYaw: number, panoramaRadius: number, showLabels: z.boolean(), snapToGrid: z.boolean(), showGround: z.boolean(), groundOpacity: number, groundHeight: number }),
    assets: z.array(z.object({
        id: z.string(), kind: z.enum(["character", "scene", "prop", "panorama"]),
        sourceType: z.enum(["model", "image"]), fileName: z.string(), name: z.string().optional(),
        url: z.string(), assetSource: z.enum(["local", "library"]).optional(),
        projectionMode: z.enum(["equirectangular", "backdrop"]).optional(),
    }).refine(asset => asset.sourceType === "image" ? image.safeParse(asset.url).success :
        /\.(fbx|obj)$/i.test(asset.fileName) && /^data:[\w.+/-]*(?:;[\w=.+-]+)*;base64,[A-Za-z0-9+/]+={0,2}$/.test(asset.url))),
    objects: z.array(z.object({
        id: z.string(), name: z.string(), kind: z.enum(["character", "scene", "prop", "camera", "panorama"]),
        visible: z.boolean(), locked: z.boolean(), transform,
        bodyType: z.enum(["mannequin", "female", "broad", "muscular", "slim", "teen", "child", "chibi"]).optional(),
        color: z.string().optional(), assetRefId: z.string().optional(),
        geometryType: z.enum(["box", "sphere", "cylinder", "torus", "cone", "pyramid"]).optional(),
        crowdId: z.string().optional(), crowdLabel: z.string().optional(), linkedCameraId: z.string().nullable().optional(),
        characterRig: z.object({ rigType: z.enum(["mannequin", "ue4-mannequin", "mixamo", "vrm", "custom-humanoid"]), posePresetId: z.string().nullable(), controls: z.record(z.string(), number) }).optional(),
    })),
    cameras: z.array(z.object({ id: z.string(), name: z.string(), fov: number, transform, targetMode: z.enum(["manual", "object"]), targetObjectId: z.string().nullable().optional(), target: vector, lastCaptureUrl: image.nullable().optional(), captures: z.array(capture).optional() })),
    activeCameraId: z.string().nullable(), panoramaAssetId: z.string().nullable(),
});
