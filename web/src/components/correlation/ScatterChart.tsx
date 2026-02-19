import { useMemo } from "react";
import UplotReact from "uplot-react";
import uPlot from "uplot";
import "uplot/dist/uPlot.min.css";
import { CorrelationPoint } from "../../api";
import { linearRegression } from "../../utils/stats";

interface Props {
  points: CorrelationPoint[];
  xLabel: string;
  yLabel: string;
}

export default function ScatterChart({ points, xLabel, yLabel }: Props) {
  const { opts, plotData } = useMemo(() => {
    if (!points || points.length === 0) return { opts: null, plotData: null };

    // Extract valid pairs
    const xs: number[] = [];
    const ys: number[] = [];
    for (const p of points) {
      if (p.x != null && p.y != null) {
        xs.push(p.x);
        ys.push(p.y);
      }
    }
    if (xs.length < 2) return { opts: null, plotData: null };

    // Compute regression line
    const reg = linearRegression(xs, ys);

    // For scatter, x-axis is not time — use x values directly
    // uPlot expects sorted x; scatter doesn't need it but we sort anyway
    const indices = xs.map((_, i) => i).sort((a, b) => xs[a] - xs[b]);
    const sortedXs = indices.map((i) => xs[i]);
    const sortedYs = indices.map((i) => ys[i]);

    // Regression line (two points: min and max x)
    const regYs: (number | null)[] = reg
      ? sortedXs.map((x) => reg.slope * x + reg.intercept)
      : sortedXs.map(() => null);

    const opts: uPlot.Options = {
      width: 0,
      height: 350,
      mode: 2, // scatter mode
      series: [
        {
          label: xLabel,
        },
        {
          label: yLabel,
          stroke: "#22d3ee",
          fill: "#22d3ee",
          paths: () => null,
          points: {
            show: true,
            size: 6,
            fill: "#22d3ee",
            stroke: "#22d3ee",
          },
        },
        {
          label: "Trend",
          stroke: "#a78bfa",
          width: 2,
          dash: [8, 4],
          points: { show: false },
        },
      ],
      axes: [
        {
          stroke: "#52525b",
          grid: { stroke: "#27272a", width: 1 },
          ticks: { stroke: "#27272a" },
          label: xLabel,
          labelSize: 14,
        },
        {
          stroke: "#52525b",
          grid: { stroke: "#27272a", width: 1 },
          ticks: { stroke: "#27272a" },
          label: yLabel,
          labelSize: 14,
        },
      ],
      scales: {
        x: { time: false },
      },
      cursor: { drag: { x: true, y: false } },
    };

    return {
      opts,
      plotData: [
        new Float64Array(sortedXs),
        sortedYs,
        regYs,
      ] as uPlot.AlignedData,
    };
  }, [points, xLabel, yLabel]);

  if (!opts || !plotData) {
    return (
      <div className="bg-zinc-900 rounded-lg p-6 text-zinc-500 text-sm">
        Not enough paired data points for scatter plot.
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
