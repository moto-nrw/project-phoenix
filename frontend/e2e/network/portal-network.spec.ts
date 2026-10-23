import {
  test,
  expect,
  type BrowserContext,
  type Page,
  type Locator,
  type FileChooser,
} from "@playwright/test";
import { randomBytes } from "node:crypto";
import { writeFile } from "node:fs/promises";

type Entry = { origin: string; path: string; contentType: string };
const sentry = "https://o0.ingest.sentry.io";
const turnstile = "https://challenges.cloudflare.com";
const port = 13843;

// Locator matchers attach a DOM snapshot on failure, including profile/form
// values. Poll only a boolean instead; PLAYWRIGHT_NO_COPY_PROMPT also disables
// the runner's automatic page snapshot.
async function visible(locator: Locator) {
  await expect.poll(() => locator.isVisible()).toBe(true);
}

async function observe(
  context: BrowserContext,
  origin: string,
  options: { analytics: boolean; captcha?: boolean },
) {
  const entries: Entry[] = [];
  const forbidden: Entry[] = [];
  context.on("request", (request) => {
    const url = new URL(request.url());
    const mime = request.headers()["content-type"]?.split(";")[0];
    const entry = {
      origin: url.origin,
      path: url.pathname,
      contentType:
        mime && /^[\w.+-]+\/[\w.+-]+$/.test(mime)
          ? mime
          : request.resourceType(),
    };
    entries.push(entry);
    const analytics =
      url.origin === origin && /^\/ingest(?:\/|$)/.test(url.pathname);
    if (
      analytics
        ? !options.analytics
        : url.origin !== origin &&
          url.origin !== sentry &&
          !(options.captcha === true && url.origin === turnstile)
    )
      forbidden.push(entry);
  });
  await context.route("**/*", async (route) => {
    const url = new URL(route.request().url());
    const analytics =
      url.origin === origin && /^\/ingest(?:\/|$)/.test(url.pathname);
    if (analytics || url.pathname === "/monitoring" || url.origin !== origin) {
      // Also abort allowed services and the same-origin PostHog proxy BEFORE
      // Next can forward them. A positive control observes an attempt only.
      await route.abort("blockedbyclient");
      return;
    }
    await route.continue();
  });
  await context.routeWebSocket(/.*/, (socket) => {
    const url = new URL(socket.url());
    const entry = {
      origin: url.origin,
      path: url.pathname,
      contentType: "websocket",
    };
    entries.push(entry);
    if (url.origin === origin.replace("http:", "ws:")) socket.connectToServer();
    else {
      forbidden.push(entry);
      socket.close();
    }
  });
  return {
    entries,
    async check(stage = "network", from = 0) {
      const path = test.info().outputPath(`${stage}.json`);
      await writeFile(path, JSON.stringify(entries.slice(from), null, 2));
      await test.info().attach(`${stage}-origins-paths-types`, {
        path,
        contentType: "application/json",
      });
      expect(
        forbidden,
        "Requests outside this scenario's explicit allowlist",
      ).toEqual([]);
    },
    analytics: () =>
      entries.filter((entry) => entry.path.startsWith("/ingest/")),
    captcha: () => entries.filter((entry) => entry.origin === turnstile),
  };
}

async function browserApi(page: Page, analytics: boolean) {
  await page.route(
    /^http:\/\/(?:proof-on|proof-off|eltern|schule)\.localhost:13843\/api\//,
    async (route) => {
      const path = new URL(route.request().url()).pathname;
      // Exercise real Auth.js login, cookies and session transition.
      if (/^\/api\/(?:parent\/|school\/)?auth\//.test(path)) {
        await route.fallback();
        return;
      }
      let data: unknown = [];
      if (path.endsWith("/analytics-policy")) data = { enabled: analytics };
      else if (path === "/api/staff-onboarding")
        data = {
          dismissed: true,
          school_ready: false,
          done_steps: [],
          skipped_steps: [],
        };
      else if (path === "/api/me/profile" || path === "/api/me/profile/avatar")
        data = {
          id: 1,
          first_name: "Test",
          last_name: "Fixture",
          email: "fixture@example.invalid",
          settings: {},
        };
      else if (path === "/api/user-context")
        data = {
          educationalGroups: [],
          supervisedGroups: [],
          currentStaff: null,
          isReady: true,
        };
      else if (path === "/api/parent/me/profile")
        data = {
          id: "1",
          first_name: "Test",
          last_name: "Fixture",
          portal_locale: "de",
        };
      else if (
        path.endsWith("/notification-preferences") ||
        path.endsWith("/notifications/preferences")
      )
        data = { tenant_enabled: false, types: [] };
      else if (path.endsWith("/unread-count")) data = { count: 0 };
      else if (path.includes("/reminders")) data = { reminders: [], total: 0 };
      else if (path.includes("/events")) {
        await route.fulfill({
          status: 200,
          contentType: "text/event-stream",
          body: ": fixture\n\n",
        });
        return;
      }
      await route.fulfill({ json: { status: "success", success: true, data } });
    },
  );
}

