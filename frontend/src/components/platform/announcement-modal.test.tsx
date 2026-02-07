import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { AnnouncementModal } from "./announcement-modal";
import type { ReactNode } from "react";

// Mock Modal component
vi.mock("~/components/ui/modal", () => ({
  Modal: ({
    isOpen,
    _onClose,
    children,
    footer,
  }: {
    isOpen: boolean;
    _onClose: () => void;
    children: ReactNode;
    footer: ReactNode;
  }) =>
    isOpen ? (
      <div data-testid="modal">
        <div data-testid="modal-content">{children}</div>
        <div data-testid="modal-footer">{footer}</div>
      </div>
    ) : null,
}));

// Hoist mock functions so they can be used in the mock return value
const mockDismiss = vi.hoisted(() => vi.fn());
const mockRefresh = vi.hoisted(() => vi.fn());

// Mock useAnnouncements hook
const mockAnnouncementsState = vi.hoisted(() => ({
  announcements: [] as Array<{
    id: number;
    title: string;
    content: string;
    type: string;
    severity: string;
    version?: string;
    published_at: string;
  }>,
  isLoading: false,
}));

vi.mock("~/lib/hooks/use-announcements", () => ({
  useAnnouncements: vi.fn(() => ({
    announcements: mockAnnouncementsState.announcements,
    dismiss: mockDismiss,
    isLoading: mockAnnouncementsState.isLoading,
    refresh: mockRefresh,
  })),
}));

