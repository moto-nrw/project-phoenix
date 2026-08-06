import { describe, it, expect, vi, afterEach } from "vitest";

import { MFAApiError, germanMFAErrorMessage, login } from "./mfa-api";

describe("germanMFAErrorMessage", () => {
  it("returns a generic fallback for non-MFAApiError errors", () => {
    expect(germanMFAErrorMessage(new Error("boom"))).toBe(
      "Anmeldefehler. Bitte versuchen Sie es erneut.",
    );
    expect(germanMFAErrorMessage("plain string")).toBe(
      "Anmeldefehler. Bitte versuchen Sie es erneut.",
    );
    expect(germanMFAErrorMessage(undefined)).toBe(
      "Anmeldefehler. Bitte versuchen Sie es erneut.",
    );
  });

  describe("401 branch", () => {
    it("maps locked/gesperrt to the 15-minute lockout message", () => {
      expect(
        germanMFAErrorMessage(new MFAApiError(401, "Account locked")),
      ).toBe(
        "Konto vorübergehend gesperrt. Bitte versuchen Sie es in 15 Minuten erneut.",
      );
      expect(
        germanMFAErrorMessage(new MFAApiError(401, "konto gesperrt")),
      ).toBe(
        "Konto vorübergehend gesperrt. Bitte versuchen Sie es in 15 Minuten erneut.",
      );
    });

    it("maps invalid/ungültig before expired so a mistyped code shows the friendlier message", () => {
      expect(
        germanMFAErrorMessage(new MFAApiError(401, "invalid or expired code")),
      ).toBe("Der eingegebene Code ist ungültig. Bitte erneut versuchen.");
      expect(germanMFAErrorMessage(new MFAApiError(401, "ungültig"))).toBe(
        "Der eingegebene Code ist ungültig. Bitte erneut versuchen.",
      );
    });

    it("maps expired/abgelaufen when no 'invalid' substring is present", () => {
      expect(germanMFAErrorMessage(new MFAApiError(401, "Token expired"))).toBe(
        "Der Code ist abgelaufen. Fordern Sie einen neuen Code an.",
      );
      expect(germanMFAErrorMessage(new MFAApiError(401, "abgelaufen"))).toBe(
        "Der Code ist abgelaufen. Fordern Sie einen neuen Code an.",
      );
    });

    it("falls back to a generic 401 message when no keyword matches", () => {
      expect(
        germanMFAErrorMessage(new MFAApiError(401, "something else")),
      ).toBe("Anmeldung fehlgeschlagen. Bitte erneut versuchen.");
    });
  });

  describe("429 branch", () => {
    it("maps locked/gesperrt to the lockout message (same as 401)", () => {
      expect(
        germanMFAErrorMessage(new MFAApiError(429, "Account locked")),
      ).toBe(
        "Konto vorübergehend gesperrt. Bitte versuchen Sie es in 15 Minuten erneut.",
      );
    });

    it("maps 'code request' / 'code-anfrage' to the code-rate-limit message", () => {
      expect(
        germanMFAErrorMessage(new MFAApiError(429, "too many code requests")),
      ).toBe(
        "Zu viele Code-Anfragen. Bitte warten Sie ein paar Minuten, bevor Sie einen neuen Code anfordern.",
      );
      expect(germanMFAErrorMessage(new MFAApiError(429, "Code-Anfrage"))).toBe(
        "Zu viele Code-Anfragen. Bitte warten Sie ein paar Minuten, bevor Sie einen neuen Code anfordern.",
      );
    });

    it("falls back to a generic 429 message", () => {
      expect(germanMFAErrorMessage(new MFAApiError(429, "rate limit"))).toBe(
        "Zu viele Versuche. Bitte warten Sie einen Moment.",
      );
    });
  });

  it("maps 5xx errors to a 'Server nicht erreichbar' message", () => {
    expect(germanMFAErrorMessage(new MFAApiError(500, "boom"))).toBe(
      "Der Server ist gerade nicht erreichbar. Bitte später erneut versuchen.",
    );
    expect(germanMFAErrorMessage(new MFAApiError(503, "gone"))).toBe(
      "Der Server ist gerade nicht erreichbar. Bitte später erneut versuchen.",
    );
  });

  it("returns a generic German fallback for unhandled status codes — never leaks the raw English backend string", () => {
    // #1430 review item #11: any status outside 401/429/5xx used to
    // return `err.message`, which surfaced raw English ("invalid request
    // body", "permission denied", "Not Found") inside the German UI.
    // The fallback now always speaks German.
    expect(germanMFAErrorMessage(new MFAApiError(418, "I'm a teapot"))).toBe(
      "Anmeldung fehlgeschlagen. Bitte erneut versuchen.",
    );
    expect(germanMFAErrorMessage(new MFAApiError(404, "Not Found"))).toBe(
      "Anmeldung fehlgeschlagen. Bitte erneut versuchen.",
    );
    expect(
      germanMFAErrorMessage(new MFAApiError(403, "permission denied")),
    ).toBe("Anmeldung fehlgeschlagen. Bitte erneut versuchen.");
    expect(
      germanMFAErrorMessage(new MFAApiError(400, "invalid request body")),
    ).toBe("Anmeldung fehlgeschlagen. Bitte erneut versuchen.");
  });
});

describe("MFAApiError", () => {
  it("carries status code and message and is an Error", () => {
    const err = new MFAApiError(401, "Invalid code");
    expect(err).toBeInstanceOf(Error);
    expect(err.status).toBe(401);
    expect(err.message).toBe("Invalid code");
    expect(err.name).toBe("MFAApiError");
  });

  it("carries the backend error code when one is given", () => {
    const err = new MFAApiError(403, "nope", "use_parent_portal");
    expect(err.code).toBe("use_parent_portal");
  });

  it("leaves code undefined when none is given", () => {
    expect(new MFAApiError(401, "Invalid code").code).toBeUndefined();
  });
});

describe("login error bodies", () => {
  const originalFetch = global.fetch;

  afterEach(() => {
    global.fetch = originalFetch;
  });

  function mockErrorBody(status: number, body: unknown) {
    global.fetch = vi.fn().mockResolvedValue(
      new Response(JSON.stringify(body), {
        status,
        headers: { "Content-Type": "application/json" },
      }),
    ) as unknown as typeof global.fetch;
  }

  // The backend's `code` is the stable contract; `error` is English prose
  // that callers must not branch on. Without this the tenant login cannot
  // tell a guardian-only account apart from any other 403.
  it("surfaces the backend code on the thrown MFAApiError", async () => {
    mockErrorBody(403, {
      status: "error",
      error: "guardian accounts must log in at the parents portal",
      code: "use_parent_portal",
    });

    await expect(
      login("tenant", { email: "a@b.de", password: "pw", tenantSlug: "s" }),
    ).rejects.toMatchObject({
      status: 403,
      code: "use_parent_portal",
    });
  });

  it("leaves code undefined when the body carries none", async () => {
    mockErrorBody(401, {
      status: "error",
      error: "invalid username or password",
    });

    await expect(
      login("tenant", { email: "a@b.de", password: "pw", tenantSlug: "s" }),
    ).rejects.toMatchObject({ status: 401, code: undefined });
  });
});
