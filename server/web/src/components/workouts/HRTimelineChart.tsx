import { useMemo } from "react";
import uPlot from "uplot";
import "uplot/dist/uPlot.min.css";
import type { WorkoutHR } from "../../api";
import AutoSizeUplot from "../AutoSizeUplot";
import { axisValues24h } from "../../utils/chartFormat";
import { useTheme } from "../../theme";
import { resolveZoneEdges } from "../../utils/stageColors";
import { tokenColor, tokenColorAlpha } from "../../utils/tokenColor";

interface Props {
  hrData: WorkoutHR[];
  /** The server's zone edges; without them the session's own peak stands in. */
  zoneEdges?: number[] | null;
}

export default function HRTimelineChart({ hrData, zoneEdges }: Props) {
  // The theme decides what the resolved token values are, so the chart is
  // rebuilt when it changes.
  const { theme } = useTheme();

  const { opts, plotData } = useMemo(() => {
    if (!hrData || hrData.length === 0) return { opts: null, plotData: null };

    const times = hrData.map((p) => Math.floor(new Date(p.Time).getTime() / 1000));
    const bpms = hrData.map((p) => p.AvgBPM ?? p.MaxBPM ?? p.MinBPM ?? null);

    const peak = Math.max(...bpms.map((b) => b ?? 0));
    const edges = resolveZoneEdges(zoneEdges ?? undefined, peak);

    const axis = tokenColor("--color-neutral-600", "#79848c");
    const grid = tokenColor("--color-neutral-300", "#cfd4d6");
    const line = tokenColor("--color-data-3", "#337475");
    const zoneTint = tokenColorAlpha("--color-data-5", 0.1);

    const opts: uPlot.Options = {
      width: 0,
      height: 300,
      series: [
        {},
        {
          label: "bpm",
          stroke: line,
          width: 1.5,
        },
      ],
      axes: [
        {
          stroke: axis,
          grid: { stroke: grid, width: 1 },
          ticks: { stroke: grid },
          font: "11px Archivo Variable, Archivo, system-ui, sans-serif",
          values: axisValues24h,
        },
        {
          stroke: axis,
          grid: { stroke: grid, width: 1 },
          ticks: { stroke: grid },
          font: "11px Archivo Variable, Archivo, system-ui, sans-serif",
        },
      ],
      scales: { x: { time: true } },
      cursor: { drag: { x: true, y: false } },
      hooks: {
        draw: [
          // Tint the top zone only. Banding all five in one tone would read
          // as a gradient rather than a threshold.
          (u: uPlot) => {
            const yScale = u.scales.y;
            if (yScale.min == null || yScale.max == null) return;
            const from = edges[edges.length - 1];
            if (from >= yScale.max) return;
            const top = u.valToPos(yScale.max, "y", true);
            const bottom = u.valToPos(Math.max(from, yScale.min), "y", true);
            u.ctx.fillStyle = zoneTint;
            u.ctx.fillRect(u.bbox.left, top, u.bbox.width, bottom - top);
          },
        ],
      },
    };

    return {
      opts,
      plotData: [new Float64Array(times), bpms] as uPlot.AlignedData,
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [hrData, zoneEdges, theme]);

  if (!opts || !plotData) {
    return (
      <p style={{ color: "var(--color-neutral-600)", fontSize: 13 }}>
        No heart rate data for this workout.
      </p>
    );
  }

  return (
    <div style={{ marginTop: 30 }}>
      <h2 style={{ fontSize: 19, fontWeight: 700 }}>Heart rate</h2>
      <div
        style={{
          borderTop: "2px solid var(--color-text)",
          marginTop: 12,
          paddingTop: 12,
        }}
      >
        <AutoSizeUplot opts={opts} data={plotData} />
      </div>
    </div>
  );
}
