import { Workout } from "../../api";

const indoorNames: Record<string, string> = {
  Cycling: "Indoor Cycling",
  Running: "Indoor Run",
  Walking: "Indoor Walk",
  Swimming: "Pool Swim",
  Rowing: "Indoor Rowing",
};

const outdoorNames: Record<string, string> = {
  Cycling: "Outdoor Cycling",
  Running: "Outdoor Run",
  Walking: "Outdoor Walk",
  Swimming: "Open Water Swim",
  Rowing: "Outdoor Rowing",
};

export function getWorkoutDisplayName(w: Workout): string {
  if (w.alpha_session_name) {
    return w.alpha_session_name;
  }
  if (w.IsIndoor === true && w.Name in indoorNames) {
    return indoorNames[w.Name];
  }
  if (w.IsIndoor === false && w.Name in outdoorNames) {
    return outdoorNames[w.Name];
  }
  return w.Name;
}

export function getWorkoutFilterKey(w: Workout): string {
  // Use base workout type for filter pills, not the Alpha session name.
  const base = { ...w, alpha_session_name: undefined };
  return getWorkoutDisplayName(base);
}
