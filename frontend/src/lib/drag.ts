const kind = "application/x-snag-downloads";

export function startDrag(e: DragEvent, ids: string[]): void {
  if (!e.dataTransfer) return;
  e.dataTransfer.setData(kind, JSON.stringify(ids));
  e.dataTransfer.effectAllowed = "move";
}

export function dragsDownloads(e: DragEvent): boolean {
  return !!e.dataTransfer && [...e.dataTransfer.types].includes(kind);
}

export function dragged(e: DragEvent): string[] {
  try {
    const ids: unknown = JSON.parse(e.dataTransfer?.getData(kind) || "[]");
    return Array.isArray(ids) ? ids.filter((id): id is string => typeof id === "string") : [];
  } catch {
    return [];
  }
}
