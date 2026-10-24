// Drag-to-reorder: the dragged row swaps with a neighbour once the pointer passes its middle.
// Rows keep their layout during the drag and their content slides by an offset; release
// reorders for real.

// Row tops are in pointer coordinates, measured when the drag began.
export interface Layout {
  start: number;
  top: Record<string, number>;
  height: Record<string, number>;
}

export const dragThreshold = 6;

function slots(order: string[], layout: Layout): Record<string, number> {
  const at: Record<string, number> = {};
  let y = layout.start;
  for (const id of order) {
    at[id] = y;
    y += layout.height[id] ?? 0;
  }
  return at;
}

export function reorder(order: string[], dragged: string, y: number, layout: Layout): string[] {
  const from = order.indexOf(dragged);
  if (from < 0) return order;
  const at = slots(order, layout);
  const middle = (id: string) => (at[id] ?? 0) + (layout.height[id] ?? 0) / 2;
  let to = from;
  for (let i = 0; i < from; i++) {
    if (y < middle(order[i]!)) {
      to = i;
      break;
    }
  }
  if (to === from) {
    for (let i = from + 1; i < order.length; i++) if (y > middle(order[i]!)) to = i;
  }
  if (to === from) return order;
  const next = order.filter((id) => id !== dragged);
  next.splice(to, 0, dragged);
  return next;
}

export function offsets(order: string[], layout: Layout): Record<string, number> {
  const at = slots(order, layout);
  return Object.fromEntries(order.map((id) => [id, (at[id] ?? 0) - (layout.top[id] ?? 0)]));
}

// A row's height includes the gap after it, so rows slide exactly into each other's places.
export function layoutOf(rows: [string, HTMLElement][]): Layout {
  const boxes = rows.map(([, row]) => row.getBoundingClientRect());
  const layout: Layout = { start: boxes[0]?.top ?? 0, top: {}, height: {} };
  let gap = 0;
  rows.forEach(([id], i) => {
    const box = boxes[i]!;
    const next = boxes[i + 1];
    if (next) gap = Math.max(0, next.top - box.top - box.height);
    layout.top[id] = box.top;
    layout.height[id] = next ? next.top - box.top : box.height + gap;
  });
  return layout;
}
