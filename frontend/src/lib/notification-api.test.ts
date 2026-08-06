import { beforeEach, describe, expect, it, vi } from "vitest";
import { sendTestNotification } from "./notification-api";

describe("sendTestNotification", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("posts an empty JSON body to the tenant proxy", async () => {
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(new Response(null, { status: 200 }));

    await sendTestNotification();

    expect(fetchMock).toHaveBeenCalledWith("/api/notifications/test", {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: "{}",
    });
  });

  it("explains when the school has disabled notifications", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(null, { status: 409 }),
    );

    await expect(sendTestNotification()).rejects.toThrow(
      "Ihre Schule hat Benachrichtigungen derzeit deaktiviert.",
    );
  });

  it("uses a safe generic message for other failures", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(null, { status: 500 }),
    );

    await expect(sendTestNotification()).rejects.toThrow(
      "Testbenachrichtigung konnte nicht gesendet werden. Prüfen Sie die Verbindung und versuchen Sie es erneut.",
    );
  });

  it("uses the same safe message when the request cannot be sent", async () => {
    vi.spyOn(globalThis, "fetch").mockRejectedValue(new TypeError("offline"));

    await expect(sendTestNotification()).rejects.toThrow(
      "Testbenachrichtigung konnte nicht gesendet werden. Prüfen Sie die Verbindung und versuchen Sie es erneut.",
    );
  });
});
