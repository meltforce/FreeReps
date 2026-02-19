import { Routes, Route } from "react-router-dom";
import Layout from "./components/Layout";
import DashboardPage from "./pages/DashboardPage";
import SleepPage from "./pages/SleepPage";
import WorkoutsPage from "./pages/WorkoutsPage";
import WorkoutDetailPage from "./pages/WorkoutDetailPage";
import MetricsPage from "./pages/MetricsPage";
import CorrelationPage from "./pages/CorrelationPage";

export default function App() {
  return (
    <Layout>
      <Routes>
        <Route path="/" element={<DashboardPage />} />
        <Route path="/sleep" element={<SleepPage />} />
        <Route path="/workouts" element={<WorkoutsPage />} />
        <Route path="/workouts/:id" element={<WorkoutDetailPage />} />
        <Route path="/metrics" element={<MetricsPage />} />
        <Route path="/correlations" element={<CorrelationPage />} />
      </Routes>
    </Layout>
  );
}
