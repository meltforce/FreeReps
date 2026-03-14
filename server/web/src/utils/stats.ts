/** Compute a simple moving average with the given window size. */
export function movingAverage(
  values: (number | null)[],
  window: number
): (number | null)[] {
  const result: (number | null)[] = [];
  for (let i = 0; i < values.length; i++) {
    let sum = 0;
    let count = 0;
    for (let j = Math.max(0, i - window + 1); j <= i; j++) {
      if (values[j] != null) {
        sum += values[j]!;
        count++;
      }
    }
    result.push(count > 0 ? sum / count : null);
  }
  return result;
}

/** Check if a value is an outlier relative to mean +/- threshold * stddev. */
export function isOutlier(
  value: number | null,
  mean: number,
  stddev: number,
  threshold: number = 1.5
): boolean {
  if (value == null || stddev === 0) return false;
  return Math.abs(value - mean) > threshold * stddev;
}

/** Compute Pearson correlation coefficient for paired values. */
export function pearsonR(
  xs: (number | null)[],
  ys: (number | null)[]
): number | null {
  const pairs: [number, number][] = [];
  for (let i = 0; i < Math.min(xs.length, ys.length); i++) {
    if (xs[i] != null && ys[i] != null) {
      pairs.push([xs[i]!, ys[i]!]);
    }
  }
  if (pairs.length < 3) return null;

  const n = pairs.length;
  let sumX = 0,
    sumY = 0,
    sumXY = 0,
    sumX2 = 0,
    sumY2 = 0;
  for (const [x, y] of pairs) {
    sumX += x;
    sumY += y;
    sumXY += x * y;
    sumX2 += x * x;
    sumY2 += y * y;
  }

  const denom = Math.sqrt(
    (n * sumX2 - sumX * sumX) * (n * sumY2 - sumY * sumY)
  );
  if (denom === 0) return null;
  return (n * sumXY - sumX * sumY) / denom;
}

/** Compute linear regression (slope and intercept) for paired values. */
export function linearRegression(
  xs: (number | null)[],
  ys: (number | null)[]
): { slope: number; intercept: number } | null {
  const pairs: [number, number][] = [];
  for (let i = 0; i < Math.min(xs.length, ys.length); i++) {
    if (xs[i] != null && ys[i] != null) {
      pairs.push([xs[i]!, ys[i]!]);
    }
  }
  if (pairs.length < 2) return null;

  const n = pairs.length;
  let sumX = 0,
    sumY = 0,
    sumXY = 0,
    sumX2 = 0;
  for (const [x, y] of pairs) {
    sumX += x;
    sumY += y;
    sumXY += x * y;
    sumX2 += x * x;
  }

  const denom = n * sumX2 - sumX * sumX;
  if (denom === 0) return null;

  const slope = (n * sumXY - sumX * sumY) / denom;
  const intercept = (sumY - slope * sumX) / n;
  return { slope, intercept };
}
