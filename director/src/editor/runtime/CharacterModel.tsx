import type { CharacterRigState } from "../schema/directorProject";
import { PrimitiveMannequin } from "./PrimitiveMannequin";
import type { CharacterBodyType } from "./mannequin/bodyTypes";

export function CharacterModel({ bodyType, color, rigState }: {
  bodyType?: CharacterBodyType;
  color?: string;
  onLabelAnchorYChange?: (anchorY: number) => void;
  rigState?: CharacterRigState;
}) {
  return <PrimitiveMannequin bodyType={bodyType} color={color} rigState={rigState} />;
}
