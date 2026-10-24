import { directorProjectSchema } from "../../../../web/src/lib/director-project";
import type { DirectorProject } from "../schema/directorProject";
export function parseProject(json: string): DirectorProject {
  return directorProjectSchema.parse(JSON.parse(json));
}
