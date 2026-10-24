import { action } from "./badge";
import { api } from "../shared/browser";

const SIZES = [16, 32, 48] as const;
const RING = "#121418";
const ON = "#188038";
const OFF = "#c5221f";

/** Paints the toolbar icon with a green (shown) or red (hidden) corner dot. */
export async function paintIcon(visible: boolean): Promise<void> {
  try {
    const imageData: Record<number, ImageData> = {};
    for (const size of SIZES) imageData[size] = await withDot(size, visible ? ON : OFF);
    await action.setIcon({ imageData });
  } catch {
    // OffscreenCanvas missing, or the PNGs not packed: the manifest's icon stays
  }
}

async function withDot(size: number, fill: string): Promise<ImageData> {
  const src = await fetch(api.runtime.getURL("icon-" + size + ".png"));
  const bitmap = await createImageBitmap(await src.blob());
  const canvas = new OffscreenCanvas(size, size);
  const ctx = canvas.getContext("2d");
  if (!ctx) throw new Error("no canvas");
  ctx.drawImage(bitmap, 0, 0, size, size);
  const r = Math.max(2, Math.round(size / 5.5));
  const cx = size - r - 1;
  const cy = size - r - 1;
  ctx.beginPath();
  ctx.arc(cx, cy, r + 1, 0, Math.PI * 2);
  ctx.fillStyle = RING;
  ctx.fill();
  ctx.beginPath();
  ctx.arc(cx, cy, r, 0, Math.PI * 2);
  ctx.fillStyle = fill;
  ctx.fill();
  return ctx.getImageData(0, 0, size, size);
}
