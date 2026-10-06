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

  // #2517: the card shows the catalog text for the code, so the client keeps
  // the backend code instead of choosing a sentence by status.
  it("keeps the code when the school has disabled notifications", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      Response.json(
        { code: "communication.notifications_disabled" },
        { status: 409 },
      ),
    );

    await expect(sendTestNotification()).rejects.toMatchObject({
      status: 409,
      code: "communication.notifications_disabled",
    });
  });

  it("classifies other failures by status", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(null, { status: 500 }),
    );

    await expect(sendTestNotification()).rejects.toMatchObject({
      status: 500,
      code: "general.server",
    });
  });

  it("reports a request that cannot be sent as unavailable", async () => {
    vi.spyOn(globalThis, "fetch").mockRejectedValue(new TypeError("offline"));

    await expect(sendTestNotification()).rejects.toMatchObject({
      code: "general.unavailable",
    });
  });
});
