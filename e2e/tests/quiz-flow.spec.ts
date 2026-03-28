import { test, expect } from "@playwright/test";

test.describe("host and player session", () => {
  test("sample quiz: host starts session, player joins, first question visible", async ({
    browser,
  }) => {
    const hostCtx = await browser.newContext();
    const playerCtx = await browser.newContext();
    const host = await hostCtx.newPage();
    const player = await playerCtx.newPage();

    await host.goto("/host");

    const sampleCard = host
      .locator(".grid")
      .filter({ hasText: "Getting Started with Quiz Forge" });
    await sampleCard.getByRole("button", { name: /Start Session/i }).click();

    await host.locator("#startModal select[name='timer']").selectOption("0");
    await host
      .getByRole("button", { name: "▶ Start", exact: true })
      .click();

    await expect(host).toHaveURL(/\/host\/session\/[A-Z0-9]{6}$/);
    const roomCode = (
      await host.locator("h1 span.text-blue-400").textContent()
    )?.trim();
    expect(roomCode).toMatch(/^[A-Z0-9]{6}$/);

    await player.goto(`/join/${roomCode}`);
    await player.getByPlaceholder("Your name").fill("E2E Player");
    await player.getByRole("button", { name: "Join Game" }).click();
    await expect(player).toHaveURL(`/play/${roomCode}`);
    await expect(
      player.getByRole("heading", { name: "Waiting for Host" }),
    ).toBeVisible();

    await host.getByRole("button", { name: "▶ Start Quiz" }).click();
    await expect(host.getByText(/What color is the sky/i)).toBeVisible();

    const question = /What color is the sky on a clear day/i;
    await expect
      .poll(
        async () => {
          if (await player.getByText(question).isVisible().catch(() => false))
            return true;
          await player.reload({ waitUntil: "domcontentloaded" });
          return player.getByText(question).isVisible().catch(() => false);
        },
        { timeout: 25_000 },
      )
      .toBeTruthy();

    // Player submits an answer (sample Q1: correct answer is Blue)
    await player.getByRole("button", { name: /Blue/i }).click();
    await expect(player.locator("#answer-status")).toContainText(
      /Answer submitted/i,
      { timeout: 15_000 },
    );

    // Host reveals, then moves to question 2
    await host.getByRole("button", { name: /Reveal Answer/i }).click();
    await expect(host.getByRole("button", { name: /Next Question/i })).toBeVisible({
      timeout: 15_000,
    });
    await host.getByRole("button", { name: /Next Question/i }).click();

    const q2 = /How many days are in a week/i;
    await expect
      .poll(
        async () => {
          if (await player.getByText(q2).isVisible().catch(() => false))
            return true;
          await player.reload({ waitUntil: "domcontentloaded" });
          return player.getByText(q2).isVisible().catch(() => false);
        },
        { timeout: 25_000 },
      )
      .toBeTruthy();

    // Sample Q2: correct answer is "7"
    await player.getByRole("button", { name: /7/i }).click();
    await expect(player.locator("#answer-status")).toContainText(
      /Answer submitted/i,
      { timeout: 15_000 },
    );

    await hostCtx.close();
    await playerCtx.close();
  });
});
