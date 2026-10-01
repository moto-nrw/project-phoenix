import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const {
  mockPush,
  mockMarkThreadUnread,
  mockFetchThread,
  mockMutate,
  mockUseSWR,
} = vi.hoisted(() => ({
  mockPush: vi.fn(),
  mockMarkThreadUnread: vi.fn(),
  mockFetchThread: vi.fn(),
  mockMutate: vi.fn(),
  mockUseSWR: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useParams: () => ({ threadId: "t1" }),
}));

vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: null }),
}));

vi.mock("swr", () => ({
  default: (...args: unknown[]) => mockUseSWR(...args),
  unstable_serialize: (key: unknown) => JSON.stringify(key),
  useSWRConfig: () => ({ cache: new Map() }),
}));

const loadedThread = {
  thread_id: "t1",
  student_id: "42",
  student_name: "Max Muster",
  guardian_name: "Anna Muster",
  messages: [
    {
      id: "m1",
      sender_kind: "guardian",
      sender_name: "Anna Muster",
      body: "Hallo",
      created_at: "2026-09-09T10:00:00Z",
      kind: "message",
    },
  ],
};

function swrResult() {
  return {
    data: loadedThread,
    error: undefined,
    isLoading: false,
    isValidating: false,
    mutate: mockMutate,
  };
}

vi.mock("~/lib/tenant-context", () => ({
  useTenant: () => ({ tenant: { messagingEnabled: true } }),
  useTenantSlugSafe: () => "schule",
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: mockPush }),
}));

vi.mock("~/lib/hooks/use-messages-activity", () => ({
  useMessagesActivity: vi.fn(),
}));

vi.mock("~/lib/hooks/use-chat-viewport-lock", () => ({
  useChatViewportLock: () => ({ current: null }),
}));

vi.mock("~/components/ui/back-button", () => ({
  BackButton: () => null,
}));

vi.mock("~/components/messaging/pickup-request-detail-modal", () => ({
  PickupRequestDetailModal: () => null,
}));

vi.mock("~/components/messaging/message-composer", () => ({
  MessageComposer: () => null,
}));

// The page scaffold is replaced by its contract: the title-row actions and the
// page content. The real overflow menu stays.
vi.mock("~/components/ui/tenant-page", () => ({
  TenantPage: ({
    actions,
    children,
  }: {
    actions?: ReactNode;
    children?: ReactNode;
  }) => (
    <div>
      {actions}
      {children}
    </div>
  ),
}));

vi.mock("~/lib/parent-messages-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/parent-messages-api")>()),
  fetchThread: mockFetchThread,
  markThreadUnread: mockMarkThreadUnread,
}));

import MessageThreadPage from "./page";

async function chooseMarkUnread() {
  fireEvent.click(screen.getByRole("button", { name: "Weitere Aktionen" }));
  fireEvent.click(
    await screen.findByRole("menuitem", { name: "Als ungelesen markieren" }),
  );
}

describe("Als ungelesen markieren", () => {
  let unreadRefreshes: number;
  const countRefresh = () => {
    unreadRefreshes++;
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseSWR.mockImplementation(swrResult);
    unreadRefreshes = 0;
    window.addEventListener("messages-unread-refresh", countRefresh);
  });

  afterEach(() => {
    window.removeEventListener("messages-unread-refresh", countRefresh);
  });

  it("marks the thread, refreshes the badge and returns to the inbox", async () => {
    mockMarkThreadUnread.mockResolvedValue(undefined);
    render(<MessageThreadPage />);
    const refreshesAfterLoad = unreadRefreshes;

    await chooseMarkUnread();

    await waitFor(() => expect(mockPush).toHaveBeenCalledWith("/messages"));
    expect(mockMarkThreadUnread).toHaveBeenCalledWith("t1");
    expect(unreadRefreshes).toBe(refreshesAfterLoad + 1);
  });

  it("stays on the thread and shows a hint when marking fails", async () => {
    mockMarkThreadUnread.mockRejectedValue(new Error("messaging: forbidden"));
    render(<MessageThreadPage />);

    await chooseMarkUnread();

    expect(
      await screen.findByText(
        "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.",
      ),
    ).toBeInTheDocument();
    expect(mockPush).not.toHaveBeenCalled();
  });

  it("waits for a pending thread read before marking unread", async () => {
    let finishLoad!: (value: typeof loadedThread) => void;
    mockFetchThread.mockReturnValue(
      new Promise((resolve) => {
        finishLoad = resolve;
      }),
    );
    let started = false;
    mockUseSWR.mockImplementation((_key, fetcher: () => Promise<unknown>) => {
      if (!started) {
        started = true;
        void fetcher();
      }
      return swrResult();
    });
    mockMarkThreadUnread.mockResolvedValue(undefined);

    render(<MessageThreadPage />);
    await chooseMarkUnread();

    expect(mockFetchThread).toHaveBeenCalledWith("t1");
    expect(mockMarkThreadUnread).not.toHaveBeenCalled();
    finishLoad(loadedThread);
    await waitFor(() => expect(mockPush).toHaveBeenCalledWith("/messages"));
    expect(mockMarkThreadUnread).toHaveBeenCalledWith("t1");
  });
});
