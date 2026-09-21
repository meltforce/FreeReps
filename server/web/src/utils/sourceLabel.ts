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
 * Names the provider that recorded a workout, rather than the path it arrived
 * on.
 *
 * A session recorded in the Oura app reaches FreeReps twice: over the Oura API,
 * and as the copy the app writes into HealthKit, which Health Auto Export
 * forwards without a source name. The list shows whichever row the source
 * priority ranks first — the HealthKit copy, which carries the GPS track and
 * the denser heart rate series — and labelling that row "Apple Health" names
 * the hub instead of the recorder.
 *
 * `sources` holds every source of the workout's 5-minute window. A named one
 * among them is the recorder, because Apple Health is the only source that
 * arrives without a name.
 */
export function recordedBy(
  source: string | null | undefined,
  sources: string[] | null | undefined,
): string {
  const named = (sources ?? []).filter((s) => s !== "");
  if ((source == null || source === "") && named.length > 0) {
    return named.join(" / ");
  }
  return sourceLabel(source);
}

/** The delivery path behind a relabelled row, for a title attribute. */
export function recordedByTitle(
  source: string | null | undefined,
  sources: string[] | null | undefined,
): string | undefined {
  const named = (sources ?? []).filter((s) => s !== "");
  if ((source == null || source === "") && named.length > 0) {
    return `Recorded by ${named.join(" / ")}, shown from the copy delivered through Apple Health`;
  }
  return undefined;
}
