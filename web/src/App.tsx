import { Routes, Route } from "react-router-dom";
import { lazy, Suspense } from "react";
import Layout from "./components/Layout";
import ErrorBoundary from "./components/ErrorBoundary";
import DashboardPage from "./pages/DashboardPage";

// Lazy-load heavier pages for code splitting
const SleepPage = lazy(() => import("./pages/SleepPage"));
const WorkoutsPage = lazy(() => import("./pages/WorkoutsPage"));
const WorkoutDetailPage = lazy(() => import("./pages/WorkoutDetailPage"));
const MetricsPage = lazy(() => import("./pages/MetricsPage"));
const CorrelationPage = lazy(() => import("./pages/CorrelationPage"));
const SettingsPage = lazy(() => import("./pages/SettingsPage"));

function PageFallback() {
  return (
    <div className="space-y-4 animate-pulse">
      <div className="h-8 w-48 bg-zinc-900 rounded" />
      <div className="h-64 bg-zinc-900 rounded-lg" />
    </div>
  );
}

function Page({ children }: { children: React.ReactNode }) {
  return (
    <ErrorBoundary>
      <Suspense fallback={<PageFallback />}>{children}</Suspense>
    </ErrorBoundary>
  );
}

export default function App() {
  return (
    <Layout>
      <Routes>
        <Route
          path="/"
          element={
            <ErrorBoundary>
              <DashboardPage />
            </ErrorBoundary>
          }
        />
        <Route
          path="/sleep"
          element={
            <Page>
              <SleepPage />
            </Page>
          }
        />
        <Route
          path="/workouts"
          element={
            <Page>
              <WorkoutsPage />
            </Page>
          }
        />
        <Route
          path="/workouts/:id"
          element={
            <Page>
              <WorkoutDetailPage />
            </Page>
          }
        />
        <Route
          path="/metrics"
          element={
            <Page>
              <MetricsPage />
            </Page>
          }
        />
        <Route
          path="/correlations"
          element={
            <Page>
              <CorrelationPage />
            </Page>
          }
        />
        <Route
          path="/settings"
          element={
            <Page>
              <SettingsPage />
            </Page>
          }
        />
      </Routes>
    </Layout>
  );
}
