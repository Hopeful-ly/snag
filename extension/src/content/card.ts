// The card the Download button opens: the best quality up front, the rest below, then other files.
import type { Option } from "../media/offers";
import { ask } from "../shared/ask";
import { durationText, hostOf, sizeText } from "../shared/format";
import { button, el, icon, withChildren } from "../shared/dom";
import { ClipMode } from "./clip-mode";
import { HaveLookup, haveRow, quality, type HaveActions } from "./have";
import { AUDIO, DOWN, FILM, STREAM } from "./icons";
import { PageOffers } from "./page-offers";
import { pageTMDB, PlaylistRow } from "./playlist-row";
import { SubtitlePicker } from "./subtitles";
import type { Trim } from "./trim";

interface CardHost {
  title: HTMLElement;
  subtitle: HTMLElement;
  body: HTMLElement;
  foot: HTMLElement;
  video(): HTMLVideoElement | null;
  trim: Trim;
  done(): void;
  reopen(): void;
}

export class Card {
  open = false;

  private readonly offers: PageOffers;
  private readonly clips: ClipMode;
  private readonly subtitles: SubtitlePicker;
  private readonly have: HaveLookup;
  private readonly playlist: PlaylistRow;
  private failure = "";

  constructor(private host: CardHost) {
    const redraw = () => this.redraw();
    const video = () => host.video();
    this.offers = new PageOffers(video, redraw);
    this.clips = new ClipMode(host.trim, video, redraw);
    this.subtitles = new SubtitlePicker(redraw);
    this.have = new HaveLookup(redraw);
    this.playlist = new PlaylistRow(
      (follow) =>
        this.handOver(
          ask("playlist", { url: location.href, tmdb: pageTMDB(), subscribe: follow }).then(
            (found) => {
              if (!found) throw new Error("Snag found no list on this page any more");
            },
          ),
        ),
      redraw,
    );
  }

  prepare(): void {
    this.offers.prepare();
  }

  show(): void {
    this.open = true;
    this.failure = "";
    this.have.reset();
    this.subtitles.reset();
    this.offers.start();
    this.render();
  }

  hide(): void {
    this.open = false;
    this.clips.close();
    this.offers.stop();
  }

  private redraw(): void {
    if (this.open) this.render();
  }

  render(): void {
    const { title, subtitle, body, foot } = this.host;
    const { video, audio, files } = this.offers.options();
    const loading = this.offers.loading();
    const found = video.length + audio.length + files.length > 0;
    title.textContent = this.offers.title();
    const first = video[0] || audio[0] || files[0];
    subtitle.textContent = first ? this.savesAs(first) : "Drag here to move";
    body.replaceChildren();
    foot.replaceChildren();

    if (!found && loading) {
      body.append(withChildren(el("div", "loading"), el("span", "spin"), "Looking for qualities…"));
    } else if (!found) {
      body.append(
        el(
          "div",
          "note",
          "Nothing to download found yet. If the video is playing, give it a few seconds: streams show up as the page loads them.",
        ),
      );
      if (this.offers.probe.error) body.append(el("div", "error", this.offers.probe.error));
    }
    // On a list's page the rest of the list may be what's wanted, so it comes first.
    body.append(
      ...this.playlist.render(this.offers.probe.result?.playlist, !this.offers.probe.loading),
    );
    if (found) body.append(...this.clips.render());
    if (video.length) {
      const duration = this.offers.duration();
      const heading = ["Video", duration ? durationText(duration) : "", hostOf(video[0]!.item.url)];
      body.append(el("div", "label", heading.filter(Boolean).join(" · ")));
      body.append(withChildren(el("div", "rows"), ...video.map((o) => this.optionRow(o))));
      body.append(...this.subtitles.render(this.offers.captions()));
    }
    if (audio.length) {
      body.append(el("div", "label", "Audio"));
      body.append(withChildren(el("div", "rows"), ...audio.map((o) => this.optionRow(o))));
    }
    if (loading && found) {
      body.append(withChildren(el("div", "more"), el("span", "spin"), "Finding more qualities…"));
    }
    if (files.length) {
      body.append(el("div", "label", "Other files"));
      for (const f of files) body.append(this.fileRow(f));
    }
    this.lookupHave([...video, ...audio, ...files]);
    if (this.failure) body.append(el("div", "error", this.failure));

    const manager = el("button", "link", "Open Snag");
    manager.addEventListener("click", () => ask("show").catch((e: Error) => this.fail(e.message)));
    foot.append(el("span", "grow", ""), manager);
  }

