import { getContext, setContext } from "svelte";
import type { Snag } from "./snag.svelte";

const key = Symbol("snag");

export function provideSnag(snag: Snag): void {
  setContext(key, snag);
}

export function useSnag(): Snag {
  const snag = getContext<Snag | undefined>(key);
  if (!snag) throw new Error("no Snag in context");
  return snag;
}
