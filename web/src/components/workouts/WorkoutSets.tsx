import { useQuery } from "@tanstack/react-query";
import { fetchWorkoutSets, WorkoutSet } from "../../api";

const STRENGTH_TYPES = new Set([
  "Traditional Strength Training",
  "Functional Strength Training",
  "High Intensity Interval Training",
  "Core Training",
]);

interface Props {
  workoutId: string;
  workoutName: string;
}

export default function WorkoutSets({ workoutId, workoutName }: Props) {
  const isStrength = STRENGTH_TYPES.has(workoutName);

  const { data, isLoading, error } = useQuery({
    queryKey: ["workoutSets", workoutId],
    queryFn: () => fetchWorkoutSets(workoutId),
    enabled: isStrength,
  });

  if (!isStrength) return null;

  if (isLoading) {
    return (
      <div className="bg-zinc-900 rounded-lg p-4 animate-pulse h-32" />
    );
  }

  if (error || !data || data.length === 0) {
    return null; // No sets data — silently hide
  }

  // Group sets by exercise name, preserving order
  const exercises: { name: string; equipment: string; sets: WorkoutSet[] }[] =
    [];
  const exerciseMap = new Map<string, number>();

  for (const set of data) {
    const key = set.ExerciseName;
    if (!exerciseMap.has(key)) {
      exerciseMap.set(key, exercises.length);
      exercises.push({ name: set.ExerciseName, equipment: set.Equipment, sets: [] });
    }
    exercises[exerciseMap.get(key)!].sets.push(set);
  }

  return (
    <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-4">
      <h3 className="text-sm font-medium text-zinc-400 mb-4">
        Exercises
      </h3>
      <div className="space-y-4">
        {exercises.map((ex) => (
          <div key={ex.name}>
            <div className="flex items-baseline gap-2 mb-2">
              <span className="text-sm font-medium text-zinc-200">
                {ex.name}
              </span>
              {ex.equipment && (
                <span className="text-xs text-zinc-500">{ex.equipment}</span>
              )}
            </div>
            <table className="w-full text-sm">
              <thead>
                <tr className="text-xs text-zinc-500 border-b border-zinc-800">
                  <th className="text-left py-1 w-12">Set</th>
                  <th className="text-right py-1">Weight</th>
                  <th className="text-right py-1">Reps</th>
                  <th className="text-right py-1">RIR</th>
                </tr>
              </thead>
              <tbody>
                {ex.sets.map((set, i) => (
                  <tr
                    key={i}
                    className={`border-b border-zinc-800/50 ${
                      set.IsWarmup ? "text-zinc-600" : "text-zinc-300"
                    }`}
                  >
                    <td className="py-1 tabular-nums">
                      {set.IsWarmup ? "W" : set.SetNumber}
                    </td>
                    <td className="text-right py-1 tabular-nums">
                      {set.WeightKg > 0
                        ? `${set.WeightKg.toFixed(1)} kg`
                        : set.IsBodyweightPlus
                          ? "BW"
                          : "-"}
                    </td>
                    <td className="text-right py-1 tabular-nums">{set.Reps}</td>
                    <td className="text-right py-1 tabular-nums">
                      {set.RIR >= 0 ? set.RIR.toFixed(1) : "-"}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ))}
      </div>
    </div>
  );
}
