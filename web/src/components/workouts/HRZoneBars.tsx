import { WorkoutHR } from "../../api";

const ZONES = [
  { name: "Z1", label: "Zone 1", min: 0, max: 120, color: "#22d3ee" },
  { name: "Z2", label: "Zone 2", min: 120, max: 140, color: "#4ade80" },
  { name: "Z3", label: "Zone 3", min: 140, max: 155, color: "#facc15" },
  { name: "Z4", label: "Zone 4", min: 155, max: 170, color: "#fb923c" },
  { name: "Z5", label: "Zone 5", min: 170, max: 999, color: "#f87171" },
];

interface Props {
  hrData: WorkoutHR[];
}

export default function HRZoneBars({ hrData }: Props) {
  if (!hrData || hrData.length < 2) return null;

  // Count time in each zone
  const zoneSecs = ZONES.map(() => 0);
  for (let i = 1; i < hrData.length; i++) {
    const bpm = hrData[i].AvgBPM ?? hrData[i].MaxBPM ?? 0;
    if (!bpm) continue;
    const dt =
      (new Date(hrData[i].Time).getTime() -
        new Date(hrData[i - 1].Time).getTime()) /
      1000;
    if (dt <= 0 || dt > 300) continue; // skip gaps > 5 min
    const zoneIdx = ZONES.findIndex((z) => bpm < z.max);
    if (zoneIdx >= 0) zoneSecs[zoneIdx] += dt;
  }

  const totalSecs = zoneSecs.reduce((a, b) => a + b, 0);
  if (totalSecs === 0) return null;

  return (
    <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-4">
      <h3 className="text-sm font-medium text-zinc-400 mb-3">
        Time in HR Zones
      </h3>
      <div className="space-y-2">
        {ZONES.map((zone, i) => {
          const pct = (zoneSecs[i] / totalSecs) * 100;
          const mins = Math.round(zoneSecs[i] / 60);
          if (mins === 0 && pct < 1) return null;
          return (
            <div key={zone.name} className="flex items-center gap-3">
              <span className="text-xs text-zinc-500 w-10 shrink-0">
                {zone.label}
              </span>
              <div className="flex-1 h-5 bg-zinc-800 rounded-sm overflow-hidden">
                <div
                  className="h-full rounded-sm transition-all"
                  style={{
                    width: `${Math.max(pct, 1)}%`,
                    backgroundColor: zone.color,
                    opacity: 0.7,
                  }}
                />
              </div>
              <span className="text-xs text-zinc-400 w-14 text-right tabular-nums">
                {mins}m ({Math.round(pct)}%)
              </span>
            </div>
          );
        })}
      </div>
    </div>
  );
}
