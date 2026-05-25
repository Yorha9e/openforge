import {
  createContext,
  createElement,
  useContext,
  useState,
  useCallback,
  type ReactNode,
  type ReactElement,
} from 'react';
import zhCN from './zh-CN.json';
import enUS from './en-US.json';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type Locale = 'zh-CN' | 'en-US';

interface I18nContextValue {
  /** Look up a translation key. Falls back to the key itself when missing. */
  t: (key: string) => string;
  /** Currently active locale. */
  locale: Locale;
  /** Switch locale (persisted to localStorage). */
  setLocale: (locale: Locale) => void;
}

// ---------------------------------------------------------------------------
// Data
// ---------------------------------------------------------------------------

const translations: Record<Locale, Record<string, string>> = {
  'zh-CN': zhCN as Record<string, string>,
  'en-US': enUS as Record<string, string>,
};

function getInitialLocale(): Locale {
  try {
    const stored = localStorage.getItem('of_locale');
    if (stored === 'zh-CN' || stored === 'en-US') return stored;
  } catch {
    /* localStorage unavailable */
  }
  return 'zh-CN';
}

// ---------------------------------------------------------------------------
// Context
// ---------------------------------------------------------------------------

const I18nContext = createContext<I18nContextValue | null>(null);

// ---------------------------------------------------------------------------
// Provider
// ---------------------------------------------------------------------------

export function I18nProvider({ children }: { children: ReactNode }): ReactElement {
  const [locale, setLocaleState] = useState<Locale>(getInitialLocale);

  const setLocale = useCallback((next: Locale) => {
    setLocaleState(next);
    try {
      localStorage.setItem('of_locale', next);
    } catch {
      /* noop */
    }
  }, []);

  const t = useCallback(
    (key: string): string => translations[locale]?.[key] ?? key,
    [locale],
  );

  return createElement(I18nContext.Provider, { value: { t, locale, setLocale } }, children);
}

// ---------------------------------------------------------------------------
// Hook
// ---------------------------------------------------------------------------

export function useI18n(): I18nContextValue {
  const ctx = useContext(I18nContext);
  if (!ctx) throw new Error('useI18n must be used within <I18nProvider>');
  return ctx;
}
