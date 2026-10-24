import { describe, expect, it } from "vitest";
import type { Download, QueueState } from "./api/client";
import {
  arranged,
  daysText,
  minutesOf,
  nextStart,
  ordinal,
  scheduleText,
  speedText,
  standing,
  startsIn,
  retryText,
} from "./queues";

const night = (over: Partial<QueueState> = {}): QueueState => ({
  id: "night",
  name: "Tonight",
  atOnce: 2,
  retries: 3,
  retryWait: 5,
  schedule: { on: true, start: "02:00", stop: "07:00", days: [] },
  running: false,
  ...over,
});

// Monday 12 October 2026, 20:48.
const monday = new Date(2026, 9, 12, 20, 48);

describe("queues in words", () => {
  it("reads times and days", () => {
    expect(minutesOf("02:30")).toBe(150);
    expect(minutesOf("24:00")).toBeNull();
    expect(daysText([])).toBe("every day");
    expect(daysText([1, 2, 3, 4, 5])).toBe("on weekdays");
    expect(daysText([6, 0])).toBe("on weekends");
    expect(daysText([1, 3])).toBe("on Mon, Wed");
  });

  it("says how a failed download is tried again", () => {
    expect(retryText(3, 5)).toBe("Retries 3 times, 5s apart");
    expect(retryText(1, 1)).toBe("Retries once, 1s apart");
    expect(retryText(0, 5)).toBe("Doesn't retry");
    expect(retryText(2, 0)).toBe("Retries 2 times, straight away");
  });

  it("describes a schedule", () => {
    expect(scheduleText(night())).toMatch(/^2:00\sAM to 7:00\sAM, every day$/);
    expect(
      scheduleText(night({ schedule: { on: true, start: "23:30", stop: "", days: [6] } })),
    ).toMatch(/^Starts at 11:30\sPM, on Sat$/);
    expect(
      scheduleText(night({ schedule: { on: false, start: "02:00", stop: "", days: [] } })),
    ).toBe("");
  });

  it("finds the next start", () => {
    expect(nextStart(night(), monday)).toEqual(new Date(2026, 9, 13, 2, 0));
    const saturdays = night({ schedule: { on: true, start: "10:00", stop: "", days: [6] } });
    expect(nextStart(saturdays, monday)).toEqual(new Date(2026, 9, 17, 10, 0));
    expect(startsIn(night(), monday)).toBe("in 5 h 12 min");
  });

  it("says where a queue stands", () => {
    const downloading = { queue: "night", status: "downloading" } as Download;
    expect(standing(night({ running: true }), [downloading, downloading], monday)).toBe(
      "2 running",
    );
    expect(standing(night({ running: true }), [], monday)).toBe("Running");
    expect(standing(night(), [], monday)).toMatch(/^Starts at 2:00\sAM$/);
    expect(
      standing(night({ schedule: { on: true, start: "10:00", stop: "", days: [6] } }), [], monday),
    ).toMatch(/^Starts Sat at 10:00\sAM$/);
    expect(
      standing(night({ schedule: { on: false, start: "", stop: "", days: [] } }), [], monday),
    ).toBe("Stopped");
  });

  it("writes speeds and places", () => {
    expect(speedText(0)).toBe("No speed limit");
    expect(speedText(500 * 1024)).toBe("Up to 500 KB/s");
    expect(speedText(1.5 * (1 << 20))).toBe("Up to 1.5 MB/s");
    expect([1, 2, 3, 4, 11, 12, 13, 21, 22].map(ordinal)).toEqual([
      "1st",
      "2nd",
      "3rd",
      "4th",
      "11th",
      "12th",
      "13th",
      "21st",
      "22nd",
    ]);
  });
});

describe("putting queues in order", () => {
  const ids = ["main", "a", "b", "c"];
  it("puts a queue before or after another", () => {
    expect(arranged(ids, "c", "a", false)).toEqual(["main", "c", "a", "b"]);
    expect(arranged(ids, "a", "c", true)).toEqual(["main", "b", "c", "a"]);
    expect(arranged(ids, "a", "b", false)).toEqual(ids);
  });

  it("keeps Main first", () => {
    expect(arranged(ids, "c", "main", false)).toEqual(["main", "c", "a", "b"]);
    expect(arranged(ids, "main", "c", true)).toEqual(ids);
  });
});