describe("AnnouncementModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAnnouncementsState.announcements = [];
    mockAnnouncementsState.isLoading = false;
    mockDismiss.mockResolvedValue(undefined);
    mockRefresh.mockResolvedValue(undefined);
  });

  it("renders nothing when there are no announcements", () => {
    render(<AnnouncementModal />);
    expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
  });

  it("renders nothing when loading", () => {
    mockAnnouncementsState.isLoading = true;
    mockAnnouncementsState.announcements = [
      {
        id: 1,
        title: "Test",
        content: "Content",
        type: "announcement",
        severity: "info",
        published_at: "2024-01-01T00:00:00Z",
      },
    ];
    render(<AnnouncementModal />);
    expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
  });

  it("displays modal when announcements are available", async () => {
    mockAnnouncementsState.announcements = [
      {
        id: 1,
        title: "New Feature",
        content: "We added something cool!",
        type: "release",
        severity: "info",
        version: "1.0.0",
        published_at: "2024-01-01T00:00:00Z",
      },
    ];

    render(<AnnouncementModal />);

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    expect(screen.getByText("New Feature")).toBeInTheDocument();
    expect(screen.getByText("We added something cool!")).toBeInTheDocument();
    expect(screen.getByText("v1.0.0")).toBeInTheDocument();
  });

  it("displays correct header for release type", async () => {
    mockAnnouncementsState.announcements = [
      {
        id: 1,
        title: "Version 2.0",
        content: "Major update!",
        type: "release",
        severity: "info",
        published_at: "2024-01-01T00:00:00Z",
      },
    ];

    render(<AnnouncementModal />);

    await waitFor(() => {
      expect(
        screen.getByText("Neue Funktionen und Verbesserungen"),
      ).toBeInTheDocument();
    });
    expect(screen.getByText("Was ist neu?")).toBeInTheDocument();
  });

  it("displays correct header for maintenance type", async () => {
    mockAnnouncementsState.announcements = [
      {
        id: 1,
        title: "Scheduled Maintenance",
        content: "System will be down",
        type: "maintenance",
        severity: "warning",
        published_at: "2024-01-01T00:00:00Z",
      },
    ];

    render(<AnnouncementModal />);

    await waitFor(() => {
      expect(screen.getByText("Geplante Systemarbeiten")).toBeInTheDocument();
    });
    expect(screen.getByText("Wartungshinweis")).toBeInTheDocument();
  });

  it("displays correct header for announcement type", async () => {
    mockAnnouncementsState.announcements = [
      {
        id: 1,
        title: "Important Notice",
        content: "Please read this",
        type: "announcement",
        severity: "info",
        published_at: "2024-01-01T00:00:00Z",
      },
    ];

    render(<AnnouncementModal />);

    await waitFor(() => {
      expect(
        screen.getByText("Wichtige Informationen für Sie"),
      ).toBeInTheDocument();
    });
    expect(screen.getByText("Neuigkeiten")).toBeInTheDocument();
  });

  it("handles dismiss of single announcement", async () => {
    mockAnnouncementsState.announcements = [
      {
        id: 1,
        title: "Test",
        content: "Content",
        type: "announcement",
        severity: "info",
        published_at: "2024-01-01T00:00:00Z",
      },
    ];

    render(<AnnouncementModal />);

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    const dismissButton = screen.getByRole("button", { name: /verstanden/i });
    fireEvent.click(dismissButton);

    await waitFor(() => {
      expect(mockDismiss).toHaveBeenCalledWith(1);
      expect(mockRefresh).toHaveBeenCalled();
    });
  });

  it("navigates through multiple announcements", async () => {
    mockAnnouncementsState.announcements = [
      {
        id: 1,
        title: "First Announcement",
        content: "First content",
        type: "announcement",
        severity: "info",
        published_at: "2024-01-01T00:00:00Z",
      },
      {
        id: 2,
        title: "Second Announcement",
        content: "Second content",
        type: "release",
        severity: "info",
        published_at: "2024-01-02T00:00:00Z",
      },
    ];

    render(<AnnouncementModal />);

    await waitFor(() => {
      expect(screen.getByText("First Announcement")).toBeInTheDocument();
    });

    // Check counter shows "1 von 2"
    expect(screen.getByText("1 von 2")).toBeInTheDocument();

    // Button should say "Weiter" for non-final announcement
    const nextButton = screen.getByRole("button", { name: /weiter/i });
    expect(nextButton).toBeInTheDocument();

    fireEvent.click(nextButton);

    await waitFor(() => {
      expect(mockDismiss).toHaveBeenCalledWith(1);
    });
  });

  it("shows progress counter for multiple announcements", async () => {
    mockAnnouncementsState.announcements = [
      {
        id: 1,
        title: "First",
        content: "Content",
        type: "announcement",
        severity: "info",
        published_at: "2024-01-01T00:00:00Z",
      },
      {
        id: 2,
        title: "Second",
        content: "Content",
        type: "announcement",
        severity: "info",
        published_at: "2024-01-02T00:00:00Z",
      },
      {
        id: 3,
        title: "Third",
        content: "Content",
        type: "announcement",
        severity: "info",
        published_at: "2024-01-03T00:00:00Z",
      },
    ];

    render(<AnnouncementModal />);

    await waitFor(() => {
      expect(screen.getByText("1 von 3")).toBeInTheDocument();
    });
  });

  it("does not show counter for single announcement", async () => {
    mockAnnouncementsState.announcements = [
      {
        id: 1,
        title: "Only One",
        content: "Single announcement",
        type: "announcement",
        severity: "info",
        published_at: "2024-01-01T00:00:00Z",
      },
    ];

    render(<AnnouncementModal />);

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    // Should not show counter text for single item
    expect(screen.queryByText(/von/)).not.toBeInTheDocument();
  });

  it("handles dismiss error gracefully", async () => {
    const consoleErrorSpy = vi
      .spyOn(console, "error")
      .mockImplementation(() => {
        // noop - suppress console.error in test
      });
    mockDismiss.mockRejectedValueOnce(new Error("Network error"));

    mockAnnouncementsState.announcements = [
      {
        id: 1,
        title: "Test",
        content: "Content",
        type: "announcement",
        severity: "info",
        published_at: "2024-01-01T00:00:00Z",
      },
    ];

    render(<AnnouncementModal />);

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    const dismissButton = screen.getByRole("button", { name: /verstanden/i });
    fireEvent.click(dismissButton);

    await waitFor(() => {
      expect(consoleErrorSpy).toHaveBeenCalled();
    });

    consoleErrorSpy.mockRestore();
  });

  it("renders version badge when version is provided", async () => {
    mockAnnouncementsState.announcements = [
      {
        id: 1,
        title: "New Release",
        content: "Check out the new features",
        type: "release",
        severity: "info",
        version: "2.5.0",
        published_at: "2024-01-01T00:00:00Z",
      },
    ];

    render(<AnnouncementModal />);

    await waitFor(() => {
      expect(screen.getByText("v2.5.0")).toBeInTheDocument();
    });
  });

  it("does not render version badge when version is not provided", async () => {
    mockAnnouncementsState.announcements = [
      {
        id: 1,
        title: "Announcement",
        content: "No version here",
        type: "announcement",
        severity: "info",
        published_at: "2024-01-01T00:00:00Z",
      },
    ];

    render(<AnnouncementModal />);

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    expect(screen.queryByText(/^v\d/)).not.toBeInTheDocument();
  });

  it("preserves whitespace in content", async () => {
    mockAnnouncementsState.announcements = [
      {
        id: 1,
        title: "Multi-line",
        content: "Line 1\nLine 2\nLine 3",
        type: "announcement",
        severity: "info",
        published_at: "2024-01-01T00:00:00Z",
      },
    ];

    render(<AnnouncementModal />);

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    // Check that the content element has whitespace-pre-wrap class
    const contentElement = screen.getByText(/Line 1/);
    expect(contentElement).toBeInTheDocument();
    expect(contentElement.className).toContain("whitespace-pre-wrap");
    expect(contentElement.textContent).toContain("Line 1");
    expect(contentElement.textContent).toContain("Line 2");
    expect(contentElement.textContent).toContain("Line 3");
  });
});
