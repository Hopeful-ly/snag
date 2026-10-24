import { api } from "../shared/browser";
import type { ExtensionSettings } from "../shared/protocol";

const DEFAULTS: ExtensionSettings = { overlay: true, intercept: true, minSize: 10 * 1024 * 1024 };

/** Filled in once ready resolves. */
export const settings: ExtensionSettings = { ...DEFAULTS };

export const settingsReady: Promise<void> = api.storage.local
  .get(DEFAULTS as unknown as Record<string, unknown>)
  .then((saved) => void Object.assign(settings, saved));

api.storage.onChanged.addListener((changes, area) => {
  if (area !== "local") return;
  for (const [key, { newValue }] of Object.entries(changes)) {
    if (key in DEFAULTS) Object.assign(settings, { [key]: newValue });
  }
});
