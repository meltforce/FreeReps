import { MetricStats } from "../../api";

function fmt(v: number | null): string {
  if (v == null) return "—";
  if (Math.abs(v) >= 100) return v.toFixed(0);
  if (Math.abs(v) >= 10) return v.toFixed(1);
  return v.toFixed(2);
}

export default function MetricStatsBar({ stats }: { stats: MetricStats }) {
  return (
    <div className="grid grid-cols-2 sm:grid-cols-5 gap-3">
      <Card label="Average" value={fmt(stats.avg)} />
      <Card label="Min" value={fmt(stats.min)} />
      <Card label="Max" value={fmt(stats.max)} />
      <Card label="Std Dev" value={fmt(stats.stddev)} />
      <Card label="Count" value={stats.count.toString()} />
    </div>
  );
}

function Card({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-3">
      <div className="text-xs text-zinc-500">{label}</div>
      <div className="text-lg font-semibold text-zinc-100 tabular-nums">
        {value}
      </div>
    </div>
  );
}
