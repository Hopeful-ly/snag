import { describe, expect, it, vi } from "vitest";
import { Window } from "@wailsio/runtime";
import { wailsApi } from "../lib/api/client";

// A window size handed back when fitWindow asks. The frame is size.height - innerHeight.
function deferSize() {
  const pending: Array<(size: { width: number; height: number }) => void> = [];
  vi.spyOn(Window, "Size").mockImplementation(
    () => new Promise((resolve) => pending.push(resolve)) as never,
  );
  return pending;
}

describe("fitWindow", () => {
  it("drops a height that is still being measured once a newer one is asked for", async () => {
    const pending = deferSize();
    const setSize = vi.spyOn(Window, "SetSize").mockResolvedValue(undefined as never);
    Object.defineProperty(window, "innerHeight", { configurable: true, value: 360 });

    const first = wailsApi.fitWindow(38);
    await vi.waitFor(() => expect(pending).toHaveLength(1));
    const second = wailsApi.fitWindow(420);

    pending[0]!({ width: 560, height: 396 });
    await vi.waitFor(() => expect(pending).toHaveLength(2));
    pending[1]!({ width: 560, height: 396 });
    await Promise.all([first, second]);

    expect(setSize).toHaveBeenCalledTimes(1);
    expect(setSize).toHaveBeenCalledWith(560, 456);
  });
});
