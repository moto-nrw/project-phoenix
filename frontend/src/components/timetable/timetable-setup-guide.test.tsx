import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import type { ComponentProps } from "react";

import { TimetableSetupGuide } from "./timetable-setup-guide";

// next/link -> plain anchor so the guide's enrollment step renders in jsdom.
vi.mock("next/link", () => ({
  default: ({
    children,
    href,
    className,
    ...rest
  }: {
    children: React.ReactNode;
    href: string;
    className?: string;
  }) => (
    <a href={href} className={className} {...rest}>
      {children}
    </a>
  ),
}));

type Props = ComponentProps<typeof TimetableSetupGuide>;

function makeProps(overrides: Partial<Props> = {}): Props {
  return {
    hasActivePeriod: false,
    activePeriodLabel: null,
    enrollmentStatus: "none",
    enrollmentLabel: null,
    hasPlan: false,
    plannedCount: 0,
    onManagePeriods: vi.fn(),
    onCreateEvent: vi.fn(),
    enrollmentHref: "/admin/enrollments",
    ...overrides,
  };
}

describe("TimetableSetupGuide", () => {
  it("shows the expanded guide for a fresh school", () => {
    render(<TimetableSetupGuide {...makeProps()} />);

    expect(screen.getByText("Betreuungsplan einrichten")).toBeInTheDocument();
    // No collapsed status header while setup is incomplete.
    expect(
      screen.queryByText("Betreuungsplan eingerichtet"),
    ).not.toBeInTheDocument();
    // Titles appear in the step list and again in the sidebar progress list.
    expect(
      screen.getAllByText("Planungszeitraum festlegen").length,
    ).toBeGreaterThan(0);
    expect(screen.getAllByText("Erste Woche planen").length).toBeGreaterThan(0);
  });

  it("collapses to a status header once period + plan are done", () => {
    render(
      <TimetableSetupGuide
        {...makeProps({
          hasActivePeriod: true,
          activePeriodLabel: "Schuljahr 2026 · gültig bis 31.07.2026",
          hasPlan: true,
          plannedCount: 5,
        })}
      />,
    );

    const header = screen.getByRole("button", {
      name: /Betreuungsplan eingerichtet/i,
    });
    expect(header).toHaveAttribute("aria-expanded", "false");
    expect(screen.getByText("Einrichtung abgeschlossen")).toBeInTheDocument();
  });

  it("hides the enrollment step when status is unknown", () => {
    render(
      <TimetableSetupGuide {...makeProps({ enrollmentStatus: "unknown" })} />,
    );

    expect(
      screen.queryByText("Mit der Anmeldung verknüpfen"),
    ).not.toBeInTheDocument();
  });

  it("invokes the create + manage callbacks from the step rows", () => {
    const onCreateEvent = vi.fn();
    const onManagePeriods = vi.fn();
    render(
      <TimetableSetupGuide
        {...makeProps({ onCreateEvent, onManagePeriods })}
      />,
    );

    fireEvent.click(
      screen.getByRole("button", { name: /Erste Woche planen/i }),
    );
    expect(onCreateEvent).toHaveBeenCalledTimes(1);

    fireEvent.click(
      screen.getByRole("button", { name: /Planungszeitraum festlegen/i }),
    );
    expect(onManagePeriods).toHaveBeenCalledTimes(1);
  });
});
