/**
 * Names a metric's source for display.
 *
 * An empty source is not an unknown one: HealthKit writes through Health Auto
 * Export without setting the field, and the source-priority rules match it as
 * the empty string. Showing "—" there would claim the origin is unrecorded when
 * it is in fact the phone.
 */
export function sourceLabel(source: string | null | undefined): string {
  if (source == null) return "—";
  if (source === "") return "Apple Health";
  return source;
}

/** The same, spelled out where there is room for the mechanism. */
export function sourceLabelLong(source: string | null | undefined): string {
  if (source === "") return "Apple Health (HealthKit)";
  return sourceLabel(source);
}

/**
 * The delivery path behind a relabelled row, for a title attribute.
 *
 * The label itself comes from the server as `Workout.RecordedBy`; see
 * `models.RecordedBy` in `internal/models/sources.go`, which holds the rule so
 * that the workout list and the MCP tool name the same provider. A row is
 * relabelled when that name differs from what its own source would read as.
 */
export function recordedByTitle(
  source: string | null | undefined,
  recordedBy: string | null | undefined,
): string | undefined {
  if (recordedBy && recordedBy !== sourceLabel(source)) {
    return `Recorded by ${recordedBy}, shown from the copy delivered through Apple Health`;
  }
  return undefined;
}
