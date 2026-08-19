import { beforeEach, describe, expect, it, vi } from "vitest";
import { reportStandaloneUsage } from "~/lib/pwa-usage-api";
import * as pushApi from "~/lib/push-api";

vi.mock("~/lib/push-api", () => ({
  isStandaloneApp: vi.fn(),
}));

const isStandaloneApp = vi.mocked(pushApi.isStandaloneApp);

describe("reportStandaloneUsage", () => {
  const fetchMock = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    sessionStorage.clear();
    vi.stubGlobal("fetch", fetchMock);
    fetchMock.mockResolvedValue({ ok: true, status: 204 });
    isStandaloneApp.mockReturnValue(true);
  });

  it("posts to the staff endpoint in standalone mode", async () => {
    await reportStandaloneUsage("tenant");
    expect(fetchMock).toHaveBeenCalledWith("/api/pwa/usage", {
      method: "POST",
      credentials: "include",
    });
  });

  it("posts to the parent endpoint for the parents portal", async () => {
    await reportStandaloneUsage("parent");
    expect(fetchMock).toHaveBeenCalledWith("/api/parent/me/pwa-usage", {
      method: "POST",
      credentials: "include",
    });
  });

  it("never posts outside standalone mode", async () => {
    isStandaloneApp.mockReturnValue(false);
    await reportStandaloneUsage("tenant");
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("reports once per browser session and portal", async () => {
    await reportStandaloneUsage("tenant");
    await reportStandaloneUsage("tenant");
    expect(fetchMock).toHaveBeenCalledTimes(1);

    // A different portal has its own session guard.
    await reportStandaloneUsage("parent");
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it("does not mark the session on a rejected report", async () => {
    fetchMock.mockResolvedValueOnce({ ok: false, status: 500 });
    await reportStandaloneUsage("tenant");
    await reportStandaloneUsage("tenant");
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it("swallows network errors", async () => {
    fetchMock.mockRejectedValueOnce(new Error("offline"));
    await expect(reportStandaloneUsage("tenant")).resolves.toBeUndefined();
  });
});
