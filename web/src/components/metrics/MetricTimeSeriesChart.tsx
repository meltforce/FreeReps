import { useMemo } from "react";
import UplotReact from "uplot-react";
import uPlot from "uplot";
import "uplot/dist/uPlot.min.css";
import { TimeSeriesPoint, MetricStats } from "../../api";
import { movingAverage } from "../../utils/stats";

interface Props {
  data: TimeSeriesPoint[];
  stats: MetricStats | null;
  label: string;
  unit: string;
}

export default function MetricTimeSeriesChart({
  data,
  stats,
  label,
  unit,
}: Props) {
  const { opts, plotData } = useMemo(() => {
    if (!data || data.length === 0) return { opts: null, plotData: null };

    const times = data.map((p) =>
      Math.floor(new Date(p.time).getTime() / 1000)
    );
    const values = data.map((p) => p.avg ?? p.min ?? p.max ?? null);
    const ma7 = movingAverage(values, 7);

    // Normal range band (mean +/- 1 stddev)
    const mean = stats?.avg ?? null;
    const sd = stats?.stddev ?? null;
    const bandUpper =
      mean != null && sd != null
        ? values.map(() => mean + sd)
        : values.map(() => null);
    const bandLower =
      mean != null && sd != null
        ? values.map(() => mean - sd)
        : values.map(() => null);

    const series: uPlot.Series[] = [
      {},
      // Normal range upper bound (invisible line)
      {
        label: "Upper Band",
        stroke: "transparent",
        width: 0,
        show: mean != null && sd != null,
      },
      // Normal range lower bound (fill between upper and lower via bands config)
      {
        label: "Normal Range",
        stroke: "transparent",
        width: 0,
        fill: "rgba(34,211,238,0.06)",
        show: mean != null && sd != null,
      },
      // Primary data line
      {
        label: `${label} (${unit})`,
        stroke: "#22d3ee",
        width: 2,
        points: {
          show: true,
          size: 4,
          fill: (u: uPlot, seriesIdx: number) => {
            // Color outliers differently
            if (!mean || !sd) return "#22d3ee";
            const idx = u.cursor.idx;
            if (idx == null) return "#22d3ee";
            const v = u.data[seriesIdx][idx];
            if (v != null && Math.abs(v - mean) > 1.5 * sd) return "#f59e0b";
            return "#22d3ee";
          },
        },
      },
      // 7-day moving average
      {
        label: "7d Avg",
        stroke: "#a78bfa",
        width: 1.5,
        dash: [6, 4],
        points: { show: false },
      },
    ];

    const opts: uPlot.Options = {
      width: 0,
      height: 300,
      series,
      axes: [
        {
          stroke: "#52525b",
          grid: { stroke: "#27272a", width: 1 },
          ticks: { stroke: "#27272a" },
        },
        {
          stroke: "#52525b",
          grid: { stroke: "#27272a", width: 1 },
          ticks: { stroke: "#27272a" },
          label: unit,
          labelSize: 14,
        },
      ],
      scales: { x: { time: true } },
      cursor: { drag: { x: true, y: false } },
      bands: [
        {
          series: [1, 2],
          fill: "rgba(34,211,238,0.06)",
        },
      ],
    };

    return {
      opts,
      plotData: [
        new Float64Array(times),
        bandUpper,
        bandLower,
        values,
        ma7,
      ] as uPlot.AlignedData,
    };
  }, [data, stats, label, unit]);

  if (!opts || !plotData) {
    return (
      <div className="bg-zinc-900 rounded-lg p-6 text-zinc-500 text-sm">
        No data available for {label}.
      </div>
    );
  }

  return (
    <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-4">
      <AutoSizeUplot opts={opts} data={plotData} />
      <div className="flex items-center gap-4 mt-2 text-xs text-zinc-500">
        <span className="flex items-center gap-1">
          <span className="w-3 h-0.5 bg-cyan-400 inline-block" /> Value
        </span>
        <span className="flex items-center gap-1">
          <span
            className="w-3 h-0.5 inline-block"
            style={{
              background: "#a78bfa",
              backgroundImage: "repeating-linear-gradient(90deg, #a78bfa 0 6px, transparent 6px 10px)",
            }}
          />{" "}
          7d Avg
        </span>
        {stats?.avg != null && stats?.stddev != null && (
          <span className="flex items-center gap-1">
            <span className="w-3 h-3 bg-cyan-400/10 inline-block rounded-sm" />{" "}
            Normal Range
          </span>
        )}
      </div>
    </div>
  );
}

function AutoSizeUplot({
  opts,
  data,
}: {
  opts: uPlot.Options;
  data: uPlot.AlignedData;
}) {
  return (
    <div className="w-full">
      <UplotReact
        options={{ ...opts, width: 1 }}
        data={data}
        onCreate={(u: uPlot) => {
          const ro = new ResizeObserver((entries) => {
            for (const entry of entries) {
              u.setSize({
                width: entry.contentRect.width,
                height: opts.height,
              });
            }
          });
          ro.observe(u.root);
        }}
      />
    </div>
  );
}
