import { describe, expect, it } from "vitest";
import { clippable, firstClip, lengthText, momentText, moveEnd, shift } from "../src/media/clip";

describe("clips", () => {
  it("can't clip a live stream or a blip", () => {
    expect(clippable(Infinity)).toBe(false);
    expect(clippable(NaN)).toBe(false);
    expect(clippable(0.8)).toBe(false);
    expect(clippable(120)).toBe(true);
  });

  it("starts with the next half minute, kept inside the video", () => {
    expect(firstClip(42.123, 600)).toEqual({ from: 42.12, to: 72.12 });
    expect(firstClip(590, 600)).toEqual({ from: 570, to: 600 });
    expect(firstClip(3, 12)).toEqual({ from: 0, to: 12 });
  });

  it("moves an end without crossing the other or leaving the video", () => {
    const clip = { from: 10, to: 20 };
    expect(moveEnd(clip, "from", 4, 60)).toEqual({ from: 4, to: 20 });
    expect(moveEnd(clip, "from", 25, 60)).toEqual({ from: 19.5, to: 20 });
    expect(moveEnd(clip, "to", 5, 60)).toEqual({ from: 10, to: 10.5 });
    expect(moveEnd(clip, "to", 99, 60)).toEqual({ from: 10, to: 60 });
  });

  it("moves the whole clip, keeping its length", () => {
    expect(shift({ from: 10, to: 20 }, 5, 60)).toEqual({ from: 15, to: 25 });
    expect(shift({ from: 10, to: 20 }, -50, 60)).toEqual({ from: 0, to: 10 });
    expect(shift({ from: 10, to: 20 }, 50, 60)).toEqual({ from: 50, to: 60 });
  });

  it("writes moments and lengths", () => {
    expect(momentText(42)).toBe("0:42.0");
    expect(momentText(78.56)).toBe("1:18.6");
    expect(momentText(3723.5)).toBe("1:02:03.5");
    expect(lengthText({ from: 42, to: 78.5 })).toBe("36.5 s");
    expect(lengthText({ from: 0, to: 30 })).toBe("30 s");
    expect(lengthText({ from: 0, to: 125 })).toBe("2 min 05 s");
  });
});
