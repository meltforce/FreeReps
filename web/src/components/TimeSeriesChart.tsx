import { useQuery } from "@tanstack/react-query";
import { fetchTimeSeries, TimeSeriesPoint } from "../api";
import UplotReact from "uplot-react";
import "uplot/dist/uPlot.min.css";
import { useMemo } from "react";
import type uPlot from "uplot";

interface Props {
  metric: string;
  start: string;
  end: string;
  label: string;
  unit: string;
}

export default function TimeSeriesChart({
  metric,
  start,
  end,
  label,
  unit,
}: Props) {
  const { data, isLoading, error } = useQuery({
    queryKey: ["timeseries", metric, start, end],
    queryFn: () => fetchTimeSeries(metric, start, end),
  });

  const { opts, plotData } = useMemo(() => {
    if (!data || data.length === 0) {
      return { opts: null, plotData: null };
    }

    const times = data.map((p: TimeSeriesPoint) =>
      Math.floor(new Date(p.time).getTime() / 1000)
    );
    const values = data.map(
      (p: TimeSeriesPoint) => p.avg ?? p.min ?? p.max ?? null
    );

    const opts: uPlot.Options = {
      width: 0, // auto-sized by container
      height: 300,
      series: [
        {},
        {
          label: `${label} (${unit})`,
          stroke: "#22d3ee",
          width: 2,
          fill: "rgba(34,211,238,0.08)",
        },
      ],
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
    };

    return {
      opts,
      plotData: [new Float64Array(times), values] as uPlot.AlignedData,
    };
  }, [data, label, unit]);

  if (isLoading) {
    return (
      <div className="bg-zinc-900 rounded-lg p-6 h-[340px] animate-pulse" />
    );
  }

  if (error || !opts || !plotData) {
    return (
      <div className="bg-zinc-900 rounded-lg p-6 text-zinc-500 text-sm">
        No data available for {label} in this time range.
      </div>
    );
  }

  return (
    <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-4">
      <AutoSizeUplot opts={opts} data={plotData} />
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
