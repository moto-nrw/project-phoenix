import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  AbsenceIcon,
  ArrivalTimeRow,
  ExceptionIcon,
  GroupIcon,
  PickupTimeIcon,
  PickupTimeRow,
  SchoolClassIcon,
  StudentAbsenceRow,
  StudentCard,
  StudentInfoRow,
  StudentPendingExcusedRow,
} from "./student-card";

// Default the photos feature flag to off so the original test suite (which
// pre-dates the avatar overlay) keeps its existing assertions. Individual
// tests below flip the mock to `true` to exercise the avatar branch.
const photosEnabledMock = vi.fn(() => ({
  enabled: false,
  isLoading: false,
}));

vi.mock("~/lib/hooks/use-student-photos-enabled", () => ({
  useStudentPhotosEnabled: () => photosEnabledMock(),
}));

beforeEach(() => {
  photosEnabledMock.mockReturnValue({ enabled: false, isLoading: false });
});

afterEach(() => {
  photosEnabledMock.mockReset();
});

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

  it("does not render the previous default gradient overlay", () => {
    const { container } = render(<StudentCard {...defaultProps} />);

    const gradientDiv = container.querySelector(".from-blue-50\\/80");
    expect(gradientDiv).not.toBeInTheDocument();
  });

  it("renders both location badge and tracking indicators when both are provided", () => {
    // Layout note: locationBadge and trackingIndicators share the right-
    // side vertical stack in the header row (badge on top, indicators
    // below). The avatar lives inline in the bottom hint row, so it
    // doesn't compete with the right-side stack.
    render(
      <StudentCard
        {...defaultProps}
        trackingIndicators={<span data-testid="tracking">Indicators</span>}
      />,
    );

    expect(screen.getByTestId("tracking")).toBeInTheDocument();
    expect(screen.getByTestId("location-badge")).toBeInTheDocument();
  });

  it("renders location badge alone without wrapper when no trackingIndicators", () => {
    const { container } = render(<StudentCard {...defaultProps} />);

    expect(screen.getByTestId("location-badge")).toBeInTheDocument();
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

  // ── Avatar overlay (per-tenant student-photos feature) ────────────────
  // Covers the photosEnabled branch and the photoUrl / fallback-initials
  // permutations on the Avatar embedded in the card header.

  it("does NOT render the avatar when the photos feature is disabled", () => {
    photosEnabledMock.mockReturnValue({ enabled: false, isLoading: false });
    render(<StudentCard {...defaultProps} photoUrl="https://x/y.jpg" />);
    // aria-label on Avatar is the trimmed full name; absence proves the
    // overlay was skipped despite a photoUrl being passed in.
    expect(screen.queryByLabelText("Max Mustermann")).toBeNull();
  });

  it("renders the avatar with the student photo when the feature is enabled", () => {
    photosEnabledMock.mockReturnValue({ enabled: true, isLoading: false });
    render(
      <StudentCard {...defaultProps} photoUrl="https://example.com/max.jpg" />,
    );
    const avatar = screen.getByLabelText("Max Mustermann");
    expect(avatar).toBeInTheDocument();
    // Image element is rendered by next/image inside the avatar — the alt
    // text mirrors the trimmed name, which is what screen readers announce.
    expect(screen.getByAltText("Max Mustermann")).toBeInTheDocument();
  });

  it("falls back to initials when photoUrl is omitted", () => {
    photosEnabledMock.mockReturnValue({ enabled: true, isLoading: false });
    render(<StudentCard {...defaultProps} photoUrl={null} />);
    const avatar = screen.getByLabelText("Max Mustermann");
    expect(avatar.textContent).toBe("MM");
  });

  it("falls back to '?' initials when both names are missing", () => {
    // Hits the `firstName ?? "" + lastName ?? ""`.trim() || "?"` branch.
    photosEnabledMock.mockReturnValue({ enabled: true, isLoading: false });
    render(
      <StudentCard
        {...defaultProps}
        firstName={undefined}
        lastName={undefined}
      />,
    );
    const avatar = screen.getByLabelText("?");
    expect(avatar.textContent).toBe("?");
  });

  it("renders the 'anmelden' (+) action icon on the tap-strip for absent students", () => {
    // Covers tapStrip.action === "anmelden" branch in the icon switch.
    const { container } = render(
      <StudentCard
        {...defaultProps}
        checkinMode
        checkinState="abwesend"
        onCheckinClick={vi.fn()}
      />,
    );
    const strip = container.querySelector("[data-checkin-tap-strip='true']");
    expect(strip).not.toBeNull();
    expect(strip?.querySelector("svg.lucide-plus")).not.toBeNull();
  });

  it("renders the 'abmelden' (−) action icon on the tap-strip for present students", () => {
    // Covers tapStrip.action === "abmelden" branch in the icon switch.
    const { container } = render(
      <StudentCard
        {...defaultProps}
        checkinMode
        checkinState="anwesend"
        onCheckinClick={vi.fn()}
      />,
    );
    const strip = container.querySelector("[data-checkin-tap-strip='true']");
    expect(strip?.querySelector("svg.lucide-minus")).not.toBeNull();
  });

  it("does not set the data-checkin-* attributes when checkinMode is off", () => {
    // Covers the `checkinMode || undefined` and `checkinMode ? ... : undefined`
    // false branches on the data attributes.
    render(<StudentCard {...defaultProps} />);
    const btn = screen.getByRole("button");
    expect(btn).not.toHaveAttribute("data-checkin-mode");
    expect(btn).not.toHaveAttribute("data-checkin-state");
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

    expect(container.querySelector("svg")).toBeInTheDocument();
  });

  it("renders a muted neutral meta icon at meta-icon size", () => {
    const { container } = render(<SchoolClassIcon />);

    const svg = container.querySelector("svg");
    expect(svg).toHaveAttribute("data-moto-duotone-tone", "neutral");
    expect(svg).toHaveAttribute("width", "14");
    expect(svg).toHaveAttribute("height", "14");
  });
});

