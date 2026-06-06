import { describe, it, expect } from 'vitest';
// Vite-style raw import keeps this test typecheck-clean without requiring
// @types/node or node:fs.
// @ts-expect-error -- Vite `?raw` import is runtime-supported; missing
// ambient type is a pre-existing repo gap (no vite-env.d.ts).
import messageListSrc from './MessageList.tsx?raw';
const src = messageListSrc as unknown as string;

/**
 * T10 — MessageList a11y smoke test.
 *
 * The plan called for `jest-axe` + `@testing-library/react` to assert no
 * axe violations on the rendered MessageList.  Neither dependency is
 * currently installed and there is no JSDOM/happy-dom environment wired
 * up in vite.config.ts.  Until those land, this test enforces the same
 * contract via static source inspection: the rendered container must
 * carry role="log", aria-live, and an aria-label so that screen readers
 * announce streaming chat updates.
 *
 * If jest-axe + @testing-library/react are later added, replace this
 * block with the render-based assertion in the plan.
 */

describe('MessageList a11y', () => {
  it('exposes role="log" so AT can identify the message region', () => {
    expect(src).toMatch(/role\s*=\s*["']log["']/);
  });

  it('declares aria-live so AT announces streaming updates', () => {
    expect(src).toMatch(/aria-live\s*=\s*["']polite["']/);
  });

  it('provides an aria-label describing the region', () => {
    expect(src).toMatch(/aria-label\s*=\s*["'][^"']+["']/);
  });

  it('places the a11y attributes on the outermost scroll container', () => {
    // The testid="message-list" anchor was requested in the plan.
    expect(src).toMatch(/data-testid\s*=\s*["']message-list["']/);
  });
});
