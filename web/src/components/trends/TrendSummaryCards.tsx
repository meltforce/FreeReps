import type { MetricStats, TimeSeriesPoint } from "../../api";
import { linearRegression } from "../../utils/stats";

interface MetricSummary {
  label: string;
  unit: string;
  color: string;
  stats: MetricStats | null;
  prevStats: MetricStats | null;
  points: TimeSeriesPoint[];
}

interface Props {
  metrics: MetricSummary[];
}

function fmt(v: number | null): string {
  if (v == null) return "—";
  if (Math.abs(v) >= 100) return v.toFixed(0);
  if (Math.abs(v) >= 10) return v.toFixed(1);
  return v.toFixed(2);
}

function pctChange(curr: number | null, prev: number | null): number | null {
  if (curr == null || prev == null || prev === 0) return null;
  return ((curr - prev) / Math.abs(prev)) * 100;
}

function trendDirection(points: TimeSeriesPoint[]): "up" | "down" | "flat" {
  const xs = points.map((_, i) => i);
  const ys = points.map((p) => p.avg);
  const reg = linearRegression(xs, ys);
  if (!reg) return "flat";
  const validYs = ys.filter((y): y is number => y != null);
  if (validYs.length === 0) return "flat";
  const mean = validYs.reduce((a, b) => a + b, 0) / validYs.length;
  if (mean === 0) return "flat";
  const relSlope = (reg.slope * points.length) / Math.abs(mean);
  if (Math.abs(relSlope) < 0.01) return "flat";
  return relSlope > 0 ? "up" : "down";
}

function changeColor(change: number): string {
  if (change > 0) return "text-green-400";
  if (change < 0) return "text-red-400";
  return "text-zinc-500";
}

function TrendArrow({ dir }: { dir: "up" | "down" | "flat" }) {
  if (dir === "up") return <span className="text-green-400">↑</span>;
  if (dir === "down") return <span className="text-red-400">↓</span>;
  return <span className="text-zinc-500">—</span>;
}

export default function TrendSummaryCards({ metrics }: Props) {
  return (
    <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-3">
      {metrics.map((m) => {
        const trend = trendDirection(m.points);
        const change = pctChange(m.stats?.avg ?? null, m.prevStats?.avg ?? null);

        return (
          <div
            key={m.label}
            className="bg-zinc-900 border border-zinc-800 rounded-lg p-3 space-y-1"
          >
            <div className="flex items-center gap-1.5">
              <span
                className="w-2 h-2 rounded-full shrink-0"
                style={{ backgroundColor: m.color }}
              />
              <span className="text-xs text-zinc-400 truncate">{m.label}</span>
            </div>

            <div className="flex items-baseline gap-1">
              <span className="text-lg font-semibold text-zinc-100 tabular-nums">
                {fmt(m.stats?.avg ?? null)}
              </span>
              <span className="text-xs text-zinc-500">{m.unit}</span>
              <TrendArrow dir={trend} />
            </div>

            <div className="flex gap-2 text-xs text-zinc-500 tabular-nums">
              <span>{fmt(m.stats?.min ?? null)}–{fmt(m.stats?.max ?? null)}</span>
            </div>

            {change != null && (
              <div className={`text-xs tabular-nums ${changeColor(change)}`}>
                {change > 0 ? "+" : ""}
                {change.toFixed(1)}% vs prev
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}
