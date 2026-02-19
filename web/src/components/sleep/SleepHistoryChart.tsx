import { useMemo } from "react";
import UplotReact from "uplot-react";
import uPlot from "uplot";
import "uplot/dist/uPlot.min.css";
import { SleepSession } from "../../api";

interface Props {
  sessions: SleepSession[];
}

export default function SleepHistoryChart({ sessions }: Props) {
  const { opts, plotData } = useMemo(() => {
    if (!sessions || sessions.length === 0) {
      return { opts: null, plotData: null };
    }

    // Sort ascending by date
    const sorted = [...sessions].sort(
      (a, b) => new Date(a.Date).getTime() - new Date(b.Date).getTime()
    );

    const times = sorted.map((s) =>
      Math.floor(new Date(s.Date).getTime() / 1000)
    );
    const deep = sorted.map((s) => s.Deep || null);
    const core = sorted.map((s) => s.Core || null);
    const rem = sorted.map((s) => s.REM || null);

    const opts: uPlot.Options = {
      width: 0,
      height: 250,
      series: [
        {},
        {
          label: "Deep (hr)",
          stroke: "#6366f1",
          fill: "rgba(99,102,241,0.3)",
          width: 2,
          paths: uPlot.paths.bars!({ size: [0.6, 100], gap: 2 }),
        },
        {
          label: "Core (hr)",
          stroke: "#3b82f6",
          fill: "rgba(59,130,246,0.3)",
          width: 2,
          paths: uPlot.paths.bars!({ size: [0.6, 100], gap: 2 }),
        },
        {
          label: "REM (hr)",
          stroke: "#8b5cf6",
          fill: "rgba(139,92,246,0.3)",
          width: 2,
          paths: uPlot.paths.bars!({ size: [0.6, 100], gap: 2 }),
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
          label: "hours",
          labelSize: 14,
        },
      ],
      scales: { x: { time: true } },
      cursor: { drag: { x: true, y: false } },
    };

    return {
      opts,
      plotData: [
        new Float64Array(times),
        deep,
        core,
        rem,
      ] as uPlot.AlignedData,
    };
  }, [sessions]);

  if (!opts || !plotData) {
    return (
      <div className="bg-zinc-900 rounded-lg p-6 text-zinc-500 text-sm">
        No sleep history data.
      </div>
    );
  }

  return (
    <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-4">
      <h3 className="text-sm font-medium text-zinc-400 mb-3">
        Sleep Stage History
      </h3>
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
