import { SleepStage } from "../../api";

const STAGE_ORDER: Record<string, number> = {
  awake: 0,
  rem: 1,
  core: 2,
  deep: 3,
};

const STAGE_COLORS: Record<string, string> = {
  deep: "#6366f1", // indigo-500
  core: "#3b82f6", // blue-500
  rem: "#8b5cf6", // violet-500
  awake: "#f59e0b", // amber-500
};

const STAGE_LABELS = ["Awake", "REM", "Core", "Deep"];

interface Props {
  stages: SleepStage[];
}

export default function Hypnogram({ stages }: Props) {
  if (stages.length === 0) {
    return (
      <div className="bg-zinc-900 rounded-lg p-6 text-zinc-500 text-sm">
        No sleep stage data available.
      </div>
    );
  }

  const startMs = new Date(stages[0].StartTime).getTime();
  const endMs = Math.max(
    ...stages.map((s) => new Date(s.EndTime).getTime())
  );
  const totalMs = endMs - startMs;
  if (totalMs <= 0) return null;

  // Generate hour labels
  const startHour = new Date(startMs);
  startHour.setMinutes(0, 0, 0);
  const hourLabels: { time: string; pct: number }[] = [];
  let h = new Date(startHour.getTime() + 3600000);
  while (h.getTime() < endMs) {
    const pct = ((h.getTime() - startMs) / totalMs) * 100;
    if (pct > 0 && pct < 100) {
      hourLabels.push({
        time: h.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" }),
        pct,
      });
    }
    h = new Date(h.getTime() + 3600000);
  }

  return (
    <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-4">
      <h3 className="text-sm font-medium text-zinc-400 mb-3">Hypnogram</h3>

      <div className="flex gap-2">
        {/* Y-axis labels */}
        <div className="flex flex-col justify-between py-1 text-xs text-zinc-500 shrink-0 w-12">
          {STAGE_LABELS.map((label) => (
            <span key={label}>{label}</span>
          ))}
        </div>

        {/* Chart area */}
        <div className="flex-1 min-w-0">
          <div className="relative h-32">
            {stages.map((stage, i) => {
              const sMs = new Date(stage.StartTime).getTime();
              const eMs = new Date(stage.EndTime).getTime();
              const left = ((sMs - startMs) / totalMs) * 100;
              const width = ((eMs - sMs) / totalMs) * 100;
              const stageIdx = STAGE_ORDER[stage.Stage] ?? 1;
              const top = (stageIdx / 4) * 100;
              const height = 25; // each lane is 25%
              const color = STAGE_COLORS[stage.Stage] ?? "#71717a";

              return (
                <div
                  key={i}
                  className="absolute rounded-sm"
                  style={{
                    left: `${left}%`,
                    width: `${Math.max(width, 0.3)}%`,
                    top: `${top}%`,
                    height: `${height}%`,
                    backgroundColor: color,
                    opacity: 0.85,
                  }}
                  title={`${stage.Stage} ${new Date(stage.StartTime).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })} - ${new Date(stage.EndTime).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}`}
                />
              );
            })}
          </div>

          {/* X-axis time labels */}
          <div className="relative h-5 mt-1">
            {hourLabels.map((hl) => (
              <span
                key={hl.pct}
                className="absolute text-xs text-zinc-500 -translate-x-1/2"
                style={{ left: `${hl.pct}%` }}
              >
                {hl.time}
              </span>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
