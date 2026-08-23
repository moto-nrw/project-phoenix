import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { StudentCard } from "./student-card";

// Selection sub-mode of the check-in mode (#2359): a tap marks the card, the
// check indicator carries the selection state, and nothing navigates or writes.

vi.mock("~/lib/hooks/use-student-photos-enabled", () => ({
  useStudentPhotosEnabled: () => ({ enabled: false, isLoading: false }),
}));

function renderCard(
  overrides: Partial<React.ComponentProps<typeof StudentCard>> = {},
) {
  const props = {
    studentId: "1",
    firstName: "Mara",
    lastName: "Muster",
    onClick: vi.fn(),
    locationBadge: <span>badge</span>,
    checkinMode: true,
    checkinState: "anwesend" as const,
    onCheckinClick: vi.fn(),
    checkinSelectMode: true,
    isCheckinSelected: false,
    ...overrides,
  };
  render(<StudentCard {...props} />);
  return props;
}

describe("StudentCard selection sub-mode", () => {
  it("renders the unselected indicator and fires onCheckinClick, not onClick", () => {
    const props = renderCard();

    const button = screen.getByRole("button");
    expect(button).toHaveAttribute("aria-pressed", "false");
    expect(
      button.querySelector('[data-checkin-selection-indicator="true"]'),
    ).toBeInTheDocument();
    expect(
      button.querySelector('[data-checkin-tap-strip="true"]'),
    ).not.toBeInTheDocument();

    fireEvent.click(button);
    expect(props.onCheckinClick).toHaveBeenCalledTimes(1);
    expect(props.onClick).not.toHaveBeenCalled();
  });

  it("renders the selected state with indicator, aria-pressed and data attribute", () => {
    renderCard({ isCheckinSelected: true });

    const button = screen.getByRole("button");
    expect(button).toHaveAttribute("aria-pressed", "true");
    expect(button).toHaveAttribute("data-checkin-selected", "true");
    expect(
      button.querySelector('[data-checkin-selection-indicator="true"]'),
    ).toBeInTheDocument();
  });

  it("keeps the presence-action strip when the select sub-mode is off", () => {
    renderCard({ checkinSelectMode: false });

    // anwesend → immediate mode shows the checkout action strip.
    expect(screen.getByText("Tippen zum Abmelden")).toBeInTheDocument();
    expect(
      screen
        .getByRole("button")
        .querySelector('[data-checkin-selection-indicator="true"]'),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("button")).not.toHaveAttribute("aria-pressed");
  });
});
