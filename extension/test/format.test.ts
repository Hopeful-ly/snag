import { describe, expect, it } from "vitest";
import {
  durationText,
  hostOf,
  isWebAddress,
  playlistText,
  sizeText,
  timeText,
} from "../src/shared/format";

describe("writing things for people", () => {
  it("writes sizes", () => {
    expect([0, 512, 2048, 5 * 1024 ** 2, 820 * 1024 ** 2, 1.4 * 1024 ** 3].map(sizeText)).toEqual([
      "",
      "512 B",
      "2 KB",
      "5.0 MB",
      "820 MB",
      "1.4 GB",
    ]);
  });

  it("says what a list holds", () => {
    expect(playlistText(182)).toBe("182 in all · Download playlist…");
  });

  it("writes lengths and time left", () => {
    expect([45, 720, 3900].map(durationText)).toEqual(["45 s", "12 min", "1 h 05 min"]);
    expect([null, 40, 200, 3900].map(timeText)).toEqual(["", "40s", "3m 20s", "1h 5m"]);
  });

  it("names sites and tells web addresses", () => {
    expect(hostOf("https://www.example.com/a")).toBe("example.com");
    expect(hostOf("nonsense")).toBe("");
    expect(isWebAddress("https://x.io")).toBe(true);
    expect(isWebAddress("blob:https://x.io/1")).toBe(false);
    expect(isWebAddress(undefined)).toBe(false);
  });
});
