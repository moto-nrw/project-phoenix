import { test, expect, type BrowserContext, type Page } from "@playwright/test";
import { randomBytes } from "node:crypto";
import { join } from "node:path";

const source = "http://school-a.mfa.localhost:13845";
const target = "http://school-b.mfa.localhost:13845";

// Chromium resolves *.localhost even when the CI Node process cannot. Read
// only session metadata in a browser tab; never render/dump the session JSON.
async function browserSession(
  context: BrowserContext,
  origin: string,
  path = "/api/auth/session",
) {
  const probe = await context.newPage();
  try {
    await probe.goto(`${origin}/api/health`);
    return await probe.evaluate(async (sessionPath) => {
      const session = await (await fetch(sessionPath)).json();
      return { tenantId: session.user?.tenantId, scope: session.user?.scope };
    }, path);
  } finally {
    await probe.close();
  }
}

async function login(page: Page, context: BrowserContext) {
  await context.route("**/*", async (route) => {
    const url = new URL(route.request().url());
    if (
      !url.hostname.endsWith(".localhost") ||
      url.pathname === "/monitoring" ||
      url.pathname.startsWith("/ingest")
    ) {
      await route.abort();
      return;
    }
    await route.continue();
  });
  await context.addInitScript(() => {
    for (const slug of ["school-a", "school-b"])
      localStorage.setItem(
        `moto.tenant.notification-setup.v1.${slug}.1`,
        JSON.stringify({ done: true }),
      );
  });
  await page.goto(source);
  await page.locator('input[type="email"]').fill("fixture@example.invalid");
  await page
    .locator('input[type="password"]')
    .fill(randomBytes(24).toString("hex"));
  await page.getByRole("button", { name: "Anmelden", exact: true }).click();
  await page.waitForURL(`${source}/home`);
  // Only inspect cookie metadata, never include session contents in evidence.
  const metadata = (await context.cookies(target))
    .filter((cookie) => cookie.name === "mfa-localhost.session-token")
    .map(({ name, domain, httpOnly }) => ({ name, domain, httpOnly }));
  expect(metadata).toEqual([
    {
      name: "mfa-localhost.session-token",
      domain: ".mfa.localhost",
      httpOnly: true,
    },
  ]);
  expect((await browserSession(context, target)).tenantId).toBe(1);
}

test("failed family revocation keeps the shared session and shows a recoverable error", async ({
  page,
  context,
}) => {
  await login(page, context);
  await page.route("**/api/auth/logout", (route) =>
    route.fulfill({
      status: 503,
      json: { status: 503, code: "general.unavailable" },
    }),
  );
  await page.getByRole("button", { name: "Einrichtung wechseln" }).click();
  await page.getByText("School B", { exact: true }).click();
  await page.getByRole("button", { name: "Zur Schulanmeldung" }).click();
  await expect(
    page.getByRole("alert").filter({ hasText: "die Abmeldung" }),
  ).toBeVisible();
  expect(page.url()).toBe(`${source}/home`);
  expect((await browserSession(context, target)).tenantId).toBe(1);
});

test("a failed Auth.js cookie deletion stops at the target guard and permits explicit recovery", async ({
  page,
  context,
}) => {
  await login(page, context);
  // Auth.js' client currently resolves signOut even for a JSON 500 response.
  await page.route("**/api/auth/signout", (route) =>
    route.fulfill({ status: 500, json: { error: "fixture signout failure" } }),
  );
  await page.getByRole("button", { name: "Einrichtung wechseln" }).click();
  await page.getByText("School B", { exact: true }).click();
  await page.getByRole("button", { name: "Zur Schulanmeldung" }).click();
  await page.waitForURL(`${target}/`);
  const recovery = page.getByRole("button", { name: "Zur Schulanmeldung" });
  await expect(recovery).toBeVisible();
  await expect(
    page.getByRole("alert").filter({ hasText: "zweiten Anmeldenachweis" }),
  ).toBeVisible();
  expect(page.url()).toBe(`${target}/`);
  // When Auth.js is available again, the explicit action reaches the form.
  await page.unroute("**/api/auth/signout");
  await recovery.click();
  await expect(page.locator('input[type="email"]')).toBeVisible();
});

test("tenant reauthentication preserves the independent school-portal session", async ({
  page,
  context,
}) => {
  const school = "http://schule.mfa.localhost:13845";
  await login(page, context);
  await page.goto(`${school}/login`);
  await page.locator('input[type="email"]').fill("fixture@example.invalid");
  await page
    .locator('input[type="password"]')
    .fill(randomBytes(24).toString("hex"));
  await page.getByRole("button", { name: "Anmelden", exact: true }).click();
  await page.waitForURL(`${school}/`);
  const metadata = (await context.cookies(school))
    .filter((cookie) => cookie.name === "school.session-token")
    .map(({ name, domain }) => ({ name, domain }));
  expect(metadata).toEqual([
    { name: "school.session-token", domain: "schule.mfa.localhost" },
  ]);
  await page.goto(`${target}/home`);
  await page.getByRole("button", { name: "Zur Schulanmeldung" }).click();
  await page.waitForURL(`${target}/`);
  await expect(page.locator('input[type="email"]')).toBeVisible();
  expect(
    (await browserSession(context, school, "/api/school/auth/session")).scope,
  ).toBe("school");
});

for (const entry of ["switcher", "guard"] as const) {
  test(`${entry}: shared-domain MFA refusal ends the old session before target login`, async ({
    page,
    context,
    request,
  }) => {
    await login(page, context);
    const before = await (
      await request.get("http://127.0.0.1:18445/__fixture/switch-state")
    ).json();
    if (entry === "switcher") {
      await page.getByRole("button", { name: "Einrichtung wechseln" }).click();
      await page.getByText("School B", { exact: true }).click();
    } else {
      await page.goto(`${target}/home`);
    }
    const relogin = page.getByRole("button", { name: "Zur Schulanmeldung" });
    await expect(relogin).toBeVisible();
    if (process.env.TENANT_SWITCH_SHOT_DIR) {
      const hint = page
        .getByRole("alert")
        .filter({ hasText: "zweiten Anmeldenachweis" });
      const region = entry === "guard" ? hint.locator("..") : hint;
      await region.screenshot({
        path: join(process.env.TENANT_SWITCH_SHOT_DIR, `${entry}.png`),
      });
    }
    await relogin.click();
    await page.waitForURL(`${target}/`);
    await expect(page.locator('input[type="email"]')).toBeVisible();
    const after = await (
      await request.get("http://127.0.0.1:18445/__fixture/switch-state")
    ).json();
    expect(after.logoutRequests - before.logoutRequests).toBe(1);
    expect(after.switchRequests - before.switchRequests).toBe(1);
    expect(
      (await context.cookies(target)).filter(
        (cookie) => cookie.name === "mfa-localhost.session-token",
      ).length,
    ).toBe(0);
  });
}
