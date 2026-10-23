import { expect, test } from "@playwright/test";
import { randomInt } from "node:crypto";
import { createServer } from "node:http";

import { localMFACode, loginToLocalTenant } from "../../e2e/local-tenant-login";

// Exercise the browser helper at the BFF/Mailpit seam without a seeded stack.
// A session exists only after the OTP exchange, not when the email field hides.
for (const enrollment of [false, true]) {
  test(`screenshot login completes ${enrollment ? "enrollment" : "verification"} and reuses its session`, async ({
    browser,
  }) => {
    const code = String(randomInt(100_000, 1_000_000));
    let loginCount = 0;
    let verified = false;
    const server = createServer((request, response) => {
      response.setHeader("Content-Type", "application/json");
      const url = new URL(request.url!, "http://localhost");
      switch (url.pathname) {
        case "/":
          response.setHeader("Content-Type", "text/html");
          response.end(`<!doctype html><html><body>
            <input type="email"><input type="password">
            <button id="login">Anmelden</button>
            <script>
              const showCode = () => {
                document.body.innerHTML = '<input autocomplete="one-time-code">';
                document.querySelector('input').oninput = async (event) => {
                  if (event.target.value.length !== 6) return;
                  const response = await fetch('/api/auth/mfa/verify', {method: 'POST', body: event.target.value});
                  if (!response.ok) return;
                  document.body.innerHTML = ${JSON.stringify(enrollment ? '<button id="continue">Weiter zum Dashboard</button>' : "Angemeldet")};
                  document.querySelector('#continue')?.addEventListener('click', () => {
                    document.cookie = 'session=ready;path=/';
                    document.body.innerHTML = 'Angemeldet';
                  });
                };
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
            }),
          );
          return;
        case "/api/auth/mfa/verify": {
          let entered = "";
          request.on("data", (chunk: Buffer) => {
            entered += chunk.toString();
          });
          request.on("end", () => {
            verified = entered === code;
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
    try {
      for (let attempt = 0; attempt < 2; attempt++) {
        const context = await browser.newContext();
        try {
          const page = await context.newPage();
          await loginToLocalTenant(page, origin, credentials);
          const session = await page.request.get(`${origin}/api/auth/session`);
          expect((await session.json()).user.id).toBe("fixture-admin");
        } finally {
          await context.close();
        }
      }
      expect(loginCount).toBe(1);
      expect(verified).toBe(true);
    } finally {
      if (previousMailpit === undefined) delete process.env.MAILPIT_URL;
      else process.env.MAILPIT_URL = previousMailpit;
      await new Promise<void>((resolve, reject) =>
        server.close((error) => (error ? reject(error) : resolve())),
      );
    }
  });
}

test("rejects nonlocal Mailpit before making a request", async () => {
  await expect(
    localMFACode("admin@example.test", Date.now(), "https://example.invalid"),
  ).rejects.toThrow("nur lokale HTTP-Adressen");
});
