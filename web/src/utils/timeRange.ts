export type TimeRange = "1d" | "7d" | "30d" | "90d" | "1y";

export function daysFromRange(range_: TimeRange): number {
  switch (range_) {
    case "1d":
      return 1;
    case "7d":
      return 7;
    case "30d":
      return 30;
    case "90d":
      return 90;
    case "1y":
      return 365;
  }
}

/** Format "2026-02-19" → "19.02." */
function fmtDate(iso: string): string {
  const [, m, d] = iso.split("-");
  return `${d}.${m}.`;
}

/** Format date range label, e.g. "21.01. – 19.02." */
export function formatDateLabel(start: string, end: string): string {
  return `${fmtDate(start)} – ${fmtDate(end)}`;
}
