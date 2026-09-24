import { render } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { PortalAnalyticsSession } from "./portal-analytics-session";

const mocks = vi.hoisted(() => ({
  useSession: vi.fn(),
  registerPortalSession: vi.fn(),
  clearPortalSession: vi.fn(),
}));

vi.mock("next-auth/react", () => ({ useSession: mocks.useSession }));
vi.mock("~/lib/analytics", () => ({
  registerPortalSession: mocks.registerPortalSession,
  clearPortalSession: mocks.clearPortalSession,
}));

describe("PortalAnalyticsSession", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("registers a parent at login without a school", () => {
    mocks.useSession.mockReturnValue({
      status: "authenticated",
      data: { user: { id: "7", scope: "parent", roles: ["guardian"] } },
    });

    render(<PortalAnalyticsSession surface="parents" />);

    expect(mocks.registerPortalSession).toHaveBeenCalledWith(
      { surface: "parents", schoolId: null, role: "guardian" },
      false,
    );
  });

  it("registers a teacher of the school portal with the school", () => {
    mocks.useSession.mockReturnValue({
      status: "authenticated",
      data: { user: { id: "9", scope: "school", tenantId: 42 } },
    });

    render(<PortalAnalyticsSession surface="school" />);

    expect(mocks.registerPortalSession).toHaveBeenCalledWith(
      { surface: "school", schoolId: "42", role: "lehrkraft" },
      false,
    );
  });

  it("clears the context at logout", () => {
    mocks.useSession.mockReturnValue({
      status: "authenticated",
      data: { user: { id: "9", scope: "school", tenantId: 42 } },
    });
    const view = render(<PortalAnalyticsSession surface="school" />);

    mocks.useSession.mockReturnValue({ status: "unauthenticated", data: null });
    view.rerender(<PortalAnalyticsSession surface="school" />);

    expect(mocks.clearPortalSession).toHaveBeenCalledOnce();
  });

  it("resets the identity when the school changes", () => {
    mocks.useSession.mockReturnValue({
      status: "authenticated",
      data: { user: { id: "9", scope: "school", tenantId: 42 } },
    });
    const view = render(<PortalAnalyticsSession surface="school" />);

    mocks.useSession.mockReturnValue({
      status: "authenticated",
      data: { user: { id: "9", scope: "school", tenantId: 43 } },
    });
    view.rerender(<PortalAnalyticsSession surface="school" />);

    expect(mocks.registerPortalSession).toHaveBeenLastCalledWith(
      { surface: "school", schoolId: "43", role: "lehrkraft" },
      true,
    );
  });

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
