import { api } from "./browser";
import type { Answer, Command, CommandName, Commands } from "./protocol";

/** Said by a page's script once the extension was updated or reloaded under it. */
const STALE = "Snag was updated. Reload this page to use it.";

/** Whether this script can still reach the extension (an update leaves old copies cut off). */
export function alive(): boolean {
  try {
    return !!api.runtime?.id;
  } catch {
    return false;
  }
}

type Args<C extends CommandName> = Commands[C]["args"];

/** tabId defaults to the sender's tab. */
export async function ask<C extends CommandName>(
  cmd: C,
  args?: Args<C>,
  tabId?: number,
): Promise<Commands[C]["result"]> {
  if (!alive()) throw new Error(STALE);
  const command = { ...args, cmd, ...(tabId != null && { tabId }) } as Command;
  let reply: Answer<Commands[C]["result"]> | undefined;
  try {
    reply = await api.runtime.sendMessage(command);
  } catch (error) {
    const text = String((error as Error)?.message || error);
    throw new Error(/invalidated/i.test(text) ? STALE : text, { cause: error });
  }
  if (!reply) throw new Error("Snag's extension didn't answer");
  if (!reply.ok) throw new Error(reply.error);
  return reply.value;
}
