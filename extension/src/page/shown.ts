// Runs in the page's own world, before its scripts, in a background tab Snag opened. A tab never
// shown gets no animation frames, and some players wait for those or for visibility before
// fetching their stream, so the page is told it is visible and frames come from timers.

/** A page already seen is left alone. */
export function actShown(page: Window & typeof globalThis): void {
  if (page.document.visibilityState !== "hidden") return;
  const proto = page.Document.prototype;
  Object.defineProperty(proto, "hidden", { configurable: true, get: () => false });
  Object.defineProperty(proto, "visibilityState", { configurable: true, get: () => "visible" });
  page.requestAnimationFrame = (draw) => page.setTimeout(() => draw(page.performance.now()), 16);
  page.cancelAnimationFrame = (id) => page.clearTimeout(id);
}

actShown(window);
