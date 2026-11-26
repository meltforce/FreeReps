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
const TrendsPage = lazy(() => import("./pages/TrendsPage"));
const SettingsPage = lazy(() => import("./pages/SettingsPage"));
const LoginPage = lazy(() => import("./pages/LoginPage"));

/* The shell is already on screen; this only reserves the page's own height so
   the tab bar does not jump up while the chunk loads. */
function PageFallback() {
  return (
    <div className="page-x" style={{ paddingTop: 22 }}>
      <span className="skel" style={{ width: 180, height: 34 }} />
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

/* Everything behind the login: the shell and the pages that query the API. */
function Shell() {
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
          path="/trends"
          element={
            <Page>
              <TrendsPage />
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

export default function App() {
  return (
    <Routes>
      {/* Outside Layout, which queries /me and would answer 401 here. */}
      <Route
        path="/login"
        element={
          <ErrorBoundary>
            <Suspense fallback={null}>
              <LoginPage />
            </Suspense>
          </ErrorBoundary>
        }
      />
      <Route path="*" element={<Shell />} />
    </Routes>
  );
}
