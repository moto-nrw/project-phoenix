import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import {
  StudentCheckoutSection,
  StudentCheckinSection,
  StudentSickReportSection,
  StudentExcusedReportSection,
  StudentStatusActionsMenu,
} from "./student-checkout-section";

describe("StudentCheckoutSection", () => {
  it("renders Abmelden label", () => {
    render(<StudentCheckoutSection onCheckoutClick={vi.fn()} />);
    expect(screen.getByText("Abmelden")).toBeInTheDocument();
  });

  it("renders subtitle text", () => {
    render(<StudentCheckoutSection onCheckoutClick={vi.fn()} />);
    expect(screen.getByText("Aus der OGS abmelden")).toBeInTheDocument();
  });

  it("calls onCheckoutClick when card is clicked", () => {
    const mockOnClick = vi.fn();
    render(<StudentCheckoutSection onCheckoutClick={mockOnClick} />);

    fireEvent.click(screen.getByText("Abmelden"));

    expect(mockOnClick).toHaveBeenCalledTimes(1);
  });

  it("elevates the interactive card on hover", () => {
    render(<StudentCheckoutSection onCheckoutClick={vi.fn()} />);

    expect(screen.getByRole("button", { name: /^Abmelden/ })).toHaveClass(
      "moto-hover-elevated",
    );
  });
});

describe("StudentCheckinSection", () => {
  it("renders Anmelden label", () => {
    render(<StudentCheckinSection onCheckinClick={vi.fn()} />);
    expect(screen.getByText("Anmelden")).toBeInTheDocument();
  });

  it("renders subtitle text", () => {
    render(<StudentCheckinSection onCheckinClick={vi.fn()} />);
    expect(
      screen.getByText("Für heute in der OGS anmelden"),
    ).toBeInTheDocument();
  });

  it("calls onCheckinClick when card is clicked", () => {
    const mockOnClick = vi.fn();
    render(<StudentCheckinSection onCheckinClick={mockOnClick} />);

    fireEvent.click(screen.getByText("Anmelden"));

    expect(mockOnClick).toHaveBeenCalledTimes(1);
  });

  it("elevates the interactive card on hover", () => {
    render(<StudentCheckinSection onCheckinClick={vi.fn()} />);

    expect(screen.getByRole("button", { name: /^Anmelden/ })).toHaveClass(
      "moto-hover-elevated",
    );
  });
});

describe("StudentSickReportSection", () => {
  const defaultProps = {
    isSick: false,
    onToggle: vi.fn(),
    isLoading: false,
  };

  it("renders 'Krank melden' when student is not sick", () => {
    render(<StudentSickReportSection {...defaultProps} />);
    expect(screen.getByText("Krank melden")).toBeInTheDocument();
  });

  it("renders 'Als krank melden' subtitle when not sick", () => {
    render(<StudentSickReportSection {...defaultProps} />);
    expect(screen.getByText("Als krank melden")).toBeInTheDocument();
  });

  it("renders 'Gesund melden' when student is sick", () => {
    render(<StudentSickReportSection {...defaultProps} isSick={true} />);
    expect(screen.getByText("Gesund melden")).toBeInTheDocument();
  });

  it("displays sick-since date when sick with sickSince", () => {
    const sickSince = "2026-02-05T08:00:00Z";
    const expectedDate = new Date(sickSince).toLocaleDateString("de-DE", {
      day: "2-digit",
      month: "2-digit",
      year: "numeric",
    });
    render(
      <StudentSickReportSection
        {...defaultProps}
        isSick={true}
        sickSince={sickSince}
      />,
    );
    expect(
      screen.getByText(`Krank gemeldet seit ${expectedDate}`),
    ).toBeInTheDocument();
  });

  it("renders 'Als krank melden' subtitle when sick but no sickSince", () => {
    render(<StudentSickReportSection {...defaultProps} isSick={true} />);
    expect(screen.getByText("Als krank melden")).toBeInTheDocument();
  });

  it("calls onToggle when card is clicked", () => {
    const mockToggle = vi.fn();
    render(
      <StudentSickReportSection {...defaultProps} onToggle={mockToggle} />,
    );

    fireEvent.click(screen.getByText("Krank melden"));

    expect(mockToggle).toHaveBeenCalledTimes(1);
  });

  it("disables button when loading", () => {
    render(<StudentSickReportSection {...defaultProps} isLoading={true} />);
    expect(screen.getByRole("button")).toBeDisabled();
  });

  it("shows spinner when loading", () => {
    const { container } = render(
      <StudentSickReportSection {...defaultProps} isLoading={true} />,
    );
    expect(container.querySelector(".animate-spin")).toBeInTheDocument();
  });

  it("elevates the interactive card on hover", () => {
    render(<StudentSickReportSection {...defaultProps} />);

    expect(screen.getByRole("button", { name: /^Krank melden/ })).toHaveClass(
      "moto-hover-elevated",
    );
  });

  it("does not show spinner when not loading", () => {
    const { container } = render(
      <StudentSickReportSection {...defaultProps} />,
    );
    // Anker: die Karte mit ihrem Titel ist gerendert.
    expect(screen.getByText("Krank melden")).toBeInTheDocument();
    expect(container.querySelector(".animate-spin")).not.toBeInTheDocument();
  });
});

