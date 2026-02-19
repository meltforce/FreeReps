import { useMemo } from "react";
import UplotReact from "uplot-react";
import uPlot from "uplot";
import "uplot/dist/uPlot.min.css";
import { CorrelationPoint } from "../../api";

interface Props {
  points: CorrelationPoint[];
  xLabel: string;
  yLabel: string;
}

export default function OverlayChart({ points, xLabel, yLabel }: Props) {
  const { opts, plotData } = useMemo(() => {
    if (!points || points.length === 0) return { opts: null, plotData: null };

    const times = points.map((p) =>
      Math.floor(new Date(p.time).getTime() / 1000)
    );
    const xs = points.map((p) => p.x);
    const ys = points.map((p) => p.y);

    const opts: uPlot.Options = {
      width: 0,
      height: 350,
      series: [
        {},
        {
          label: xLabel,
          stroke: "#22d3ee",
          width: 2,
          scale: "x-metric",
        },
        {
          label: yLabel,
          stroke: "#a78bfa",
          width: 2,
          scale: "y-metric",
        },
      ],
      axes: [
        {
          stroke: "#52525b",
          grid: { stroke: "#27272a", width: 1 },
          ticks: { stroke: "#27272a" },
        },
        {
          stroke: "#22d3ee",
          grid: { stroke: "#27272a", width: 1 },
          ticks: { stroke: "#27272a" },
          label: xLabel,
          labelSize: 14,
          scale: "x-metric",
          side: 3,
        },
        {
          stroke: "#a78bfa",
          grid: { show: false },
          ticks: { stroke: "#27272a" },
          label: yLabel,
          labelSize: 14,
          scale: "y-metric",
          side: 1,
        },
      ],
      scales: {
        x: { time: true },
        "x-metric": { auto: true },
        "y-metric": { auto: true },
      },
      cursor: { drag: { x: true, y: false } },
    };

    return {
      opts,
      plotData: [new Float64Array(times), xs, ys] as uPlot.AlignedData,
    };
  }, [points, xLabel, yLabel]);

  if (!opts || !plotData) {
    return (
      <div className="bg-zinc-900 rounded-lg p-6 text-zinc-500 text-sm">
        No data available for overlay chart.
      </div>
    );
  }

  return (
    <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-4">
      <AutoSizeUplot opts={opts} data={plotData} />
      <div className="flex items-center gap-4 mt-2 text-xs text-zinc-500">
        <span className="flex items-center gap-1">
          <span className="w-3 h-0.5 bg-cyan-400 inline-block" /> {xLabel}
        </span>
        <span className="flex items-center gap-1">
          <span className="w-3 h-0.5 inline-block bg-violet-400" /> {yLabel}
        </span>
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
