import { test, expect } from '@playwright/test';

/**
 * 01 — Login flow
 * Verifies the unauthenticated landing page redirects to login,
 * the user can submit credentials, and lands on the dashboard.
 */
test('01 login flow', async ({ page }) => {
  await page.goto('/');
  // Should bounce to /login when no session.
  await expect(page).toHaveURL(/\/login$/);

  const email = page.getByLabel('Email');
  const password = page.getByLabel('Password');
  await expect(email).toBeVisible();
  await expect(password).toBeVisible();

  await email.fill('demo@openforge.dev');
  await password.fill('demo-password');
  await page.getByRole('button', { name: /sign in|log in/i }).click();

  // Successful login lands on the dashboard route.
  await expect(page).toHaveURL(/\/dashboard$/);
  await expect(page.getByRole('heading', { name: /dashboard/i })).toBeVisible();
});

/**
 * 02 — Project list
 * After login, the dashboard must render the list of projects
 * the user has access to.
 */
test('02 project list', async ({ page }) => {
  // Pre-seed a session via localStorage to skip the login UI.
  await page.addInitScript(() => {
    window.localStorage.setItem('openforge.session', 'demo-session');
  });
  await page.goto('/dashboard');

  const projectList = page.getByTestId('project-list');
  await expect(projectList).toBeVisible();

  // At least one project card should be rendered.
  const cards = projectList.getByTestId('project-card');
  await expect(cards.first()).toBeVisible();
  expect(await cards.count()).toBeGreaterThan(0);
});
