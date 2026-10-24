import js from "@eslint/js";
import ts from "typescript-eslint";
import globals from "globals";

export default ts.config(
  js.configs.recommended,
  ...ts.configs.recommended,
  {
    languageOptions: { globals: { ...globals.browser, ...globals.webextensions } },
  },
  {
    files: ["build.mjs"],
    languageOptions: { globals: { ...globals.node } },
  },
  {
    // The page's own markup never goes in as HTML: everything is built from elements.
    rules: {
      "no-restricted-properties": [
        "error",
        { property: "innerHTML", message: "Build elements instead (see shared/dom.ts)." },
        { property: "outerHTML", message: "Build elements instead (see shared/dom.ts)." },
      ],
    },
  },
  {
    // Tests set up the pages they read.
    files: ["test/**"],
    rules: { "no-restricted-properties": "off" },
  },
  {
    ignores: ["build/", "node_modules/"],
  },
);
