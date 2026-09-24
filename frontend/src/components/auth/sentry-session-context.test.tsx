import { render } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { SentrySessionContext } from "./sentry-session-context";

const mocks = vi.hoisted(() => ({
  useSession: vi.fn(),
  setUser: vi.fn(),
  setTags: vi.fn(),
}));

vi.mock("next-auth/react", () => ({ useSession: mocks.useSession }));
vi.mock("@sentry/nextjs", () => ({
  setUser: mocks.setUser,
  setTags: mocks.setTags,
}));

describe("SentrySessionContext", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("attaches the signed-in account, its role and school", () => {
    mocks.useSession.mockReturnValue({
      status: "authenticated",
      data: {
        user: {
          id: "42",
          name: "Mia Muster",
          email: "mia@example.com",
          scope: "",
          tenantId: 12,
        },
      },
    });

    render(<SentrySessionContext portal="tenant" />);

    expect(mocks.setUser).toHaveBeenCalledWith({ id: "42" });
    expect(mocks.setTags).toHaveBeenCalledWith({
      role: "staff",
      school_id: "12",
    });
  });

  it("attaches a parent without a school", () => {
    mocks.useSession.mockReturnValue({
      status: "authenticated",
      data: { user: { id: "7", scope: "parent" } },
    });

    render(<SentrySessionContext portal="parent" />);

    expect(mocks.setUser).toHaveBeenCalledWith({ id: "7" });
    expect(mocks.setTags).toHaveBeenCalledWith({
      role: "guardian",
      school_id: undefined,
    });
  });

  it("clears the account at logout", () => {
    mocks.useSession.mockReturnValue({
      status: "authenticated",
      data: { user: { id: "42", tenantId: 12 } },
    });
    const { rerender } = render(<SentrySessionContext portal="tenant" />);

    mocks.useSession.mockReturnValue({ status: "unauthenticated", data: null });
    rerender(<SentrySessionContext portal="tenant" />);

    expect(mocks.setUser).toHaveBeenLastCalledWith(null);
    expect(mocks.setTags).toHaveBeenLastCalledWith({
      role: undefined,
      school_id: undefined,
    });
  });

  it("keeps the context while the session reloads", () => {
    mocks.useSession.mockReturnValue({ status: "loading", data: null });

    render(<SentrySessionContext portal="tenant" />);

    expect(mocks.setUser).not.toHaveBeenCalled();
    expect(mocks.setTags).not.toHaveBeenCalled();
  });
});
