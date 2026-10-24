import { describe, expect, it } from "vitest";
import {
  CARD_WIDTH,
  cardPosition,
  dragged,
  isSpot,
  pillPosition,
  roomFor,
  snapped,
  trimPosition,
} from "../src/content/placement";

const view = { width: 1600, height: 900 };
const video = { left: 100, top: 100, width: 1000, height: 600 };
const pill = { width: 150, height: 38 };

describe("placing the button and its card", () => {
  it("puts the button in the video's top right by default, inside its margin", () => {
    expect(roomFor(video, pill)).toEqual({ width: 830, height: 542 });
    expect(pillPosition(video, pill, { fx: 1, fy: 0 }, view)).toEqual({ left: 940, top: 110 });
    expect(pillPosition(video, pill, { fx: 0, fy: 1 }, view)).toEqual({ left: 110, top: 652 });
  });

  it("keeps the button inside the window when the video runs off it", () => {
    const offscreen = { left: -500, top: -300, width: 2400, height: 1400 };
    expect(pillPosition(offscreen, pill, { fx: 0, fy: 0 }, view)).toEqual({ left: 4, top: 4 });
  });

  it("opens the card toward the middle of the video", () => {
    const topRight = pillPosition(video, pill, { fx: 1, fy: 0 }, view);
    expect(cardPosition(video, pill, topRight, 400, view)).toEqual({
      left: 940 + 150 - CARD_WIDTH,
      top: 110,
    });
    const bottomLeft = pillPosition(video, pill, { fx: 0, fy: 1 }, view);
    expect(cardPosition(video, pill, bottomLeft, 400, view)).toEqual({
      left: 110,
      top: 652 + 38 - 400,
    });
  });

  it("drags within the video and snaps to edges when dropped near them", () => {
    const room = roomFor(video, pill);
    expect(dragged({ fx: 1, fy: 0 }, -415, 271, room)).toEqual({ fx: 0.5, fy: 0.5 });
    expect(dragged({ fx: 0.5, fy: 0.5 }, 5000, -5000, room)).toEqual({ fx: 1, fy: 0 });
    expect(snapped({ fx: 0.97, fy: 0.03 })).toEqual({ fx: 1, fy: 0 });
    expect(snapped({ fx: 0.5, fy: 0.3 })).toEqual({ fx: 0.5, fy: 0.3 });
  });

  it("only trusts saved spots that are spots", () => {
    expect(isSpot({ fx: 0.2, fy: 1 })).toBe(true);
    expect(isSpot({ fx: "1" })).toBe(false);
    expect(isSpot(null)).toBe(false);
  });
});

describe("placing the trim bar", () => {
  it("runs along the bottom of the video, above the player's controls", () => {
    expect(trimPosition(video, null, 70, view)).toEqual({
      left: 112,
      top: 570,
      width: 976,
      height: 70,
    });
  });

  it("runs beside the card where the card is in the way", () => {
    const card = { left: 748, top: 110, width: CARD_WIDTH, height: 560 };
    expect(trimPosition(video, card, 70, view)).toEqual({
      left: 112,
      top: 570,
      width: 628,
      height: 70,
    });
  });

  it("leaves a card that is out of the way alone", () => {
    const card = { left: 748, top: 110, width: CARD_WIDTH, height: 300 };
    expect(trimPosition(video, card, 70, view).width).toBe(976);
  });

  it("goes under the video when beside the card is too narrow", () => {
    const small = { left: 100, top: 100, width: 560, height: 315 };
    const card = { left: 300, top: 104, width: CARD_WIDTH, height: 500 };
    expect(trimPosition(small, card, 70, view)).toEqual({
      left: 112,
      top: 423,
      width: 536,
      height: 70,
    });
  });

  it("stays inside the window", () => {
    const tall = { left: 0, top: 0, width: 1600, height: 900 };
    const card = { left: 0, top: 0, width: 1600, height: 900 };
    expect(trimPosition(tall, card, 70, view).top).toBe(826);
  });
});
