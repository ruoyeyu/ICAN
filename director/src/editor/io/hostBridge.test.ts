import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { clearDirectorDeskHostBridge, initDirectorDeskHostBridge } from "./hostBridge";
import { createDefaultDirectorProject, useDirectorStore } from "../store/directorStore";
function send(type: string, payload: unknown, source: MessageEventSource | null = window.parent, origin = window.location.origin) {
  window.dispatchEvent(new MessageEvent("message", { source, origin, data: { type: `storyai:director-${type}`, payload } }));
}
beforeEach(() => { useDirectorStore.getState().replaceProject(createDefaultDirectorProject()); initDirectorDeskHostBridge(); });
afterEach(() => { clearDirectorDeskHostBridge(); vi.restoreAllMocks(); });
it("accepts only the exact parent and its origin", () => {
  send("session", { theme: "light" }, null);
  expect(document.documentElement.dataset.theme).toBe("dark");
  send("session", { theme: "light" }, window.parent, "https://untrusted.example");
  expect(document.documentElement.dataset.theme).toBe("dark");
  send("session", { theme: "light" });
  expect(document.documentElement.dataset.theme).toBe("light");
});
it("restores a host scene and sends subsequent edits back", () => {
  const project = createDefaultDirectorProject(); project.scene.backgroundColor = "#123456";
  send("session", { instanceId: "card-a", project });
  expect(useDirectorStore.getState().project.scene.backgroundColor).toBe("#123456");
  const post = vi.spyOn(window.parent, "postMessage").mockImplementation(() => {});
  useDirectorStore.getState().updateScene({ backgroundColor: "#abcdef" });
  expect(post).toHaveBeenCalledWith(expect.objectContaining({ type: "storyai:director-project-changed" }), window.location.origin);
});
it("rejects invalid legacy scenes without sending replacement data", () => {
  const post = vi.spyOn(window.parent, "postMessage").mockImplementation(() => {});
  send("session", { project: { version: 999 } });
  useDirectorStore.getState().updateScene({ backgroundColor: "#abcdef" });
  expect(post).toHaveBeenCalledWith(expect.objectContaining({ type: "storyai:director-error" }), window.location.origin);
  expect(post).not.toHaveBeenCalledWith(expect.objectContaining({ type: "storyai:director-project-changed" }), expect.anything());
});
it("rejects remote and active panorama URLs and deduplicates pixel data", () => {
  send("session", {});
  for (const imageUrl of ["http://127.0.0.1/private", "https://example.com/image", "data:image/svg+xml;base64,PHN2Zz4="]) {
    send("panoramas", { panoramas: [{ imageUrl }] });
    expect(useDirectorStore.getState().project.panoramaAssetId).toBeNull();
  }
  const payload = { panoramas: [{ edgeId: "edge", sourceNodeId: "image", imageUrl: "data:image/png;base64,aGVsbG8=", fileName: "test.png" }] };
  send("panoramas", payload);
  const id = useDirectorStore.getState().project.panoramaAssetId;
  expect(id).toBeTruthy(); send("panoramas", payload);
  expect(useDirectorStore.getState().project.panoramaAssetId).toBe(id);
  const post = vi.spyOn(window.parent, "postMessage").mockImplementation(() => {});
  useDirectorStore.getState().removePanoramaAsset();
  expect(post).toHaveBeenCalledWith({ type: "storyai:director-panorama-removed", payload: { edgeId: "edge", sourceNodeId: "image" } }, window.location.origin);
});
