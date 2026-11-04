import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { resumeSWRSession } from "~/lib/swr/scope";
import { SWRConfig } from "~/lib/swr/testing";
import { SWRScopeBoundary } from "~/lib/swr/scope-boundary";
import type { Session } from "next-auth";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

vi.unmock("swr");
vi.mock("~/lib/session-cache", () => ({ announceSessionChange: vi.fn() }));
const { sessionState } = vi.hoisted(() => ({
  sessionState: { data: null as Session | null, status: "authenticated" },
}));
vi.mock("next-auth/react", () => ({ useSession: () => sessionState }));
vi.mock("~/lib/tenant-context", () => ({ useTenantSlugSafe: () => null }));

function render(ui: React.ReactElement) {
  const cache = new Map();
  return renderPlain(ui, {
    wrapper: ({ children }) => (
      <SWRConfig value={{ provider: () => cache, errorRetryCount: 0 }}>
        <ToastProvider>
          <SWRScopeBoundary />
          {children}
        </ToastProvider>
      </SWRConfig>
    ),
  });
}

import {
  render as renderPlain,
  act,
  screen,
  fireEvent,
  waitFor,
} from "@testing-library/react";
import { AnnouncementViewsAccordion } from "./announcement-views-accordion";
import type { AnnouncementViewDetail } from "~/lib/operator/announcements-helpers";

// Mock the announcements service
const { mockOperatorAnnouncementsService } = vi.hoisted(() => ({
  mockOperatorAnnouncementsService: {
    fetchViewDetails: vi.fn(),
    fetchStats: vi.fn(async () => ({ dismissed_count: 3 })),
  },
}));

vi.mock("~/lib/operator/announcements-api", () => ({
  operatorAnnouncementsService: mockOperatorAnnouncementsService,
}));

// Mock format utils
vi.mock("~/lib/format-utils", () => ({
  getRelativeTime: vi.fn((_date: string) => "vor 2 Stunden"),
  getInitial: vi.fn((name: string) => name.charAt(0).toUpperCase()),
}));

afterEach(() => resumeSWRSession());

