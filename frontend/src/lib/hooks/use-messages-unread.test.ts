import { beforeEach, describe, expect, it, vi } from "vitest";
import { renderHook } from "@testing-library/react";
import { useMessagesUnread } from "./use-messages-unread";

const { state, useUnreadCount } = vi.hoisted(() => ({
  state: {
    elternAppBooked: true,
    messagingEnabled: true,
    messagesUnread: 4,
  },
  useUnreadCount: vi.fn(() => ({
    unreadCount: 0,
    isLoading: false,
    refresh: vi.fn(),
  })),
}));

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { id: "account-1" } },
    status: "authenticated",
  }),
}));

vi.mock("~/lib/auth-utils", () => ({ hasPermission: () => true }));
vi.mock("~/lib/parent-messages-api", () => ({
  fetchUnreadCount: vi.fn(),
}));
vi.mock("~/lib/shell-auth-context", () => ({
  useShellAuth: () => ({ mode: "teacher" }),
}));
vi.mock("~/lib/shell-seed", () => ({
  useShellSeed: () => ({ counts: { messagesUnread: state.messagesUnread } }),
}));
vi.mock("~/lib/tenant-context", () => ({
  useModuleBooked: () => state.elternAppBooked,
  useTenantSafe: () => ({
    tenant: { messagingEnabled: state.messagingEnabled },
  }),
  useTenantSlugSafe: () => "testschule",
}));
vi.mock("./use-unread-count", () => ({ useUnreadCount }));

describe("useMessagesUnread", () => {
  beforeEach(() => {
    state.elternAppBooked = true;
    state.messagingEnabled = true;
    state.messagesUnread = 4;
    useUnreadCount.mockClear();
  });

  it("uses the server seed when parent messaging is available", () => {
    renderHook(() => useMessagesUnread());

    expect(useUnreadCount).toHaveBeenCalledWith(
      expect.objectContaining({ initialCount: 4 }),
    );
  });

  it("does not use the server seed without the Eltern-App module", () => {
    state.elternAppBooked = false;

    renderHook(() => useMessagesUnread());

    expect(useUnreadCount).toHaveBeenCalledWith(
      expect.objectContaining({ initialCount: undefined }),
    );
  });

  it("does not use the server seed while parent messaging is disabled", () => {
    state.messagingEnabled = false;

    renderHook(() => useMessagesUnread());

    expect(useUnreadCount).toHaveBeenCalledWith(
      expect.objectContaining({ initialCount: undefined }),
    );
  });
});
