/**
 * Sleep stages and heart rate zones share one sequential data ramp. The order
 * lives in lightness alone, so it survives greyscale and colour vision
 * deficiency. Zones take the steps by index; stages go through the
 * --color-stage-* aliases, because their mapping turns over with the theme —
 * Deep belongs at the dark end of the ramp and Awake at the light one. None of
 * these is the accent, which means brand and selection.
 *
 * One mapping, shared by the desktop and phone layouts.
 */

export const STAGE_COLOR: Record<string, string> = {
  Deep: "var(--color-stage-deep)",
  Core: "var(--color-stage-core)",
  REM: "var(--color-stage-rem)",
  Awake: "var(--color-stage-awake)",
};

/** Top to bottom in the hypnogram: shallowest first. */
export const STAGE_LANES = ["Awake", "REM", "Core", "Deep"] as const;

/** Darkest = deepest, so the composition bar reads as a depth ramp. */
export const STAGE_COMPOSITION_ORDER = ["Deep", "Core", "REM", "Awake"] as const;

export function stageColor(stage: string): string {
  return STAGE_COLOR[stage] ?? "var(--color-neutral-500)";
}

export const ZONE_COLORS = [
  "var(--color-data-1)",
  "var(--color-data-2)",
  "var(--color-data-3)",
  "var(--color-data-4)",
  "var(--color-data-5)",
];

/**
 * Zone boundaries as fractions of max heart rate. The server derives the real
 * edges from the heart rate reserve (storage.ZoneEdges); these fractions only
 * serve a session viewed before the server has reported its edges.
 */
export const ZONE_BOUNDS = [0.6, 0.7, 0.8, 0.9];

/** The four bpm edges from the server, or fractions of `peak` without them. */
export function resolveZoneEdges(edges: number[] | undefined, peak: number): number[] {
  if (edges && edges.length === ZONE_BOUNDS.length) return edges;
  return ZONE_BOUNDS.map((f) => f * peak);
}

export function zoneBands(zoneEdges: number[]): string[] {
  const edges = zoneEdges.map((e) => Math.round(e));
  return [
    `< ${edges[0]}`,
    `${edges[0]}–${edges[1]}`,
    `${edges[1]}–${edges[2]}`,
    `${edges[2]}–${edges[3]}`,
    `> ${edges[3]}`,
  ];
}

/** Which zone a heart rate lands in, 0-indexed. */
export function zoneOf(bpm: number, edges: number[]): number {
  for (let i = 0; i < edges.length; i++) {
    if (bpm < edges[i]) return i;
  }
  return edges.length;
}
