import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

export default defineConfig(({ command }) => ({
  plugins: [react()],
  define: command === "build" ? { "process.env.NODE_ENV": JSON.stringify("production") } : {},
  build: {
    lib: { entry: "src/main.tsx", name: "ICANDirector", formats: ["iife"], fileName: () => "director.js", cssFileName: "director" },
    rollupOptions: { output: { inlineDynamicImports: true } },
  },
  test: { environment: "jsdom", globals: true, pool: "threads", maxWorkers: 1, setupFiles: "./src/test/setup.ts" },
}));
