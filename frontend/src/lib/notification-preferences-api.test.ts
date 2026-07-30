import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

import {
  disableAllNotificationPreferences,
  fetchNotificationPreferences,
  setNotificationPreference,
} from "./notification-preferences-api";

/**
 * The two portals speak to different paths and wrap their payloads
 * differently. Both details are invisible in the card and would fail silently:
 * a wrong path 404s into a generic error, and a missed envelope renders an
 * empty list that looks exactly like "no notification types exist".
 */
describe("notification preferences API client", () => {
  const fetchMock = vi.fn();

  beforeEach(() => {
    fetchMock.mockReset();
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  function jsonResponse(body: unknown, status = 200) {
    return {
      ok: status >= 200 && status < 300,
      status,
      text: () => Promise.resolve(JSON.stringify(body)),
    };
  }

  const payload = {
    tenant_enabled: true,
    types: [
      {
        key: "pickup_upcoming",
        label: "Anstehende Abholung",
        description: "…",
        group: "abholung",
        enabled: true,
        available: true,
      },
    ],
  };

  it("reads the staff catalogue from the tenant path", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({ data: payload }));

    const result = await fetchNotificationPreferences();

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/notifications/preferences",
      expect.objectContaining({ credentials: "include" }),
    );
    expect(result.tenant_enabled).toBe(true);
    expect(result.types).toHaveLength(1);
    expect(result.types[0]?.key).toBe("pickup_upcoming");
  });

  it("accepts an unwrapped payload from the parent path", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(payload));

    const result = await fetchNotificationPreferences("parent");

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/parent/me/notification-preferences",
    );
    expect(result.types).toHaveLength(1);
  });

  it("rejects a successful response without a preference payload", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({}));

    await expect(fetchNotificationPreferences()).rejects.toThrow();
  });

  it("rejects malformed preference rows", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        data: {
          tenant_enabled: true,
          types: [{ key: "pickup_upcoming", enabled: "yes" }],
        },
      }),
    );

    await expect(fetchNotificationPreferences()).rejects.toThrow();
  });

  it("sends one decision as a PUT on the encoded type", async () => {
    fetchMock.mockResolvedValueOnce({ ok: true, status: 204 });

    await setNotificationPreference("pickup_upcoming", true);

    const [url, init] = fetchMock.mock.calls[0] ?? [];
    expect(url).toBe("/api/notifications/preferences/pickup_upcoming");
    expect(init).toMatchObject({
      method: "PUT",
      body: JSON.stringify({ enabled: true }),
    });
  });

  it("routes a parent decision to the parent path", async () => {
    fetchMock.mockResolvedValueOnce({ ok: true, status: 204 });

    await setNotificationPreference("parent_announcement", false, "parent");

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/parent/me/notification-preferences/parent_announcement",
    );
  });

  it("switches everything off with a DELETE", async () => {
    fetchMock.mockResolvedValueOnce({ ok: true, status: 204 });

    await disableAllNotificationPreferences("parent");

    const [url, init] = fetchMock.mock.calls[0] ?? [];
    expect(url).toBe("/api/parent/me/notification-preferences");
    expect(init).toMatchObject({ method: "DELETE" });
  });

  it("surfaces the backend's message on failure", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse({ error: "unknown notification type" }, 400),
    );

    await expect(setNotificationPreference("nope", true)).rejects.toThrow(
      "unknown notification type",
    );
  });

  it("falls back to a German message when the error carries no body", async () => {
    fetchMock.mockResolvedValueOnce({
      ok: false,
      status: 500,
      text: () => Promise.resolve(""),
    });

    await expect(fetchNotificationPreferences()).rejects.toThrow(
      /Einstellung konnte nicht geladen werden \(500\)/,
    );
  });
});
