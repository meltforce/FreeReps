import { useMemo } from "react";
import uPlot from "uplot";
import "uplot/dist/uPlot.min.css";
import { WorkoutHR } from "../../api";
import AutoSizeUplot from "../AutoSizeUplot";

// HR Zone boundaries (bpm) and colors
const ZONES = [
  { name: "Z1", max: 120, color: "rgba(34,211,238,0.08)" }, // cyan
  { name: "Z2", max: 140, color: "rgba(74,222,128,0.08)" }, // green
  { name: "Z3", max: 155, color: "rgba(250,204,21,0.08)" }, // yellow
  { name: "Z4", max: 170, color: "rgba(251,146,60,0.08)" }, // orange
  { name: "Z5", max: 220, color: "rgba(248,113,113,0.08)" }, // red
];

interface Props {
  hrData: WorkoutHR[];
}

export default function HRTimelineChart({ hrData }: Props) {
  const { opts, plotData } = useMemo(() => {
    if (!hrData || hrData.length === 0) return { opts: null, plotData: null };

    const times = hrData.map((p) =>
      Math.floor(new Date(p.Time).getTime() / 1000)
    );
    const bpms = hrData.map((p) => p.AvgBPM ?? p.MaxBPM ?? p.MinBPM ?? null);

    const opts: uPlot.Options = {
      width: 0,
      height: 300,
      series: [
        {},
        {
          label: "HR (bpm)",
          stroke: "#ef4444",
          width: 1.5,
          fill: "rgba(239,68,68,0.06)",
        },
      ],
      axes: [
        {
          stroke: "#52525b",
          grid: { stroke: "#27272a", width: 1 },
          ticks: { stroke: "#27272a" },
          values: (_u: uPlot, vals: number[]) =>
            vals.map((v) => {
              const d = new Date(v * 1000);
              return `${d.getHours()}:${d.getMinutes().toString().padStart(2, "0")}`;
            }),
        },
        {
          stroke: "#52525b",
          grid: { stroke: "#27272a", width: 1 },
          ticks: { stroke: "#27272a" },
          label: "bpm",
          labelSize: 14,
        },
      ],
      scales: { x: { time: false } },
      cursor: { drag: { x: true, y: false } },
      hooks: {
        draw: [
          (u: uPlot) => {
            const ctx = u.ctx;
            const yScale = u.scales.y;
            if (!yScale.min || !yScale.max) return;

            const left = u.bbox.left;
            const width = u.bbox.width;
            let prevMax = yScale.min;

            for (const zone of ZONES) {
              const zoneTop = Math.min(zone.max, yScale.max);
              const zoneBottom = Math.max(prevMax, yScale.min);
              if (zoneBottom >= yScale.max || zoneTop <= yScale.min) {
                prevMax = zone.max;
                continue;
              }
              const top = u.valToPos(zoneTop, "y", true);
              const bottom = u.valToPos(zoneBottom, "y", true);
              ctx.fillStyle = zone.color;
              ctx.fillRect(left, top, width, bottom - top);
              prevMax = zone.max;
            }
          },
        ],
      },
    };

    return {
      opts,
      plotData: [new Float64Array(times), bpms] as uPlot.AlignedData,
    };
  }, [hrData]);

  if (!opts || !plotData) {
    return (
      <div className="bg-zinc-900 rounded-lg p-6 text-zinc-500 text-sm">
        No heart rate data for this workout.
      </div>
    );
  }

  return (
    <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-4">
      <h3 className="text-sm font-medium text-zinc-400 mb-3">
        Heart Rate Timeline
      </h3>
      <AutoSizeUplot opts={opts} data={plotData} />
    </div>
  );
}
