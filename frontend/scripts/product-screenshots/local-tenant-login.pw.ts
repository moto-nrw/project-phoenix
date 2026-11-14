import { expect, test } from "@playwright/test";
import { randomInt } from "node:crypto";
import { createServer } from "node:http";
import { spawn } from "node:child_process";
import { mkdtempSync, readdirSync, rmSync, statSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { localMFACode, loginToLocalTenant } from "../../e2e/local-tenant-login";
import { sessionFor } from "./capture";

async function loginInFreshWorker(
  origin: string,
  cacheDirectory: string,
): Promise<void> {
  const result = await new Promise<number | null>((resolve, reject) => {
    const child = spawn(
      process.execPath,
      [
        join(process.cwd(), "node_modules/@playwright/test/cli.js"),
        "test",
        "-c",
        "playwright.screenshots.config.ts",
        "local-tenant-login.pw.ts",
        "--grep",
        "isolated login worker",
        "--output",
        join(cacheDirectory, "..", "worker-output"),
      ],
      {
        env: {
          ...process.env,
          LOCAL_LOGIN_WORKER_ORIGIN: origin,
          LOCAL_LOGIN_WORKER_CACHE: cacheDirectory,
          MAILPIT_URL: origin,
        },
        stdio: "ignore",
      },
    );
    child.once("error", reject);
    child.once("exit", resolve);
  });
  expect(result, "fresh worker must finish the real login helper").toBe(0);
}

const childOrigin = process.env.LOCAL_LOGIN_WORKER_ORIGIN;
if (childOrigin) {
  test("isolated login worker", async ({ page }) => {
    await loginToLocalTenant(
      page,
      childOrigin,
      {
        email: "admin@example.test",
        password: "example-test-password",
      },
      process.env.LOCAL_LOGIN_WORKER_CACHE,
    );
    const session = await page.request.get(`${childOrigin}/api/auth/session`);
    expect((await session.json()).user.id).toBe("fixture-admin");
  });
}
// Exercise the browser helper at the BFF/Mailpit seam without a seeded stack.
// A session exists only after the OTP exchange, not when the email field hides.
for (const { enrollment, deviceChoice } of childOrigin
  ? []
  : [
      { enrollment: false, deviceChoice: "none" },
      { enrollment: false, deviceChoice: "legacy" },
      { enrollment: false, deviceChoice: "current" },
      { enrollment: false, deviceChoice: "default" },
      { enrollment: true, deviceChoice: "none" },
    ]) {
  test(`local login completes ${enrollment ? "enrollment" : "verification"}${deviceChoice !== "none" ? ` with ${deviceChoice} device choice` : ""} and reuses its session`, async ({
    browser,
  }) => {
    const trustedDeviceEnabled =
      deviceChoice === "default" ? undefined : deviceChoice !== "none";
    const showDeviceChoice = !enrollment && trustedDeviceEnabled !== false;
    const confirmLabel =
      deviceChoice === "legacy"
        ? "Nur dieses Mal bestätigen"
        : "Nein, nur anmelden";
    const code = String(randomInt(100_000, 1_000_000));
    let loginCount = 0;
    let verified = false;
    let rememberedDevice: boolean | undefined;
    const server = createServer((request, response) => {
      response.setHeader("Content-Type", "application/json");
      const url = new URL(request.url!, "http://localhost");
      switch (url.pathname) {
        case "/":
        case "/login":
          response.setHeader("Content-Type", "text/html; charset=utf-8");
          response.end(`<!doctype html><html><body>
          <input id="email" type="email"><input id="password" type="password">
          <button id="login" type="submit">Anmelden</button>
          <script>
            const showCode = () => {
              document.body.innerHTML = '<input autocomplete="one-time-code">' + ${JSON.stringify(
                showDeviceChoice
                  ? `<button id="once">${confirmLabel}<span>Für Geräte, die mehrere Personen nutzen.</span></button><button id="remember">Bestätigen und dieses Gerät 30 Tage merken</button>`
                  : "",
              )};
              const verify = async (rememberDevice) => {
                const code = document.querySelector('input').value;
                if (code.length !== 6) return;
                const response = await fetch('/api/auth/mfa/verify', {method: 'POST', body: JSON.stringify({code, remember_device: rememberDevice})});
                if (!response.ok) return;
                if (location.pathname === '/login') {
                  location.assign('/');
                  return;
                }
                document.body.innerHTML = ${JSON.stringify(enrollment ? '<button id="continue">Weiter zum Dashboard</button>' : "Angemeldet")};
                document.querySelector('#continue')?.addEventListener('click', () => {
                  document.cookie = 'session=ready;path=/';
                  document.body.innerHTML = 'Angemeldet';
                });
              };
              if (${showDeviceChoice}) {
                document.querySelector('#once').onclick = () => verify(false);
                document.querySelector('#remember').onclick = () => verify(true);
              } else document.querySelector('input').oninput = () => verify(false);
            };
            document.querySelector('#login').onclick = async () => {
              await fetch('/api/auth/login', {method: 'POST'});
              if (${enrollment}) {
                document.body.innerHTML = '<button id="start">Code an meine E-Mail senden</button>';
                document.querySelector('#start').onclick = showCode;
              } else showCode();
            };
          </script></body></html>`);
          return;
        case "/api/auth/login":
          loginCount++;
          response.end(
            JSON.stringify({
              status: enrollment ? "mfa_enrollment_required" : "mfa_required",
              trusted_device_enabled: trustedDeviceEnabled,
            }),
          );
          return;
        case "/api/auth/mfa/verify": {
          let entered = "";
          request.on("data", (chunk: Buffer) => {
            entered += chunk.toString();
          });
          request.on("end", () => {
            const payload = JSON.parse(entered) as {
              code: string;
              remember_device: boolean;
            };
            verified = payload.code === code;
            rememberedDevice = payload.remember_device;
            response.statusCode = verified ? 200 : 401;
            if (verified && !enrollment)
              response.setHeader("Set-Cookie", "session=ready;Path=/;HttpOnly");
            response.end("{}");
          });
          return;
        }
        case "/api/auth/session":
          response.end(
            JSON.stringify(
              verified && request.headers.cookie?.includes("session=ready")
                ? { user: { id: "fixture-admin" } }
                : {},
            ),
          );
          return;
        case "/api/v1/messages":
          response.end(
            JSON.stringify({
              messages: [
                {
                  ID: "foreign",
                  Created: new Date().toISOString(),
                  To: [{ Address: "other@example.test" }],
                },
                {
                  ID: "old",
                  Created: new Date(Date.now() - 60_000).toISOString(),
                  To: [{ Address: "admin@example.test" }],
                },
                {
                  ID: "current",
                  Created: new Date().toISOString(),
                  To: [{ Address: "ADMIN@example.test" }],
                },
              ],
            }),
          );
          return;
        case "/api/v1/message/current":
          response.end(JSON.stringify({ Text: `Ihr Code: ${code}` }));
          return;
        default:
          response.statusCode = 404;
          response.end("{}");
      }
    });
    await new Promise<void>((resolve) =>
      server.listen(0, "127.0.0.1", resolve),
    );
    const address = server.address();
    if (!address || typeof address === "string")
      throw new Error("Missing test server port");
    const origin = `http://localhost:${address.port}`;
    const previousMailpit = process.env.MAILPIT_URL;
    process.env.MAILPIT_URL = origin;
    const credentials = {
      email: "admin@example.test",
      password: "example-test-password",
    };
    const directory = mkdtempSync(join(tmpdir(), "moto-e2e-login-test-"));
    const cacheDirectory = join(directory, "sessions");
    try {
      for (let attempt = 0; attempt < 2; attempt++) {
        const context = await browser.newContext();
        try {
          const page = await context.newPage();
          await loginToLocalTenant(page, origin, credentials, cacheDirectory);
          const session = await page.request.get(`${origin}/api/auth/session`);
          expect((await session.json()).user.id).toBe("fixture-admin");
        } finally {
          await context.close();
        }
      }
      // A restarted Playwright worker has no module-local session map.
      await loginInFreshWorker(origin, cacheDirectory);
      expect(loginCount).toBe(1);
      expect(readdirSync(cacheDirectory)).toHaveLength(1);
      for (const file of readdirSync(cacheDirectory)) {
        expect(statSync(join(cacheDirectory, file)).mode & 0o777).toBe(0o600);
      }
      expect(verified).toBe(true);
      expect(rememberedDevice).toBe(false);
      // A rejected saved session must complete MFA again, not be trusted locally.
      verified = false;
      await loginInFreshWorker(origin, cacheDirectory);
      expect(loginCount).toBe(2);
      expect(verified).toBe(true);
      expect(rememberedDevice).toBe(false);
      if (showDeviceChoice) {
        verified = false;
        const state = await sessionFor(
          browser,
          new Map(),
          {
            tenantOrigin: origin,
            parentsOrigin: origin,
            traegerOrigin: origin,
            admin: credentials,
            betreuer: null,
            eltern: null,
            traeger: credentials,
          },
          {
            id: "traeger-login",
            titel: "Träger-Login",
            portal: "traeger",
            rolle: "traeger",
            pfad: "/",
          },
        );
        expect(state.cookies.some(({ name }) => name === "session")).toBe(true);
        expect(loginCount).toBe(3);
        expect(verified).toBe(true);
        expect(rememberedDevice).toBe(false);
      }
    } finally {
      rmSync(directory, { recursive: true, force: true });
      if (previousMailpit === undefined) delete process.env.MAILPIT_URL;
      else process.env.MAILPIT_URL = previousMailpit;
      await new Promise<void>((resolve, reject) =>
        server.close((error) => (error ? reject(error) : resolve())),
      );
    }
  });
}

if (!childOrigin)
  test("rejects nonlocal Mailpit before making a request", async () => {
    await expect(
      localMFACode("admin@example.test", Date.now(), "https://example.invalid"),
    ).rejects.toThrow("nur lokale HTTP-Adressen");
  });