// An actual browser task exercises the SDK's automatic error capture. The
// placeholder DSN must produce an intercepted envelope, never a live request.
async function diagnosticControl(page: Page, entries: Entry[]) {
  const before = entries.filter((entry) => entry.path === "/monitoring").length;
  await page.evaluate(() => {
    setTimeout(() => {
      throw new Error("network-evidence-control");
    }, 0);
  });
  await expect
    .poll(
      () =>
        entries.filter((entry) => entry.path === "/monitoring").length > before,
      { timeout: 15_000 },
    )
    .toBe(true);
}

test("network guard aborts dedicated-worker CDN imports before sending", async ({
  page,
  context,
}) => {
  const origin = `http://proof-off.localhost:${port}`;
  const proof = await observe(context, origin, { analytics: false });
  // A route fixture without CSP tests the interceptor independently of the
  // app's additional worker-src protection. No product route is added.
  await page.route(`${origin}/network-guard`, (route) =>
    route.fulfill({
      contentType: "text/html",
      body: "<!doctype html><title>Network guard</title>",
    }),
  );
  await page.goto(`${origin}/network-guard`);
  const failed = context.waitForEvent("requestfailed", {
    predicate: (request) =>
      new URL(request.url()).hostname === "cdn.jsdelivr.net",
    timeout: 10_000,
  });
  const outcome = await page.evaluate(
    () =>
      new Promise((resolve) => {
        const source = new Blob(
          [
            "try { importScripts('https://cdn.jsdelivr.net/network-evidence.js'); postMessage('loaded'); } catch { postMessage('blocked'); }",
          ],
          { type: "text/javascript" },
        );
        const url = URL.createObjectURL(source);
        const worker = new Worker(url);
        worker.onmessage = (event) => {
          worker.terminate();
          URL.revokeObjectURL(url);
          resolve(event.data);
        };
      }),
  );
  expect(outcome).toBe("blocked");
  const request = await failed;
  expect(request.failure()?.errorText).toMatch(
    /^net::ERR_BLOCKED_BY_CLIENT(?:\.Inspector)?$/,
  );
  await expect(proof.check()).rejects.toThrow(
    "Requests outside this scenario's explicit allowlist",
  );
});

for (const analytics of [false, true]) {
  for (const surface of ["ogs", "parents", "school"] as const) {
    test(`${surface}: analytics ${analytics ? "on" : "off"}, before and after login`, async ({
      page,
      context,
    }) => {
      const host =
        surface === "ogs"
          ? `proof-${analytics ? "on" : "off"}`
          : surface === "parents"
            ? "eltern"
            : "schule";
      const origin = `http://${host}.localhost:${port}`;
      const proof = await observe(context, origin, { analytics });
      await browserApi(page, analytics);
      // Same pre-completed notification setup as tenant-session.ts. These
      // independent introductory dialogs are not part of the upload scenario.
      await context.addInitScript((slug) => {
        for (const portal of ["parent", "school"])
          localStorage.setItem(
            `moto.${portal}.notification-setup.v1.1`,
            JSON.stringify({ done: true }),
          );
        localStorage.setItem(
          `moto.tenant.notification-setup.v1.${slug}.1`,
          JSON.stringify({ done: true }),
        );
      }, host);
      try {
        await page.goto(`${origin}${surface === "ogs" ? "/" : "/login"}`, {
          waitUntil: "domcontentloaded",
        });
        // Anker: the real, hydrated login form is ready for interaction.
        await visible(page.locator('input[type="email"]'));
        await visible(page.locator('input[type="password"]'));
        await diagnosticControl(page, proof.entries);
        if (surface !== "ogs" || !analytics)
          expect(proof.analytics()).toEqual([]);
        else
          await expect.poll(() => proof.analytics().length).toBeGreaterThan(0);
        await proof.check("network-before-login");
        const beforeLogin = proof.entries.length;
        const previousAnalytics = proof.analytics().length;

        await page
          .locator('input[type="email"]')
          .fill("fixture@example.invalid");
        await page
          .locator('input[type="password"]')
          .fill(randomBytes(24).toString("hex"));
        await page
          .getByRole("button", { name: "Anmelden", exact: true })
          .click();
        await expect
          .poll(() => page.locator('input[type="email"]').count())
          .toBe(0);
        // School sign-in does a full document navigation. Do not race its
        // pending redirect with navigation to the protected evidence page.
        await page.waitForURL(
          (url) => url.pathname === (surface === "ogs" ? "/home" : "/"),
          { waitUntil: "domcontentloaded" },
        );
        if (surface === "school")
          await visible(
            page.getByText("Keine Klassen zugewiesen", { exact: true }),
          );
        else await visible(page.locator("main h1").first());
        const target =
          surface === "ogs"
            ? "/profile"
            : surface === "parents"
              ? "/settings"
              : "/einstellungen";
        await page.goto(`${origin}${target}`, {
          waitUntil: "domcontentloaded",
        });
        // Anker: authenticated product content, not a login or loading shell.
        await visible(
          page.getByRole("heading", {
            name: surface === "ogs" ? "Profil" : "Einstellungen",
            exact: true,
          }),
        );
        const sessionValid = await test.step("verify portal session", () =>
          page.evaluate(async (portal) => {
            const base =
              portal === "ogs"
                ? "/api/auth"
                : portal === "parents"
                  ? "/api/parent/auth"
                  : "/api/school/auth";
            const session = await (
              await fetch(`${base}/session`, {
                signal: AbortSignal.timeout(5000),
              })
            ).json();
            return (
              Boolean(session.user?.id && session.user?.token) &&
              (session.user.scope ?? "") ===
                (portal === "ogs"
                  ? ""
                  : portal === "parents"
                    ? "parent"
                    : "school")
            );
          }, surface));
        expect(sessionValid).toBe(true);
        if (surface !== "ogs")
          await expect
            .poll(() =>
              proof.entries.some((entry) =>
                entry.path.endsWith("/analytics-policy"),
              ),
            )
            .toBe(true);
        if (surface === "ogs") {
          // A real PNG enters the actual profile/compression/upload flow.
          const png = await page.evaluate(async () => {
            const canvas = document.createElement("canvas");
            canvas.width = 1024;
            canvas.height = 1024;
            canvas.getContext("2d")!.fillRect(0, 0, 1024, 1024);
            return canvas.toDataURL("image/png").split(",")[1]!;
          });
          const selected: { chooser?: FileChooser } = {};
          // SSR already exposes the button before React attaches its handler.
          // Wait for the real chooser, not only for server-rendered markup.
          await expect(async () => {
            const [chooser] = await Promise.all([
              page.waitForEvent("filechooser", { timeout: 1000 }),
              page
                .getByRole("button", { name: "Profilbild ändern", exact: true })
                .click({ timeout: 1000 }),
            ]);
            selected.chooser = chooser;
          }).toPass({ timeout: 15_000 });
          if (!selected.chooser)
            throw new Error("Profile image chooser did not open");
          const [request] = await Promise.all([
            page.waitForRequest(
              (request) =>
                new URL(request.url()).pathname === "/api/me/profile/avatar" &&
                request.method() === "POST",
              { timeout: 15_000 },
            ),
            selected.chooser.setFiles({
              name: "fixture.png",
              mimeType: "image/png",
              buffer: Buffer.from(png, "base64"),
            }),
          ]);
          expect(request.headers()["content-type"]?.split(";")[0]).toBe(
            "multipart/form-data",
          );
          await visible(
            page.getByText("Ihr Profilbild ist gespeichert.", { exact: true }),
          );
        }
        await diagnosticControl(page, proof.entries);
        if (analytics)
          await expect
            .poll(() => proof.analytics().length)
            .toBeGreaterThan(previousAnalytics);
        else expect(proof.analytics()).toEqual([]);
        await proof.check("network-after-login", beforeLogin);
      } finally {
        await proof.check();
      }
    });
  }
}

