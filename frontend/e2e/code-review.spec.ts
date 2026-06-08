import { test, expect } from '@playwright/test';

/**
 * 05 — ProMode opens Diff
 * Opening a code-review task in ProMode must surface a Diff view
 * comparing the proposed change against the current branch HEAD.
 */
test('05 ProMode opens Diff', async ({ page }) => {
  await page.addInitScript(() => {
    window.localStorage.setItem('openforge.session', 'demo-session');
  });
  await page.goto('/code-review');

  // Switch to ProMode via the toggle.
  const proToggle = page.getByRole('switch', { name: /pro mode/i });
  await expect(proToggle).toBeVisible();
  await proToggle.check();

  // Pick the first review task.
  const firstTask = page.getByTestId('review-task').first();
  await expect(firstTask).toBeVisible();
  await firstTask.click();

  // The diff panel must render with at least one added/removed line.
  const diff = page.getByTestId('diff-view');
  await expect(diff).toBeVisible();
  await expect(diff.locator('[data-diff-line]').first()).toBeVisible();
});

/**
 * 06 — Gate approval
 * Approving a gate must record the decision, advance the workflow
 * to the next stage, and make the Approve button unavailable
 * (or show a "Approved" badge) for the same gate.
 */
test('06 Gate approval', async ({ page }) => {
  await page.addInitScript(() => {
    window.localStorage.setItem('openforge.session', 'demo-session');
  });
  await page.goto('/code-review');

  const gate = page.getByTestId('gate-card').first();
  await expect(gate).toBeVisible();

  const approveButton = gate.getByRole('button', { name: /approve/i });
  await expect(approveButton).toBeEnabled();
  await approveButton.click();

  // After approval, the gate should display an Approved badge.
  await expect(gate.getByText(/approved/i)).toBeVisible();

  // The workflow should advance to the next stage indicator.
  await expect(page.getByTestId('workflow-stage').nth(1)).toHaveAttribute(
    'data-active',
    'true',
  );
});
