import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import { StudentHeader } from "./StudentHeader";

vi.mock("@/components/ui/location-badge", () => ({
  LocationBadge: () => <div data-testid="location-badge">Badge</div>,
}));

const defaultProps = {
  firstName: "Max",
  secondName: "Mustermann",
  badgeStudent: {},
  myGroups: [],
  myGroupRooms: [],
  mySupervisedRooms: [],
};

describe("StudentHeader", () => {
  it("renders student full name", () => {
    render(<StudentHeader {...defaultProps} />);

    expect(screen.getByText("Max Mustermann")).toBeInTheDocument();
  });

  it("renders group name when provided", () => {
    render(<StudentHeader {...defaultProps} groupName="Gruppe A" />);

    expect(screen.getByText("Gruppe A")).toBeInTheDocument();
  });

  it("does not render group name when not provided", () => {
    render(<StudentHeader {...defaultProps} />);

    expect(screen.queryByText("Gruppe A")).not.toBeInTheDocument();
  });

  it("renders LocationBadge", () => {
    render(<StudentHeader {...defaultProps} />);

    expect(screen.getByTestId("location-badge")).toBeInTheDocument();
  });
});
