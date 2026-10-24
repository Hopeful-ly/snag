// The browser's extension API: Firefox's `browser`, else Chrome's `chrome`. Both have promise-based
// calls for everything Snag uses, so Chrome's types serve for both.
export const api: typeof chrome = (globalThis as { browser?: typeof chrome }).browser ?? chrome;
