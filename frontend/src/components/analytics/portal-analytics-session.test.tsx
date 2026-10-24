import { act, render, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { PortalAnalyticsSession } from "./portal-analytics-session";

const mocks = vi.hoisted(() => ({
  useSession: vi.fn(),
  registerPortalSession: vi.fn(),
  clearPortalSession: vi.fn(),
  setAnalyticsContext: vi.fn(),
  startAnalyticsConsent: vi.fn(),
  stopAnalyticsConsent: vi.fn(),
  setAnalyticsConsentSchool: vi.fn(),
}));

vi.mock("next-auth/react", () => ({ useSession: mocks.useSession }));
vi.mock("~/lib/analytics", () => ({
  registerPortalSession: mocks.registerPortalSession,
  clearPortalSession: mocks.clearPortalSession,
}));

vi.mock("~/lib/posthog-client", () => ({
  setAnalyticsContext: mocks.setAnalyticsContext,
}));
// The consent store and its prompt have their own tests; here only the
// session wiring counts.
vi.mock("~/lib/analytics-consent", () => ({
  startAnalyticsConsent: mocks.startAnalyticsConsent,
  stopAnalyticsConsent: mocks.stopAnalyticsConsent,
  setAnalyticsConsentSchool: mocks.setAnalyticsConsentSchool,
}));
vi.mock("~/components/analytics/analytics-consent-prompt", () => ({
  AnalyticsConsentPrompt: () => null,
}));
vi.mock("~/components/analytics/analytics-consent-copy", () => ({
  germanAnalyticsConsentCopy: () => ({}),
  useParentAnalyticsConsentCopy: () => ({}),
}));

describe("PortalAnalyticsSession", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubGlobal(
      "fetch",
      vi.fn(
        async () => new Response(JSON.stringify({ data: { enabled: true } })),
      ),
    );
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it.each(["parents", "school"] as const)(
    "refreshes a visible %s tab every five minutes and cleans up the timer",
    async (surface) => {
      vi.useFakeTimers();
      mocks.useSession.mockReturnValue({
        status: "authenticated",
        data: {
          user: {
            id: "7",
            scope: surface === "parents" ? "parent" : "school",
            tenantId: 42,
          },
        },
      });
      const visible = vi
        .spyOn(document, "visibilityState", "get")
        .mockReturnValue("visible");
      const view = render(<PortalAnalyticsSession surface={surface} />);
      await act(async () => {});
      expect(mocks.registerPortalSession).toHaveBeenCalledOnce();
      visible.mockReturnValue("hidden");
      await act(async () => {
        await vi.advanceTimersByTimeAsync(5 * 60 * 1_000);
      });
      expect(fetch).toHaveBeenCalledTimes(1);
      visible.mockReturnValue("visible");
      vi.mocked(fetch).mockRejectedValueOnce(new Error("Policy unavailable"));
      mocks.setAnalyticsContext.mockClear();
      await act(async () => {
        await vi.advanceTimersByTimeAsync(5 * 60 * 1_000);
      });
      expect(fetch).toHaveBeenCalledTimes(2);
      expect(mocks.setAnalyticsContext).toHaveBeenLastCalledWith({
        surface,
        enabled: false,
      });
      expect(mocks.registerPortalSession).toHaveBeenCalledOnce();
      view.unmount();
      await act(async () => {
        await vi.advanceTimersByTimeAsync(5 * 60 * 1_000);
      });
      document.dispatchEvent(new Event("visibilitychange"));
      expect(fetch).toHaveBeenCalledTimes(2);
    },
  );

  it("registers a parent at login without a school", async () => {
    mocks.useSession.mockReturnValue({
      status: "authenticated",
      data: { user: { id: "7", scope: "parent", roles: ["guardian"] } },
    });

    render(<PortalAnalyticsSession surface="parents" />);

    await waitFor(() =>
      expect(mocks.registerPortalSession).toHaveBeenCalledWith(
        { surface: "parents", schoolId: null, role: "guardian", enabled: true },
        false,
      ),
    );
  });

  it.each([
    { surface: "parents", scope: "parent", portal: "parent" },
    { surface: "school", scope: "school", portal: "school" },
  ] as const)(
    "reads the $surface account's consent and the school decision for its prompt",
    async ({ surface, scope, portal }) => {
      mocks.useSession.mockReturnValue({
        status: "authenticated",
        data: { user: { id: "7", scope, tenantId: 42 } },
      });

      render(<PortalAnalyticsSession surface={surface} />);

      expect(mocks.startAnalyticsConsent).toHaveBeenCalledWith(portal, "7");
      await waitFor(() =>
        expect(mocks.setAnalyticsConsentSchool).toHaveBeenCalledWith(
          portal,
          true,
        ),
      );
    },
  );

  it("registers a teacher of the school portal with the school", async () => {
    mocks.useSession.mockReturnValue({
      status: "authenticated",
      data: { user: { id: "9", scope: "school", tenantId: 42 } },
    });

    render(<PortalAnalyticsSession surface="school" />);

    await waitFor(() =>
      expect(mocks.registerPortalSession).toHaveBeenCalledWith(
        { surface: "school", schoolId: "42", role: "lehrkraft", enabled: true },
        false,
      ),
    );
  });

  it.each([false, undefined])(
    "keeps collection off for a restrictive or failed policy (%s)",
    async (enabled) => {
      mocks.useSession.mockReturnValue({
        status: "authenticated",
        data: { user: { id: "7", scope: "parent" } },
      });
      vi.mocked(global.fetch).mockResolvedValue(
        new Response(JSON.stringify({ data: { enabled } })),
      );
      render(<PortalAnalyticsSession surface="parents" />);
      await waitFor(() =>
        expect(mocks.registerPortalSession).toHaveBeenCalledWith(
          {
            surface: "parents",
            schoolId: null,
            role: "guardian",
            enabled: false,
          },
          false,
        ),
      );
    },
  );

  it("clears the context at logout", () => {
    mocks.useSession.mockReturnValue({
      status: "authenticated",
      data: { user: { id: "9", scope: "school", tenantId: 42 } },
    });
    const view = render(<PortalAnalyticsSession surface="school" />);

    mocks.useSession.mockReturnValue({ status: "unauthenticated", data: null });
    view.rerender(<PortalAnalyticsSession surface="school" />);

    expect(mocks.clearPortalSession).toHaveBeenCalledOnce();
    expect(mocks.stopAnalyticsConsent).toHaveBeenCalledWith("school");
  });

  it("resets the identity when the school changes", async () => {
    mocks.useSession.mockReturnValue({
      status: "authenticated",
      data: { user: { id: "9", scope: "school", tenantId: 42 } },
    });
    const view = render(<PortalAnalyticsSession surface="school" />);
    await waitFor(() =>
      expect(mocks.registerPortalSession).toHaveBeenCalledOnce(),
    );

    mocks.useSession.mockReturnValue({
      status: "authenticated",
      data: { user: { id: "9", scope: "school", tenantId: 43 } },
    });
    view.rerender(<PortalAnalyticsSession surface="school" />);

    await waitFor(() =>
      expect(mocks.registerPortalSession).toHaveBeenLastCalledWith(
        { surface: "school", schoolId: "43", role: "lehrkraft", enabled: true },
        true,
      ),
    );
  });

  it.each([
    { surface: "parents", scope: "parent", schoolId: null },
    { surface: "school", scope: "school", schoolId: "42" },
  ] as const)(
    "resets the $surface session when only the account changes",
    async ({ surface, scope, schoolId }) => {
      const user = { id: "7", scope, tenantId: schoolId };
      mocks.useSession.mockReturnValue({
        status: "authenticated",
        data: { user },
      });
      const view = render(<PortalAnalyticsSession surface={surface} />);
      await waitFor(() =>
        expect(mocks.registerPortalSession).toHaveBeenCalledOnce(),
      );

      mocks.useSession.mockReturnValue({
        status: "authenticated",
        data: { user: { ...user, id: "8" } },
      });
      view.rerender(<PortalAnalyticsSession surface={surface} />);

      await waitFor(() =>
        expect(mocks.registerPortalSession).toHaveBeenLastCalledWith(
          {
            surface,
            schoolId,
            role: surface === "parents" ? "guardian" : "lehrkraft",
            enabled: true,
          },
          true,
        ),
      );
    },
  );

  it("registers nothing for a session of another portal", () => {
    mocks.useSession.mockReturnValue({
      status: "authenticated",
      data: { user: { id: "9", scope: "", tenantId: 42 } },
    });

    render(<PortalAnalyticsSession surface="school" />);

    expect(mocks.registerPortalSession).not.toHaveBeenCalled();
    expect(mocks.clearPortalSession).toHaveBeenCalledOnce();
  });

  it("waits while the session loads", () => {
    mocks.useSession.mockReturnValue({ status: "loading", data: null });

    render(<PortalAnalyticsSession surface="parents" />);

    expect(mocks.registerPortalSession).not.toHaveBeenCalled();
    expect(mocks.clearPortalSession).not.toHaveBeenCalled();
  });
});