describe("GroupIcon", () => {
  it("renders an SVG icon", () => {
    const { container } = render(<GroupIcon />);

    expect(container.querySelector("svg")).toBeInTheDocument();
  });

  it("renders a muted neutral meta icon at meta-icon size", () => {
    const { container } = render(<GroupIcon />);

    const svg = container.querySelector("svg");
    expect(svg).toHaveAttribute("data-moto-duotone-tone", "neutral");
    expect(svg).toHaveAttribute("width", "14");
    expect(svg).toHaveAttribute("height", "14");
  });
});

describe("PickupTimeIcon", () => {
  it("renders an SVG icon", () => {
    const { container } = render(<PickupTimeIcon />);

    expect(container.querySelector("svg")).toBeInTheDocument();
  });

  it("renders the same muted outline clock as the with-time Gehzeit rows", () => {
    const { container } = render(<PickupTimeIcon />);

    const svg = container.querySelector("svg");
    expect(svg?.className).toContain("lucide-clock");
    expect(svg?.className).toContain("text-gray-400");
    expect(svg).not.toHaveAttribute("data-moto-duotone-tone");
  });
});

describe("ExceptionIcon", () => {
  it("renders an orange exception marker", () => {
    const { container } = render(<ExceptionIcon />);

    const svg = container.querySelector("svg");
    expect(svg).toBeInTheDocument();
    expect(svg).toHaveAttribute("data-moto-duotone-tone", "orange");
  });
});

describe("AbsenceIcon", () => {
  it("renders a neutral calendar-x marker, not a clock or an amber warning", () => {
    const { container } = render(<AbsenceIcon />);

    const svg = container.querySelector("svg");
    expect(svg).toBeInTheDocument();
    // Distinct glyph: absence lines must never share the Gehzeit clock.
    expect(svg?.className).not.toContain("lucide-clock");
    expect(svg).toHaveAttribute("data-moto-duotone-tone", "neutral");
    expect(svg).toHaveAttribute("width", "14");
    expect(svg?.className).not.toContain("text-moto-orange-strong");
  });
});

describe("PickupTimeRow", () => {
  const now = new Date("2025-01-15T14:00:00");

  it("renders the planned pickup time", () => {
    render(<PickupTimeRow pickupTime="15:30" isException={false} now={now} />);

    expect(screen.getByText(/Gehzeit:/)).toBeInTheDocument();
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

    const exceptionSvg = container.querySelector(
      "svg[data-moto-duotone-tone='orange']",
    );
    expect(exceptionSvg).toBeInTheDocument();
  });

  it("renders exception text when there is no pickup time", () => {
    render(
      <PickupTimeRow isException={true} notes="Ganztägig abwesend" now={now} />,
    );

    expect(screen.getByText("Ganztägig abwesend")).toBeInTheDocument();
    expect(screen.queryByText(/Gehzeit:/)).not.toBeInTheDocument();
  });

  it("renders 'Abwesend' fallback when isException is true but no notes", () => {
    render(<PickupTimeRow isException={true} now={now} />);

    expect(screen.getByText("Abwesend")).toBeInTheDocument();
  });

  it("renders the dash fallback when no pickup time exists", () => {
    render(<PickupTimeRow isException={false} now={now} />);

    expect(screen.getByText("Gehzeit: –")).toBeInTheDocument();
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

    expect(
      container.querySelector("svg[data-moto-duotone-tone='orange']"),
    ).toBeInTheDocument();
  });

  it("renders the exception fallback when exception has no time", () => {
    render(<ArrivalTimeRow isException={true} isAbsent={false} now={now} />);

    expect(screen.getByText("Kommt heute nicht")).toBeInTheDocument();
  });

  it("falls back to a dash when no arrival info exists", () => {
    render(<ArrivalTimeRow isException={false} isAbsent={false} now={now} />);

    expect(screen.getByText("Ankunftszeit: –")).toBeInTheDocument();
  });
});

describe("StudentAbsenceRow", () => {
  it("renders a single neutral 'not coming' line for a sick student", () => {
    const { container } = render(<StudentAbsenceRow label="krank gemeldet" />);

    expect(
      screen.getByText("Kommt heute nicht (krank gemeldet)"),
    ).toBeInTheDocument();
    // Neutral duotone calendar-x: a known absence is information, not a
    // warning. Never the amber exception triangle, never the red overdue
    // warning, and never the Gehzeit clock glyph.
    expect(
      container.querySelector('svg[data-moto-duotone-tone="neutral"]'),
    ).toBeInTheDocument();
    expect(
      container.querySelector("svg.text-moto-orange-strong"),
    ).not.toBeInTheDocument();
    expect(
      container.querySelector("svg.lucide-triangle-alert"),
    ).not.toBeInTheDocument();
  });

  it("renders the excused label", () => {
    render(<StudentAbsenceRow label="entschuldigt" />);

    expect(
      screen.getByText("Kommt heute nicht (entschuldigt)"),
    ).toBeInTheDocument();
  });
});

describe("StudentPendingExcusedRow", () => {
  it("uses the shared amber status tone", () => {
    render(<StudentPendingExcusedRow note="Noch offen" />);

    expect(screen.getByText("Freigabe ausstehend")).toHaveStyle({
      backgroundColor: "#FEF3C7",
      color: "#92400E",
    });
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
