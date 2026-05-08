import { apiTest as test, apiExpect as expect } from "../fixtures";

/**
 * Cross-repo contract tests for the /api/iot/* surface. PyrePortal (the
 * Raspberry Pi kiosk) consumes these endpoints with a device API key and
 * staff PIN; its German UI text is hardcoded against the response shapes
 * here, so a silent contract drift would manifest as kiosk regressions
 * that no one notices in this repo's tests.
 *
 * These run as pure HTTP requests — no browser, no UI, no auth fixture.
 *
 * Error-string contract: PyrePortal's `ERROR_TRANSLATIONS` map in
 * `PyrePortal/src/services/api.ts` matches these substrings exactly:
 *
 *   - "invalid device API key"
 *   - "device API key is required"
 *   - "invalid API key format"
 *   - "invalid staff PIN"
 *   - "staff PIN is required"
 *
 * If the backend changes any of those strings, the kiosk falls back to a
 * generic German error and a real diagnostic gets buried. The tests
 * below assert on the *exact* substrings — looser regex like /api key/i
 * would let drift slip through silently.
 */

test.describe("IoT API auth contract", () => {
  test("missing Authorization header returns 401 with the exact PyrePortal substring", async ({
    backendApi,
  }) => {
    const res = await backendApi.get("/api/iot/config");
    expect(res.status()).toBe(401);

    const body = (await res.json()) as { status?: string; error?: string };
    expect(body.status).toBe("error");
    // PyrePortal matches this exact substring → "API-Schlüssel nicht
    // konfiguriert. Bitte .env Datei prüfen.". Coordinate any change with
    // PyrePortal/src/services/api.ts:ERROR_TRANSLATIONS before merging.
    expect(body.error ?? "").toContain("device API key is required");
  });

  test("invalid API key returns 401 with the exact PyrePortal substring", async ({
    backendApi,
  }) => {
    const res = await backendApi.get("/api/iot/config", {
      headers: { Authorization: "Bearer dev_not_a_real_key" },
    });
    expect(res.status()).toBe(401);

    const body = (await res.json()) as { status?: string; error?: string };
    expect(body.status).toBe("error");
    // PyrePortal: → "API-Schlüssel ungültig. Bitte Geräte-Konfiguration prüfen."
    expect(body.error ?? "").toContain("invalid device API key");
  });

  test("malformed Authorization header (no Bearer prefix) returns 401 with the exact PyrePortal substring", async ({
    backendApi,
    checkinDevice,
  }) => {
    const res = await backendApi.get("/api/iot/config", {
      headers: { Authorization: checkinDevice.api_key },
    });
    expect(res.status()).toBe(401);

    const body = (await res.json()) as { status?: string; error?: string };
    expect(body.status).toBe("error");
    // Backend emits "invalid API key format - use Bearer token". PyrePortal
    // matches the prefix → "API-Schlüssel Format ungültig. Bearer Token
    // erwartet.". Asserting the substring keeps both halves stable.
    expect(body.error ?? "").toContain("invalid API key format");
  });
});

test.describe("GET /api/iot/config", () => {
  test("returns the full config contract for a valid device", async ({
    deviceApi,
  }) => {
    const res = await deviceApi.get("/api/iot/config");

    expect(res.status()).toBe(200);

    const body = (await res.json()) as {
      data: {
        checkout: {
          raumwechsel_enabled: unknown;
          schulhof_enabled: unknown;
          wc_enabled: unknown;
          daily_checkout_time: unknown;
        };
        feedback: { enabled: unknown };
        presence_mode: unknown;
      };
    };

    // Top-level contract: the kiosk reads exactly these three groups.
    expect(body.data).toBeDefined();
    expect(body.data.checkout).toBeDefined();
    expect(body.data.feedback).toBeDefined();

    // presence_mode is the cross-repo contract field documented in
    // CLAUDE.md. Old kiosk builds default to "detailed" if the field is
    // missing — but the field MUST be present so newer builds branch
    // their UI deterministically. Allowed values: "detailed" | "binary".
    expect(typeof body.data.presence_mode).toBe("string");
    expect(["detailed", "binary"]).toContain(body.data.presence_mode);

    // checkout button toggles — kiosk hides Raumwechsel/Schulhof/WC buttons
    // based on these. All three must be booleans.
    expect(typeof body.data.checkout.raumwechsel_enabled).toBe("boolean");
    expect(typeof body.data.checkout.schulhof_enabled).toBe("boolean");
    expect(typeof body.data.checkout.wc_enabled).toBe("boolean");

    // daily_checkout_time is null OR an "HH:MM" string. The kiosk needs
    // to handle both.
    const dailyCheckoutTime = body.data.checkout.daily_checkout_time;
    if (dailyCheckoutTime !== null) {
      expect(typeof dailyCheckoutTime).toBe("string");
      expect(dailyCheckoutTime).toMatch(/^\d{2}:\d{2}$/);
    }

    expect(typeof body.data.feedback.enabled).toBe("boolean");
  });

  test("presence_mode defaults to 'detailed' for a freshly seeded tenant", async ({
    deviceApi,
  }) => {
    const res = await deviceApi.get("/api/iot/config");

    expect(res.status()).toBe(200);
    const body = (await res.json()) as { data: { presence_mode: string } };

    // Backwards-compat default — if this ever flips to "binary" by accident,
    // every kiosk in the field that runs in detailed mode will break silently.
    expect(body.data.presence_mode).toBe("detailed");
  });
});

test.describe("PIN-protected endpoints", () => {
  test("POST /api/iot/checkin without PIN returns 401", async ({
    backendApi,
    checkinDevice,
  }) => {
    const res = await backendApi.post("/api/iot/checkin", {
      headers: {
        Authorization: `Bearer ${checkinDevice.api_key}`,
      },
      data: { student_rfid: "TEST-RFID-001" },
    });
    expect(res.status()).toBe(401);

    const body = (await res.json()) as { status?: string; error?: string };
    expect(body.status).toBe("error");
    // PyrePortal: → "PIN nicht angegeben."
    expect(body.error ?? "").toContain("staff PIN is required");
  });

  test("POST /api/iot/checkin with wrong PIN returns 401", async ({
    backendApi,
    checkinDevice,
  }) => {
    const res = await backendApi.post("/api/iot/checkin", {
      headers: {
        Authorization: `Bearer ${checkinDevice.api_key}`,
        "X-Staff-PIN": "0000",
      },
      data: { student_rfid: "TEST-RFID-001" },
    });
    expect(res.status()).toBe(401);

    const body = (await res.json()) as { status?: string; error?: string };
    expect(body.status).toBe("error");
    // PyrePortal: → "Ungültiger PIN. Bitte erneut versuchen."
    expect(body.error ?? "").toContain("invalid staff PIN");
  });

  // Sanity check: the PIN we derived from the Go e2e manifest actually works. We
  // don't assert on a successful checkin (no real RFID tag provisioned),
  // but we expect any error other than 401 — meaning auth passed.
  test("POST /api/iot/checkin with valid PIN passes auth (status != 401)", async ({
    deviceApi,
  }) => {
    const res = await deviceApi.post("/api/iot/checkin", {
      data: { student_rfid: "TEST-DOES-NOT-EXIST" },
    });
    expect(res.status()).not.toBe(401);
  });
});
