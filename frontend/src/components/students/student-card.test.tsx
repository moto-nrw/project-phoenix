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
