import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { HashRouter } from 'react-router-dom';
import { AuthProvider } from './shared/auth';
import { I18nProvider } from './shared/i18n';
import { ToastProvider } from './shared/toast';
import { ThemeProvider } from './shared/theme-provider';
import { NotificationProvider } from './shared/notification';
import { App } from './App';
import { initRUM } from './rum';
import './global.css';
import 'dockview/dist/styles/dockview.css';

initRUM();

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <I18nProvider>
      <HashRouter>
        <ThemeProvider>
          <NotificationProvider>
            <ToastProvider>
              <AuthProvider>
                <App />
              </AuthProvider>
            </ToastProvider>
          </NotificationProvider>
        </ThemeProvider>
      </HashRouter>
    </I18nProvider>
  </StrictMode>
);