for (const analytics of [false, true]) {
  for (const captcha of [false, true]) {
    test(`public enrollment: analytics ${analytics}, captcha ${captcha}`, async ({
      page,
      context,
    }) => {
      const origin = `http://proof-${analytics ? "on" : "off"}.localhost:${port}`;
      const proof = await observe(context, origin, { analytics, captcha });
      await page.route("**/api/enrollment/form-bootstrap/public/**", (route) =>
        route.fulfill({
          json: {
            status: "success",
            data: {
              phase: {
                id: "1",
                name: "Netzwerktest",
                kind: "school_year",
                service_start_date: "2026-08-01",
                service_end_date: "2027-07-31",
                show_status_reason_to_parent: false,
              },
              schema: { id: "1", version: 1, fields: [] },
              offerings: [],
              care_offering_selection_mode: "optional",
              care_required: false,
              collect_grade_level: false,
              care_offerings_enabled: false,
              // Deliberately contradictory when disabled: catches the original bug.
              captcha_config: {
                enabled: captcha,
                site_key: "1x00000000000000000000AA",
              },
              legal_texts: {
                agb: "",
                dsgvo: "",
                email_contact: "",
                photo: "",
                terms_enabled: false,
                dsgvo_enabled: false,
                email_contact_enabled: false,
                photo_enabled: false,
                blocks: [],
              },
            },
          },
        }),
      );
      try {
        await page.goto(`${origin}/anmeldung/1`, {
          waitUntil: "domcontentloaded",
        });
        // Anker: core form fields and submit button have actually loaded.
        await visible(page.locator('input[type="email"]').first());
        await visible(
          page.getByRole("button", {
            name: /Anmeldung.*(senden|abschicken)|Verbindlich anmelden/,
          }),
        );
        await page
          .locator('input[type="email"]')
          .first()
          .fill("fixture@example.invalid");
        await diagnosticControl(page, proof.entries);
        if (captcha)
          await expect.poll(() => proof.captcha().length).toBeGreaterThan(0);
        else expect(proof.captcha()).toEqual([]);
        if (analytics)
          await expect.poll(() => proof.analytics().length).toBeGreaterThan(0);
        else expect(proof.analytics()).toEqual([]);
      } finally {
        await proof.check();
      }
    });
  }
}
