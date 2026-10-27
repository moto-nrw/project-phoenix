/**
 * Two tabs of the same host share localStorage, but each runs its own copy
 * of the modules. These tests load the modules once per tab and play the
 * orders in which a late writer in one tab meets a logout or account switch
 * in the other (#5000).
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("~/lib/push-api", () => ({
  isStandaloneApp: () => true,
}));

type PersonalStorage = typeof import("~/lib/personal-storage");
type PwaUsage = typeof import("~/lib/pwa-usage-api");

async function openTab(): Promise<{
  storage: PersonalStorage;
  pwa: PwaUsage;
}> {
  vi.resetModules();
  return {
    storage: await import("~/lib/personal-storage"),
    pwa: await import("~/lib/pwa-usage-api"),
  };
}

const MARK_OF_7 = "moto-pwa-usage-reported.tenant.7.1";
const fetchMock = vi.fn();

function holdAnswer() {
  let answer!: () => void;
  fetchMock.mockReturnValueOnce(
    new Promise((resolve) => {
      answer = () => resolve({ ok: true, status: 204 });
    }),
  );
  return () => answer();
}

beforeEach(() => {
  localStorage.clear();
  sessionStorage.clear();
  fetchMock.mockReset();
  fetchMock.mockResolvedValue({ ok: true, status: 204 });
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
  localStorage.clear();
  sessionStorage.clear();
});

describe("personal entries across tabs (#5000)", () => {
  it("keeps a tab that still shows account 7 from writing after the other tab switched to 8", async () => {
    const tabA = await openTab();
    const tabB = await openTab();
    tabA.storage.bindPersonalStorage("1:7");

    tabB.storage.bindPersonalStorage("1:8");
    // Tab A has not seen the switch yet and starts a write for account 7.
    await tabA.pwa.reportStandaloneUsage("tenant", "7", 1);

    // Anker: the report itself went out.
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(localStorage.getItem("moto:personal-storage-owner")).toBe("1:8");
    expect(sessionStorage.getItem(MARK_OF_7)).toBeNull();
    expect(tabA.storage.mayWritePersonalStorage("1:7")).toBe(false);
  });

  it("drops an answer for account 7 that arrives after the other tab switched to 8", async () => {
    const tabA = await openTab();
    const tabB = await openTab();
    tabA.storage.bindPersonalStorage("1:7");
    const answer = holdAnswer();

    const report = tabA.pwa.reportStandaloneUsage("tenant", "7", 1);
    tabB.storage.bindPersonalStorage("1:8");
    answer();
    await report;

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(sessionStorage.getItem(MARK_OF_7)).toBeNull();
  });

  // Logout and new sign-in of the same account in the other tab: the late
  // answer is account 7's own and arrives while account 7 is signed in
  // again. Nothing of another account appears; both criteria of #5000 hold.
  // It is dropped while nobody is signed in, and when another account signs
  // in instead.
  it.each([
    ["the same account signs in again", "1:7", "1"],
    ["another account signs in", "1:8", null],
  ])(
    "after a logout in the other tab, when %s",
    async (_case, nextOwner, expected) => {
      const tabA = await openTab();
      const tabB = await openTab();
      tabA.storage.bindPersonalStorage("1:7");
      const answer = holdAnswer();

      const report = tabA.pwa.reportStandaloneUsage("tenant", "7", 1);
      tabB.storage.clearPersonalStorage();
      expect(tabA.storage.mayWritePersonalStorage("1:7")).toBe(false);
      tabB.storage.bindPersonalStorage(nextOwner);
      answer();
      await report;

      expect(fetchMock).toHaveBeenCalledTimes(1);
      expect(sessionStorage.getItem(MARK_OF_7)).toBe(expected);
    },
  );

  it("drops an answer that arrives while nobody is signed in", async () => {
    const tabA = await openTab();
    const tabB = await openTab();
    tabA.storage.bindPersonalStorage("1:7");
    const answer = holdAnswer();

    const report = tabA.pwa.reportStandaloneUsage("tenant", "7", 1);
    tabB.storage.clearPersonalStorage();
    answer();
    await report;

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(sessionStorage.getItem(MARK_OF_7)).toBeNull();
    expect(localStorage.getItem("moto:personal-storage-owner")).toBeNull();
  });
});
