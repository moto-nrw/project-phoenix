import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { OverflowMenuItem } from "~/components/ui/page-header/OverflowMenu";

const {
  mockMarkAllMessagesRead,
  mockMutate,
  mockUnread,
  mockToastSuccess,
  mockPush,
  mockInbox,
  mockTenant,
} = vi.hoisted(() => ({
  mockMarkAllMessagesRead: vi.fn(),
  mockMutate: vi.fn(),
  mockUnread: { unreadCount: 0 },
  mockToastSuccess: vi.fn(),
  mockPush: vi.fn(),
  mockInbox: { threads: [] as Array<Record<string, unknown>> },
  mockTenant: { messagingEnabled: true },
}));

vi.mock("swr", () => ({
  default: () => ({
    data: mockInbox.threads,
    error: undefined,
    isLoading: false,
    mutate: mockMutate,
  }),
}));

vi.mock("~/lib/tenant-context", () => ({
  useTenant: () => ({ tenant: mockTenant }),
  useTenantSlugSafe: () => "schule",
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: mockPush }),
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

function openMenu() {
  fireEvent.click(screen.getByRole("button", { name: "Weitere Aktionen" }));
}

async function findMarkAllRead() {
  openMenu();
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
    mockInbox.threads = [];
    mockTenant.messagingEnabled = true;
    mockMutate.mockResolvedValue(undefined);
    unreadRefreshes = 0;
    window.addEventListener("messages-unread-refresh", countRefresh);
  });

  afterEach(() => {
    window.removeEventListener("messages-unread-refresh", countRefresh);
  });

  it("marks all read, refreshes the badge and the inbox and confirms", async () => {
    mockMarkAllMessagesRead.mockResolvedValue(0);
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

  it("says why team-marked conversations stay unread", async () => {
    mockMarkAllMessagesRead.mockResolvedValue(1);
    render(<MessagesPage />);

    fireEvent.click(await findMarkAllRead());

    await waitFor(() =>
      expect(mockToastSuccess).toHaveBeenCalledWith(
        "Gelesen. Vom Team als ungelesen markierte Unterhaltungen bleiben ungelesen.",
      ),
    );
    expect(unreadRefreshes).toBe(1);
  });

  // #3673: an action that can do nothing is not in the menu, instead of a
  // greyed-out entry without a reason.
  it("is not offered without anything unread", async () => {
    mockUnread.unreadCount = 0;
    render(<MessagesPage />);

    openMenu();

    expect(
      await screen.findByRole("menuitem", {
        name: "Zahl bei Nachrichten einstellen",
      }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("menuitem", { name: "Alle als gelesen markieren" }),
    ).not.toBeInTheDocument();
    expect(mockMarkAllMessagesRead).not.toHaveBeenCalled();
  });

  // A person whose own counter skips some conversations (#3673) still sees
  // them unread in the inbox and can clear them.
  it("is offered for unread inbox rows while the own counter is zero", async () => {
    mockUnread.unreadCount = 0;
    mockInbox.threads = [
      {
        thread_id: "7",
        student_id: "3",
        student_name: "Felix Schneider",
        guardian_name: "Sabine Schneider",
        unread_count: 2,
      },
    ];
    mockMarkAllMessagesRead.mockResolvedValue(0);
    render(<MessagesPage />);

    fireEvent.click(await findMarkAllRead());

    await waitFor(() =>
      expect(mockMarkAllMessagesRead).toHaveBeenCalledTimes(1),
    );
  });

  // #3673: with a counter that skips conversations the reported count is 0,
  // yet a team-marked conversation stays unread in the inbox.
  it("explains team-marked conversations the counter does not count", async () => {
    mockMarkAllMessagesRead.mockResolvedValue(0);
    mockMutate.mockResolvedValue([
      {
        thread_id: "7",
        student_id: "3",
        student_name: "Felix Schneider",
        guardian_name: "Sabine Schneider",
        unread_count: 1,
      },
    ]);
    render(<MessagesPage />);

    fireEvent.click(await findMarkAllRead());

    await waitFor(() =>
      expect(mockToastSuccess).toHaveBeenCalledWith(
        "Gelesen. Vom Team als ungelesen markierte Unterhaltungen bleiben ungelesen.",
      ),
    );
  });

  it("leads to the own count setting in the profile", async () => {
    render(<MessagesPage />);

    openMenu();
    fireEvent.click(
      await screen.findByRole("menuitem", {
        name: "Zahl bei Nachrichten einstellen",
      }),
    );

    expect(mockPush).toHaveBeenCalledWith("/profile");
  });

  it("does not offer the hidden setting while messaging is off", () => {
    mockTenant.messagingEnabled = false;
    render(<MessagesPage />);

    expect(
      screen.queryByRole("button", { name: "Weitere Aktionen" }),
    ).not.toBeInTheDocument();
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