describe("AnnouncementViewsAccordion", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockOperatorAnnouncementsService.fetchStats.mockResolvedValue({
      dismissed_count: 3,
    });
    sessionState.data = {
      user: { id: "operator-a", scope: "platform" },
      expires: "2099-01-01",
    };
    sessionState.status = "authenticated";
  });

  it.each([0, 5])("shows count %d only while opened", async (count) => {
    mockOperatorAnnouncementsService.fetchStats.mockResolvedValue({
      dismissed_count: count,
    });
    mockOperatorAnnouncementsService.fetchViewDetails.mockResolvedValue([]);
    render(<AnnouncementViewsAccordion announcementId="42" />);
    const toggle = screen.getByRole("button", { name: "Lesebestätigungen" });
    // Anker: the collapsed card is available without a statistics response.
    expect(toggle).toBeInTheDocument();
    expect(toggle).not.toHaveTextContent(/\(\d+\)/);
    expect(mockOperatorAnnouncementsService.fetchStats).not.toHaveBeenCalled();
    fireEvent.click(toggle);
    expect(await screen.findByText(`(${count})`)).toBeInTheDocument();
    expect(toggle).toHaveAttribute("aria-expanded", "true");
    fireEvent.click(toggle);
    // Anker: the same card remains present after closing.
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(toggle).not.toHaveTextContent(`(${count})`);
  });

  it("toggles accordion on header click", async () => {
    mockOperatorAnnouncementsService.fetchViewDetails.mockResolvedValue([]);

    render(<AnnouncementViewsAccordion announcementId="42" />);

    const button = screen.getByRole("button");
    expect(screen.queryByText("Laden...")).not.toBeInTheDocument();

    fireEvent.click(button);

    await waitFor(() => {
      expect(
        mockOperatorAnnouncementsService.fetchViewDetails,
      ).toHaveBeenCalledWith("42");
    });
  });

  it("shows loading state when fetching", async () => {
    mockOperatorAnnouncementsService.fetchViewDetails.mockImplementation(
      () => new Promise((resolve) => setTimeout(() => resolve([]), 100)),
    );

    render(<AnnouncementViewsAccordion announcementId="42" />);

    const button = screen.getByRole("button");
    fireEvent.click(button);

    await waitFor(() => {
      expect(screen.getByText("Laden...")).toBeInTheDocument();
    });
  });

  it("shows a failed load with retry", async () => {
    mockOperatorAnnouncementsService.fetchViewDetails
      .mockRejectedValueOnce(new ApiError("Network error", 503))
      .mockResolvedValueOnce([]);

    render(<AnnouncementViewsAccordion announcementId="42" />);

    const button = screen.getByRole("button");
    fireEvent.click(button);

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Liste der Lesebestätigungen"),
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    await waitFor(() =>
      expect(
        mockOperatorAnnouncementsService.fetchViewDetails,
      ).toHaveBeenCalledTimes(2),
    );
  });

  it("displays confirmed users after loading", async () => {
    const mockViewDetails: AnnouncementViewDetail[] = [
      {
        userId: "1",
        userName: "Alice",
        seenAt: "2024-01-01T10:00:00Z",
        dismissed: true,
      },
      {
        userId: "2",
        userName: "Bob",
        seenAt: "2024-01-01T11:00:00Z",
        dismissed: true,
      },
    ];

    mockOperatorAnnouncementsService.fetchViewDetails.mockResolvedValue(
      mockViewDetails,
    );

    render(<AnnouncementViewsAccordion announcementId="42" />);

    const button = screen.getByRole("button");
    fireEvent.click(button);

    await waitFor(() => {
      expect(screen.getByText("Alice")).toBeInTheDocument();
      expect(screen.getByText("Bob")).toBeInTheDocument();
    });
  });

  it("only shows dismissed users", async () => {
    const mockViewDetails: AnnouncementViewDetail[] = [
      {
        userId: "1",
        userName: "Alice",
        seenAt: "2024-01-01T10:00:00Z",
        dismissed: true,
      },
      {
        userId: "2",
        userName: "Bob",
        seenAt: "2024-01-01T11:00:00Z",
        dismissed: false,
      },
    ];

    mockOperatorAnnouncementsService.fetchViewDetails.mockResolvedValue(
      mockViewDetails,
    );

    render(<AnnouncementViewsAccordion announcementId="42" />);

    const button = screen.getByRole("button");
    fireEvent.click(button);

    await waitFor(() => {
      expect(screen.getByText("Alice")).toBeInTheDocument();
      expect(screen.queryByText("Bob")).not.toBeInTheDocument();
    });
  });

  it("shows an explicit empty state for zero confirmations after opening", async () => {
    mockOperatorAnnouncementsService.fetchStats.mockResolvedValue({
      dismissed_count: 0,
    });
    mockOperatorAnnouncementsService.fetchViewDetails.mockResolvedValue([]);
    render(<AnnouncementViewsAccordion announcementId="42" />);
    fireEvent.click(screen.getByRole("button", { name: "Lesebestätigungen" }));
    expect(
      await screen.findByText("Noch keine Lesebestätigungen."),
    ).toBeInTheDocument();
    expect(screen.getByText("(0)")).toBeInTheDocument();
  });

  it("stops statistics polling after closing", async () => {
    vi.useFakeTimers();
    mockOperatorAnnouncementsService.fetchViewDetails.mockResolvedValue([]);
    render(<AnnouncementViewsAccordion announcementId="42" />);
    fireEvent.click(screen.getByRole("button", { name: "Lesebestätigungen" }));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1);
    });
    expect(screen.getByText("(3)")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Lesebestätigungen/ }));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(30001);
    });
    expect(
      screen.getByRole("button", { name: "Lesebestätigungen" }),
    ).toHaveAttribute("aria-expanded", "false");
    expect(mockOperatorAnnouncementsService.fetchStats).toHaveBeenCalledTimes(
      1,
    );
  });

  it("shows empty state message when all users not dismissed", async () => {
    const mockViewDetails: AnnouncementViewDetail[] = [
      {
        userId: "1",
        userName: "Alice",
        seenAt: "2024-01-01T10:00:00Z",
        dismissed: false,
      },
    ];

    mockOperatorAnnouncementsService.fetchViewDetails.mockResolvedValue(
      mockViewDetails,
    );

    render(<AnnouncementViewsAccordion announcementId="42" />);

    const button = screen.getByRole("button");
    fireEvent.click(button);

    await waitFor(() => {
      expect(
        screen.getByText("Noch keine Lesebestätigungen."),
      ).toBeInTheDocument();
    });
  });

  it("rotates chevron icon when opening", async () => {
    mockOperatorAnnouncementsService.fetchViewDetails.mockResolvedValue([]);

    const { container } = renderPlain(
      <AnnouncementViewsAccordion announcementId="42" />,
    );

    const button = screen.getByRole("button");
    const chevron = container.querySelector("svg");

    expect(chevron).not.toHaveClass("rotate-180");

    fireEvent.click(button);

    await waitFor(() => {
      expect(chevron).toHaveClass("rotate-180");
    });
  });

  it("only loads data once", async () => {
    mockOperatorAnnouncementsService.fetchViewDetails.mockResolvedValue([]);

    render(<AnnouncementViewsAccordion announcementId="42" />);

    const button = screen.getByRole("button");

    // Open
    fireEvent.click(button);
    await waitFor(() => {
      expect(
        mockOperatorAnnouncementsService.fetchViewDetails,
      ).toHaveBeenCalledTimes(1);
    });

    // Close
    fireEvent.click(button);
    await waitFor(() => {
      expect(
        mockOperatorAnnouncementsService.fetchViewDetails,
      ).toHaveBeenCalledTimes(1);
    });

    // Open again
    fireEvent.click(button);
    await waitFor(() => {
      expect(
        mockOperatorAnnouncementsService.fetchViewDetails,
      ).toHaveBeenCalledTimes(1); // Still 1, not reloaded
    });
  });

  it.each(["account", "logout", "other-tab-account", "other-tab-logout"])(
    "withdraws confirmations on %s while the component stays mounted",
    async (change) => {
      mockOperatorAnnouncementsService.fetchViewDetails.mockResolvedValue([
        {
          userId: "1",
          userName: "Person A",
          seenAt: "2026-09-09T10:00:00Z",
          dismissed: true,
        },
      ]);
      const view = render(<AnnouncementViewsAccordion announcementId="42" />);
      fireEvent.click(
        screen.getByRole("button", { name: /Lesebestätigungen/ }),
      );
      expect(await screen.findByText("Person A")).toBeInTheDocument();
      mockOperatorAnnouncementsService.fetchViewDetails.mockImplementation(
        () => new Promise(() => undefined),
      );
      act(() => {
        if (change.startsWith("other-tab")) {
          globalThis.dispatchEvent(
            new StorageEvent("storage", {
              key: "moto:personal-storage-owner",
              newValue: change.endsWith("logout") ? null : "account:operator-b",
            }),
          );
        } else {
          sessionState.data =
            change === "logout"
              ? null
              : {
                  user: { id: "operator-b", scope: "platform" },
                  expires: "2099-01-01",
                };
          sessionState.status =
            change === "logout" ? "unauthenticated" : "authenticated";
          view.rerender(<AnnouncementViewsAccordion announcementId="42" />);
        }
      });
      // Anker: the same mounted, expanded accordion remains available.
      expect(
        screen.getByRole("button", { name: /Lesebestätigungen/ }),
      ).toBeInTheDocument();
      expect(screen.queryByText("Person A")).not.toBeInTheDocument();
    },
  );

  it("starts neither details nor statistics before opening", () => {
    render(<AnnouncementViewsAccordion announcementId="42" />);
    // Anker: the loaded card offers its confirmations before any stats exist.
    expect(
      screen.getByRole("button", { name: /Lesebestätigungen/ }),
    ).toBeInTheDocument();
    expect(
      mockOperatorAnnouncementsService.fetchViewDetails,
    ).not.toHaveBeenCalled();
    expect(mockOperatorAnnouncementsService.fetchStats).not.toHaveBeenCalled();
  });

  it("stops event propagation on accordion", () => {
    const parentClick = vi.fn();

    render(
      // eslint-disable-next-line jsx-a11y/click-events-have-key-events, jsx-a11y/no-static-element-interactions
      <div onClick={parentClick} role="presentation">
        <AnnouncementViewsAccordion announcementId="42" />
      </div>,
    );

    const button = screen.getByRole("button");
    fireEvent.click(button);

    expect(parentClick).not.toHaveBeenCalled();
  });
});
