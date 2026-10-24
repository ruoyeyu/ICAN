import { expect, it } from "vitest";
import { parseProject } from "./importProjectJson";
import { createDefaultDirectorProject } from "../store/directorStore";
it("round trips scenes, strips unknown fields and rejects non-finite transforms", () => {
  const scene = createDefaultDirectorProject();
  expect(parseProject(JSON.stringify({ ...scene, apiKey: "not-forwarded" }))).toEqual(scene);
  scene.objects[0].transform.position[0] = Infinity;
  expect(() => parseProject(JSON.stringify(scene))).toThrow();
});
it("rejects internal, remote, blob and active-content asset references", () => {
  for (const url of ["http://169.254.169.254/", "https://example.com/image.png", "blob:https://example.com/id", "data:text/html;base64,PHNjcmlwdD4=", "data:image/svg+xml;base64,PHN2Zz4="]) {
    const scene = createDefaultDirectorProject();
    scene.assets.push({ id: "test", kind: "panorama", sourceType: "image", fileName: "image.png", url });
    expect(() => parseProject(JSON.stringify(scene))).toThrow();
  }
});
