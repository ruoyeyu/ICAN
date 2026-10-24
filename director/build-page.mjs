import { writeFileSync } from "node:fs";
// A classic script can load in an opaque-origin iframe without granting same-origin access.
writeFileSync("dist/index.html", `<!doctype html><html lang="zh-CN" data-theme="dark"><head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>ICAN · 3D导演台</title><link rel="stylesheet" href="./director.css"></head><body><div id="root"></div><script src="./director.js"></script></body></html>`);

// Preserve notices for packages used while building/bundling the editor.
import { readdirSync, readFileSync, existsSync } from "node:fs";
const notices = [readFileSync("LICENSE", "utf8")];
function collect(directory) {
  for (const entry of readdirSync(directory, { withFileTypes: true })) {
    if (!entry.isDirectory() || entry.name.startsWith(".")) continue;
    const path = `${directory}/${entry.name}`;
    if (entry.name.startsWith("@")) { collect(path); continue; }
    if (!existsSync(`${path}/package.json`)) continue;
    const pkg = JSON.parse(readFileSync(`${path}/package.json`, "utf8"));
    for (const file of readdirSync(path)) {
      if (/^(licen[cs]e|notice|copying)(\.|$)/i.test(file)) {
        try { notices.push(`${pkg.name}@${pkg.version}\n${readFileSync(`${path}/${file}`, "utf8")}`); } catch { /* A license directory is not a text notice. */ }
      }
    }
    if (existsSync(`${path}/node_modules`)) collect(`${path}/node_modules`);
  }
}
collect("node_modules");
writeFileSync("dist/THIRD_PARTY_NOTICES.txt", notices.join("\n\n----------------\n\n"));
