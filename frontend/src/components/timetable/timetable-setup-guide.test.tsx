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
    careOfferingLinkStatus: "unlinked",
    careOfferingLinkLabel: "0 von 3 Angeboten verknüpft",
    hasPlan: false,
    plannedCount: 0,
    onManagePeriods: vi.fn(),
    onCreateEvent: vi.fn(),
    careOfferingsHref: "/care-offerings",
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

  it("hides the enrollment step when the care-offering linkage is unknown", () => {
    render(
      <TimetableSetupGuide
        {...makeProps({ careOfferingLinkStatus: "unknown" })}
      />,
    );

    expect(
      screen.queryByText("Mit der Anmeldung verknüpfen"),
    ).not.toBeInTheDocument();
  });

  // Issue #1651: the step used to tick as soon as any enrollment phase was
  // active, even when no care offering pointed at a Regeltermin.
  it("leaves the enrollment step open and links to the offerings when nothing is linked", () => {
    render(
      <TimetableSetupGuide
        {...makeProps({
          careOfferingLinkStatus: "unlinked",
          careOfferingLinkLabel: "0 von 3 Angeboten verknüpft",
        })}
      />,
    );

    const step = screen.getByRole("link", {
      name: /Mit der Anmeldung verknüpfen/,
    });
    expect(step).toHaveAttribute("href", "/care-offerings");
    expect(step).toHaveTextContent("0 von 3 Angeboten verknüpft");
    expect(step).toHaveTextContent("Angebote verknüpfen");
  });

  it("marks the enrollment step done once a care offering is linked", () => {
    render(
      <TimetableSetupGuide
        {...makeProps({
          careOfferingLinkStatus: "linked",
          careOfferingLinkLabel: "2 von 3 Angeboten verknüpft",
        })}
      />,
    );

    const step = screen.getByRole("link", {
      name: /Mit der Anmeldung verknüpfen/,
    });
    expect(step).toHaveTextContent("2 von 3 Angeboten verknüpft");
    expect(step).toHaveTextContent("Angebote öffnen");
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
