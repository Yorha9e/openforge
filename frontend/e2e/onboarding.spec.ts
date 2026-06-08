import { test, expect } from '@playwright/test';

/**
 * 09 — Onboarding persists role
 * Picking a role in the onboarding wizard must be persisted,
 * so a reload of the app skips the wizard and lands directly
 * on the dashboard for the chosen role.
 */
test('09 onboarding persists role', async ({ page }) => {
  await page.addInitScript(() => {
    window.localStorage.setItem('openforge.session', 'new-user-session');
    window.localStorage.removeItem('openforge.role');
  });
  await page.goto('/onboarding');

  // Step 1: pick a role.
  const roleOption = page.getByRole('radio', { name: /engineer/i });
  await expect(roleOption).toBeVisible();
  await roleOption.check();
  await page.getByRole('button', { name: /next|continue/i }).click();

  // Step 2: confirm and finish.
  const finishButton = page.getByRole('button', { name: /finish|done|get started/i });
  await expect(finishButton).toBeVisible();
  await finishButton.click();

  // We should be on the dashboard.
  await expect(page).toHaveURL(/\/dashboard$/);

  // Reload — wizard should not re-appear.
  await page.reload();
  await expect(page).toHaveURL(/\/dashboard$/);
  await expect(page.getByTestId('onboarding-wizard')).toHaveCount(0);
});

/**
 * 10 — Notification bell
 * The notification bell must render unread badge with a count
 * that decreases after the user opens the panel and marks
 * notifications as read.
 */
test('10 notification bell', async ({ page }) => {
  await page.addInitScript(() => {
    window.localStorage.setItem('openforge.session', 'demo-session');
  });
  await page.goto('/dashboard');

  const bell = page.getByRole('button', { name: /notifications/i });
  await expect(bell).toBeVisible();

  const badge = page.getByTestId('notification-badge');
  const initial = parseInt((await badge.textContent()) ?? '0', 10);
  expect(initial).toBeGreaterThan(0);

  // Open panel and mark all as read.
  await bell.click();
  const markAllRead = page.getByRole('button', { name: /mark all as read/i });
  await expect(markAllRead).toBeVisible();
  await markAllRead.click();

  // Badge should be gone or show 0.
  await expect(page.getByTestId('notification-badge')).toHaveText(/^0?$/);
});
