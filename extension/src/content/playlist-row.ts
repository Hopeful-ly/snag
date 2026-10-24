// The card's row for the list the video or page is in: opens Snag's Download playlist window,
// or with Subscribe, opens it ready to follow the list.
import { ask } from "../shared/ask";
import { button, el, icon, withChildren } from "../shared/dom";
import { playlistText } from "../shared/format";
import { tmdbCopyIn } from "../media/resolve";
import type { Playlist } from "../shared/protocol";
import { DOWN, FOLLOW, LIST } from "./icons";

export class PlaylistRow {
  private askedFor = ""; // the page asked about, when the page check named no list
  private found: Playlist | null = null;

  constructor(
    private open: (follow: boolean) => void,
    private changed: () => void,
  ) {}

  /**
   * Once the page check finishes without a list, Snag is asked about the page itself (a streaming
   * site's season).
   */
  render(fromCheck: Playlist | null | undefined, checked: boolean): HTMLElement[] {
    if (!fromCheck && checked) this.lookup();
    const playlist = fromCheck || (this.askedFor === location.href ? this.found : null);
    if (!playlist) return [];
    const b = button(
      "file playlist",
      `Choose what to download of “${playlist.title || "this list"}” in Snag`,
      withChildren(el("span", "ficon"), icon(LIST)),
      withChildren(
        el("span", "ftexts"),
        el("span", "fname", playlist.title || "Playlist"),
        el("span", "fmeta", playlistText(playlist.count)),
      ),
      withChildren(el("span", "fgo"), icon(DOWN)),
    );
    b.addEventListener("click", () => this.open(false));
    const follow = button(
      "follow",
      `Subscribe: download what is new in “${playlist.title || "this list"}” as it comes`,
      icon(FOLLOW),
      el("span", "", "Subscribe"),
    );
    follow.addEventListener("click", () => this.open(true));
    return [withChildren(el("div", "prow"), b, follow)];
  }

  /** Once per page (sites change pages without loading one). */
  private lookup(): void {
    const page = location.href;
    if (this.askedFor === page) return;
    this.askedFor = page;
    this.found = null;
    ask("playlist", { url: page, open: false, tmdb: pageTMDB() }).then(
      (found) => {
        if (found && this.askedFor === page) {
          this.found = found;
          this.changed();
        }
      },
      () => {}, // no list to offer
    );
  }
}

/** From the page's own resource list, which includes cache hits the extension never sees. */
export function pageTMDB(): string {
  try {
    return tmdbCopyIn(performance.getEntriesByType("resource").map((e) => e.name));
  } catch {
    return "";
  }
}
