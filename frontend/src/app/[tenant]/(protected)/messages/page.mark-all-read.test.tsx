import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { OverflowMenuItem } from "~/components/ui/page-header/OverflowMenu";

const { mockMarkAllMessagesRead, mockMutate, mockUnread, mockToastSuccess } =
  vi.hoisted(() => ({
    mockMarkAllMessagesRead: vi.fn(),
    mockMutate: vi.fn(),
    mockUnread: { unreadCount: 0 },
    mockToastSuccess: vi.fn(),
  }));

vi.mock("swr", () => ({
  default: () => ({
    data: [],
    error: undefined,
    isLoading: false,
    mutate: mockMutate,
  }),
}));

vi.mock("~/lib/tenant-context", () => ({
  useTenant: () => ({ tenant: { messagingEnabled: true } }),
  useTenantSlugSafe: () => "schule",
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: vi.fn() }),
}));

vi.mock("~/lib/hooks/use-messages-activity", () => ({
  useMessagesActivity: vi.fn(),
}));

vi.mock("~/lib/hooks/use-messages-unread", () => ({
  useMessagesUnread: () => ({
    unreadCount: mockUnread.unreadCount,
    isLoading: false,
    refresh: vi.fn(),
  }),
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({ success: mockToastSuccess }),
}));

vi.mock("~/components/messaging/new-message-modal", () => ({
  NewMessageModal: () => null,
}));

// The page scaffold is replaced by its contract: the overflow menu and the
// page content. The real overflow menu stays.
vi.mock("~/components/ui/tenant-page", async () => {
  const { OverflowMenu } =
    await import("~/components/ui/page-header/OverflowMenu");
  return {
    TenantPage: ({
      overflowMenu,
      children,
    }: {
      overflowMenu?: readonly OverflowMenuItem[];
      children?: ReactNode;
    }) => (
      <div>
        {overflowMenu && (
          <OverflowMenu
            ariaLabel="Weitere Aktionen"
            items={[...overflowMenu]}
          />
        )}
        {children}
      </div>
    ),
  };
});

vi.mock("~/lib/parent-messages-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/parent-messages-api")>()),
  markAllMessagesRead: mockMarkAllMessagesRead,
}));

import MessagesPage from "./page";

async function findMarkAllRead() {
  fireEvent.click(screen.getByRole("button", { name: "Weitere Aktionen" }));
  return screen.findByRole("menuitem", { name: "Alle als gelesen markieren" });
}

describe("Alle als gelesen markieren", () => {
  let unreadRefreshes: number;
  const countRefresh = () => {
    unreadRefreshes++;
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockUnread.unreadCount = 3;
    unreadRefreshes = 0;
    window.addEventListener("messages-unread-refresh", countRefresh);
  });

  afterEach(() => {
    window.removeEventListener("messages-unread-refresh", countRefresh);
  });

  it("marks all read, refreshes the badge and the inbox and confirms", async () => {
    mockMarkAllMessagesRead.mockResolvedValue(undefined);
    render(<MessagesPage />);

    fireEvent.click(await findMarkAllRead());

    await waitFor(() =>
      expect(mockToastSuccess).toHaveBeenCalledWith(
        "Alle Nachrichten sind für Sie als gelesen markiert.",
      ),
    );
    expect(mockMarkAllMessagesRead).toHaveBeenCalledTimes(1);
    expect(unreadRefreshes).toBe(1);
    expect(mockMutate).toHaveBeenCalled();
  });

  it("is disabled without unread messages", async () => {
    mockUnread.unreadCount = 0;
    render(<MessagesPage />);

    const item = await findMarkAllRead();

    expect(item).toBeDisabled();
    fireEvent.click(item);
    expect(mockMarkAllMessagesRead).not.toHaveBeenCalled();
  });

  it("shows a hint and no confirmation when marking fails", async () => {
    mockMarkAllMessagesRead.mockRejectedValue(
      new Error("messaging: forbidden"),
    );
    render(<MessagesPage />);

    fireEvent.click(await findMarkAllRead());

    expect(
      await screen.findByText(
        "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.",
      ),
    ).toBeInTheDocument();
    expect(mockToastSuccess).not.toHaveBeenCalled();
    expect(unreadRefreshes).toBe(0);
  });
});
