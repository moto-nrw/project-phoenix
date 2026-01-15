import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { LocationBadge } from "./location-badge";
import { LOCATION_STATUSES, LOCATION_COLORS } from "@/lib/location-helper";
import type { StudentLocationContext } from "@/lib/location-helper";

// =============================================================================
// TEST HELPERS
// =============================================================================

function createStudent(
  overrides: Partial<StudentLocationContext> = {},
): StudentLocationContext {
  return {
    current_location: "Anwesend - Raum 101",
    location_since: "2024-01-15T10:00:00Z",
    group_id: "1",
    group_name: "Gruppe A",
    ...overrides,
  };
}

// =============================================================================
// BASIC RENDERING TESTS
// =============================================================================

describe("LocationBadge", () => {
  describe("basic rendering", () => {
    it("renders with modern variant by default", () => {
      const student = createStudent();
      render(<LocationBadge student={student} displayMode="roomName" />);

      expect(screen.getByText("Raum 101")).toBeInTheDocument();
    });

    it("renders with simple variant", () => {
      const student = createStudent();
      render(
        <LocationBadge
          student={student}
          displayMode="roomName"
          variant="simple"
        />,
      );

      expect(screen.getByText("Raum 101")).toBeInTheDocument();
    });

    it("shows location_since time when enabled", () => {
      const student = createStudent({
        current_location: "Anwesend - Raum 101",
        location_since: "2024-01-15T10:30:00Z",
      });
      render(
        <LocationBadge
          student={student}
          displayMode="roomName"
          showLocationSince={true}
        />,
      );

      expect(screen.getByText(/seit.*Uhr/)).toBeInTheDocument();
    });
  });

  describe("display modes", () => {
    it("shows group name for groupName mode", () => {
      const student = createStudent({ group_name: "Tiger Group" });
      render(<LocationBadge student={student} displayMode="groupName" />);

      expect(screen.getByText("Tiger Group")).toBeInTheDocument();
    });

    it("shows room name for roomName mode", () => {
      const student = createStudent({
        current_location: "Anwesend - Classroom B",
      });
      render(<LocationBadge student={student} displayMode="roomName" />);

      expect(screen.getByText("Classroom B")).toBeInTheDocument();
    });

    it("shows status without room for students at home", () => {
      const student = createStudent({ current_location: "Zuhause" });
      render(<LocationBadge student={student} displayMode="roomName" />);

      expect(screen.getByText("Zuhause")).toBeInTheDocument();
    });
  });

  // ===========================================================================
  // SICK STATUS TESTS - Core functionality for the new feature
  // ===========================================================================

  describe("sick status display", () => {
    describe("when student is sick AND at home", () => {
      it("shows Krank badge instead of Zuhause", () => {
        const student = createStudent({
          current_location: "Zuhause",
          sick: true,
          sick_since: "2024-01-15T08:00:00Z",
        });

        render(<LocationBadge student={student} displayMode="roomName" />);

        // Should show "Krank" instead of "Zuhause"
        expect(screen.getByText(LOCATION_STATUSES.SICK)).toBeInTheDocument();
        expect(screen.queryByText("Zuhause")).not.toBeInTheDocument();
      });

      it("applies sick color (amber) to the badge", () => {
        const student = createStudent({
          current_location: "Zuhause",
          sick: true,
        });

        render(<LocationBadge student={student} displayMode="roomName" />);

        const badge = screen.getByText(LOCATION_STATUSES.SICK);
        const badgeContainer = badge.closest("span");
        expect(badgeContainer).toHaveStyle({
          backgroundColor: LOCATION_COLORS.SICK,
        });
      });

      it("shows sick_since timestamp when showLocationSince is enabled", () => {
        const student = createStudent({
          current_location: "Zuhause",
          sick: true,
          sick_since: "2024-01-15T09:45:00Z",
        });

        render(
          <LocationBadge
            student={student}
            displayMode="roomName"
            showLocationSince={true}
          />,
        );

        expect(screen.getByText(/seit.*Uhr/)).toBeInTheDocument();
      });

      it("sets correct data-location-status attribute", () => {
        const student = createStudent({
          current_location: "Zuhause",
          sick: true,
        });

        render(<LocationBadge student={student} displayMode="roomName" />);

        // The status attribute should still reflect original location
        const badge = screen.getByText(LOCATION_STATUSES.SICK).closest("span");
        expect(badge).toHaveAttribute("data-location-status", "Zuhause");
      });
    });

    describe("when student is sick AND present at school", () => {
      it("shows location badge with additional Krank indicator", () => {
        const student = createStudent({
          current_location: "Anwesend - Raum 101",
          sick: true,
        });

        render(<LocationBadge student={student} displayMode="roomName" />);

        // Should show both the room AND the sick indicator
        expect(screen.getByText("Raum 101")).toBeInTheDocument();
        expect(screen.getByText(LOCATION_STATUSES.SICK)).toBeInTheDocument();
      });

      it("renders sick indicator with data attribute", () => {
        const student = createStudent({
          current_location: "Anwesend - Raum 101",
          sick: true,
        });

        render(<LocationBadge student={student} displayMode="roomName" />);

        const sickIndicator = screen.getByText(LOCATION_STATUSES.SICK);
        expect(sickIndicator.closest("[data-sick-indicator]")).toHaveAttribute(
          "data-sick-indicator",
          "true",
        );
      });

      it("applies amber color to sick indicator", () => {
        const student = createStudent({
          current_location: "Anwesend - Raum 101",
          sick: true,
        });

        render(<LocationBadge student={student} displayMode="roomName" />);

        const sickIndicator = screen.getByText(LOCATION_STATUSES.SICK);
        expect(sickIndicator.closest("span")).toHaveStyle({
          backgroundColor: LOCATION_COLORS.SICK,
        });
      });

      it("works with Schulhof location", () => {
        const student = createStudent({
          current_location: "Schulhof",
          sick: true,
        });

        render(<LocationBadge student={student} displayMode="roomName" />);

        // Should show Schulhof AND sick indicator
        expect(screen.getByText("Schulhof")).toBeInTheDocument();
        expect(screen.getByText(LOCATION_STATUSES.SICK)).toBeInTheDocument();
      });

      it("works with Unterwegs location", () => {
        const student = createStudent({
          current_location: "Unterwegs",
          sick: true,
        });

        render(<LocationBadge student={student} displayMode="roomName" />);

        // Should show Unterwegs AND sick indicator
        expect(screen.getByText("Unterwegs")).toBeInTheDocument();
        expect(screen.getByText(LOCATION_STATUSES.SICK)).toBeInTheDocument();
      });
    });

    describe("when student is NOT sick", () => {
      it("does not show sick indicator", () => {
        const student = createStudent({
          current_location: "Anwesend - Raum 101",
          sick: false,
        });

        render(<LocationBadge student={student} displayMode="roomName" />);

        expect(screen.getByText("Raum 101")).toBeInTheDocument();
        expect(
          screen.queryByText(LOCATION_STATUSES.SICK),
        ).not.toBeInTheDocument();
      });

      it("shows normal Zuhause badge for healthy student at home", () => {
        const student = createStudent({
          current_location: "Zuhause",
          sick: false,
        });

        render(<LocationBadge student={student} displayMode="roomName" />);

        expect(screen.getByText("Zuhause")).toBeInTheDocument();
        expect(
          screen.queryByText(LOCATION_STATUSES.SICK),
        ).not.toBeInTheDocument();
      });

      it("handles undefined sick field gracefully", () => {
        const student = createStudent({
          current_location: "Zuhause",
          // sick field not set
        });

        render(<LocationBadge student={student} displayMode="roomName" />);

        expect(screen.getByText("Zuhause")).toBeInTheDocument();
        expect(
          screen.queryByText(LOCATION_STATUSES.SICK),
        ).not.toBeInTheDocument();
      });
    });

    describe("sick badge with simple variant", () => {
      it("shows Krank badge in simple variant when sick at home", () => {
        const student = createStudent({
          current_location: "Zuhause",
          sick: true,
        });

        render(
          <LocationBadge
            student={student}
            displayMode="roomName"
            variant="simple"
          />,
        );

        expect(screen.getByText(LOCATION_STATUSES.SICK)).toBeInTheDocument();
        expect(screen.queryByText("Zuhause")).not.toBeInTheDocument();
      });

      it("shows additional sick indicator in simple variant when present", () => {
        const student = createStudent({
          current_location: "Anwesend - Raum 101",
          sick: true,
        });

        render(
          <LocationBadge
            student={student}
            displayMode="roomName"
            variant="simple"
          />,
        );

        expect(screen.getByText("Raum 101")).toBeInTheDocument();
        expect(screen.getByText(LOCATION_STATUSES.SICK)).toBeInTheDocument();
      });
    });
  });

  // ===========================================================================
  // CONTEXT-AWARE MODE WITH SICK STATUS
  // ===========================================================================

  describe("contextAware mode with sick status", () => {
    it("shows Krank for sick student at home in contextAware mode", () => {
      const student = createStudent({
        current_location: "Zuhause",
        sick: true,
        group_id: "1",
      });

      render(
        <LocationBadge
          student={student}
          displayMode="contextAware"
          userGroups={["1"]}
        />,
      );

      expect(screen.getByText(LOCATION_STATUSES.SICK)).toBeInTheDocument();
    });

    it("shows sick indicator for present sick student in own group", () => {
      const student = createStudent({
        current_location: "Anwesend - Raum 101",
        sick: true,
        group_id: "1",
      });

      render(
        <LocationBadge
          student={student}
          displayMode="contextAware"
          userGroups={["1"]}
        />,
      );

      // Should see room details (own group) AND sick indicator
      expect(screen.getByText("Raum 101")).toBeInTheDocument();
      expect(screen.getByText(LOCATION_STATUSES.SICK)).toBeInTheDocument();
    });

    it("shows sick indicator for present sick student in other group", () => {
      const student = createStudent({
        current_location: "Anwesend - Raum 101",
        sick: true,
        group_id: "2",
      });

      render(
        <LocationBadge
          student={student}
          displayMode="contextAware"
          userGroups={["1"]}
        />,
      );

      // Should see "Anwesend" (limited access) AND sick indicator
      expect(screen.getByText("Anwesend")).toBeInTheDocument();
      expect(screen.getByText(LOCATION_STATUSES.SICK)).toBeInTheDocument();
    });
  });

  // ===========================================================================
  // SIZE VARIANTS
  // ===========================================================================

  describe("size variants", () => {
    it("renders with small size", () => {
      const student = createStudent({
        sick: true,
        current_location: "Zuhause",
      });
      render(
        <LocationBadge student={student} displayMode="roomName" size="sm" />,
      );

      expect(screen.getByText(LOCATION_STATUSES.SICK)).toBeInTheDocument();
    });

    it("renders with large size", () => {
      const student = createStudent({
        sick: true,
        current_location: "Zuhause",
      });
      render(
        <LocationBadge student={student} displayMode="roomName" size="lg" />,
      );

      expect(screen.getByText(LOCATION_STATUSES.SICK)).toBeInTheDocument();
    });
  });
});
