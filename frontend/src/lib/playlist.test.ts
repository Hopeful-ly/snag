import { describe, expect, it } from "vitest";
import {
  clockOf,
  groupsOf,
  paint,
  toggleGroup,
  minutesOf,
  countOf,
  fromCurrent,
  lengthOf,
  nameOf,
  picked,
  startingPicks,
  toggle,
} from "./playlist";

describe("choosing from a list", () => {
  it("starts with the rest of the list, leaving out what Snag has", () => {
    expect(startingPicks(4, 1, [false, false, true, false])).toEqual([false, true, false, true]);
    expect(startingPicks(3, -1)).toEqual([true, true, true]);
  });

  it("ticks from the playing one on", () => {
    expect(fromCurrent(3, 1)).toEqual([false, true, true]);
    expect(fromCurrent(2, -1)).toEqual([false, false]);
  });

  it("flips one, or a range with shift", () => {
    expect(toggle([false, false, false], 1)).toEqual([false, true, false]);
    expect(toggle([true, false, false, true], 2, 0)).toEqual([true, true, true, true]);
    expect(toggle([true, true, true, true], 0, 2)).toEqual([false, false, false, true]);
  });

  it("paints a sweep over entries one way", () => {
    expect(paint([false, true, false, false], 3, 1, true)).toEqual([false, true, true, true]);
    expect(paint([true, true, true], 0, 1, false)).toEqual([false, false, true]);
  });

  it("ticks a season, or unticks it when it is all ticked", () => {
    const url = "https://x.example/e";
    const groups = groupsOf([
      { url, group: "Season 1" },
      { url, group: "Season 1" },
      { url, group: "Season 2" },
      { url },
    ]);
    expect(groups).toEqual([
      { name: "Season 1", entries: [0, 1] },
      { name: "Season 2", entries: [2] },
    ]);
    const first = groups[0]!;
    expect(toggleGroup([true, false, false, false], first)).toEqual([true, true, false, false]);
    expect(toggleGroup([true, true, true, false], first)).toEqual([false, false, true, false]);
    expect(groupsOf([{ url }])).toEqual([]);
  });

  it("lists the picked indexes in order", () => {
    expect(picked([true, false, true])).toEqual([0, 2]);
  });
});

describe("describing a list", () => {
  it("adds up the lengths it knows", () => {
    const url = "https://example.com/v";
    expect(
      lengthOf([
        { url, duration: 3600 },
        { url, duration: 600 },
      ]),
    ).toBe("1 h 10 min");
    expect(lengthOf([{ url, duration: 90 }, { url }])).toBe("over 1 min 30 s");
    expect(lengthOf([{ url }])).toBe("");
  });

  it("names entries without a title after their address", () => {
    expect(nameOf({ url: "https://soundcloud.com/forss/flickermood" })).toBe("flickermood");
    expect(nameOf({ url: "https://x.com/a", title: "Episode 1" })).toBe("Episode 1");
    expect(nameOf({ url: "not a link" })).toBe("not a link");
  });

  it("shows lengths as players do", () => {
    expect(clockOf(187)).toBe("3:07");
    expect(clockOf(3723)).toBe("1:02:03");
    expect(clockOf(undefined)).toBe("");
  });

  it("shows lengths known to the minute as such", () => {
    expect(minutesOf(49 * 60)).toBe("~49 min");
    expect(minutesOf(62 * 60)).toBe("~1 h 2 min");
    expect(minutesOf(undefined)).toBe("");
  });

  it("counts videos or tracks", () => {
    expect(countOf(182, false)).toBe("182 videos");
    expect(countOf(1, true)).toBe("1 track");
  });
});
