import type uPlot from "uplot";

/**
 * 24h time axis formatter for uPlot.
 * Shows "HH:MM" for sub-day tick spacing, "DD.MM." for daily spacing.
 */
export const axisValues24h: uPlot.Axis.Values = (
  _u: uPlot,
  vals: number[]
): string[] =>
  vals.map((v) => {
    const d = new Date(v * 1000);
    const span = vals.length > 1 ? Math.abs(vals[1] - vals[0]) : 86400;
    if (span < 86400) {
      return d.toLocaleString("de-DE", {
        hour: "2-digit",
        minute: "2-digit",
        hour12: false,
      });
    }
    return d.toLocaleDateString("de-DE", {
      day: "2-digit",
      month: "2-digit",
    });
  });
