import { SleepSession } from "../../api";

function hoursToHHMM(hours: number): string {
  const h = Math.floor(hours);
  const m = Math.round((hours - h) * 60);
  return `${h}h ${m.toString().padStart(2, "0")}m`;
}

const STAGE_COLORS: Record<string, string> = {
  deep: "bg-indigo-500",
  core: "bg-blue-500",
  rem: "bg-violet-500",
  awake: "bg-amber-500",
};

const STAGE_LABELS: Record<string, string> = {
  deep: "Deep",
  core: "Core",
  rem: "REM",
  awake: "Awake",
};

export default function SleepMetricCards({
  session,
}: {
  session: SleepSession;
}) {
  const efficiency =
    session.InBed > 0
      ? Math.round((session.TotalSleep / session.InBed) * 100)
      : null;

  const stages = [
    { key: "deep", hours: session.Deep },
    { key: "core", hours: session.Core },
    { key: "rem", hours: session.REM },
  ].filter((s) => s.hours > 0);

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <Card label="Total Sleep" value={hoursToHHMM(session.TotalSleep)} />
        <Card label="Time in Bed" value={hoursToHHMM(session.InBed)} />
        <Card
          label="Efficiency"
          value={efficiency !== null ? `${efficiency}%` : "—"}
        />
        <Card
          label="Bedtime"
          value={
            session.SleepStart
              ? new Date(session.SleepStart).toLocaleTimeString([], {
                  hour: "2-digit",
                  minute: "2-digit",
                })
              : "—"
          }
        />
      </div>

      {stages.length > 0 && (
        <div className="flex flex-wrap gap-3">
          {stages.map((s) => (
            <div key={s.key} className="flex items-center gap-2">
              <span
                className={`w-3 h-3 rounded-full ${STAGE_COLORS[s.key]}`}
              />
              <span className="text-sm text-zinc-400">
                {STAGE_LABELS[s.key]}
              </span>
              <span className="text-sm font-medium text-zinc-200">
                {hoursToHHMM(s.hours)}
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function Card({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-4">
      <div className="text-xs text-zinc-500 mb-1">{label}</div>
      <div className="text-xl font-semibold text-zinc-100 tabular-nums">
        {value}
      </div>
    </div>
  );
}
