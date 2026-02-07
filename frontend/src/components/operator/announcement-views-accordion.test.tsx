import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { AnnouncementViewsAccordion } from "./announcement-views-accordion";
import type { AnnouncementViewDetail } from "~/lib/operator/announcements-helpers";

// Mock the announcements service
const { mockOperatorAnnouncementsService } = vi.hoisted(() => ({
  mockOperatorAnnouncementsService: {
    fetchViewDetails: vi.fn(),
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

describe("AnnouncementViewsAccordion", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("does not render when dismissedCount is 0", () => {
    const { container } = render(
      <AnnouncementViewsAccordion announcementId="42" dismissedCount={0} />,
    );

    expect(container).toBeEmptyDOMElement();
  });

  it("renders accordion header when dismissedCount > 0", () => {
    render(
      <AnnouncementViewsAccordion announcementId="42" dismissedCount={5} />,
    );

    expect(screen.getByText("Lesebestätigungen")).toBeInTheDocument();
    expect(screen.getByText("(5)")).toBeInTheDocument();
  });

  it("toggles accordion on header click", async () => {
    mockOperatorAnnouncementsService.fetchViewDetails.mockResolvedValue([]);

    render(
      <AnnouncementViewsAccordion announcementId="42" dismissedCount={3} />,
    );

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

    render(
      <AnnouncementViewsAccordion announcementId="42" dismissedCount={3} />,
    );

    const button = screen.getByRole("button");
    fireEvent.click(button);

    await waitFor(() => {
      expect(screen.getByText("Laden...")).toBeInTheDocument();
    });
  });

  it("displays error message on fetch failure", async () => {
    mockOperatorAnnouncementsService.fetchViewDetails.mockRejectedValue(
      new Error("Network error"),
    );

    render(
      <AnnouncementViewsAccordion announcementId="42" dismissedCount={3} />,
    );

    const button = screen.getByRole("button");
    fireEvent.click(button);

    await waitFor(() => {
      expect(
        screen.getByText("Ansichten konnten nicht geladen werden."),
      ).toBeInTheDocument();
    });
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

    render(
      <AnnouncementViewsAccordion announcementId="42" dismissedCount={2} />,
    );

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

    render(
      <AnnouncementViewsAccordion announcementId="42" dismissedCount={1} />,
    );

    const button = screen.getByRole("button");
    fireEvent.click(button);

    await waitFor(() => {
      expect(screen.getByText("Alice")).toBeInTheDocument();
      expect(screen.queryByText("Bob")).not.toBeInTheDocument();
    });
  });

  it("shows empty state when no confirmations", async () => {
    mockOperatorAnnouncementsService.fetchViewDetails.mockResolvedValue([]);

    render(
      <AnnouncementViewsAccordion announcementId="42" dismissedCount={0} />,
    );

    // Component should not render at all
    const { container } = render(
      <AnnouncementViewsAccordion announcementId="42" dismissedCount={0} />,
    );
    expect(container).toBeEmptyDOMElement();
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

    render(
      <AnnouncementViewsAccordion announcementId="42" dismissedCount={1} />,
    );

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

    const { container } = render(
      <AnnouncementViewsAccordion announcementId="42" dismissedCount={1} />,
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

    render(
      <AnnouncementViewsAccordion announcementId="42" dismissedCount={1} />,
    );

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

  it("stops event propagation on accordion", () => {
    const parentClick = vi.fn();

    render(
      <div onClick={parentClick}>
        <AnnouncementViewsAccordion announcementId="42" dismissedCount={1} />
      </div>,
    );

    const button = screen.getByRole("button");
    fireEvent.click(button);

    expect(parentClick).not.toHaveBeenCalled();
  });
});
