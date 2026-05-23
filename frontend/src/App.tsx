import { Routes, Route, Navigate } from 'react-router-dom';
import { useAuth } from './shared/auth';
import { LoginPage } from './features/login/LoginPage';
import { DashboardPage } from './features/dashboard/DashboardPage';
import { ProjectPage } from './features/project/ProjectPage';
import { ChatPanel } from './features/chat/ChatPanel';
import { ProModePage } from './features/code-review/ProModePage';
import { ReviewInboxPage } from './features/review-inbox/ReviewInboxPage';
import { CostDashboardPage } from './features/cost-dashboard/CostDashboardPage';

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { token } = useAuth();
  if (!token) return <Navigate to="/login" replace />;
  return <>{children}</>;
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
    </Routes>
  );
}
