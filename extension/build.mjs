// Builds the extension into build/, which the Go app embeds with manifests/ (see extension.go).
//
//   node build.mjs           once
//   node build.mjs --watch   rebuild on change
import * as esbuild from "esbuild";
import { copyFile, mkdir, readdir, rm } from "node:fs/promises";
import { fileURLToPath } from "node:url";

const out = new URL("./build/", import.meta.url);

// Firefox 115 and Chrome 111 are the oldest the manifests accept.
const options = {
  entryPoints: {
    background: "src/background/index.ts",
    content: "src/content/index.ts",
    shown: "src/page/shown.ts",
  },
  outdir: fileURLToPath(out), // a URL's path is no file path on Windows ("/D:/...")
  bundle: true,
  format: "iife",
  target: ["chrome111", "firefox115"],
  loader: { ".css": "text" },
  // Kept readable: Mozilla reviews the code, and so may anyone curious about what it does.
  minify: false,
  logLevel: "info",
};

// Clear out earlier builds, but keep the placeholder git tracks so the Go embed always finds the folder.
await mkdir(out, { recursive: true });
for (const name of await readdir(out)) {
  if (name !== ".gitkeep") await rm(new URL(name, out), { recursive: true });
}

async function copyIcons() {
  // The toolbar paints a green/red dot on these.
  for (const n of [16, 32, 48, 128]) {
    await copyFile(
      new URL(`../packaging/icons/snag-${n}.png`, import.meta.url),
      new URL(`icon-${n}.png`, out),
    );
  }
}

if (process.argv.includes("--watch")) {
  await copyIcons();
  const context = await esbuild.context(options);
  await context.watch();
} else {
  await esbuild.build(options);
  await copyIcons();
}
