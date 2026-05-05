import { test, expect } from "@playwright/test";
import {
  BACKEND_URL,
  IOT_HEADERS,
  getDeviceApiKey,
  getDevicePIN,
} from "../helpers/iot";

/**
 * Cross-repo contract tests for the /api/iot/* surface. PyrePortal (the
 * Raspberry Pi kiosk) consumes these endpoints with a device API key and
 * staff PIN; its German UI text is hardcoded against the response shapes
 * here, so a silent contract drift would manifest as kiosk regressions
 * that no one notices in this repo's tests.
 *
 * These run as pure HTTP requests — no browser, no UI, no auth fixture.
 */

test.describe("IoT API auth contract", () => {
  test("missing Authorization header returns 401", async ({ request }) => {
    const res = await request.get(`${BACKEND_URL}/api/iot/config`);
    expect(res.status()).toBe(401);

    const body = (await res.json()) as { status?: string; error?: string };
    expect(body.status).toBe("error");
    // PyrePortal maps this exact substring to a German UI message.
    // If the wording changes, coordinate with PyrePortal/src/services/api.ts.
    expect(body.error).toMatch(/api key/i);
  });

  test("invalid API key returns 401", async ({ request }) => {
    const res = await request.get(`${BACKEND_URL}/api/iot/config`, {
      headers: IOT_HEADERS.apiKey("dev_not_a_real_key"),
    });
    expect(res.status()).toBe(401);

    const body = (await res.json()) as { status?: string; error?: string };
    expect(body.status).toBe("error");
    expect(body.error).toMatch(/api key/i);
  });

  test("malformed Authorization header (no Bearer prefix) returns 401", async ({
    request,
  }) => {
    const apiKey = getDeviceApiKey();
    const res = await request.get(`${BACKEND_URL}/api/iot/config`, {
      headers: { Authorization: apiKey },
    });
    expect(res.status()).toBe(401);

    const body = (await res.json()) as { status?: string; error?: string };
    expect(body.status).toBe("error");
    // Backend distinguishes "format" from "missing" — the message specifically
    // mentions Bearer to guide kiosk integrators.
    expect(body.error).toMatch(/bearer/i);
  });
});

test.describe("GET /api/iot/config", () => {
  test("returns the full config contract for a valid device", async ({
    request,
  }) => {
    const apiKey = getDeviceApiKey();
    const res = await request.get(`${BACKEND_URL}/api/iot/config`, {
      headers: IOT_HEADERS.apiKey(apiKey),
    });

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
    request,
  }) => {
    const apiKey = getDeviceApiKey();
    const res = await request.get(`${BACKEND_URL}/api/iot/config`, {
      headers: IOT_HEADERS.apiKey(apiKey),
    });

    expect(res.status()).toBe(200);
    const body = (await res.json()) as { data: { presence_mode: string } };

    // Backwards-compat default — if this ever flips to "binary" by accident,
    // every kiosk in the field that runs in detailed mode will break silently.
    expect(body.data.presence_mode).toBe("detailed");
  });
});

test.describe("PIN-protected endpoints", () => {
  test("POST /api/iot/checkin without PIN returns 401", async ({ request }) => {
    const apiKey = getDeviceApiKey();
    const res = await request.post(`${BACKEND_URL}/api/iot/checkin`, {
      headers: IOT_HEADERS.apiKey(apiKey),
      data: { rfid_tag: "TEST-RFID-001" },
    });
    expect(res.status()).toBe(401);

    const body = (await res.json()) as { status?: string; error?: string };
    expect(body.status).toBe("error");
    expect(body.error?.toLowerCase() ?? "").toContain("pin");
  });

  test("POST /api/iot/checkin with wrong PIN returns 401", async ({
    request,
  }) => {
    const apiKey = getDeviceApiKey();
    const res = await request.post(`${BACKEND_URL}/api/iot/checkin`, {
      headers: IOT_HEADERS.apiKeyAndPin(apiKey, "0000"),
      data: { rfid_tag: "TEST-RFID-001" },
    });
    expect(res.status()).toBe(401);

    const body = (await res.json()) as { status?: string; error?: string };
    expect(body.status).toBe("error");
    expect(body.error?.toLowerCase() ?? "").toContain("pin");
  });

  // Sanity check: the PIN we derived from seed state actually works. We
  // don't assert on a successful checkin (no real RFID tag provisioned),
  // but we expect any error other than 401 — meaning auth passed.
  test("POST /api/iot/checkin with valid PIN passes auth (status != 401)", async ({
    request,
  }) => {
    const apiKey = getDeviceApiKey();
    const pin = getDevicePIN();
    const res = await request.post(`${BACKEND_URL}/api/iot/checkin`, {
      headers: IOT_HEADERS.apiKeyAndPin(apiKey, pin),
      data: { rfid_tag: "TEST-DOES-NOT-EXIST" },
    });
    expect(res.status()).not.toBe(401);
  });
});
