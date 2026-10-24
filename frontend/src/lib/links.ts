import type { Link } from "./api/client";

// In the order the window shows them.
export const groupNames: Record<string, string> = {
  archives: "Archives",
  programs: "Programs",
  videos: "Videos",
  audio: "Audio",
  documents: "Documents",
  images: "Images",
  videoPages: "Video pages",
  pages: "Web pages",
  other: "Other",
};

const fileGroups = new Set(["archives", "programs", "videos", "audio", "documents"]);

// Files start ticked.
export function isFile(l: Pick<Link, "group">): boolean {
  return fileGroups.has(l.group);
}

export function startingPicks(links: Pick<Link, "group">[], have: boolean[]): boolean[] {
  return links.map((l, i) => isFile(l) && !have[i]);
}

// Every typed word must appear in the name, words or address; * is a wildcard ("*.pdf").
export function matches(
  l: Pick<Link, "url" | "name" | "text" | "server">,
  filter: string,
): boolean {
  const words = filter.trim().toLowerCase().split(/\s+/).filter(Boolean);
  if (!words.length) return true;
  const name = (l.server || l.name).toLowerCase();
  const all = [name, l.name.toLowerCase(), (l.text ?? "").toLowerCase(), l.url.toLowerCase()];
  return words.every((word) => {
    if (!word.includes("*")) return all.some((text) => text.includes(word));
    const pattern = new RegExp(
      "^" +
        word
          .split("*")
          .map((part) => part.replace(/[.+?^${}()|[\]\\]/g, "\\$&"))
          .join(".*") +
        "$",
    );
    return pattern.test(name) || pattern.test(l.name.toLowerCase());
  });
}