  private savedName(o: Option): string {
    const item = o.item;
    if (item.kind === "http" && item.filename) return item.filename;
    const base =
      this.offers.pageTitle || this.offers.probe.result?.title || document.title || "video";
    let ext = ".mp4";
    if (item.audio === "mp3") ext = ".mp3";
    else if (/audio/i.test(o.label)) ext = ".m4a";
    else if (item.kind === "http")
      ext = new URL(item.url).pathname.match(/\.[a-z0-9]{2,5}$/i)?.[0] || ".mp4";
    return (item.kind === "ytdlp" && item.title && !item.media ? item.title : base) + ext;
  }

  /** "Saves as Name.mp4"; a clip is named by the app, after the part it is. */
  private savesAs(o: Option): string {
    return (this.clips.clip ? "Saves a clip of " : "Saves as ") + this.savedName(o);
  }

  private describe(o: Option): string {
    return [o.label + " · " + o.detail, this.savesAs(o), "From: " + o.item.url].join("\n");
  }

  private optionRow(o: Option): HTMLElement {
    const had = this.have.get(o.key);
    if (had) return haveRow(o, had, "opt", this.haveActions(o));
    const words = o.label === "Audio" ? [o.format] : [o.name, o.format];
    if (o.best) words.push("Best");
    const b = button(
      "opt" + (o.best ? " top" : ""),
      this.describe(o),
      quality(o.label),
      el("span", "d", words.filter(Boolean).join(" · ")),
      el("span", "sz", o.size ? "~" + sizeText(this.clips.size(o.size)) : ""),
      withChildren(el("span", "go"), icon(DOWN)),
    );
    b.addEventListener("click", () => this.grab(o));
    return b;
  }

  private fileRow(o: Option): HTMLElement {
    const item = o.item;
    const stream = item.media === "hls" || item.media === "dash";
    const site = hostOf(item.url);
    const name = item.filename || (stream ? "Stream" : o.label + " file");
    const fileIcon = () =>
      withChildren(
        el("span", "ficon"),
        icon(item.media === "audio" ? AUDIO : stream ? STREAM : FILM),
      );
    const had = this.have.get(o.key);
    if (had) {
      const row = haveRow(o, had, "file", this.haveActions(o), el("span", "fname", name));
      row.prepend(fileIcon());
      return row;
    }
    const clipping = !!this.clips.clip;
    const size = sizeText(item.size || 0);
    const facts = o.detail
      .split(" · ")
      .filter((f) => f && f !== item.filename && f !== site && !(clipping && f === size));
    if (clipping && item.size) facts.push("~" + sizeText(this.clips.size(item.size)) + " clip");
    const b = button(
      "file",
      this.describe(o),
      fileIcon(),
      withChildren(
        el("span", "ftexts"),
        el("span", "fname", name),
        el("span", "fmeta", [...facts, site].filter(Boolean).join(" · ")),
      ),
      withChildren(el("span", "fgo"), icon(DOWN)),
    );
    b.addEventListener("click", () => this.grab(o));
    return b;
  }

  private haveActions(o: Option): HaveActions {
    return {
      title: this.describe(o),
      again: () => this.grab(o),
      failed: (m: string) => this.fail(m),
    };
  }

  /** A clip is always new, whatever the list holds. */
  private lookupHave(opts: Option[]): void {
    if (this.clips.clip) {
      this.have.reset();
      return;
    }
    this.have.lookup(
      opts.map((o) => ({
        key: o.key,
        item: {
          url: o.item.url,
          label: o.item.label || o.label,
          filename: this.savedName(o),
          format: o.item.format || "",
          audio: o.item.audio || "",
        },
      })),
    );
  }

  private grab(o: Option): void {
    const item = { ...o.item };
    this.clips.apply(item); // before the card closes, which drops the clip
    this.subtitles.apply(item, this.offers.captions());
    // Snag's window takes over: the Save window when it asks where to, else the list.
    this.handOver(
      ask("download", { item, referer: location.href }).then((d) => ask("show", { id: d.id })),
    );
  }

  private handOver(request: Promise<unknown>): void {
    this.host.done();
    request.catch((error: Error) => {
      this.host.reopen();
      this.fail(error.message);
    });
  }

  private fail(message: string): void {
    this.failure = message;
    this.redraw();
  }
}
