import { describe, expect, it } from "vitest";
import { layoutOf, offsets, reorder, type Layout } from "./sortable";

// Three rows 40 high from y 100: a at 100, b at 140, c at 180.
const layout: Layout = {
  start: 100,
  top: { a: 100, b: 140, c: 180 },
  height: { a: 40, b: 40, c: 40 },
};
const order = ["a", "b", "c"];

describe("rearranging by dragging", () => {
  it("trades places once the pointer passes a neighbour's middle", () => {
    expect(reorder(order, "a", 155, layout)).toEqual(order); // not past b's middle (160)
    expect(reorder(order, "a", 165, layout)).toEqual(["b", "a", "c"]);
    expect(reorder(order, "a", 205, layout)).toEqual(["b", "c", "a"]);
    expect(reorder(order, "c", 115, layout)).toEqual(["c", "a", "b"]);
    expect(reorder(order, "x", 115, layout)).toEqual(order);
  });

  it("slides each row to where the order puts it", () => {
    expect(offsets(["b", "a", "c"], layout)).toEqual({ b: -40, a: 40, c: 0 });
    expect(offsets(order, layout)).toEqual({ a: 0, b: 0, c: 0 });
  });

  it("works out slots from rows of different heights", () => {
    const mixed: Layout = { start: 0, top: { a: 0, b: 90 }, height: { a: 90, b: 40 } };
    expect(offsets(["b", "a"], mixed)).toEqual({ b: -90, a: 40 });
  });
});

describe("measuring rows", () => {
  it("counts the gap between rows in each one's height", () => {
    const row = (top: number) =>
      ({ getBoundingClientRect: () => ({ top, height: 42 }) as DOMRect }) as HTMLElement;
    const layout = layoutOf([
      ["a", row(100)],
      ["b", row(144)],
    ]);
    expect(layout).toEqual({ start: 100, top: { a: 100, b: 144 }, height: { a: 44, b: 44 } });
    expect(offsets(["b", "a"], layout)).toEqual({ b: -44, a: 44 });
  });
});
