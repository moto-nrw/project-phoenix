import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import {
  StudentCheckoutSection,
  StudentCheckinSection,
  getStudentActionType,
} from "./student-checkout-section";

describe("getStudentActionType", () => {
  describe("access control", () => {
    it("returns none when user has no access (not in group, not supervising room)", () => {
      const result = getStudentActionType(
        { group_id: "1", current_location: "Raum A" },
        ["2", "3"], // different groups
        ["Raum B"], // different room
      );
      expect(result).toBe("none");
    });

    it("grants access when user is in student's group", () => {
      const result = getStudentActionType(
        { group_id: "1", current_location: "Raum A" },
        ["1", "2"], // includes student's group
        [],
      );
      expect(result).toBe("checkout");
    });

    it("grants access when user supervises student's room", () => {
      const result = getStudentActionType(
        { group_id: "1", current_location: "Raum A" },
        [], // no group access
        ["Raum A"], // supervises the room
      );
      expect(result).toBe("checkout");
    });

    it("grants access when user both is in group and supervises room", () => {
      const result = getStudentActionType(
        { group_id: "1", current_location: "Raum A" },
        ["1"],
        ["Raum A"],
      );
      expect(result).toBe("checkout");
    });
  });

  describe("student at home scenarios", () => {
    it("returns checkin when student is at home and user is in student's group", () => {
      const result = getStudentActionType(
        { group_id: "1", current_location: "Zuhause" },
        ["1"],
        [],
      );
      expect(result).toBe("checkin");
    });

    it("returns checkin when student has no location (at home) and user is in group", () => {
      const result = getStudentActionType(
        { group_id: "1", current_location: undefined },
        ["1"],
        [],
      );
      expect(result).toBe("checkin");
    });

    it("returns checkin when student location is empty string and user is in group", () => {
      const result = getStudentActionType(
        { group_id: "1", current_location: "" },
        ["1"],
        [],
      );
      expect(result).toBe("checkin");
    });

    it("returns none when student is at home but user only supervises rooms (not in group)", () => {
      const result = getStudentActionType(
        { group_id: "1", current_location: "Zuhause" },
        [], // not in student's group
        ["Raum A"], // only room supervisor
      );
      expect(result).toBe("none");
    });

    it("handles Zuhause with additional text", () => {
      const result = getStudentActionType(
        { group_id: "1", current_location: "Zuhause (Abgemeldet)" },
        ["1"],
        [],
      );
      expect(result).toBe("checkin");
    });
  });

  describe("student checked in scenarios", () => {
    it("returns checkout when student is in a room and user is in group", () => {
      const result = getStudentActionType(
        { group_id: "1", current_location: "Raum A" },
        ["1"],
        [],
      );
      expect(result).toBe("checkout");
    });

    it("returns checkout when student is in a room and user supervises room", () => {
      const result = getStudentActionType(
        { group_id: "1", current_location: "Raum A" },
        [],
        ["Raum A"],
      );
      expect(result).toBe("checkout");
    });

    it("returns checkout for various room locations", () => {
      const locations = ["Schulhof", "Mensa", "Turnhalle", "Bibliothek"];
      locations.forEach((location) => {
        const result = getStudentActionType(
          { group_id: "1", current_location: location },
          ["1"],
          [],
        );
        expect(result).toBe("checkout");
      });
    });
  });

  describe("edge cases", () => {
    it("handles student without group_id", () => {
      const result = getStudentActionType(
        { group_id: undefined, current_location: "Raum A" },
        ["1"],
        ["Raum A"],
      );
      expect(result).toBe("checkout");
    });

    it("returns none when student has no group and no access via room", () => {
      const result = getStudentActionType(
        { group_id: undefined, current_location: "Raum A" },
        [],
        [],
      );
      expect(result).toBe("none");
    });

    it("handles empty arrays for groups and rooms", () => {
      const result = getStudentActionType(
        { group_id: "1", current_location: "Raum A" },
        [],
        [],
      );
      expect(result).toBe("none");
    });

    it("handles partial room name match in supervised rooms", () => {
      const result = getStudentActionType(
        { group_id: "1", current_location: "Raum A - Erdgeschoss" },
        [],
        ["Raum A"],
      );
      expect(result).toBe("checkout");
    });
  });
});

describe("StudentCheckoutSection", () => {
  it("renders checkout button", () => {
    render(<StudentCheckoutSection onCheckoutClick={vi.fn()} />);
    expect(
      screen.getByRole("button", { name: /kind abmelden/i }),
    ).toBeInTheDocument();
  });

  it("renders Abmeldung heading", () => {
    render(<StudentCheckoutSection onCheckoutClick={vi.fn()} />);
    expect(screen.getByText("Abmeldung")).toBeInTheDocument();
  });

  it("calls onCheckoutClick when button is clicked", () => {
    const mockOnClick = vi.fn();
    render(<StudentCheckoutSection onCheckoutClick={mockOnClick} />);

    fireEvent.click(screen.getByRole("button", { name: /kind abmelden/i }));

    expect(mockOnClick).toHaveBeenCalledTimes(1);
  });
});

describe("StudentCheckinSection", () => {
  it("renders checkin button", () => {
    render(<StudentCheckinSection onCheckinClick={vi.fn()} />);
    expect(
      screen.getByRole("button", { name: /kind anmelden/i }),
    ).toBeInTheDocument();
  });

  it("renders Anmeldung heading", () => {
    render(<StudentCheckinSection onCheckinClick={vi.fn()} />);
    expect(screen.getByText("Anmeldung")).toBeInTheDocument();
  });

  it("calls onCheckinClick when button is clicked", () => {
    const mockOnClick = vi.fn();
    render(<StudentCheckinSection onCheckinClick={mockOnClick} />);

    fireEvent.click(screen.getByRole("button", { name: /kind anmelden/i }));

    expect(mockOnClick).toHaveBeenCalledTimes(1);
  });
});
