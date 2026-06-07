import { test, expect } from '@playwright/test';

/**
 * 03 — Chat send streams
 * Sending a chat message should stream incremental assistant
 * output into the message thread (multiple chunks within 5s).
 */
test('03 chat send streams', async ({ page }) => {
  await page.addInitScript(() => {
    window.localStorage.setItem('openforge.session', 'demo-session');
  });
  await page.goto('/chat');

  const composer = page.getByLabel('Message');
  await expect(composer).toBeVisible();
  await composer.fill('Summarise the repo structure');
  await page.getByRole('button', { name: /send/i }).click();

  // The user message should appear immediately.
  const userMessage = page.getByTestId('message-user').last();
  await expect(userMessage).toContainText('Summarise the repo structure');

  // Assistant message should appear and accumulate streamed chunks.
  const assistantMessage = page.getByTestId('message-assistant').last();
  await expect(assistantMessage).toBeVisible();

  await expect.poll(async () => {
    const text = (await assistantMessage.textContent()) ?? '';
    return text.length;
  }, { timeout: 5_000 }).toBeGreaterThan(20);
});

/**
 * 04 — Chat stop cancels
 * While a stream is in progress, clicking Stop should cancel
 * the in-flight request and the message thread should not
 * continue growing.
 */
test('04 chat stop cancels', async ({ page }) => {
  await page.addInitScript(() => {
    window.localStorage.setItem('openforge.session', 'demo-session');
  });
  await page.goto('/chat');

  const composer = page.getByLabel('Message');
  await composer.fill('Tell me a long story');
  await page.getByRole('button', { name: /send/i }).click();

  // Stop button should be present while streaming.
  const stopButton = page.getByRole('button', { name: /stop|cancel/i });
  await expect(stopButton).toBeVisible({ timeout: 2_000 });

  // Capture the current length, click stop, wait, then ensure no growth.
  const assistantMessage = page.getByTestId('message-assistant').last();
  const before = (await assistantMessage.textContent())?.length ?? 0;
  await stopButton.click();

  await page.waitForTimeout(1_500);
  const after = (await assistantMessage.textContent())?.length ?? 0;
  expect(after).toBeLessThanOrEqual(before + 5);

  // The composer should be re-enabled for the next message.
  await expect(composer).toBeEnabled();
});
