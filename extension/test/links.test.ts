import { afterEach, describe, expect, it } from "vitest";
import { collectLinks } from "../src/media/links";

function show(html: string) {
  document.body.innerHTML = html;
}

afterEach(() => {
  document.getSelection()?.removeAllRanges();
  document.body.innerHTML = "";
});

describe("collecting a page's links", () => {
  it("finds links, images, videos and sounds, each once", () => {
    show(`
      <a href="https://example.org/a.zip">The <b>tool</b></a>
      <a href="https://example.org/a.zip">again</a>
      <a href="https://example.org/get?id=1" download="Manual.pdf">Manual</a>
      <a href="mailto:me@example.org">Mail</a>
      <a href="javascript:void(0)">Nothing</a>
      <a href="https://example.org/pic"><img src="https://example.org/thumb.jpg" alt="A picture"></a>
      <video src="https://example.org/v.mp4"></video>
      <audio><source src="https://example.org/s.mp3"></audio>`);
    expect(collectLinks(false)).toEqual([
      { url: "https://example.org/a.zip", text: "The tool" },
      { url: "https://example.org/get?id=1", text: "Manual", name: "Manual.pdf" },
      { url: "https://example.org/pic", text: "A picture" },
      { url: "https://example.org/thumb.jpg", text: "A picture" },
      { url: "https://example.org/v.mp4" },
      { url: "https://example.org/s.mp3" },
    ]);
  });

  it("finds only the selected part's links", () => {
    show(`<p id="one"><a href="https://example.org/1.zip">one</a></p>
      <p id="two"><a href="https://example.org/2.zip">two</a> <a href="https://example.org/3.zip">three</a></p>`);
    const range = document.createRange();
    range.selectNodeContents(document.getElementById("two")!);
    document.getSelection()!.addRange(range);
    expect(collectLinks(true).map((l) => l.url)).toEqual([
      "https://example.org/2.zip",
      "https://example.org/3.zip",
    ]);
  });

  it("finds nothing in a frame with nothing selected", () => {
    show(`<a href="https://example.org/1.zip">one</a>`);
    expect(collectLinks(true)).toEqual([]);
  });
});
