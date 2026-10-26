import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  bindPersonalStorage,
  clearPersonalStorage,
} from "~/lib/personal-storage";
import { reportStandaloneUsage } from "~/lib/pwa-usage-api";
import * as pushApi from "~/lib/push-api";

vi.mock("~/lib/push-api", () => ({
  isStandaloneApp: vi.fn(),
}));

const isStandaloneApp = vi.mocked(pushApi.isStandaloneApp);

describe("reportStandaloneUsage", () => {
  const fetchMock = vi.fn();
  const accountID = "account-1";

  beforeEach(() => {
    vi.clearAllMocks();
    sessionStorage.clear();
    vi.stubGlobal("fetch", fetchMock);
    fetchMock.mockResolvedValue({ ok: true, status: 204 });
    isStandaloneApp.mockReturnValue(true);
  });

  it("posts to the staff endpoint in standalone mode", async () => {
    await reportStandaloneUsage("tenant", accountID, 1);
    expect(fetchMock).toHaveBeenCalledWith("/api/pwa/usage", {
      method: "POST",
      credentials: "include",
    });
  });

  it("posts to the parent endpoint for the parents portal", async () => {
    await reportStandaloneUsage("parent", accountID);
    expect(fetchMock).toHaveBeenCalledWith("/api/parent/me/pwa-usage", {
      method: "POST",
      credentials: "include",
    });
  });

  it("does not report school sessions to the tenant-only endpoint", async () => {
    await reportStandaloneUsage("school", accountID, 1);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("never posts outside standalone mode", async () => {
    isStandaloneApp.mockReturnValue(false);
    await reportStandaloneUsage("tenant", accountID, 1);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("reports once per browser session, portal, account, and tenant", async () => {
    await reportStandaloneUsage("tenant", accountID, 1);
    await reportStandaloneUsage("tenant", accountID, 1);
    expect(fetchMock).toHaveBeenCalledTimes(1);

    await reportStandaloneUsage("tenant", "account-2", 1);
    await reportStandaloneUsage("tenant", accountID, 2);
    await reportStandaloneUsage("parent", accountID);
    expect(fetchMock).toHaveBeenCalledTimes(4);
  });

  // Logout and account switch remove the mark (#5000). The next sign-in in
  // this tab reports once more; the backend only advances last_seen_at of
  // the same (school, account, portal) row, so no count grows.
  it("reports once more after logout cleared the mark", async () => {
    await reportStandaloneUsage("tenant", accountID, 1);
    expect(sessionStorage.length).toBe(1);

    clearPersonalStorage();
    expect(sessionStorage.length).toBe(0);

    await reportStandaloneUsage("tenant", accountID, 1);
    await reportStandaloneUsage("tenant", accountID, 1);
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  // The report starts for account 7, the tab then switches to account 8 or
  // logs out, and only then the answer arrives (#5000).
  it.each([
    ["an account switch", () => bindPersonalStorage("1:8")],
    ["a logout", () => clearPersonalStorage()],
  ])(
    "does not bring the mark back after %s during the report",
    async (_change, change) => {
      bindPersonalStorage("1:7");
      let answer!: (response: { ok: boolean; status: number }) => void;
      fetchMock.mockReturnValueOnce(
        new Promise((resolve) => {
          answer = resolve;
        }),
      );

      const report = reportStandaloneUsage("tenant", accountID, 1);
      change();
      answer({ ok: true, status: 204 });
      await report;

      expect(
        sessionStorage.getItem("moto-pwa-usage-reported.tenant.account-1.1"),
      ).toBeNull();
      localStorage.clear();
    },
  );

  it("marks the session when the account stayed during the report", async () => {
    bindPersonalStorage("1:7");

    await reportStandaloneUsage("tenant", accountID, 1);

    expect(
      sessionStorage.getItem("moto-pwa-usage-reported.tenant.account-1.1"),
    ).toBe("1");
    localStorage.clear();
  });

  it("does not mark the session on a rejected report", async () => {
    fetchMock.mockResolvedValueOnce({ ok: false, status: 500 });
    await reportStandaloneUsage("tenant", accountID, 1);
    await reportStandaloneUsage("tenant", accountID, 1);
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it("swallows network errors", async () => {
    fetchMock.mockRejectedValueOnce(new Error("offline"));
    await expect(
      reportStandaloneUsage("tenant", accountID, 1),
    ).resolves.toBeUndefined();
  });
});
