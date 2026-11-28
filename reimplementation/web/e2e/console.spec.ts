import { expect, test } from "@playwright/test";

const username = process.env.SPECTER_ADMIN_USERNAME ?? "admin";
const password = process.env.SPECTER_ADMIN_PASSWORD;

test("administrator can use the rendered research console", async ({ page }) => {
  test.skip(!password, "SPECTER_ADMIN_PASSWORD is required for the live-stack test");
  const browserMessages: string[] = [];
  page.on("console", (entry) => browserMessages.push(entry.text()));

  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Specter Research Console" })).toBeVisible();
  await page.getByLabel("Username").fill(username);
  await page.getByLabel("Password").fill(password!);
  await page.getByRole("button", { name: "Sign in" }).click();

  await expect(page.getByRole("heading", { name: "Research status" })).toBeVisible();
  await expect(page.locator("nav button")).toHaveCount(34);

  await page.getByRole("button", { name: "Beacon Builds" }).click();
  await expect(page.getByTestId("beacon-builds")).toContainText("Beacon builds");
  await expect(page.getByTestId("beacon-builds")).toContainText("e2e");
  const bundleDownload = page.waitForEvent("download");
  await page.getByRole("button", { name: "Build and download launch bundle" }).click();
  expect((await bundleDownload).suggestedFilename()).toContain("specter-beacon-e2e-linux-amd64.zip");

  await page.getByRole("button", { name: "Beacon Sessions" }).click();
  await expect(page.getByTestId("real-beacons")).toContainText("Beacon sessions");
  await expect(page.getByTestId("simulator-sessions")).toContainText("Simulator sessions");
  await expect(page.getByTestId("real-beacons").locator(".session-tabs button").first()).toBeVisible({ timeout: 20_000 });
  const commandConsole = page.getByTestId("command-console");
  await commandConsole.getByRole("textbox", { name: "Executable", exact: true }).fill("/bin/echo");
  await commandConsole.getByLabel("Arguments (one per line)").fill("playwright-exec");
  await commandConsole.getByRole("button", { name: "Run command" }).click();
  await expect(commandConsole.locator(".connection")).toHaveText("live", { timeout: 20_000 });
  await expect(commandConsole.locator("pre").first()).toContainText("playwright-exec", { timeout: 20_000 });

  await commandConsole.getByLabel("Executor").selectOption("sh");
  await commandConsole.getByLabel("Command").fill("printf playwright-sh");
  await commandConsole.getByRole("button", { name: "Run command" }).click();
  await expect(commandConsole.locator("pre").first()).toContainText("playwright-sh", { timeout: 20_000 });

  await commandConsole.getByLabel("Executor").selectOption("sh");
  await commandConsole.getByLabel("Command").fill("sleep 30");
  const taskHistory = commandConsole.getByLabel("Task history");
  const previousTaskID = await taskHistory.inputValue();
  await commandConsole.getByRole("button", { name: "Run command" }).click();
  await expect(taskHistory).not.toHaveValue(previousTaskID, { timeout: 10_000 });
  const cancelTaskID = await taskHistory.inputValue();
  const cancelRow = page.getByTestId("real-beacons").locator("table tbody tr").filter({ hasText: cancelTaskID });
  await expect(cancelRow).toContainText("running", { timeout: 20_000 });
  page.once("dialog", (dialog) => dialog.accept());
  await cancelRow.getByRole("button", { name: "Cancel…" }).click();
  await expect(cancelRow).toContainText("cancelled", { timeout: 20_000 });

  await commandConsole.getByLabel("Remote file path").fill("/tmp/specter-e2e-remote.txt");
  await commandConsole.getByRole("button", { name: "Retrieve file" }).click();
  await expect(commandConsole.locator(".object-results")).toContainText("specter-e2e-remote.txt", { timeout: 20_000 });
  const remoteDownload = page.waitForEvent("download");
  await commandConsole.locator(".object-results button").first().click();
  expect((await remoteDownload).suggestedFilename()).toContain("specter-e2e-remote.txt");

  await page.getByRole("button", { name: "Enrollment" }).click();
  await page.getByRole("button", { name: "Create 10-minute token" }).click();
  const oneTimeSecret = await page.locator("[data-sensitive=true]").textContent();
  expect(oneTimeSecret).toBeTruthy();
  await page.getByRole("button", { name: "I stored it — hide secret" }).click();
  await expect(page.locator("[data-sensitive=true]")).toHaveCount(0);
  await page.reload();
  await expect(page.locator("body")).not.toContainText(oneTimeSecret!);
  const usernameInput = page.getByLabel("Username");
  if (await usernameInput.isVisible()) {
    await usernameInput.fill(username);
    await page.getByLabel("Password").fill(password!);
    await page.getByRole("button", { name: "Sign in" }).click();
  }
  await page.getByRole("button", { name: "Enrollment" }).click();
  await expect(page.locator("[data-sensitive=true]")).toHaveCount(0);
  expect(browserMessages.join("\n")).not.toContain(oneTimeSecret!);
  expect(browserMessages.join("\n")).not.toContain("ui-health-check");

  await page.getByRole("button", { name: "Capability Matrix" }).click();
  await expect(page.getByRole("heading", { name: "Compatibility capability matrix" })).toBeVisible();
  await expect(page.getByRole("cell", { name: "artifact-content" })).toBeVisible();

  await page.getByRole("button", { name: /^Files\b/ }).click();
  await page.getByLabel("Source host").fill("browser-fixture");
  await page.getByLabel("Artifact (max 16 MiB)").setInputFiles({ name: "browser-artifact.bin", mimeType: "application/octet-stream", buffer: Buffer.from("browser artifact fixture") });
  await page.getByRole("button", { name: "Ingest artifact" }).click();
  await expect(page.getByRole("heading", { name: "browser-artifact.bin" })).toBeVisible();

  await page.getByRole("button", { name: /^Updates\b/ }).click();
  await page.getByLabel("Version").fill("7.0.2.0");
  await page.getByLabel("Package (max 32 MiB)").setInputFiles({ name: "browser-update.pkg", mimeType: "application/octet-stream", buffer: Buffer.from("browser update fixture") });
  await page.getByRole("button", { name: "Stage package" }).click();
  await expect(page.getByRole("heading", { name: "7.0.2.0" })).toBeVisible();

  await page.getByRole("button", { name: "Scenario Engine" }).click();
  await expect(page.getByRole("heading", { name: "Research scenario engine" })).toBeVisible();
  await page.getByRole("button", { name: "Run scenario" }).click();
  await expect(page.getByRole("heading", { name: "Run report" })).toBeVisible();
  await expect(page.getByText("PASS", { exact: true }).first()).toBeVisible();

  await page.getByRole("button", { name: "Events" }).click();
  await expect(page.getByRole("heading", { name: "Audit stream" })).toBeVisible();
  await page.getByRole("button", { name: "Emit fixture-live-window" }).click();
  await expect(page.getByRole("cell", { name: "fixture-live-window" }).last()).toBeVisible();

  await page.getByRole("button", { name: "Sign out" }).click();
  await expect(page.getByRole("heading", { name: "Specter Research Console" })).toBeVisible();
});
