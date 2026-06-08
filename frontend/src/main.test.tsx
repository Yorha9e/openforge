import { describe, it, expect } from 'vitest';
// Vite-style raw imports keep this test typecheck-clean without requiring
// @types/node or the node:fs / node:path built-ins.
// The repo has no vite/client.d.ts ambient file, so we type the imports
// via a runtime-only fallback (cast on first use).
import enUS from './shared/i18n/en-US.json';
import zhCN from './shared/i18n/zh-CN.json';
// @ts-expect-error -- Vite `?raw` import is runtime-supported; missing
// ambient type is a pre-existing repo gap (no vite-env.d.ts).
import mainSrc from './main.tsx?raw';
const mainSource = mainSrc as unknown as string;

/**
 * T10 — i18n Provider smoke tests.
 *
 * The DOM-rendering tests called for in the plan require
 * `@testing-library/react` and a JSDOM environment, neither of which is
 * currently installed in this repo.  Until they are, these tests act as a
 * static-presence smoke check: they verify (a) the I18nProvider module
 * exposes the expected API, (b) the bundled translation dictionaries
 * cover the keys callers depend on, and (c) main.tsx actually mounts the
 * provider around the router tree.
 *
 * Once `@testing-library/react` is added, replace the static `main.tsx`
 * check with the render-based assertions from the plan.
 */

describe('I18nProvider module', () => {
  it('exports I18nProvider and useI18n as callable functions', async () => {
    const mod = await import('./shared/i18n');
    expect(typeof mod.I18nProvider).toBe('function');
    expect(typeof mod.useI18n).toBe('function');
    expect(mod.I18nProvider.name).toBe('I18nProvider');
  });
});

describe('Translation dictionaries', () => {
  it('zh-CN.json covers the keys declared in en-US.json', () => {
    for (const key of Object.keys(enUS)) {
      expect(zhCN[key as keyof typeof zhCN], `zh-CN.json missing key: ${key}`).toBeTypeOf('string');
      expect(
        (zhCN[key as keyof typeof zhCN] as unknown as string).length,
        `zh-CN.json has empty value for: ${key}`,
      ).toBeGreaterThan(0);
    }
  });

  it('en-US.json covers the keys declared in zh-CN.json', () => {
    for (const key of Object.keys(zhCN)) {
      expect(enUS[key as keyof typeof enUS], `en-US.json missing key: ${key}`).toBeTypeOf('string');
      expect(
        (enUS[key as keyof typeof enUS] as unknown as string).length,
        `en-US.json has empty value for: ${key}`,
      ).toBeGreaterThan(0);
    }
  });
});

describe('main.tsx wraps the app with <I18nProvider>', () => {
  it('imports I18nProvider and uses it around the router tree', () => {
    expect(mainSource).toMatch(/from\s+['"]\.\/shared\/i18n['"]/);
    expect(mainSource).toMatch(/<I18nProvider[\s>]/);
    // Must wrap the router, not just sit beside it.
    const openIdx = mainSource.indexOf('<I18nProvider');
    const routerOpen = mainSource.indexOf('<HashRouter');
    const routerClose = mainSource.indexOf('</HashRouter>');
    const i18nClose = mainSource.indexOf('</I18nProvider>');
    expect(openIdx).toBeGreaterThanOrEqual(0);
    expect(routerOpen).toBeGreaterThan(openIdx);
    expect(routerClose).toBeGreaterThan(routerOpen);
    expect(i18nClose).toBeGreaterThan(routerClose);
  });
});
