import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import {
  StudentCard,
  SchoolClassIcon,
  GroupIcon,
  StudentInfoRow,
  PickupTimeIcon,
  ExceptionIcon,
  PickupTimeRow,
  renderPickupIcon,
  ArrivalTimeRow,
} from "./student-card";

describe("StudentCard", () => {
  const defaultProps = {
    studentId: "1",
    firstName: "Max",
    lastName: "Mustermann",
    onClick: vi.fn(),
    locationBadge: <span data-testid="location-badge">In Room</span>,
  };

  it("renders student first and last name", () => {
    render(<StudentCard {...defaultProps} />);

    expect(screen.getByText("Max")).toBeInTheDocument();
    expect(screen.getByText("Mustermann")).toBeInTheDocument();
  });

  it("renders location badge", () => {
    render(<StudentCard {...defaultProps} />);

    expect(screen.getByTestId("location-badge")).toBeInTheDocument();
  });

  it("calls onClick when clicked", () => {
    const onClick = vi.fn();
    render(<StudentCard {...defaultProps} onClick={onClick} />);

    fireEvent.click(screen.getByRole("button"));

    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it("has accessible aria-label", () => {
    render(<StudentCard {...defaultProps} />);

    expect(
      screen.getByLabelText("Max Mustermann - Tippen für mehr Infos"),
    ).toBeInTheDocument();
  });

  it("renders click hint text", () => {
    render(<StudentCard {...defaultProps} />);

    expect(screen.getByText("Tippen für mehr Infos")).toBeInTheDocument();
  });

  it("renders extra content when provided", () => {
    render(
      <StudentCard
        {...defaultProps}
        extraContent={<span data-testid="extra">Class 1a</span>}
      />,
    );

    expect(screen.getByTestId("extra")).toBeInTheDocument();
    expect(screen.getByText("Class 1a")).toBeInTheDocument();
  });

  it("applies custom gradient class", () => {
    const { container } = render(
      <StudentCard
        {...defaultProps}
        gradient="from-red-50/80 to-pink-100/80"
      />,
    );

    const gradientDiv = container.querySelector(".from-red-50\\/80");
    expect(gradientDiv).toBeInTheDocument();
  });

  it("applies default gradient when not specified", () => {
    const { container } = render(<StudentCard {...defaultProps} />);

    const gradientDiv = container.querySelector(".from-blue-50\\/80");
    expect(gradientDiv).toBeInTheDocument();
  });

  it("wraps location badge and tracking indicators in flex-col when trackingIndicators provided", () => {
    render(
      <StudentCard
        {...defaultProps}
        trackingIndicators={<span data-testid="tracking">Indicators</span>}
      />,
    );

    expect(screen.getByTestId("tracking")).toBeInTheDocument();
    expect(screen.getByTestId("location-badge")).toBeInTheDocument();
    // Both should be inside a flex-col wrapper
    const trackingEl = screen.getByTestId("tracking");
    const wrapper = trackingEl.parentElement;
    expect(wrapper?.className).toContain("flex");
    expect(wrapper?.className).toContain("flex-col");
  });

  it("renders location badge alone without wrapper when no trackingIndicators", () => {
    const { container } = render(<StudentCard {...defaultProps} />);

    expect(screen.getByTestId("location-badge")).toBeInTheDocument();
    // No tracking indicators present
    expect(container.querySelector("[data-testid='tracking']")).toBeNull();
  });

  // ── School check-in mode ───────────────────────────────────────────────
  // The check-in mode flips the card into a mutation surface: tint by
  // attendance state, click fires onCheckinClick instead of onClick, the
  // hint copy changes, and a spinner overlays during pendingCount > 0.

  it("uses navigation copy + onClick when checkinMode is false (default)", () => {
    const onClick = vi.fn();
    const onCheckinClick = vi.fn();
    render(
      <StudentCard
        {...defaultProps}
        onClick={onClick}
        onCheckinClick={onCheckinClick}
      />,
    );
    // Navigation hint copy.
    expect(screen.getByText(/Tippen für mehr Infos/)).toBeInTheDocument();
    // The default click goes to onClick, NOT onCheckinClick.
    fireEvent.click(screen.getByRole("button"));
    expect(onClick).toHaveBeenCalledTimes(1);
    expect(onCheckinClick).not.toHaveBeenCalled();
  });

  it("renders the tap-strip with state-specific copy when checkinMode is true", () => {
    // Per StudentCheckinState the tap-strip flips copy: anwesend/schulhof
    // → "Abmelden" (already present, tap checks out); abwesend → "Anmelden".
    // The "An-/Abmelden" generic only shows up on the unknown state.
    const onCheckinClick = vi.fn();
    render(
      <StudentCard
        {...defaultProps}
        checkinMode
        checkinState="anwesend"
        onCheckinClick={onCheckinClick}
      />,
    );
    expect(screen.getByText(/Tippen zum Abmelden/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button"));
    expect(onCheckinClick).toHaveBeenCalledTimes(1);
  });

  it("uses Anmelden copy on absent students (tap = check in)", () => {
    render(
      <StudentCard
        {...defaultProps}
        checkinMode
        checkinState="abwesend"
        onCheckinClick={vi.fn()}
      />,
    );
    expect(screen.getByText(/Tippen zum Anmelden/)).toBeInTheDocument();
  });

  it("falls back to An-/Abmelden generic on unknown state", () => {
    render(
      <StudentCard
        {...defaultProps}
        checkinMode
        checkinState="unknown"
        onCheckinClick={vi.fn()}
      />,
    );
    expect(screen.getByText(/Tippen zum An-\/Abmelden/)).toBeInTheDocument();
  });

  it("routes click to onCheckinClick instead of onClick when in check-in mode", () => {
    const onClick = vi.fn();
    const onCheckinClick = vi.fn();
    render(
      <StudentCard
        {...defaultProps}
        checkinMode
        checkinState="anwesend"
        onClick={onClick}
        onCheckinClick={onCheckinClick}
      />,
    );
    fireEvent.click(screen.getByRole("button"));
    expect(onCheckinClick).toHaveBeenCalledTimes(1);
    expect(onClick).not.toHaveBeenCalled();
  });

  it("falls back to onClick when checkinMode is true but onCheckinClick is omitted", () => {
    // Defensive branch — partial adoption shouldn't break navigation.
    const onClick = vi.fn();
    render(
      <StudentCard
        {...defaultProps}
        checkinMode
        checkinState="anwesend"
        onClick={onClick}
      />,
    );
    fireEvent.click(screen.getByRole("button"));
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it.each([["anwesend"], ["schulhof"], ["abwesend"], ["unknown"]])(
    "exposes checkinState=%s on the data-checkin-state attribute when active",
    (state) => {
      render(
        <StudentCard
          {...defaultProps}
          checkinMode
          checkinState={
            state as "anwesend" | "schulhof" | "abwesend" | "unknown"
          }
        />,
      );
      const btn = screen.getByRole("button");
      expect(btn).toHaveAttribute("data-checkin-mode", "true");
      expect(btn).toHaveAttribute("data-checkin-state", state);
    },
  );

  it("renders the spinner inside the tap-strip and disables the button when isCheckinPending", () => {
    const onCheckinClick = vi.fn();
    const { container } = render(
      <StudentCard
        {...defaultProps}
        checkinMode
        checkinState="anwesend"
        isCheckinPending
        onCheckinClick={onCheckinClick}
      />,
    );
    // The tap-strip itself remains present (preserves card height under
    // toggle), and the spinner replaces the action icon while pending.
    const strip = container.querySelector("[data-checkin-tap-strip='true']");
    expect(strip).not.toBeNull();
    expect(strip?.querySelector(".animate-spin")).not.toBeNull();
    const btn = screen.getByRole("button");
    expect(btn).toHaveAttribute("aria-busy", "true");
    expect(btn).toBeDisabled();
    // Disabled button must NOT fire onCheckinClick.
    fireEvent.click(btn);
    expect(onCheckinClick).not.toHaveBeenCalled();
  });

  it("hides the navigation arrow + 'mehr Infos' hint when checkinMode is true", () => {
    // Affordances scoped to navigation mode only; the tap-strip carries
    // the action signal in check-in mode instead.
    render(
      <StudentCard
        {...defaultProps}
        checkinMode
        checkinState="anwesend"
        onCheckinClick={vi.fn()}
      />,
    );
    expect(screen.queryByText(/Tippen für mehr Infos/)).toBeNull();
  });
});

describe("SchoolClassIcon", () => {
  it("renders an SVG icon", () => {
    const { container } = render(<SchoolClassIcon />);

    const svg = container.querySelector("svg");
    expect(svg).toBeInTheDocument();
  });

  it("has proper styling classes", () => {
    const { container } = render(<SchoolClassIcon />);

    const svg = container.querySelector("svg");
    expect(svg?.className).toContain("h-3.5");
    expect(svg?.className).toContain("w-3.5");
    expect(svg?.className).toContain("text-gray-400");
  });
});

describe("GroupIcon", () => {
  it("renders an SVG icon", () => {
    const { container } = render(<GroupIcon />);

    const svg = container.querySelector("svg");
    expect(svg).toBeInTheDocument();
  });

  it("has proper styling classes", () => {
    const { container } = render(<GroupIcon />);

    const svg = container.querySelector("svg");
    expect(svg?.className).toContain("h-3.5");
    expect(svg?.className).toContain("w-3.5");
    expect(svg?.className).toContain("text-gray-400");
  });
});

describe("PickupTimeIcon", () => {
  it("renders an SVG icon", () => {
    const { container } = render(<PickupTimeIcon />);
    expect(container.querySelector("svg")).toBeInTheDocument();
  });
});

describe("ExceptionIcon", () => {
  it("renders an SVG icon with orange color", () => {
    const { container } = render(<ExceptionIcon />);
    const svg = container.querySelector("svg");
    expect(svg).toBeInTheDocument();
    expect(svg?.className).toContain("text-orange-500");
  });
});

describe("PickupTimeRow", () => {
  // Fixed "now" for deterministic urgency calculations.
  // 14:00 means a pickup at 23:59 is "normal", at 14:15 is "soon", at 13:00 is "overdue".
  const now = new Date("2025-01-15T14:00:00");

  it("renders pickup time with urgency icon when pickupTime is provided", () => {
    render(
      <PickupTimeRow
        pickupTime="15:30"
        isException={false}
        isHome={false}
        now={now}
      />,
    );

    expect(screen.getByText(/Abholzeit:/)).toBeInTheDocument();
    expect(screen.getByText(/15:30 Uhr/)).toBeInTheDocument();
  });

  it("renders notes alongside pickup time", () => {
    render(
      <PickupTimeRow
        pickupTime="15:30"
        isException={false}
        notes="Arzttermin"
        isHome={false}
        now={now}
      />,
    );

    expect(screen.getByText(/15:30 Uhr/)).toBeInTheDocument();
    expect(screen.getByText("(Arzttermin)")).toBeInTheDocument();
  });

  it("renders exception icon when isException is true and has pickup time", () => {
    const { container } = render(
      <PickupTimeRow
        pickupTime="14:00"
        isException={true}
        isHome={false}
        now={now}
      />,
    );

    // Exception icon is an orange SVG, not the urgency AlertTriangle
    const orangeSvg = container.querySelector("svg.text-orange-500");
    expect(orangeSvg).toBeInTheDocument();
    // Should NOT render AlertTriangle (red)
    expect(container.querySelector("svg.text-red-500")).not.toBeInTheDocument();
  });

  it("renders exception reason when isException is true and no pickup time", () => {
    render(
      <PickupTimeRow
        isException={true}
        notes="Ganztägig abwesend"
        isHome={false}
        now={now}
      />,
    );

    expect(screen.getByText("Ganztägig abwesend")).toBeInTheDocument();
    expect(screen.queryByText(/Abholzeit:/)).not.toBeInTheDocument();
  });

  it("renders 'Abwesend' fallback when isException is true but no notes", () => {
    render(<PickupTimeRow isException={true} isHome={false} now={now} />);

    expect(screen.getByText("Abwesend")).toBeInTheDocument();
  });

  it("renders dash fallback when no pickup time and no exception", () => {
    render(<PickupTimeRow isException={false} isHome={false} now={now} />);

    expect(screen.getByText("Abholzeit: —")).toBeInTheDocument();
  });

  it("suppresses urgency for at-home students (isHome=true)", () => {
    const { container } = render(
      <PickupTimeRow
        pickupTime="13:00"
        isException={false}
        isHome={true}
        now={now}
      />,
    );

    // 13:00 is overdue, but isHome=true should yield gray clock, not red triangle
    expect(screen.getByText(/13:00 Uhr/)).toBeInTheDocument();
    expect(container.querySelector("svg.text-red-500")).not.toBeInTheDocument();
    // Should render gray PickupTimeIcon
    expect(container.querySelector("svg.text-gray-400")).toBeInTheDocument();
  });
});

describe("renderPickupIcon", () => {
  it("renders red AlertTriangle for overdue", () => {
    const { container } = render(<>{renderPickupIcon("overdue")}</>);
    expect(container.querySelector("svg.text-red-500")).toBeInTheDocument();
  });

  it("renders pulsing orange Clock for soon", () => {
    const { container } = render(<>{renderPickupIcon("soon")}</>);
    const clock = container.querySelector("svg.text-orange-500");
    expect(clock).toBeInTheDocument();
    expect(clock?.className).toContain("animate-pulse");
  });

  it("renders gray PickupTimeIcon for normal", () => {
    const { container } = render(<>{renderPickupIcon("normal")}</>);
    expect(container.querySelector("svg.text-gray-400")).toBeInTheDocument();
  });

  it("renders gray PickupTimeIcon for none", () => {
    const { container } = render(<>{renderPickupIcon("none")}</>);
    expect(container.querySelector("svg.text-gray-400")).toBeInTheDocument();
  });
});

describe("ArrivalTimeRow", () => {
  const now = new Date("2025-01-15T08:00:00");

  it("shows absence message with reason when isAbsent", () => {
    render(
      <ArrivalTimeRow
        isException={false}
        isAbsent={true}
        notes="Arzttermin"
        isHome={false}
        now={now}
      />,
    );

    expect(
      screen.getByText("Kommt heute nicht (Arzttermin)"),
    ).toBeInTheDocument();
  });

  it("shows default absence message when isAbsent without notes", () => {
    render(
      <ArrivalTimeRow
        isException={false}
        isAbsent={true}
        isHome={false}
        now={now}
      />,
    );

    expect(screen.getByText("Kommt heute nicht")).toBeInTheDocument();
  });

  it("shows arrival time when arrivalTime provided", () => {
    render(
      <ArrivalTimeRow
        arrivalTime="08:00"
        isException={false}
        isAbsent={false}
        isHome={false}
        now={now}
      />,
    );

    expect(screen.getByText(/08:00 Uhr/)).toBeInTheDocument();
  });

  it("shows notes next to arrival time", () => {
    render(
      <ArrivalTimeRow
        arrivalTime="08:00"
        isException={false}
        isAbsent={false}
        notes="Testnote"
        isHome={false}
        now={now}
      />,
    );

    expect(screen.getByText("(Testnote)")).toBeInTheDocument();
  });

  it("uses exception icon when isException and has arrivalTime", () => {
    const { container } = render(
      <ArrivalTimeRow
        arrivalTime="08:15"
        isException={true}
        isAbsent={false}
        isHome={false}
        now={now}
      />,
    );

    expect(container.querySelector("svg.text-orange-500")).toBeInTheDocument();
  });

  it("falls back to dash when no arrival info", () => {
    render(
      <ArrivalTimeRow
        isException={false}
        isAbsent={false}
        isHome={false}
        now={now}
      />,
    );

    expect(screen.getByText("Ankunftszeit: —")).toBeInTheDocument();
  });
});

describe("StudentInfoRow", () => {
  it("renders icon and children", () => {
    render(
      <StudentInfoRow icon={<span data-testid="icon">Icon</span>}>
        Class 2b
      </StudentInfoRow>,
    );

    expect(screen.getByTestId("icon")).toBeInTheDocument();
    expect(screen.getByText("Class 2b")).toBeInTheDocument();
  });

  it("has proper styling", () => {
    const { container } = render(
      <StudentInfoRow icon={<span>Icon</span>}>Info</StudentInfoRow>,
    );

    const wrapper = container.firstChild as HTMLElement;
    expect(wrapper.className).toContain("flex");
    expect(wrapper.className).toContain("items-center");
  });
});
