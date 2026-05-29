import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import { AuthProvider } from './shared/auth';
import { ToastProvider } from './shared/toast';
import { ThemeProvider } from './shared/theme-provider';
import { App } from './App';
import { initRUM } from './rum';
import './global.css';
import 'dockview/dist/styles/dockview.css';

initRUM();

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <BrowserRouter>
      <ThemeProvider>
        <ToastProvider>
          <AuthProvider>
            <App />
          </AuthProvider>
        </ToastProvider>
      </ThemeProvider>
    </BrowserRouter>
  </StrictMode>
);
