import { useMemo } from "react";
import type uPlot from "uplot";
import "uplot/dist/uPlot.min.css";
import AutoSizeUplot from "../AutoSizeUplot";
import { axisValues24h } from "../../utils/chartFormat";
import type { TimeSeriesPoint } from "../../api";

export interface SeriesData {
  metric: string;
  label: string;
  unit: string;
  color: string;
  points: TimeSeriesPoint[];
}

interface Props {
  seriesData: SeriesData[];
}

export default function TrendsChart({ seriesData }: Props) {
  const { opts, plotData } = useMemo(() => {
    if (seriesData.length === 0) return { opts: null, plotData: null };

    // Collect all unique timestamps across all series
    const timeSet = new Set<number>();
    for (const s of seriesData) {
      for (const p of s.points) {
        timeSet.add(Math.floor(new Date(p.time).getTime() / 1000));
      }
    }
    const times = Array.from(timeSet).sort((a, b) => a - b);
    if (times.length === 0) return { opts: null, plotData: null };

    // Build a time→index map for fast lookup
    const timeIndex = new Map<number, number>();
    times.forEach((t, i) => timeIndex.set(t, i));

    // Build aligned data arrays (null-fill gaps)
    const aligned: (number | null)[][] = seriesData.map((s) => {
      const arr: (number | null)[] = new Array(times.length).fill(null);
      for (const p of s.points) {
        const t = Math.floor(new Date(p.time).getTime() / 1000);
        const idx = timeIndex.get(t);
        if (idx !== undefined) arr[idx] = p.avg;
      }
      return arr;
    });

    // Each metric gets its own scale so different value ranges don't
    // compress each other (e.g. SpO2 ~95% vs Body Fat ~13%).
    const series: uPlot.Series[] = [
      {},
      ...seriesData.map((s, i) => ({
        label: s.label,
        stroke: s.color,
        width: 2,
        scale: `metric-${i}`,
        spanGaps: true,
      })),
    ];

    // Visible axes: left for first metric, right for the first metric
    // with a different unit (if any). All other scales auto-range invisibly.
    const uniqueUnits = [...new Set(seriesData.map((s) => s.unit))];
    const rightUnit = uniqueUnits.length > 1 ? uniqueUnits[1] : null;
    const rightIdx = rightUnit
      ? seriesData.findIndex((s) => s.unit === rightUnit)
      : -1;

    const axes: uPlot.Axis[] = [
      {
        stroke: "#52525b",
        grid: { stroke: "#27272a", width: 1 },
        ticks: { stroke: "#27272a" },
        values: axisValues24h,
      },
      {
        stroke: "#52525b",
        grid: { stroke: "#27272a", width: 1 },
        ticks: { stroke: "#27272a33" },
        label: seriesData[0].unit,
        labelSize: 14,
        scale: "metric-0",
        side: 3,
        size: 60,
      },
    ];

    if (rightIdx > 0) {
      axes.push({
        stroke: "#52525b",
        grid: { show: false },
        ticks: { stroke: "#52525b33" },
        label: rightUnit!,
        labelSize: 14,
        scale: `metric-${rightIdx}`,
        side: 1,
        size: 60,
      });
    }

    const scales: uPlot.Scales = { x: { time: true } };
    seriesData.forEach((_, i) => {
      scales[`metric-${i}`] = { auto: true };
    });

    const opts: uPlot.Options = {
      width: 0,
      height: 350,
      series,
      axes,
      scales,
      cursor: { drag: { x: true, y: false } },
      padding: [null, 10, null, 10],
    };

    const plotData: uPlot.AlignedData = [
      new Float64Array(times),
      ...aligned,
    ] as uPlot.AlignedData;

    return { opts, plotData };
  }, [seriesData]);

  if (!opts || !plotData) {
    return (
      <div className="bg-zinc-900 rounded-lg p-6 text-zinc-500 text-sm">
        No data available. Select at least one metric.
      </div>
    );
  }

  return (
    <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-4">
      <AutoSizeUplot opts={opts} data={plotData} />
      <div className="flex flex-wrap items-center gap-4 mt-2 text-xs text-zinc-500">
        {seriesData.map((s) => (
          <span key={s.metric} className="flex items-center gap-1">
            <span
              className="w-3 h-0.5 inline-block"
              style={{ backgroundColor: s.color }}
            />
            {s.label}
          </span>
        ))}
      </div>
    </div>
  );
}
