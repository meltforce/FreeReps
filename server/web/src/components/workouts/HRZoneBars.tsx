import type { ReactNode } from "react";
import type { WorkoutHR } from "../../api";
import { ZONE_COLORS, resolveZoneEdges, zoneBands, zoneOf } from "../../utils/stageColors";

interface Props {
  hrData: WorkoutHR[];
  /** The server's zone edges; without them the session's own peak stands in. */
  zoneEdges?: number[] | null;
}

/**
 * Time in each zone, measured from the gaps between samples rather than by
 * counting them — a strength session's rest periods would otherwise inflate
 * the low zones.
 */
export default function HRZoneBars({ hrData, zoneEdges }: Props) {
  if (!hrData || hrData.length < 5) {
    return (
      <Section>
        <p style={{ color: "var(--color-neutral-600)", fontSize: 13, margin: 0 }}>
          Not enough heart rate data for zone analysis.
        </p>
      </Section>
    );
  }

  const peak = Math.max(...hrData.map((d) => d.MaxBPM ?? d.AvgBPM ?? 0));
  const edges = resolveZoneEdges(zoneEdges ?? undefined, peak);
  if (edges[0] <= 0) return null;
  const zoneSecs: number[] = new Array(ZONE_COLORS.length).fill(0);

  for (let i = 1; i < hrData.length; i++) {
    const bpm = hrData[i].AvgBPM ?? hrData[i].MaxBPM ?? 0;
    if (!bpm) continue;
    const dt =
      (new Date(hrData[i].Time).getTime() -
        new Date(hrData[i - 1].Time).getTime()) /
      1000;
    // Skip gaps over ten minutes: those are rests, not time in a zone.
    if (dt <= 0 || dt > 600) continue;
    zoneSecs[zoneOf(bpm, edges)] += dt;
  }

  const total = zoneSecs.reduce((a, b) => a + b, 0);
  if (total === 0) {
    return (
      <Section>
        <p style={{ color: "var(--color-neutral-600)", fontSize: 13, margin: 0 }}>
          Samples are too sparse to measure time in zones.
        </p>
      </Section>
    );
  }

  const bands = zoneBands(edges);

  return (
    <Section>
      {zoneSecs.map((secs, i) => {
        const pct = (secs / total) * 100;
        const mins = Math.round(secs / 60);
        if (mins === 0 && pct < 1) return null;
        return (
          <div
            key={i}
            style={{
              display: "flex",
              alignItems: "center",
              gap: 12,
              padding: "7px 0",
            }}
          >
            <span className="kick" style={{ width: 52, flex: "none" }}>
              Zone {i + 1}
            </span>
            <span
              className="num"
              style={{
                width: 70,
                flex: "none",
                font: "400 11.5px var(--font-body)",
                color: "var(--color-neutral-600)",
              }}
            >
              {bands[i]}
            </span>
            <div
              style={{
                flex: 1,
                height: 14,
                border: "1px solid var(--color-divider)",
              }}
            >
              <div
                style={{
                  width: `${Math.max(pct, 1)}%`,
                  height: "100%",
                  background: ZONE_COLORS[i],
                }}
              />
            </div>
            <span
              className="num"
              style={{
                width: 84,
                flex: "none",
                textAlign: "right",
                font: "500 12.5px var(--font-body)",
              }}
            >
              {mins}m
              <span style={{ color: "var(--color-neutral-600)", fontWeight: 400 }}>
                {" "}
                {Math.round(pct)}%
              </span>
            </span>
          </div>
        );
      })}
    </Section>
  );
}

function Section({ children }: { children: ReactNode }) {
  return (
    <div style={{ marginTop: 30 }}>
      <h2 style={{ fontSize: 19, fontWeight: 700 }}>Time in heart rate zones</h2>
      <div
        style={{
          borderTop: "2px solid var(--color-text)",
          marginTop: 12,
          paddingTop: 8,
        }}
      >
        {children}
      </div>
    </div>
  );
}
