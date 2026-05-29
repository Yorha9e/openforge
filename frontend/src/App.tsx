import { lazy, Suspense, useEffect, useState } from 'react';
import { Routes, Route, Navigate } from 'react-router-dom';
import { useAuth, useCanAccess } from './shared/auth';
import { api, FeatureFlags } from './shared/api';
import { LoginPage } from './features/login/LoginPage';
import { DashboardPage } from './features/dashboard/DashboardPage';
import { ProjectPage } from './features/project/ProjectPage';
import { ChatPanel } from './features/chat/ChatPanel';

const ProModePage = lazy(() => import('./features/code-review/ProModePage'));
const CostDashboardPage = lazy(() => import('./features/cost-dashboard/CostDashboardPage'));
const ReviewInboxPage = lazy(() => import('./features/review-inbox/ReviewInboxPage'));
const SettingsPage = lazy(() => import('./features/settings/SettingsPage'));
const OnboardingFlow = lazy(() => import('./features/onboarding/OnboardingFlow'));
const ErrorPage = lazy(() => import('./features/errors/ErrorPage'));
const CircuitBreakerPage = lazy(() => import('./features/errors/CircuitBreakerPage'));
const AdminPage = lazy(() => import('./features/admin/AdminPage'));
const SkillPanel = lazy(() => import('./features/admin/SkillPanel'));
const ComplianceReportPage = lazy(() => import('./features/compliance/ComplianceReportPage'));
const GrafanaPage = lazy(() => import('./features/monitoring/GrafanaPage'));
const ADRPage = lazy(() => import('./features/adr/ADRPage'));

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

function LoadingFallback() {
  return (
    <div style={{
      minHeight: '100vh', background: '#0F172A', display: 'flex',
      alignItems: 'center', justifyContent: 'center', color: '#94a3b8',
      fontFamily: "'Fira Sans', sans-serif", fontSize: 14,
    }}>
      Loading...
    </div>
  );
}

export function App() {
  const [featureFlags, setFeatureFlags] = useState<FeatureFlags | null>(null);
  const [isElectron, setIsElectron] = useState(false);
  const [connectionStatus, setConnectionStatus] = useState<string>('disconnected');

  useEffect(() => {
    // Load feature flags for conditional routing
    api.getFeatureFlags()
      .then(flags => setFeatureFlags(flags))
      .catch(() => {}); // Silently fail - flags default to false
    
    // Detect Electron environment
    if (window.electronAPI) {
      setIsElectron(true);
      
      // Get initial connection status
      window.electronAPI.getConnectionStatus()
        .then(status => setConnectionStatus(status))
        .catch(() => {});
      
      // Listen for connection status changes
      const unsubscribe = window.electronAPI.onConnectionChange((status: string) => {
        setConnectionStatus(status);
      });
      
      return () => {
        if (typeof unsubscribe === 'function') {
          unsubscribe();
        }
      };
    }
    return undefined;
  }, []);

  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/" element={<ProtectedRoute><DashboardPage /></ProtectedRoute>} />
      <Route path="/project/:id" element={<ProtectedRoute><ProjectPage /></ProtectedRoute>} />
      <Route path="/project/:id/chat" element={<ProtectedRoute><ChatPanel /></ProtectedRoute>} />
      <Route path="/project/:id/pipeline/:pid" element={
        <ProtectedRoute><Suspense fallback={<LoadingFallback />}><ProModePage /></Suspense></ProtectedRoute>
      } />
      <Route path="/project/:id/costs" element={
        <ProtectedRoute><Suspense fallback={<LoadingFallback />}><CostDashboardPage /></Suspense></ProtectedRoute>
      } />
      <Route path="/review-inbox" element={
        <ProtectedRoute><Suspense fallback={<LoadingFallback />}><ReviewInboxPage /></Suspense></ProtectedRoute>
      } />
      <Route path="/settings" element={
        <ProtectedRoute><Suspense fallback={<LoadingFallback />}><SettingsPage /></Suspense></ProtectedRoute>
      } />
      <Route path="/onboarding" element={
        <Suspense fallback={<LoadingFallback />}><OnboardingFlow /></Suspense>
      } />
      <Route path="/error" element={<Suspense fallback={<LoadingFallback />}><ErrorPage /></Suspense>} />
      <Route path="/circuit-breaker" element={
        <ProtectedRoute><Suspense fallback={<LoadingFallback />}><CircuitBreakerPage /></Suspense></ProtectedRoute>
      } />
      <Route path="/admin" element={
        <AdminRoute><Suspense fallback={<LoadingFallback />}><AdminPage /></Suspense></AdminRoute>
      } />
      <Route path="/admin/skills" element={
        <AdminRoute><Suspense fallback={<LoadingFallback />}><SkillPanel /></Suspense></AdminRoute>
      } />
      
      {/* Compliance suite: conditionally registered when compliance_suite flag is ON */}
      {featureFlags?.compliance_suite && (
        <Route path="/compliance" element={
          <ProtectedRoute><Suspense fallback={<LoadingFallback />}><ComplianceReportPage /></Suspense></ProtectedRoute>
        } />
      )}
      
      {/* Production ops: conditionally registered when production_ops flag is ON */}
      {featureFlags?.production_ops && (
        <Route path="/monitoring" element={
          <ProtectedRoute><Suspense fallback={<LoadingFallback />}><GrafanaPage /></Suspense></ProtectedRoute>
        } />
      )}
      
      {/* Distribution artifacts: conditionally registered when distribution_artifacts flag is ON */}
      {featureFlags?.distribution_artifacts && (
        <Route path="/adr" element={
          <ProtectedRoute><Suspense fallback={<LoadingFallback />}><ADRPage /></Suspense></ProtectedRoute>
        } />
      )}
    </Routes>
  );
}
