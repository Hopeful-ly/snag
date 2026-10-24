import { describe, expect, it } from "vitest";
import { pageFeeds } from "../src/media/feeds";

function page(head: string, base = "https://show.example/episodes/"): Document {
  const doc = document.implementation.createHTMLDocument("t");
  const b = doc.createElement("base");
  b.href = base;
  doc.head.append(b);
  doc.head.insertAdjacentHTML("beforeend", head);
  return doc;
}

describe("a page's feeds", () => {
  it("finds RSS and Atom feeds, made absolute, once each", () => {
    const doc = page(`
      <link rel="alternate" type="application/rss+xml" title="Podcast" href="/feed/podcast">
      <link rel="Alternate" type="application/atom+xml; charset=utf-8" href="atom.xml">
      <link rel="alternate" type="application/rss+xml" href="/feed/podcast#top">
    `);
    expect(pageFeeds(doc)).toEqual([
      "https://show.example/feed/podcast",
      "https://show.example/episodes/atom.xml",
    ]);
  });

  it("passes over what isn't a feed", () => {
    const doc = page(`
      <link rel="alternate" hreflang="fr" type="text/html" href="/fr/">
      <link rel="alternate" type="application/json" href="/feed.json">
      <link rel="stylesheet" type="application/rss+xml" href="/odd.rss">
      <link rel="alternate" type="application/rss+xml" href="javascript:alert(1)">
      <link rel="alternate" type="application/rss+xml">
    `);
    expect(pageFeeds(doc)).toEqual([]);
  });
});
