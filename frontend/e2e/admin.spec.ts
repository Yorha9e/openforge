import { test, expect } from '@playwright/test';

/**
 * 07 — Admin toggle flag
 * An admin can flip a feature flag in the admin panel and
 * the new state should be reflected in the UI (and persisted
 * across a reload).
 */
test('07 admin toggle flag', async ({ page }) => {
  await page.addInitScript(() => {
    window.localStorage.setItem('openforge.session', 'admin-session');
  });
  await page.goto('/admin/flags');

  const flagRow = page.getByTestId('flag-row').filter({ hasText: 'new-agent' });
  await expect(flagRow).toBeVisible();

  const toggle = flagRow.getByRole('switch');
  const initial = await toggle.isChecked();
  await toggle.click();
  await expect(toggle).toBeChecked({ checked: !initial });

  // Reload and confirm persistence.
  await page.reload();
  const toggleAfter = page
    .getByTestId('flag-row')
    .filter({ hasText: 'new-agent' })
    .getByRole('switch');
  await expect(toggleAfter).toBeChecked({ checked: !initial });
});

/**
 * 08 — Runbook page
 * The runbook page must list at least one runbook and
 * navigating to its detail view should render Markdown content
 * (with at least one heading).
 */
test('08 runbook page', async ({ page }) => {
  await page.addInitScript(() => {
    window.localStorage.setItem('openforge.session', 'admin-session');
  });
  await page.goto('/admin/runbooks');

  const list = page.getByTestId('runbook-list');
  await expect(list).toBeVisible();

  const firstRunbook = list.getByTestId('runbook-item').first();
  await expect(firstRunbook).toBeVisible();
  await firstRunbook.click();

  // Detail page renders markdown — at least one heading.
  await expect(
    page.getByRole('heading', { level: 1 }).or(page.getByRole('heading', { level: 2 })).first(),
  ).toBeVisible();
});
