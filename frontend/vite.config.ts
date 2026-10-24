/// <reference types="vitest/config" />
import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";

export default defineConfig({
  plugins: [svelte()],
  server: { port: 9245, strictPort: true },
  build: {
    rollupOptions: { input: { main: "index.html", remote: "remote.html" } },
  },
  // Test components against Svelte's browser build.
  resolve: process.env.VITEST ? { conditions: ["browser"] } : undefined,
  test: {
    environment: "jsdom",
    setupFiles: ["src/test/setup.ts"],
  },
});
