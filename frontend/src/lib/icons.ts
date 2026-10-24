// Line icons on a 24-unit grid, each one stroked path (see Icon.svelte), in the spirit of Lucide.

const circle = (cx: number, cy: number, r: number) =>
  `M${cx - r} ${cy}a${r} ${r} 0 1 0 ${2 * r} 0a${r} ${r} 0 1 0 ${-2 * r} 0`;

const roundedRect = (x: number, y: number, w: number, h: number, r: number) =>
  `M${x + r} ${y}h${w - 2 * r}a${r} ${r} 0 0 1 ${r} ${r}v${h - 2 * r}a${r} ${r} 0 0 1 ${-r} ${r}` +
  `h${-(w - 2 * r)}a${r} ${r} 0 0 1 ${-r} ${-r}v${-(h - 2 * r)}a${r} ${r} 0 0 1 ${r} ${-r}z`;

const dot = (x: number, y: number) => `M${x} ${y}h0.01`;

export const icons = {
  plus: "M12 5v14M5 12h14",
  subscribe: "M5 11a8 8 0 0 1 8 8M5 5a14 14 0 0 1 14 14" + dot(6, 18),
  back: "M19 12H5M11 18l-6-6 6-6",
  minus: "M5 12h14",
  pause: "M8.5 5.5v13M15.5 5.5v13",
  play: "M7.5 5.2v13.6a.6.6 0 0 0 .9.5l10.6-6.8a.6.6 0 0 0 0-1L8.4 4.7a.6.6 0 0 0-.9.5z",
  x: "M18 6 6 18M6 6l12 12",
  trash: "M4 7h16M10 11v6M14 11v6M6 7l1 12a2 2 0 0 0 2 2h6a2 2 0 0 0 2-2l1-12M9 7V4h6v3",
  folder: "M3 8a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z",
  open: "M14 4h6v6M20 4l-9 9M18 14v4a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4",
  retry: "M3.5 12a8.5 8.5 0 1 0 2.6-6.1M3.5 4v5h5",
  download: "M12 4v11M7 10l5 5 5-5M5 20h14",
  save: "M5 4h11l3 3v12a1 1 0 0 1-1 1H6a1 1 0 0 1-1-1zM8 4v5h7V4M8 20v-6h8v6",
  settings:
    "M4 7h9M17 7h3M4 17h3M11 17h9M4 12h2M10 12h10" +
    circle(15, 7, 2) +
    circle(9, 17, 2) +
    circle(8, 12, 2),
  paste:
    roundedRect(8, 3, 8, 4, 1) +
    "M16 5h1a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V7a2 2 0 0 1 2-2h1",
  link: "M10 14a4.2 4.2 0 0 0 6 0l3-3a4.2 4.2 0 0 0-6-6l-1 1M14 10a4.2 4.2 0 0 0-6 0l-3 3a4.2 4.2 0 0 0 6 6l1-1",
  more: circle(12, 5, 1) + circle(12, 12, 1) + circle(12, 19, 1),
  check: "M5 12.5l4.5 4.5L19 7.5",
  list: "M9 6h11M9 12h11M9 18h11" + dot(4.5, 6) + dot(4.5, 12) + dot(4.5, 18),
  downloading: circle(12, 12, 9) + "M12 7.5v8.5M8.5 12.5 12 16l3.5-3.5",
  done: circle(12, 12, 9) + "M8 12.5l2.7 2.7L16.2 9.5",
  failed: circle(12, 12, 9) + "M12 7.5v5.5" + dot(12, 16.3),
  clock: circle(12, 12, 9) + "M12 7v5l3.2 2",
  video: roundedRect(3, 5, 18, 14, 2.5) + "M10 9.3v5.4l4.6-2.7z",
  music: "M9 17V5.5l11-2V15" + circle(6.5, 17, 2.5) + circle(17.5, 15, 2.5),
  archive: roundedRect(3, 4, 18, 4.5, 1) + "M5 8.5V19a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1V8.5M10 12.5h4",
  program: "M12 3 20 7.5v9L12 21 4 16.5v-9zM12 12l8-4.5M12 12v9M12 12 4 7.5",
  document: "M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8zM14 3v5h5M9 13h6M9 17h6",
  file: "M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8zM14 3v5h5",
  search: circle(11, 11, 7) + "M20.5 20.5 16 16",
  inbox: "M3 13h5l2 3h4l2-3h5M6 5h12l3 8v6a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1v-6z",
  clearDone: "M4 7h10M4 12h7M4 17h6M13.5 16l2.5 2.5 5-5.5",
  speed: "M5 18a8 8 0 1 1 14 0M12 14l3.5-5",
  queue: "M12 3 3 8l9 5 9-5zM3 13l9 5 9-5",
  grip: dot(9, 6) + dot(15, 6) + dot(9, 12) + dot(15, 12) + dot(9, 18) + dot(15, 18),
  power: "M12 3v8M6.4 6.4a8 8 0 1 0 11.2 0",
  front: "M5 4h14M12 20V9M7 13l5-5 5 5",
  up: "M12 19V5M6 11l6-6 6 6",
  down: "M12 5v14M6 13l6 6 6-6",
  reverse: "M7 4v13M4.5 14.5 7 17l2.5-2.5M17 20V7M14.5 9.5 17 7l2.5 2.5",
  stop: roundedRect(6, 6, 12, 12, 2),
} as const;

export type IconName = keyof typeof icons;
