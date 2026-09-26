import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  DEMO_SETUP_STEPS,
  demoEntryEvent,
  demoLinkFragment,
  demoSetupStep,
  isDemoEntryPath,
  takeDemoLinkFromFragment,
  waitForDemoSchool,
} from "./demo-access";

// A restart (#3470) travels through the fragment like a role switch does,
// and the entry into the fresh school reports it as such.
describe("demoEntryEvent", () => {
  it("reports an entry, a switch or a restart", () => {
    expect(demoEntryEvent({ token: "t" })).toBe("demo_entered");
    expect(demoEntryEvent({ token: "t", switched: true })).toBe(
      "demo_role_switched",
    );
    expect(demoEntryEvent({ token: "t", restarted: true })).toBe(
      "demo_restarted",
    );
  });

  it("carries the restart through the fragment", () => {
    expect(
      demoLinkFragment({ token: "t", role: "lead", restarted: true }),
    ).toBe("#token=t&role=lead&restarted=1");
    expect(demoLinkFragment({ token: "t" })).toBe("#token=t");
  });
});

describe("isDemoEntryPath", () => {
  it("matches the entry page on a subdomain and in path mode", () => {
    expect(isDemoEntryPath("/demo", "messe-demo")).toBe(true);
    expect(isDemoEntryPath("/demo/", undefined)).toBe(true);
    expect(isDemoEntryPath("/messe-demo/demo", "messe-demo")).toBe(true);
  });

  it("matches nothing else", () => {
    expect(isDemoEntryPath(null, "messe-demo")).toBe(false);
    expect(isDemoEntryPath("/", "messe-demo")).toBe(false);
    expect(isDemoEntryPath("/demo/settings", "messe-demo")).toBe(false);
    expect(isDemoEntryPath("/other/demo", "messe-demo")).toBe(false);
  });
});

// The lines follow the time waited, spread over a seed of about 30 to 40
// seconds.
describe("demoSetupStep", () => {
  it("moves through the setup lines while the school is prepared", () => {
    expect(demoSetupStep(0)).toBe(0);
    expect(demoSetupStep(9_999)).toBe(0);
    expect(demoSetupStep(10_000)).toBe(1);
    expect(demoSetupStep(24_999)).toBe(1);
    expect(demoSetupStep(25_000)).toBe(2);
  });

  it("stays on the last line until the school is ready", () => {
    expect(demoSetupStep(10 * 60_000)).toBe(DEMO_SETUP_STEPS.length - 1);
  });

  it("keeps the first line when the clock was set back", () => {
    expect(demoSetupStep(-5_000)).toBe(0);
  });
});

function openDemoPage(hash: string) {
  globalThis.history.replaceState(null, "", `/demo${hash}`);
}

