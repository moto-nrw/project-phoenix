/**
 * Tests for UnclaimedRooms Component
 * Tests the rendering and claiming functionality for Schulhof room
 */
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { UnclaimedRooms } from "./unclaimed-rooms";

// Mock activeService
vi.mock("~/lib/active-api", () => ({
  activeService: {
    getActiveGroups: vi.fn(),
    getActiveGroupSupervisors: vi.fn(),
    claimActiveGroup: vi.fn(),
  },
}));

// Mock userContextService
vi.mock("~/lib/usercontext-api", () => ({
  userContextService: {
    getCurrentStaff: vi.fn(),
  },
}));

import { activeService } from "~/lib/active-api";
import { userContextService } from "~/lib/usercontext-api";

const mockGroups = [
  {
    id: "1",
    room: { name: "Schulhof" },
  },
  {
    id: "2",
    room: { name: "Raum A" },
  },
];

const mockSupervisors = [
  {
    staffId: "s1",
    staffName: "John Doe",
    isActive: true,
  },
  {
    staffId: "s2",
    staffName: "Jane Smith",
    isActive: true,
  },
];

describe("UnclaimedRooms", () => {
  const mockOnClaimed = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    vi.mocked(activeService.getActiveGroups).mockResolvedValue(
      mockGroups as never,
    );
    vi.mocked(activeService.getActiveGroupSupervisors).mockResolvedValue(
      mockSupervisors as never,
    );
    vi.mocked(activeService.claimActiveGroup).mockResolvedValue(undefined);
    vi.mocked(userContextService.getCurrentStaff).mockResolvedValue({
      id: "s3",
    } as never);
  });

  it("renders nothing while loading", () => {
    const { container } = render(<UnclaimedRooms onClaimed={mockOnClaimed} />);

    expect(container).toBeEmptyDOMElement();
  });

  it("renders banner for Schulhof when user is not supervisor", async () => {
    render(<UnclaimedRooms onClaimed={mockOnClaimed} />);

    await waitFor(() => {
      expect(
        screen.getByText("Schulhof-Aufsicht verfügbar"),
      ).toBeInTheDocument();
    });
  });

  it("displays current supervisors", async () => {
    render(<UnclaimedRooms onClaimed={mockOnClaimed} />);

    await waitFor(() => {
      expect(screen.getByText(/Aktuelle Aufsicht:/)).toBeInTheDocument();
      expect(screen.getByText(/John Doe, Jane Smith/)).toBeInTheDocument();
    });
  });

  it("renders claim button", async () => {
    render(<UnclaimedRooms onClaimed={mockOnClaimed} />);

    await waitFor(() => {
      expect(screen.getByText("Beaufsichtigen")).toBeInTheDocument();
    });
  });

  it("shows no supervisors message when list is empty", async () => {
    vi.mocked(activeService.getActiveGroupSupervisors).mockResolvedValue(
      [] as never,
    );

    render(<UnclaimedRooms onClaimed={mockOnClaimed} />);

    await waitFor(() => {
      expect(
        screen.getByText(/Der Schulhof hat derzeit keine Aufsicht/),
      ).toBeInTheDocument();
    });
  });

  it("calls claimActiveGroup when claim button clicked", async () => {
    render(<UnclaimedRooms onClaimed={mockOnClaimed} />);

    await waitFor(() => {
      const claimButton = screen.getByText("Beaufsichtigen");
      fireEvent.click(claimButton);
    });

    await waitFor(() => {
      expect(activeService.claimActiveGroup).toHaveBeenCalledWith("1");
      expect(mockOnClaimed).toHaveBeenCalled();
    });
  });

  it("shows loading state during claim", async () => {
    vi.mocked(activeService.claimActiveGroup).mockImplementation(
      () => new Promise((resolve) => setTimeout(resolve, 1000)),
    );

    render(<UnclaimedRooms onClaimed={mockOnClaimed} />);

    await waitFor(() => {
      const claimButton = screen.getByText("Beaufsichtigen");
      fireEvent.click(claimButton);
    });

    await waitFor(() => {
      expect(screen.getByText("Wird übernommen...")).toBeInTheDocument();
    });
  });

  it("renders dismiss button when supervisors exist", async () => {
    render(<UnclaimedRooms onClaimed={mockOnClaimed} />);

    await waitFor(() => {
      expect(screen.getByLabelText("Banner schließen")).toBeInTheDocument();
    });
  });

  it("hides banner after dismiss button clicked", async () => {
    render(<UnclaimedRooms onClaimed={mockOnClaimed} />);

    await waitFor(() => {
      const dismissButton = screen.getByLabelText("Banner schließen");
      fireEvent.click(dismissButton);
    });

    await waitFor(() => {
      expect(
        screen.queryByText("Schulhof-Aufsicht verfügbar"),
      ).not.toBeInTheDocument();
    });
  });

  it("does not render when user is already a supervisor", async () => {
    vi.mocked(userContextService.getCurrentStaff).mockResolvedValue({
      id: "s1",
    } as never);

    const { container } = render(<UnclaimedRooms onClaimed={mockOnClaimed} />);

    await waitFor(() => {
      expect(container).toBeEmptyDOMElement();
    });
  });

  it("does not render when Schulhof group is not active", async () => {
    vi.mocked(activeService.getActiveGroups).mockResolvedValue([
      { id: "2", room: { name: "Raum A" } },
    ] as never);

    const { container } = render(<UnclaimedRooms onClaimed={mockOnClaimed} />);

    await waitFor(() => {
      expect(container).toBeEmptyDOMElement();
    });
  });

  it("uses provided active groups when available", async () => {
    render(
      <UnclaimedRooms onClaimed={mockOnClaimed} activeGroups={mockGroups} />,
    );

    await waitFor(() => {
      expect(
        screen.getByText("Schulhof-Aufsicht verfügbar"),
      ).toBeInTheDocument();
    });

    // Should not fetch groups again
    expect(activeService.getActiveGroups).not.toHaveBeenCalled();
  });

  it("uses provided staff ID when available", async () => {
    render(<UnclaimedRooms onClaimed={mockOnClaimed} currentStaffId="s3" />);

    await waitFor(() => {
      expect(
        screen.getByText("Schulhof-Aufsicht verfügbar"),
      ).toBeInTheDocument();
    });

    // Should not fetch staff
    expect(userContextService.getCurrentStaff).not.toHaveBeenCalled();
  });
});
