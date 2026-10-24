import type { Icon } from "../shared/dom";

const line = (width: string) => ({
  fill: "none",
  stroke: "currentColor",
  "stroke-width": width,
  "stroke-linecap": "round",
  "stroke-linejoin": "round",
});

export const DOWN: Icon = {
  viewBox: "0 0 24 24",
  style: line("2.3"),
  shapes: [
    ["path", { d: "M12 4v11" }],
    ["path", { d: "m7 10 5 5 5-5" }],
    ["path", { d: "M5 20h14" }],
  ],
};

export const GRIP: Icon = {
  viewBox: "0 0 10 16",
  style: { fill: "currentColor" },
  shapes: [3, 8, 13].flatMap((cy) =>
    [2.5, 7.5].map((cx): [string, Record<string, string>] => [
      "circle",
      { cx: String(cx), cy: String(cy), r: "1.4" },
    ]),
  ),
};

export const CLOSE: Icon = {
  viewBox: "0 0 24 24",
  style: line("2.2"),
  shapes: [["path", { d: "M6 6l12 12M18 6 6 18" }]],
};

export const MINIMIZE: Icon = {
  viewBox: "0 0 24 24",
  style: line("2.2"),
  shapes: [["path", { d: "M6 12h12" }]],
};

export const FILM: Icon = {
  viewBox: "0 0 24 24",
  style: line("2"),
  shapes: [
    ["rect", { x: "3", y: "5", width: "18", height: "14", rx: "2.5" }],
    ["path", { d: "M10 9.3v5.4l4.6-2.7z" }],
  ],
};

export const AUDIO: Icon = {
  viewBox: "0 0 24 24",
  style: line("2"),
  shapes: [
    ["path", { d: "M9 17V5.5l11-2V15" }],
    ["circle", { cx: "6.5", cy: "17", r: "2.5" }],
    ["circle", { cx: "17.5", cy: "15", r: "2.5" }],
  ],
};

export const STREAM: Icon = {
  viewBox: "0 0 24 24",
  style: line("2"),
  shapes: [["path", { d: "M4 7h16M4 12h16M4 17h10" }]],
};

export const LIST: Icon = {
  viewBox: "0 0 24 24",
  style: line("2"),
  shapes: [
    ["path", { d: "M9 6h11M9 12h11M9 18h11" }],
    ...[6, 12, 18].map((cy): [string, Record<string, string>] => [
      "circle",
      { cx: "4.5", cy: String(cy), r: "1" },
    ]),
  ],
};

export const FOLLOW: Icon = {
  viewBox: "0 0 24 24",
  style: line("2.2"),
  shapes: [
    ["path", { d: "M5 11a8 8 0 0 1 8 8M5 5a14 14 0 0 1 14 14" }],
    ["circle", { cx: "6", cy: "18", r: "1" }],
  ],
};

export const SCISSORS: Icon = {
  viewBox: "0 0 24 24",
  style: line("2"),
  shapes: [
    ["circle", { cx: "6", cy: "6", r: "3" }],
    ["circle", { cx: "6", cy: "18", r: "3" }],
    ["path", { d: "M20 4 8.1 15.9M14.5 14.5 20 20M8.1 8.1 12 12" }],
  ],
};

/** Start the clip here: a bracket opening to the right. */
export const MARK_IN: Icon = {
  viewBox: "0 0 24 24",
  style: line("2.2"),
  shapes: [
    ["path", { d: "M10 5H6v14h4" }],
    ["path", { d: "M13 12h6M16 9l3 3-3 3" }],
  ],
};

/** End the clip here: a bracket closing from the left. */
export const MARK_OUT: Icon = {
  viewBox: "0 0 24 24",
  style: line("2.2"),
  shapes: [
    ["path", { d: "M14 5h4v14h-4" }],
    ["path", { d: "M5 12h6M8 9l3 3-3 3" }],
  ],
};

export const PLAY: Icon = {
  viewBox: "0 0 24 24",
  style: { fill: "currentColor" },
  shapes: [
    [
      "path",
      { d: "M8 5.5v13a1 1 0 0 0 1.5.86l10.2-6.5a1 1 0 0 0 0-1.72L9.5 4.64A1 1 0 0 0 8 5.5z" },
    ],
  ],
};

export const PAUSE: Icon = {
  viewBox: "0 0 24 24",
  style: { fill: "currentColor" },
  shapes: [
    ["rect", { x: "6", y: "5", width: "4", height: "14", rx: "1" }],
    ["rect", { x: "14", y: "5", width: "4", height: "14", rx: "1" }],
  ],
};
