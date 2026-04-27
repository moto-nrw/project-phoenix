import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import {
  ArrivalTimeRow,
  ExceptionIcon,
  GroupIcon,
  PickupTimeIcon,
  PickupTimeRow,
  SchoolClassIcon,
  StudentCard,
  StudentInfoRow,
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
    const trackingEl = screen.getByTestId("tracking");
    const wrapper = trackingEl.parentElement;
    expect(wrapper?.className).toContain("flex");
    expect(wrapper?.className).toContain("flex-col");
  });

  it("renders location badge alone without wrapper when no trackingIndicators", () => {
    const { container } = render(<StudentCard {...defaultProps} />);

    expect(screen.getByTestId("location-badge")).toBeInTheDocument();
    expect(container.querySelector("[data-testid='tracking']")).toBeNull();
  });
});

describe("SchoolClassIcon", () => {
  it("renders an SVG icon", () => {
    const { container } = render(<SchoolClassIcon />);

    expect(container.querySelector("svg")).toBeInTheDocument();
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

    expect(container.querySelector("svg")).toBeInTheDocument();
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
  const now = new Date("2025-01-15T14:00:00");

  it("renders the planned pickup time", () => {
    render(<PickupTimeRow pickupTime="15:30" isException={false} now={now} />);

    expect(screen.getByText(/Abholzeit:/)).toBeInTheDocument();
    expect(screen.getByText(/15:30 Uhr/)).toBeInTheDocument();
  });

  it("renders notes alongside pickup time", () => {
    render(
      <PickupTimeRow
        pickupTime="15:30"
        isException={false}
        notes="Arzttermin"
        now={now}
      />,
    );

    expect(screen.getByText(/15:30 Uhr/)).toBeInTheDocument();
    expect(screen.getByText("(Arzttermin)")).toBeInTheDocument();
  });

  it("shows the actual time instead of the planned time once resolved", () => {
    render(
      <PickupTimeRow
        pickupTime="15:30"
        actualTime="15:42"
        isException={false}
        notes="Arzttermin"
        now={now}
      />,
    );

    expect(screen.getByText(/15:42 Uhr/)).toBeInTheDocument();
    expect(screen.queryByText(/15:30 Uhr/)).not.toBeInTheDocument();
    expect(screen.getByText("(Arzttermin)")).toBeInTheDocument();
  });

  it("renders a pulsing icon when the pickup is approaching", () => {
    const { container } = render(
      <PickupTimeRow pickupTime="14:15" isException={false} now={now} />,
    );

    expect(container.querySelector("svg.animate-pulse")).toBeInTheDocument();
  });

  it("renders exception icon when isException is true and has pickup time", () => {
    const { container } = render(
      <PickupTimeRow pickupTime="14:00" isException={true} now={now} />,
    );

    const orangeSvg = container.querySelector("svg.text-orange-500");
    expect(orangeSvg).toBeInTheDocument();
  });

  it("renders exception text when there is no pickup time", () => {
    render(
      <PickupTimeRow isException={true} notes="Ganztägig abwesend" now={now} />,
    );

    expect(screen.getByText("Ganztägig abwesend")).toBeInTheDocument();
    expect(screen.queryByText(/Abholzeit:/)).not.toBeInTheDocument();
  });

  it("renders 'Abwesend' fallback when isException is true but no notes", () => {
    render(<PickupTimeRow isException={true} now={now} />);

    expect(screen.getByText("Abwesend")).toBeInTheDocument();
  });

  it("renders the dash fallback when no pickup time exists", () => {
    render(<PickupTimeRow isException={false} now={now} />);

    expect(screen.getByText("Abholzeit: —")).toBeInTheDocument();
  });

  it("suppresses overdue urgency once an actual pickup time is recorded", () => {
    // Plan was 13:00 (overdue against 14:00 now), but the student was actually
    // picked up at 13:55 — row should resolve to a check icon, not a red warning.
    const { container } = render(
      <PickupTimeRow
        pickupTime="13:00"
        actualTime="13:55"
        isException={false}
        now={now}
      />,
    );

    expect(screen.getByText(/13:55 Uhr/)).toBeInTheDocument();
    expect(
      container.querySelector("svg.lucide-triangle-alert"),
    ).not.toBeInTheDocument();
    expect(container.querySelector("svg.lucide-check")).toBeInTheDocument();
  });
});

describe("ArrivalTimeRow", () => {
  const now = new Date("2025-01-15T08:00:00");

  it("shows absence message with reason when absent", () => {
    render(
      <ArrivalTimeRow
        isException={false}
        isAbsent={true}
        notes="Arzttermin"
        now={now}
      />,
    );

    expect(
      screen.getByText("Kommt heute nicht (Arzttermin)"),
    ).toBeInTheDocument();
  });

  it("shows default absence message when isAbsent without notes", () => {
    render(<ArrivalTimeRow isException={false} isAbsent={true} now={now} />);

    expect(screen.getByText("Kommt heute nicht")).toBeInTheDocument();
  });

  it("shows arrival time when arrivalTime provided", () => {
    render(
      <ArrivalTimeRow
        arrivalTime="08:00"
        isException={false}
        isAbsent={false}
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
        now={now}
      />,
    );

    expect(screen.getByText("(Testnote)")).toBeInTheDocument();
  });

  it("shows the actual arrival time instead of the planned time", () => {
    render(
      <ArrivalTimeRow
        arrivalTime="08:00"
        actualTime="08:07"
        isException={false}
        isAbsent={false}
        notes="Testnote"
        now={now}
      />,
    );

    expect(screen.getByText(/08:07 Uhr/)).toBeInTheDocument();
    expect(screen.queryByText(/08:00 Uhr/)).not.toBeInTheDocument();
    expect(screen.getByText("(Testnote)")).toBeInTheDocument();
  });

  it("uses exception icon when isException and has arrivalTime", () => {
    const { container } = render(
      <ArrivalTimeRow
        arrivalTime="08:15"
        isException={true}
        isAbsent={false}
        now={now}
      />,
    );

    expect(container.querySelector("svg.text-orange-500")).toBeInTheDocument();
  });

  it("renders the exception fallback when exception has no time", () => {
    render(<ArrivalTimeRow isException={true} isAbsent={false} now={now} />);

    expect(screen.getByText("Kommt heute nicht")).toBeInTheDocument();
  });

  it("falls back to a dash when no arrival info exists", () => {
    render(<ArrivalTimeRow isException={false} isAbsent={false} now={now} />);

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
