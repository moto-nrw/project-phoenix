/**
 * Tests for UnclaimedRooms Component
 *
 * NOTE: The Schulhof banner feature is currently DISABLED (ENABLE_SCHULHOF_BANNER = false)
 * because Schulhof is now handled as a permanent tab in active-supervisions page.
 * These tests verify the component returns null when the feature is disabled.
 */
import { render } from "@testing-library/react";
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

  // The Schulhof banner is disabled - all renders should return null
  // Schulhof is now handled as a permanent tab in active-supervisions page
  it("renders nothing because Schulhof banner is disabled (permanent tab replaces it)", () => {
    const { container } = render(<UnclaimedRooms onClaimed={mockOnClaimed} />);

    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing even with provided active groups", () => {
    const { container } = render(
      <UnclaimedRooms onClaimed={mockOnClaimed} activeGroups={mockGroups} />,
    );

    expect(container).toBeEmptyDOMElement();
    // Should not fetch groups since banner is disabled
    expect(activeService.getActiveGroups).not.toHaveBeenCalled();
  });

  it("renders nothing even with provided staff ID", () => {
    const { container } = render(
      <UnclaimedRooms onClaimed={mockOnClaimed} currentStaffId="s3" />,
    );

    expect(container).toBeEmptyDOMElement();
    // Should not fetch staff since banner is disabled
    expect(userContextService.getCurrentStaff).not.toHaveBeenCalled();
  });
});
