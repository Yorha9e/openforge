import { Routes, Route, Navigate } from 'react-router-dom';
import { useAuth, useCanAccess } from './shared/auth';
import { LoginPage } from './features/login/LoginPage';
import { DashboardPage } from './features/dashboard/DashboardPage';
import { ProjectPage } from './features/project/ProjectPage';
import { ChatPanel } from './features/chat/ChatPanel';
import { ProModePage } from './features/code-review/ProModePage';
import { ReviewInboxPage } from './features/review-inbox/ReviewInboxPage';
import { CostDashboardPage } from './features/cost-dashboard/CostDashboardPage';
import { SettingsPage } from './features/settings/SettingsPage';
import { OnboardingFlow } from './features/onboarding/OnboardingFlow';
import { ErrorPage } from './features/errors/ErrorPage';
import { CircuitBreakerPage } from './features/errors/CircuitBreakerPage';
import { AdminPage } from './features/admin/AdminPage';
import { SkillPanel } from './features/admin/SkillPanel';

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { token } = useAuth();
  if (!token) return <Navigate to="/login" replace />;
  return <div className="page-enter">{children}</div>;
}

function AdminRoute({ children }: { children: React.ReactNode }) {
  const { token } = useAuth();
  const canAccess = useCanAccess('admin');
  if (!token) return <Navigate to="/login" replace />;
  if (!canAccess) return <Navigate to="/" replace />;
  return <div className="page-enter">{children}</div>;
}

export function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/" element={<ProtectedRoute><DashboardPage /></ProtectedRoute>} />
      <Route path="/project/:id" element={<ProtectedRoute><ProjectPage /></ProtectedRoute>} />
      <Route path="/project/:id/chat" element={<ProtectedRoute><ChatPanel /></ProtectedRoute>} />
      <Route path="/project/:id/pipeline/:pid" element={<ProtectedRoute><ProModePage /></ProtectedRoute>} />
      <Route path="/review-inbox" element={<ProtectedRoute><ReviewInboxPage /></ProtectedRoute>} />
      <Route path="/project/:id/costs" element={<ProtectedRoute><CostDashboardPage /></ProtectedRoute>} />
      <Route path="/settings" element={<ProtectedRoute><SettingsPage /></ProtectedRoute>} />
      <Route path="/onboarding" element={<ProtectedRoute><OnboardingFlow /></ProtectedRoute>} />
      <Route path="/error" element={<ErrorPage />} />
      <Route path="/circuit-breaker" element={<ProtectedRoute><CircuitBreakerPage /></ProtectedRoute>} />
      <Route path="/admin" element={<AdminRoute><AdminPage /></AdminRoute>} />
      <Route path="/admin/skills" element={<AdminRoute><SkillPanel /></AdminRoute>} />
    </Routes>
  );
}