// The fragment leaves the address bar; the tab keeps the link, so a reload of
// the waiting room still finds it.
describe("takeDemoLinkFromFragment", () => {
  beforeEach(() => {
    sessionStorage.clear();
    vi.stubGlobal("location", {
      ...globalThis.location,
      get hash() {
        return new URL(document.URL).hash;
      },
      pathname: "/demo",
      search: "",
    });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("reads the link from the fragment and removes it from the address bar", () => {
    openDemoPage("#token=secret-token&role=lead&switched=1&restarted=1");

    expect(takeDemoLinkFromFragment()).toEqual({
      token: "secret-token",
      role: "lead",
      switched: true,
      restarted: true,
    });
    expect(document.URL).not.toContain("secret-token");
  });

  it("finds the kept link again after a reload without the fragment", () => {
    openDemoPage("#token=secret-token&role=lead&restarted=1");
    takeDemoLinkFromFragment();

    openDemoPage("");

    expect(takeDemoLinkFromFragment()).toEqual({
      token: "secret-token",
      role: "lead",
      restarted: true,
    });
  });

  it("prefers a new link in the fragment over the kept one", () => {
    openDemoPage("#token=old-token&role=lead&switched=1");
    takeDemoLinkFromFragment();

    openDemoPage("#token=new-token&role=caregiver");
    expect(takeDemoLinkFromFragment()).toEqual({
      token: "new-token",
      role: "caregiver",
    });

    openDemoPage("");
    expect(takeDemoLinkFromFragment()).toEqual({
      token: "new-token",
      role: "caregiver",
    });
  });

  it("has no link without a fragment and without a kept link", () => {
    openDemoPage("");

    expect(takeDemoLinkFromFragment()).toBeNull();
  });

  it("ignores a kept link it cannot read", () => {
    sessionStorage.setItem("moto-demo-link", "{not json");
    openDemoPage("");
    expect(takeDemoLinkFromFragment()).toBeNull();

    sessionStorage.setItem("moto-demo-link", JSON.stringify({ role: "lead" }));
    expect(takeDemoLinkFromFragment()).toBeNull();
  });

  it("reads the fragment as before when the storage throws", () => {
    const blocked = () => {
      throw new DOMException("blocked", "SecurityError");
    };
    vi.spyOn(sessionStorage, "setItem").mockImplementation(blocked);
    vi.spyOn(sessionStorage, "getItem").mockImplementation(blocked);

    openDemoPage("#token=secret-token&role=lead");
    expect(takeDemoLinkFromFragment()).toEqual({
      token: "secret-token",
      role: "lead",
    });
    expect(document.URL).not.toContain("secret-token");

    openDemoPage("");
    expect(takeDemoLinkFromFragment()).toBeNull();
  });
});

describe("waitForDemoSchool", () => {
  const fetchMock = vi.fn();

  function answer(body: Record<string, unknown>) {
    return Promise.resolve({
      ok: true,
      status: 200,
      json: () => Promise.resolve(body),
    } as Response);
  }

  beforeEach(() => {
    fetchMock.mockReset();
    vi.stubGlobal("fetch", fetchMock);
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("keeps waiting past two minutes while the school is prepared", async () => {
    fetchMock.mockImplementation(() =>
      answer({ status: "preparing", school_name: "OGS Nord" }),
    );
    const steps: number[] = [];
    let settled = false;
    const waiting = waitForDemoSchool("t", { cancelled: false }, (progress) =>
      steps.push(progress.step),
    ).finally(() => {
      settled = true;
    });

    // 70 polls of two seconds: more than the old limit of 60.
    for (let poll = 0; poll < 70; poll++) {
      await vi.advanceTimersByTimeAsync(2000);
    }
    expect(settled).toBe(false);
    expect(fetchMock.mock.calls.length).toBeGreaterThan(61);
    // The lines follow the time waited, one poll every two seconds.
    expect(steps[0]).toBe(0);
    expect(steps[4]).toBe(0);
    expect(steps[5]).toBe(1);
    expect(steps[12]).toBe(1);
    expect(steps[13]).toBe(2);
    expect(steps.at(-1)).toBe(2);

    fetchMock.mockImplementation(() =>
      answer({
        status: "ready",
        school_url: "https://ogs-nord.demo.example",
        school_name: "OGS Nord",
      }),
    );
    await vi.advanceTimersByTimeAsync(2000);

    await expect(waiting).resolves.toEqual({
      phase: "ready",
      schoolUrl: "https://ogs-nord.demo.example",
      schoolName: "OGS Nord",
    });
  });

  it("stops at once when the backend reports the setup as failed", async () => {
    fetchMock
      .mockImplementationOnce(() => answer({ status: "preparing" }))
      .mockImplementationOnce(() => answer({ status: "failed" }));

    const waiting = waitForDemoSchool("t", { cancelled: false }, () => {});
    await vi.advanceTimersByTimeAsync(2000);

    await expect(waiting).resolves.toEqual({ phase: "unavailable" });
    await vi.advanceTimersByTimeAsync(10_000);
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it("gives up after five minutes of preparing", async () => {
    fetchMock.mockImplementation(() => answer({ status: "preparing" }));
    let settled = false;
    const waiting = waitForDemoSchool(
      "t",
      { cancelled: false },
      () => {},
    ).finally(() => {
      settled = true;
    });

    await vi.advanceTimersByTimeAsync(5 * 60_000 - 2000);
    expect(settled).toBe(false);
    await vi.advanceTimersByTimeAsync(2000);

    await expect(waiting).resolves.toEqual({ phase: "failed" });
  });
});
