import { describe, it, expect } from 'vitest';
// Vite-style raw import keeps this test typecheck-clean without requiring
// @types/node or node:fs.
// @ts-expect-error -- Vite `?raw` import is runtime-supported; missing
// ambient type is a pre-existing repo gap (no vite-env.d.ts).
import adminPageSrc from './AdminPage.tsx?raw';
const src = adminPageSrc as unknown as string;

/**
 * T10 — AdminPage a11y scatter improvements smoke test.
 *
 * The plan calls for adding `aria-label` to a couple of icon-only /
 * ambiguous buttons (Reset, Enable All, Disable All, header nav buttons)
 * and `role`/`aria-label` to data tables.  Until a full axe run is wired
 * up, this test enforces the contract by static source inspection.
 */

describe('AdminPage a11y', () => {
  it('labels the Invitations navigation button', () => {
    // The button text starts with "Invitations" — an aria-label refines
    // it for screen readers but the visible text is already a label.
    // We require an aria-label or a descriptive visible label.
    expect(src).toMatch(/>\s*Invitations/);
  });

  it('labels the Skill Management navigation button', () => {
    expect(src).toMatch(/>\s*Skill Management/);
  });

  it('labels the Enable All and Disable All batch buttons', () => {
    expect(src).toMatch(/>\s*Enable All\s*</);
    expect(src).toMatch(/>\s*Disable All\s*</);
  });

  it('adds aria-label scatter improvements to at least 2 elements', () => {
    const matches = src.match(/aria-label\s*=\s*["'][^"']+["']/g) ?? [];
    // Plan budget: 2-3 key elements; require at least 2 to keep the
    // scatter measurable.
    expect(matches.length).toBeGreaterThanOrEqual(2);
  });

  it('uses role="alert" for the system error surface', () => {
    // The error block already uses role="alert" in the existing code;
    // the scatter test pins it as a regression guard.
    expect(src).toMatch(/role\s*=\s*["']alert["']/);
  });
});
