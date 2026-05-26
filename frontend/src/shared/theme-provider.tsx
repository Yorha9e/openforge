import { createContext, useContext, useState, useEffect, type ReactNode } from 'react';

export type Theme = 'dark' | 'light' | 'high-contrast';

export interface ThemeTokens {
  bg: string;
  surface: string;
  border: string;
  text: string;
  muted: string;
  cta: string;
  ctaHover: string;
  ctaText: string;
  error: string;
  warning: string;
  userBubble: string;
  fontHeading: string;
  fontBody: string;
  transition: string;
}

const darkTokens: ThemeTokens = {
  bg: '#0F172A',
  surface: '#1E293B',
  border: '#334155',
  text: '#F8FAFC',
  muted: '#94a3b8',
  cta: '#22C55E',
  ctaHover: '#16A34A',
  ctaText: '#0F172A',
  error: '#EF4444',
  warning: '#F59E0B',
  userBubble: '#334155',
  fontHeading: "'Fira Code', monospace",
  fontBody: "'Fira Sans', sans-serif",
  transition: 'background 200ms, border-color 200ms, opacity 200ms, color 200ms',
};

const lightTokens: ThemeTokens = {
  bg: '#F8FAFC',
  surface: '#FFFFFF',
  border: '#E2E8F0',
  text: '#0F172A',
  muted: '#64748B',
  cta: '#16A34A',
  ctaHover: '#15803D',
  ctaText: '#FFFFFF',
  error: '#DC2626',
  warning: '#D97706',
  userBubble: '#F1F5F9',
  fontHeading: "'Fira Code', monospace",
  fontBody: "'Fira Sans', sans-serif",
  transition: 'background 200ms, border-color 200ms, opacity 200ms, color 200ms',
};

const highContrastTokens: ThemeTokens = {
  bg: '#000000',
  surface: '#1A1A1A',
  border: '#FFFFFF',
  text: '#FFFFFF',
  muted: '#CCCCCC',
  cta: '#00FF00',
  ctaHover: '#00CC00',
  ctaText: '#000000',
  error: '#FF0000',
  warning: '#FFFF00',
  userBubble: '#333333',
  fontHeading: "'Fira Code', monospace",
  fontBody: "'Fira Sans', sans-serif",
  transition: 'background 200ms, border-color 200ms, opacity 200ms, color 200ms',
};

const tokenMap: Record<Theme, ThemeTokens> = {
  dark: darkTokens,
  light: lightTokens,
  'high-contrast': highContrastTokens,
};

interface ThemeContextValue {
  theme: Theme;
  setTheme: (t: Theme) => void;
  tokens: ThemeTokens;
}

const ThemeContext = createContext<ThemeContextValue | null>(null);

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setTheme] = useState<Theme>(() => {
    const saved = localStorage.getItem('of_theme');
    if (saved === 'light' || saved === 'dark' || saved === 'high-contrast') {
      return saved;
    }
    return 'dark';
  });

  useEffect(() => {
    localStorage.setItem('of_theme', theme);
  }, [theme]);

  const value: ThemeContextValue = {
    theme,
    setTheme,
    tokens: tokenMap[theme],
  };

  return (
    <ThemeContext.Provider value={value}>
      {children}
    </ThemeContext.Provider>
  );
}

export function useTheme(): ThemeContextValue {
  const ctx = useContext(ThemeContext);
  if (!ctx) {
    throw new Error('useTheme must be used within a <ThemeProvider>');
  }
  return ctx;
}