describe("StudentExcusedReportSection", () => {
  const defaultProps = {
    isExcused: false,
    onToggle: vi.fn(),
    isLoading: false,
  };

  it("renders 'Entschuldigen' label when student is not excused", () => {
    render(<StudentExcusedReportSection {...defaultProps} />);
    expect(screen.getByText("Entschuldigen")).toBeInTheDocument();
  });

  it("renders 'Als entschuldigt markieren' subtitle when not excused", () => {
    render(<StudentExcusedReportSection {...defaultProps} />);
    expect(screen.getByText("Als entschuldigt markieren")).toBeInTheDocument();
  });

  it("renders 'Entschuldigung aufheben' when student is excused", () => {
    render(<StudentExcusedReportSection {...defaultProps} isExcused={true} />);
    expect(screen.getByText("Entschuldigung aufheben")).toBeInTheDocument();
  });

  it("displays excused-since date when excused with excusedSince", () => {
    const excusedSince = "2026-02-10T08:00:00Z";
    const expectedDate = new Date(excusedSince).toLocaleDateString("de-DE", {
      day: "2-digit",
      month: "2-digit",
      year: "numeric",
    });
    render(
      <StudentExcusedReportSection
        {...defaultProps}
        isExcused={true}
        excusedSince={excusedSince}
      />,
    );
    expect(
      screen.getByText(`Entschuldigt seit ${expectedDate}`),
    ).toBeInTheDocument();
  });

  it("renders 'Als entschuldigt markieren' subtitle when excused but no excusedSince", () => {
    render(<StudentExcusedReportSection {...defaultProps} isExcused={true} />);
    expect(screen.getByText("Als entschuldigt markieren")).toBeInTheDocument();
  });

  it("calls onToggle when card is clicked", () => {
    const mockToggle = vi.fn();
    render(
      <StudentExcusedReportSection {...defaultProps} onToggle={mockToggle} />,
    );
    fireEvent.click(screen.getByText("Entschuldigen"));
    expect(mockToggle).toHaveBeenCalledTimes(1);
  });

  it("disables button when loading", () => {
    render(<StudentExcusedReportSection {...defaultProps} isLoading={true} />);
    expect(screen.getByRole("button")).toBeDisabled();
  });

  it("shows spinner when loading", () => {
    const { container } = render(
      <StudentExcusedReportSection {...defaultProps} isLoading={true} />,
    );
    expect(container.querySelector(".animate-spin")).toBeInTheDocument();
  });

  it("elevates the interactive card on hover", () => {
    render(<StudentExcusedReportSection {...defaultProps} />);

    expect(screen.getByRole("button", { name: /^Entschuldigen/ })).toHaveClass(
      "moto-hover-elevated",
    );
  });

  it("does not show spinner when not loading", () => {
    const { container } = render(
      <StudentExcusedReportSection {...defaultProps} />,
    );
    // Anker: die Karte mit ihrem Titel ist gerendert.
    expect(screen.getByText("Entschuldigen")).toBeInTheDocument();
    expect(container.querySelector(".animate-spin")).not.toBeInTheDocument();
  });
});

describe("StudentStatusActionsMenu", () => {
  it("renders its floating actions in the popover surface", () => {
    render(
      <StudentStatusActionsMenu
        isClassTrip={false}
        onPlanClassTrip={vi.fn()}
        isLoading={false}
      />,
    );

    fireEvent.click(
      screen.getByRole("button", { name: "Weitere Statusaktionen" }),
    );

    expect(screen.getByRole("menu")).toHaveClass(
      "moto-popover-surface",
      "rounded-xl",
    );
  });

  it("elevates the interactive card on hover", () => {
    render(
      <StudentStatusActionsMenu
        isClassTrip={false}
        onPlanClassTrip={vi.fn()}
        isLoading={false}
      />,
    );

    expect(
      screen.getByRole("button", { name: "Weitere Statusaktionen" }),
    ).toHaveClass("moto-hover-elevated");
  });

  describe("StudentStatusActionsMenu keyboard", () => {
    it("works with the keyboard", () => {
      const onPlanClassTrip = vi.fn();
      render(
        <StudentStatusActionsMenu
          isClassTrip={false}
          onPlanClassTrip={onPlanClassTrip}
          isLoading={false}
        />,
      );
      const trigger = screen.getByRole("button", {
        name: "Weitere Statusaktionen",
      });
      fireEvent.click(trigger, { detail: 0 });

      const entry = screen.getByRole("menuitem", {
        name: "Klassenfahrt planen",
      });
      expect(entry).toHaveFocus();
      fireEvent.keyDown(entry, { key: " " });

      expect(onPlanClassTrip).toHaveBeenCalledOnce();
      expect(trigger).toHaveFocus();
    });
  });
});
