import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";
import {
  fetchDashboardInit,
  type DashboardInitResponse,
  type LatestMetricsResponse,
} from "../api";

/**
 * Fetches available metrics, latest metrics, and daily sums in a single
 * request via /api/v1/dashboard/init. Seeds the individual React Query
 * caches so useAvailableMetrics and DailyOverview work without extra fetches.
 */
export function useDashboardInit() {
  const queryClient = useQueryClient();

  const query = useQuery<DashboardInitResponse>({
    queryKey: ["dashboard-init"],
    queryFn: fetchDashboardInit,
    staleTime: 60_000,
  });

  // Seed individual caches when the combined response arrives.
  useEffect(() => {
    if (!query.data) return;

    queryClient.setQueryData(
      ["available-metrics"],
      query.data.available_metrics
    );

    queryClient.setQueryData(["latestMetrics"], {
      latest: query.data.latest,
      daily_sums: query.data.daily_sums,
    } satisfies LatestMetricsResponse);
  }, [query.data, queryClient]);

  return query;
}
