import { test, expect, type BrowserContext, type Page } from "@playwright/test";
import { randomBytes } from "node:crypto";
import { join } from "node:path";

// Same defaults and overrides as e2e/network/server.mjs.
const port = process.env.TENANT_SWITCH_PORT ?? "13845";
const fixture = `http://127.0.0.1:${process.env.TENANT_SWITCH_API_PORT ?? "18445"}`;
const source = `http://school-a.mfa.localhost:${port}`;
const target = `http://school-b.mfa.localhost:${port}`;

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
  await page.unroute("**/api/auth/logout");
  await page.getByRole("button", { name: "Wiederholen" }).click();
  await page.waitForURL(`${target}/`);
  await expect(page.locator('input[type="email"]')).toBeVisible();
});

// #5126: the failed attempt logs an error, and its batched shipment to the
// authenticated log route can still be open when "Wiederholen" deletes the
// cookie. Its answer carries the source session cookie again.
test("a log shipment answered late cannot restore the source session after explicit recovery", async ({
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
  let signedOut!: () => void;
  const signout = new Promise<void>((resolve) => (signedOut = resolve));
  page.on("response", (response) => {
    if (new URL(response.url()).pathname === "/api/auth/signout") signedOut();
  });
  let shipped!: () => void;
  const shipment = new Promise<void>((resolve) => (shipped = resolve));
  // Answer the shipment only after the cookie deletion, unless the product
  // waits for it first; then answer it after two seconds. The answer applies
  // the session cookie the request left with, as the real route does. Node
  // cannot resolve *.localhost in CI, so the route itself is not called.
  await context.route("**/api/logs", async (route) => {
    shipped();
    const sentWith = (await context.cookies(source)).filter(
      (cookie) => cookie.name === "mfa-localhost.session-token",
    );
    await Promise.race([
      signout,
      new Promise((resolve) => setTimeout(resolve, 2_000)),
    ]);
    await context.addCookies(sentWith);
    await route.fulfill({ status: 200, json: {} });
  });
  await page.getByRole("button", { name: "Einrichtung wechseln" }).click();
  await page.getByText("School B", { exact: true }).click();
  await page.getByRole("button", { name: "Zur Schulanmeldung" }).click();
  await expect(
    page.getByRole("alert").filter({ hasText: "die Abmeldung" }),
  ).toBeVisible();
  await shipment;
  await page.unroute("**/api/auth/logout");
  await page.getByRole("button", { name: "Wiederholen" }).click();
  await page.waitForURL(`${target}/`);
  if (process.env.TENANT_SWITCH_SHOT_DIR) {
    // The target settles on its login form, or on the source session's hint.
    await expect(
      page
        .locator('input[type="email"]')
        .or(page.getByText("zweiten Anmeldenachweis")),
    ).toBeVisible();
    await page.screenshot({
      path: join(process.env.TENANT_SWITCH_SHOT_DIR, "relogin-target.png"),
    });
  }
  await expect(page.locator('input[type="email"]')).toBeVisible();
  expect(
    (await context.cookies(target)).filter(
      (cookie) => cookie.name === "mfa-localhost.session-token",
    ),
  ).toHaveLength(0);
});

// #5126: a write started while the logout holds the tab's requests is never
// sent. If the navigation to the target school is aborted, the tab stays: the
// write is reported as not saved, and requests flow again.
test("an aborted navigation after sign-out reports a held write as not saved", async ({
  page,
  context,
}) => {
  await login(page, context);
  let writes = 0;
  await context.route("**/api/fixture-write", (route) => {
    writes++;
    return route.fulfill({ status: 200, json: {} });
  });
  let navigationStarted!: () => void;
  const navigation = new Promise<void>(
    (resolve) => (navigationStarted = resolve),
  );
  // The target school's page never answers; the test aborts its navigation.
  await context.route(`${target}/`, () => navigationStarted());
  // The write starts at the moment the logout begins to hold requests.
  await page.evaluate(() => {
    window.addEventListener(
      "phoenix:session-end",
      (event) => {
        if ((event as CustomEvent<{ ending: boolean }>).detail.ending)
          void fetch("/api/fixture-write", { method: "POST" }).catch(
            () => undefined,
          );
      },
      { once: true },
    );
  });
  await page.getByRole("button", { name: "Einrichtung wechseln" }).click();
  await page.getByText("School B", { exact: true }).click();
  await page.getByRole("button", { name: "Zur Schulanmeldung" }).click();
  await navigation;
  // The browser's stop button: the tab keeps the source page.
  await (await context.newCDPSession(page)).send("Page.stopLoading");

  const notice = page
    .getByRole("alert")
    .filter({ hasText: "nicht gespeichert" });
  if (process.env.TENANT_SWITCH_SHOT_DIR) {
    await notice.waitFor({ timeout: 12_000 }).catch(() => undefined);
    // Let the toast finish fading in; endless spinners are skipped.
    await page.evaluate(() =>
      Promise.all(
        document
          .getAnimations()
          .filter(
            (animation) =>
              animation.effect?.getTiming().iterations !== Infinity,
          )
          .map((animation) => animation.finished.catch(() => undefined)),
      ),
    );
    await page.screenshot({
      path: join(process.env.TENANT_SWITCH_SHOT_DIR, "aborted-navigation.png"),
    });
  }
  await expect(notice).toHaveText(
    "Ihre letzte Änderung wurde nicht gespeichert, weil Sie gerade abgemeldet wurden. Bitte speichern Sie sie noch einmal.",
  );
  expect(page.url()).toBe(`${source}/home`);
  expect(writes).toBe(0);
});

// #5126: a sign-out that never answers must not keep the tab locked. The
// barrier aborts it, the person sees that the logout failed, and the retry
// reaches the target school's login.
test("a hanging sign-out releases the tab and reports a recoverable logout failure", async ({
  page,
  context,
}) => {
  await login(page, context);
  await page.route("**/api/auth/signout", () => undefined);
  await page.getByRole("button", { name: "Einrichtung wechseln" }).click();
  await page.getByText("School B", { exact: true }).click();
  await page.getByRole("button", { name: "Zur Schulanmeldung" }).click();
  const failure = page
    .getByRole("alert")
    .filter({ hasText: "Abmeldung ist gerade nicht erreichbar" });
  await expect(failure).toBeVisible();
  expect(page.url()).toBe(`${source}/home`);
  if (process.env.TENANT_SWITCH_SHOT_DIR)
    await page.screenshot({
      path: join(process.env.TENANT_SWITCH_SHOT_DIR, "hanging-sign-out.png"),
    });
  await page.unroute("**/api/auth/signout");
  await failure.getByRole("button", { name: "Wiederholen" }).click();
  await page.waitForURL(`${target}/`);
  await expect(page.locator('input[type="email"]')).toBeVisible();
});

// #5126: the menu logout opens a modal above the toasts. When the sign-out
// hangs, the modal closes so the failure and its retry stay usable.
test("a hanging menu logout closes the modal and offers a usable retry", async ({
  page,
  context,
}) => {
  await login(page, context);
  await page.route("**/api/auth/signout", () => undefined);
  await page
    .locator('button[aria-haspopup="dialog"]')
    .filter({ hasText: "Test Fixture" })
    .first()
    .click();
  await page.getByRole("button", { name: "Abmelden" }).first().click();
  const modal = page.getByRole("dialog").filter({ hasText: "abmelden?" });
  await modal.getByRole("button", { name: "Abmelden" }).click();
  const failure = page
    .getByRole("alert")
    .filter({ hasText: "Abmeldung ist gerade nicht erreichbar" });
  await expect(failure).toBeVisible();
  await expect(modal).toBeHidden();
  expect(page.url()).toBe(`${source}/home`);
  if (process.env.TENANT_SWITCH_SHOT_DIR)
    await page.screenshot({
      path: join(process.env.TENANT_SWITCH_SHOT_DIR, "hanging-menu-logout.png"),
    });
  await page.unroute("**/api/auth/signout");
  // A real click: nothing may cover the toast's retry.
  await failure.getByRole("button", { name: "Wiederholen" }).click();
  await page.waitForURL(`${source}/`);
  await expect(page.locator('input[type="email"]')).toBeVisible();
});

// #5126: one logout per tab. After a hanging menu logout, a double click on
// „Wiederholen“ starts one logout (one revocation) and leads to one message.
test("a double click on Wiederholen after a menu logout runs one logout", async ({
  page,
  context,
  request,
}) => {
  await login(page, context);
  await page.route("**/api/auth/signout", () => undefined);
  await page
    .locator('button[aria-haspopup="dialog"]')
    .filter({ hasText: "Test Fixture" })
    .first()
    .click();
  await page.getByRole("button", { name: "Abmelden" }).first().click();
  await page
    .getByRole("dialog")
    .filter({ hasText: "abmelden?" })
    .getByRole("button", { name: "Abmelden" })
    .click();
  const failure = page
    .getByRole("alert")
    .filter({ hasText: "Abmeldung ist gerade nicht erreichbar" });
  await expect(failure).toHaveCount(1);
  const before = await (
    await request.get(`${fixture}/__fixture/switch-state`)
  ).json();

  await failure.getByRole("button", { name: "Wiederholen" }).dblclick();
  // The retry closes the toast; its one logout fails at the deadline again.
  await expect(failure).toHaveCount(0);
  await expect(failure).toHaveCount(1, { timeout: 20_000 });
  const after = await (
    await request.get(`${fixture}/__fixture/switch-state`)
  ).json();
  expect(after.logoutRequests - before.logoutRequests).toBe(1);
  await expect(failure).toHaveCount(1);
  expect(page.url()).toBe(`${source}/home`);
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
  const school = `http://schule.mfa.localhost:${port}`;
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
      await request.get(`${fixture}/__fixture/switch-state`)
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
      await request.get(`${fixture}/__fixture/switch-state`)
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
