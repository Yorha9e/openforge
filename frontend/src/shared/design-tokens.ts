/**
 * Design tokens for the OpenForge dark theme.
 *
 * @deprecated Use ThemeProvider + useTheme() from `theme-provider.tsx` instead.
 * The ThemeProvider dynamically resolves tokens for dark, light, and high-contrast
 * themes, and is the preferred way to access themed values throughout the app.
 */
export const tokens = {
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
} as const;
