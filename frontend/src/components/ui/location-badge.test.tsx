import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { LocationBadge } from "./location-badge";
import {
  LOCATION_STATUSES,
  LOCATION_COLORS,
  getLocationBadgeTone,
} from "@/lib/location-helper";
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
          backgroundColor: getLocationBadgeTone(LOCATION_COLORS.SICK)
            .backgroundColor,
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

      it("falls back to location_since when sick_since is missing", () => {
        const student = createStudent({
          current_location: "Zuhause",
          location_since: "2024-01-15T10:30:00Z",
          sick: true,
          sick_since: undefined, // Missing sick_since - should fall back
        });

        render(
          <LocationBadge
            student={student}
            displayMode="roomName"
            showLocationSince={true}
          />,
        );

        // Should still show timestamp from location_since as fallback
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
          backgroundColor: getLocationBadgeTone(LOCATION_COLORS.SICK)
            .backgroundColor,
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
  // EXCUSED STATUS TESTS — mirrors the Krank pattern with purple badge
  // ===========================================================================

  describe("excused status display", () => {
    describe("when student is excused AND at home", () => {
      it("shows Entschuldigt badge instead of Zuhause", () => {
        const student = createStudent({
          current_location: "Zuhause",
          excused: true,
          excused_since: "2024-01-15T08:00:00Z",
        });
        render(<LocationBadge student={student} displayMode="roomName" />);

        expect(screen.getByText(LOCATION_STATUSES.EXCUSED)).toBeInTheDocument();
        expect(screen.queryByText("Zuhause")).not.toBeInTheDocument();
      });

      it("applies purple color (EXCUSED) to the badge", () => {
        const student = createStudent({
          current_location: "Zuhause",
          excused: true,
        });
        render(<LocationBadge student={student} displayMode="roomName" />);

        const badge = screen.getByText(LOCATION_STATUSES.EXCUSED);
        expect(badge.closest("span")).toHaveStyle({
          backgroundColor: getLocationBadgeTone(LOCATION_COLORS.EXCUSED)
            .backgroundColor,
        });
      });

      it("shows excused_since timestamp when showLocationSince is enabled", () => {
        const student = createStudent({
          current_location: "Zuhause",
          excused: true,
          excused_since: "2024-01-15T09:45:00Z",
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

      it("falls back to location_since when excused_since is missing", () => {
        const student = createStudent({
          current_location: "Zuhause",
          location_since: "2024-01-15T10:30:00Z",
          excused: true,
          excused_since: undefined,
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

    describe("when student is excused AND present at school", () => {
      it("shows location badge with additional Entschuldigt indicator", () => {
        const student = createStudent({
          current_location: "Anwesend - Raum 101",
          excused: true,
        });
        render(<LocationBadge student={student} displayMode="roomName" />);

        expect(screen.getByText("Raum 101")).toBeInTheDocument();
        expect(screen.getByText(LOCATION_STATUSES.EXCUSED)).toBeInTheDocument();
      });

      it("renders excused indicator with data attribute and purple color", () => {
        const student = createStudent({
          current_location: "Anwesend - Raum 101",
          excused: true,
        });
        render(<LocationBadge student={student} displayMode="roomName" />);

        const ind = screen.getByText(LOCATION_STATUSES.EXCUSED);
        expect(ind.closest("[data-excused-indicator]")).toHaveAttribute(
          "data-excused-indicator",
          "true",
        );
        expect(ind.closest("span")).toHaveStyle({
          backgroundColor: getLocationBadgeTone(LOCATION_COLORS.EXCUSED)
            .backgroundColor,
        });
      });
    });

    describe("sick takes precedence over excused when both flags are set", () => {
      it("shows only Krank badge, not Entschuldigt, when both are true at home", () => {
        const student = createStudent({
          current_location: "Zuhause",
          sick: true,
          excused: true,
        });
        render(<LocationBadge student={student} displayMode="roomName" />);

        expect(screen.getByText(LOCATION_STATUSES.SICK)).toBeInTheDocument();
        expect(
          screen.queryByText(LOCATION_STATUSES.EXCUSED),
        ).not.toBeInTheDocument();
      });

      it("shows only Krank indicator, not Entschuldigt, when both are true and present", () => {
        const student = createStudent({
          current_location: "Anwesend - Raum 101",
          sick: true,
          excused: true,
        });
        render(<LocationBadge student={student} displayMode="roomName" />);

        expect(screen.getByText(LOCATION_STATUSES.SICK)).toBeInTheDocument();
        expect(
          screen.queryByText(LOCATION_STATUSES.EXCUSED),
        ).not.toBeInTheDocument();
      });
    });

    describe("when student is NOT excused", () => {
      it("does not show excused indicator", () => {
        const student = createStudent({
          current_location: "Anwesend - Raum 101",
          excused: false,
        });
        render(<LocationBadge student={student} displayMode="roomName" />);

        expect(screen.getByText("Raum 101")).toBeInTheDocument();
        expect(
          screen.queryByText(LOCATION_STATUSES.EXCUSED),
        ).not.toBeInTheDocument();
      });
    });

    describe("excused badge with simple variant", () => {
      it("shows Entschuldigt badge in simple variant when excused at home", () => {
        const student = createStudent({
          current_location: "Zuhause",
          excused: true,
        });
        render(
          <LocationBadge
            student={student}
            displayMode="roomName"
            variant="simple"
          />,
        );
        expect(screen.getByText(LOCATION_STATUSES.EXCUSED)).toBeInTheDocument();
      });

      it("shows additional excused indicator in simple variant when present", () => {
        const student = createStudent({
          current_location: "Anwesend - Raum 101",
          excused: true,
        });
        render(
          <LocationBadge
            student={student}
            displayMode="roomName"
            variant="simple"
          />,
        );
        expect(screen.getByText("Raum 101")).toBeInTheDocument();
        expect(screen.getByText(LOCATION_STATUSES.EXCUSED)).toBeInTheDocument();
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

  // ===========================================================================
  // Per-room color (Issue #1324) — make sure the badge actually paints the
  // configured hex when student.current_room_color is set, and falls back to
  // OTHER_ROOM blue otherwise. Drives the differentiation users will see.
  // ===========================================================================
  describe("per-room color (current_room_color)", () => {
    it("paints the per-room color when set in roomName mode", () => {
      const student = createStudent({
        current_location: "Anwesend - Bibliothek",
        current_room_color: "#A3D977",
      });
      render(<LocationBadge student={student} displayMode="roomName" />);
      const badgeContainer = screen.getByText("Bibliothek").closest("span");
      expect(badgeContainer).toHaveStyle({ backgroundColor: "#F9FAFB" });
    });

    it("falls back to OTHER_ROOM blue when no per-room color is set", () => {
      const student = createStudent({
        current_location: "Anwesend - Bibliothek",
        current_room_color: null,
      });
      render(<LocationBadge student={student} displayMode="roomName" />);
      const badgeContainer = screen.getByText("Bibliothek").closest("span");
      expect(badgeContainer).toHaveStyle({
        backgroundColor: getLocationBadgeTone(LOCATION_COLORS.OTHER_ROOM)
          .backgroundColor,
      });
    });

    it("keeps GROUP_ROOM green when room is the viewer's own group room (hard constraint)", () => {
      // Per-room color must NOT override the eigener-Gruppenraum green —
      // see Issue #1324 discussion: all non-blue colors keep their meaning.
      const student = createStudent({
        current_location: "Anwesend - Raum A",
        current_room_color: "#A3D977",
      });
      render(
        <LocationBadge
          student={student}
          displayMode="roomName"
          groupRooms={["Raum A"]}
        />,
      );
      const badgeContainer = screen.getByText("Raum A").closest("span");
      expect(badgeContainer).toHaveStyle({
        backgroundColor: getLocationBadgeTone(LOCATION_COLORS.GROUP_ROOM)
          .backgroundColor,
      });
    });

    it("paints the yard color for a foreign student on the Schulhof (contextAware)", () => {
      // Binary mode: the yard arrives as the plain "Schulhof" status with no
      // room visit behind it. A viewer without detailed access still sees that
      // status, so it must follow the school's configured yard color (#2405).
      const student = createStudent({
        current_location: LOCATION_STATUSES.SCHOOLYARD,
        current_room_color: "#A3D977",
      });
      render(
        <LocationBadge
          student={student}
          displayMode="contextAware"
          userGroups={["99"]}
        />,
      );
      const badgeContainer = screen
        .getByText(LOCATION_STATUSES.SCHOOLYARD)
        .closest("span");
      expect(badgeContainer).toHaveStyle({
        backgroundColor: getLocationBadgeTone("#A3D977").backgroundColor,
      });
    });

    it("falls back to orange for a foreign student on the Schulhof without a color", () => {
      const student = createStudent({
        current_location: LOCATION_STATUSES.SCHOOLYARD,
        current_room_color: null,
      });
      render(
        <LocationBadge
          student={student}
          displayMode="contextAware"
          userGroups={["99"]}
        />,
      );
      const badgeContainer = screen
        .getByText(LOCATION_STATUSES.SCHOOLYARD)
        .closest("span");
      expect(badgeContainer).toHaveStyle({
        backgroundColor: getLocationBadgeTone(LOCATION_COLORS.SCHOOLYARD)
          .backgroundColor,
      });
    });

    it("renders correctly when current_room_color key is absent (forward compat)", () => {
      // Old backend builds (rolled back during a deploy or older PyrePortal
      // proxy in the request path) won't include current_room_color in their
      // payload at all — TypeScript represents that as `undefined`. The
      // badge must fall back to OTHER_ROOM blue without crashing or
      // rendering an empty backgroundColor.
      const student: StudentLocationContext = {
        current_location: "Anwesend - Bibliothek",
        location_since: "2024-01-15T10:00:00Z",
        // current_room_color intentionally omitted
      };
      render(<LocationBadge student={student} displayMode="roomName" />);
      const badgeContainer = screen.getByText("Bibliothek").closest("span");
      expect(badgeContainer).toHaveStyle({
        backgroundColor: getLocationBadgeTone(LOCATION_COLORS.OTHER_ROOM)
          .backgroundColor,
      });
    });
  });
});
