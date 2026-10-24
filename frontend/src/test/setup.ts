import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/svelte";
import { afterEach } from "vitest";

afterEach(cleanup);

// jsdom has no web animations, which Svelte's transitions use: finish them at once.
Element.prototype.animate ??= function () {
  const animation = {
    onfinish: null as null | (() => void),
    finished: Promise.resolve(),
    cancel() {},
    play() {},
    pause() {},
    reverse() {},
    finish() {},
    currentTime: 0,
    playState: "finished",
    effect: { getComputedTiming: () => ({ progress: 1 }) },
  };
  queueMicrotask(() => animation.onfinish?.());
  return animation as unknown as Animation;
};
Element.prototype.getAnimations ??= () => [];
Element.prototype.scrollIntoView ??= () => {};

// Nor does it lay anything out: resizeAll fires every ResizeObserver, as a layout change would.
const watchers = new Set<{ fire: () => void }>();
export function resizeAll() {
  for (const w of watchers) w.fire();
}
afterEach(() => watchers.clear());
globalThis.ResizeObserver = class {
  private targets: Element[] = [];
  private watcher = { fire: () => this.callback([], this as unknown as ResizeObserver) };
  constructor(private callback: ResizeObserverCallback) {}
  observe(target: Element) {
    this.targets.push(target);
    watchers.add(this.watcher);
    this.watcher.fire(); // a browser reports the size once on observing
  }
  unobserve() {}
  disconnect() {
    watchers.delete(this.watcher);
  }
};

// Nor drag events: a mouse event that carries the drag's data, so drags keep their position.
globalThis.DragEvent ??= class extends MouseEvent {
  dataTransfer: DataTransfer | null = null;
} as unknown as typeof DragEvent;
