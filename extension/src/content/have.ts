// Offers Open / Show in folder / Download again for options Snag already has.
import type { Option } from "../media/offers";
import { ask } from "../shared/ask";
import { button, el, withChildren } from "../shared/dom";
import type { Download, HaveItem } from "../shared/protocol";

export class HaveLookup {
  private have: Record<string, Download | null> = {}; // option key -> what Snag has
  private asked = "";

  constructor(private changed: () => void) {}

  reset(): void {
    this.have = {};
    this.asked = "";
  }

  get(key: string): Download | undefined {
    return this.have[key] ?? undefined;
  }

  /** Skipped when it was just asked about the same ones. */
  lookup(options: { key: string; item: HaveItem }[]): void {
    const items = options.map((o) => o.item);
    const asked = JSON.stringify(items);
    if (asked === this.asked) return;
    this.asked = asked;
    if (!items.length) {
      this.have = {};
      return;
    }
    ask("have", { items }).then(
      (list) => {
        if (this.asked !== asked) return;
        this.have = Object.fromEntries(options.map((o, i) => [o.key, list[i] ?? null]));
        this.changed();
      },
      () => {
        if (this.asked === asked) this.have = {};
      },
    );
  }
}

export interface HaveActions {
  title: string;
  again(): void;
  failed(message: string): void;
}

export function haveRow(
  o: Option,
  d: Download,
  kind: "opt" | "file",
  actions: HaveActions,
  heading?: HTMLElement,
): HTMLElement {
  const control = (action: "open" | "reveal") =>
    ask("control", { action, id: d.id }).catch((e: Error) => actions.failed(e.message));
  const acts = el("div", "acts");
  if (d.status === "done") acts.append(action("Open", "Open the file", () => control("open")));
  acts.append(
    action("Folder", "Show in folder", () => control("reveal")),
    action("Again", "Download again", actions.again),
  );
  const main = withChildren(
    el("span", "have-main"),
    ...(heading ? [heading] : []),
    el("span", kind === "opt" ? "d" : "fmeta", haveText(d)),
    acts,
  );
  const row =
    kind === "opt"
      ? withChildren(el("div", "opt have"), quality(o.label), main)
      : withChildren(el("div", "file have"), main);
  row.title = actions.title;
  return row;
}

/** A quality's name, in figures when it is one ("1080p"). */
export function quality(label: string): HTMLElement {
  const q = el("span", "q", label);
  if (/^\d+p/.test(label)) q.classList.add("mono");
  return q;
}

function action(label: string, title: string, onClick: () => void): HTMLButtonElement {
  const b = button("act", title, label);
  b.addEventListener("click", (e) => {
    e.stopPropagation();
    onClick();
  });
  return b;
}

function haveText(d: Download): string {
  if (d.status === "done") return "You have this";
  if (d.status === "error") return "Failed in Snag";
  if (d.status === "paused" || d.status === "pending") return "In Snag";
  return "Downloading";
}
