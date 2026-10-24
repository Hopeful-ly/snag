import { api } from "../shared/browser";

/** The toolbar button: `action` in Manifest 3 (Chrome), `browserAction` in Firefox's Manifest 2. */
export const action: typeof chrome.action =
  api.action ?? (api as unknown as { browserAction: typeof chrome.action }).browserAction;

export function setCount(tabId: number, count: number): void {
  action.setBadgeText({ tabId, text: count ? String(count) : "" })?.catch?.(() => {});
}

/** For a few seconds; the details go to the console. */
export function showError(error: unknown): void {
  console.warn("snag:", error);
  action.setBadgeText({ text: "!" })?.catch?.(() => {});
  action.setBadgeBackgroundColor({ color: "#c62828" })?.catch?.(() => {});
  setTimeout(() => action.setBadgeText({ text: "" })?.catch?.(() => {}), 4000);
}
