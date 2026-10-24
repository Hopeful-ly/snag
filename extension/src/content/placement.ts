// Where the button and its card go on a video. Pure rectangle arithmetic, for tests.

/** Where on the video the button sits: 0..1 across the room left for it, from the top left. */
export interface Spot {
  fx: number;
  fy: number;
}

export interface Box {
  left: number;
  top: number;
  width: number;
  height: number;
}

interface Size {
  width: number;
  height: number;
}

interface Point {
  left: number;
  top: number;
}

export const DEFAULT_SPOT: Spot = { fx: 1, fy: 0 };
export const CARD_WIDTH = 352;
const MARGIN = 10; // from the video's edges
const SNAP = 0.06; // dropped this close to an edge (a fraction of the room): onto it
const EDGE = 4; // kept this far inside the window

export function roomFor(video: Box, pill: Size): Size {
  return {
    width: Math.max(0, video.width - pill.width - 2 * MARGIN),
    height: Math.max(0, video.height - pill.height - 2 * MARGIN),
  };
}

const clamp = (v: number, lo: number, hi: number) => Math.max(lo, Math.min(v, hi));

export function pillPosition(video: Box, pill: Size, spot: Spot, view: Size): Point {
  const room = roomFor(video, pill);
  return {
    left: Math.round(
      clamp(video.left + MARGIN + spot.fx * room.width, EDGE, view.width - pill.width - EDGE),
    ),
    top: Math.round(
      clamp(video.top + MARGIN + spot.fy * room.height, EDGE, view.height - pill.height - EDGE),
    ),
  };
}

/**
 * The card opens where the button is, growing toward the middle of the video: from the button's
 * right edge on the video's right half, and upward from its bottom on the lower half.
 */
export function cardPosition(
  video: Box,
  pill: Size,
  at: Point,
  cardHeight: number,
  view: Size,
): Point {
  const rightSide = at.left + pill.width / 2 > video.left + video.width / 2;
  const lowerHalf = at.top + pill.height / 2 > video.top + video.height / 2;
  const left = rightSide ? at.left + pill.width - CARD_WIDTH : at.left;
  const top = lowerHalf ? at.top + pill.height - cardHeight : at.top;
  return {
    left: clamp(left, EDGE, view.width - CARD_WIDTH - EDGE),
    top: clamp(top, EDGE, view.height - cardHeight - EDGE),
  };
}

export function dragged(from: Spot, dx: number, dy: number, room: Size): Spot {
  return {
    fx: room.width ? clamp(from.fx + dx / room.width, 0, 1) : from.fx,
    fy: room.height ? clamp(from.fy + dy / room.height, 0, 1) : from.fy,
  };
}

export function snapped(spot: Spot): Spot {
  const snap = (v: number) => (v < SNAP ? 0 : v > 1 - SNAP ? 1 : v);
  return { fx: snap(spot.fx), fy: snap(spot.fy) };
}

export function isSpot(value: unknown): value is Spot {
  const v = value as Spot | null;
  return !!v && typeof v.fx === "number" && typeof v.fy === "number";
}

/** The trim bar is never narrower than this beside the card; it moves under the video instead. */
const TRIM_MIN_WIDTH = 300;
const TRIM_INSET = 12; // from the video's sides
const TRIM_LIFT = 60; // above the video's bottom, clear of most players' own controls
const TRIM_GAP = 8; // from the card

/**
 * Where the trim bar goes: along the bottom of the video, above the player's controls. When the
 * card is in the way it runs beside it, or under the video if that leaves it too narrow.
 */
export function trimPosition(video: Box, card: Box | null, height: number, view: Size): Box {
  let left = Math.max(video.left + TRIM_INSET, EDGE);
  let right = Math.min(video.left + video.width - TRIM_INSET, view.width - EDGE);
  let top = Math.max(video.top + video.height - TRIM_LIFT - height, video.top + TRIM_INSET);
  const covers = (t: number, l: number, r: number) =>
    !!card &&
    t < card.top + card.height + TRIM_GAP &&
    t + height > card.top - TRIM_GAP &&
    l < card.left + card.width + TRIM_GAP &&
    r > card.left - TRIM_GAP;
  if (card && covers(top, left, right)) {
    const leftRoom = card.left - TRIM_GAP - left;
    const rightRoom = right - (card.left + card.width + TRIM_GAP);
    if (Math.max(leftRoom, rightRoom) >= TRIM_MIN_WIDTH) {
      if (leftRoom >= rightRoom) right = card.left - TRIM_GAP;
      else left = card.left + card.width + TRIM_GAP;
    } else {
      top = video.top + video.height + TRIM_GAP;
    }
  }
  top = clamp(top, EDGE, view.height - height - EDGE);
  return { left: Math.round(left), top: Math.round(top), width: Math.round(right - left), height };
}
