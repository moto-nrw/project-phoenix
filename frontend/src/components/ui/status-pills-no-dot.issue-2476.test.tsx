import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { DataTableStatusBadge } from "./data-table";
import { LocationBadge } from "~/components/students/presence/location-badge";
import { OccupancyBadges } from "~/components/activities/occupancy-badges";
import { OriginChip } from "./planning/origin-chip";
import { VisibilityBadge } from "./visibility-badge";
import { PresenceBadge } from "~/components/students/presence/presence-badge";
import { StatusBadge } from "./status-badge";
import { StatusColorBadge } from "./status-color-badge";
import { RoomStatusBadge } from "~/components/rooms/room-status-badge";
import { LOCATION_COLORS, LOCATION_STATUSES } from "~/lib/location-helper";
import { getPillTone } from "~/lib/kit-palette";

// #2476: a status pill is text on a tinted surface, nothing else. The leading
// dot repeated what tint and label already said, so every kit pill renders its
// label as the pill's sole content and leaves no spacer where the dot sat.
function expectPlainPill(pill: Element | null, label?: string) {
  expect(pill).not.toBeNull();
  const el = pill as HTMLElement;
  expect(el.children).toHaveLength(0);
  expect(el.textContent).toBe(label ?? el.textContent);
  expect(el.textContent).not.toBe("");
  expect(el.className).not.toMatch(/\bgap-|\bmr-/);
}

// #5617: the warning pill's only child is its warning icon, hidden from
// screen readers; no dot element beside it.
function expectWarningPill(pill: Element | null, label: string) {
  expect(pill).not.toBeNull();
  const el = pill as HTMLElement;
  expect(Array.from(el.children).map((child) => child.tagName)).toEqual([
    "svg",
  ]);
  expect(el.querySelector("svg")).toHaveAttribute("aria-hidden", "true");
  expect(el.textContent).toBe(label);
  expect(el.className).not.toMatch(/\bmr-/);
}

// #4371: a fact next to the location is a neutral text line with a concept
// icon, never a second tinted pill.
function expectHintLine(hint: Element | null, label: string) {
  expect(hint).not.toBeNull();
  const el = hint as HTMLElement;
  expect(el.textContent).toBe(label);
  expect(el.className).not.toMatch(/\brounded-full\b/);
  expect(el.style.backgroundColor).toBe("");
  expect(el.querySelector("svg")).toHaveAttribute("aria-hidden", "true");
}

const SIZES = ["sm", "md", "lg"] as const;
const VARIANTS = ["modern", "simple"] as const;

describe("LocationBadge without decorative dot", () => {
  describe.each(VARIANTS)("%s variant", (variant) => {
    it.each(SIZES)("renders the %s pill as plain text", (size) => {
      const { container } = render(
        <LocationBadge
          student={{ current_location: "Anwesend - Raum 101" }}
          displayMode="roomName"
          variant={variant}
          size={size}
        />,
      );
      expectPlainPill(container.querySelector("[data-location-status]"));
    });

    it.each([
      ["sick", "data-sick-indicator", LOCATION_STATUSES.SICK],
      ["class_trip", "data-class-trip-indicator", LOCATION_STATUSES.CLASS_TRIP],
      ["excused", "data-excused-indicator", LOCATION_STATUSES.EXCUSED],
      [
        "arrival_is_exception",
        "data-not-arrival-indicator",
        LOCATION_STATUSES.UNPLANNED_PRESENT,
      ],
    ] as const)(
      "renders the %s hint as a text line, not a second pill",
      (flag, attr, label) => {
        const { container } = render(
          <LocationBadge
            student={{ current_location: "Anwesend - Raum 101", [flag]: true }}
            displayMode="roomName"
            variant={variant}
          />,
        );
        expectHintLine(container.querySelector(`[${attr}]`), label);
        expect(container.querySelectorAll(".rounded-full")).toHaveLength(1);
      },
    );
  });

  it("renders the replaced at-home status as plain text", () => {
    const { container } = render(
      <LocationBadge
        student={{ current_location: "Zuhause", sick: true }}
        displayMode="roomName"
      />,
    );
    expectPlainPill(
      container.querySelector("[data-location-status]"),
      LOCATION_STATUSES.SICK,
    );
  });
});

describe("PresenceBadge without decorative dot", () => {
  describe.each(VARIANTS)("%s variant", (variant) => {
    it.each(SIZES)("renders the %s pill as plain text", (size) => {
      const { container } = render(
        <PresenceBadge
          student={{ current_location: "Anwesend" }}
          variant={variant}
          size={size}
        />,
      );
      expectPlainPill(
        container.querySelector("[data-presence-state]"),
        LOCATION_STATUSES.PRESENT,
      );
    });

    it.each([
      ["sick", "data-sick-indicator", LOCATION_STATUSES.SICK],
      ["class_trip", "data-class-trip-indicator", LOCATION_STATUSES.CLASS_TRIP],
      ["excused", "data-excused-indicator", LOCATION_STATUSES.EXCUSED],
      [
        "arrival_is_exception",
        "data-not-arrival-indicator",
        LOCATION_STATUSES.UNPLANNED_PRESENT,
      ],
    ] as const)(
      "renders the %s hint as a text line, not a second pill",
      (flag, attr, label) => {
        const { container } = render(
          <PresenceBadge
            student={{ current_location: "Anwesend", [flag]: true }}
            variant={variant}
          />,
        );
        expectHintLine(container.querySelector(`[${attr}]`), label);
        expect(container.querySelectorAll(".rounded-full")).toHaveLength(1);
      },
    );
  });

  it("renders the Schulhof and Abwesend states as plain text", () => {
    const yard = render(
      <PresenceBadge student={{ current_location: "Schulhof" }} />,
    );
    expectPlainPill(
      yard.container.querySelector("[data-presence-state]"),
      LOCATION_STATUSES.SCHOOLYARD,
    );
    yard.unmount();

    const home = render(<PresenceBadge student={{ current_location: "" }} />);
    expectPlainPill(
      home.container.querySelector("[data-presence-state]"),
      LOCATION_STATUSES.HOME,
    );
  });
});

describe("StatusBadge, StatusColorBadge, DataTableStatusBadge without decorative dot", () => {
  it("StatusBadge renders every tone and size as plain text", () => {
    for (const tone of [
      "blue",
      "green",
      "orange",
      "red",
      "gray",
      "purple",
      "cyan",
      "outline",
    ] as const) {
      for (const size of ["md", "xs"] as const) {
        for (const compact of [false, true]) {
          const label = `Ton ${tone} ${size} ${compact}`;
          const view = render(
            <StatusBadge
              label={label}
              tone={tone}
              size={size}
              compact={compact}
            />,
          );
          expectPlainPill(screen.getByText(label), label);
          view.unmount();
        }
      }
    }
  });

  it("StatusColorBadge renders a palette hex and a custom room hex as plain text", () => {
    const palette = render(
      <StatusColorBadge label="Krank" color={LOCATION_COLORS.SICK} />,
    );
    expectPlainPill(screen.getByText("Krank"), "Krank");
    palette.unmount();

    const room = render(<StatusColorBadge label="Bauraum" color="#A3D977" />);
    expectPlainPill(screen.getByText("Bauraum"), "Bauraum");
    room.unmount();

    render(
      <StatusColorBadge
        label="Eingeladen"
        color={LOCATION_COLORS.WARNING}
        size="xs"
      />,
    );
    expectWarningPill(screen.getByText("Eingeladen"), "Eingeladen");
  });

  // #5617: the warning pill carries a warning icon as its second cue next to
  // the Schulhof pill. A state icon, not the decorative dot #2476 removed.
  it("StatusBadge renders the warning tone with its icon and no dot", () => {
    for (const size of ["md", "xs"] as const) {
      const label = `Ton warning ${size}`;
      const view = render(
        <StatusBadge label={label} tone="warning" size={size} />,
      );
      expectWarningPill(screen.getByText(label), label);
      view.unmount();
    }
  });

  it.each([
    ["Aktiv", { active: true }, LOCATION_COLORS.GROUP_ROOM],
    ["Inaktiv", { active: false }, LOCATION_COLORS.DANGER],
    ["Unbekannt", { active: false, unknown: true }, LOCATION_COLORS.UNKNOWN],
  ] as const)(
    "DataTableStatusBadge renders %s as plain text in the tint of its state",
    (label, props, color) => {
      const view = render(<DataTableStatusBadge {...props} />);
      const pill = screen.getByText(label);
      expectPlainPill(pill, label);
      // #5584: the DataTable pill shares the StatusBadge base, so it carries
      // the tone of its state instead of a gray-50 surface.
      const tone = getPillTone(color);
      expect(pill.style.color).toBe(tone.textColor);
      expect(pill.style.backgroundColor).toBe(tone.backgroundColor);
      view.unmount();
    },
  );
});

describe("location pills with a long room name", () => {
  it("lets the label break instead of squeezing the name beside it", () => {
    const location = render(
      <LocationBadge
        student={{ current_location: "Anwesend - Mehrzweckraum Erdgeschoss" }}
        displayMode="roomName"
      />,
    );
    expect(
      location.container.querySelector("[data-location-status]"),
    ).toHaveClass("whitespace-normal");
    location.unmount();

    const presence = render(
      <PresenceBadge student={{ current_location: "Anwesend" }} />,
    );
    expect(
      presence.container.querySelector("[data-presence-state]"),
    ).toHaveClass("whitespace-normal");
  });
});

describe("OriginChip without decorative dot", () => {
  it("renders the source label as a plain kit pill", () => {
    render(<OriginChip label="Soll aus Arbeitszeitmodell" />);
    expectPlainPill(
      screen.getByText("Soll aus Arbeitszeitmodell"),
      "Soll aus Arbeitszeitmodell",
    );
  });
});

describe("OccupancyBadges without decorative dot", () => {
  it("renders the count and the overbooked state as plain pills", () => {
    render(
      <OccupancyBadges
        occupancy={{ count: 66, limit: 45 }}
        nfcEnabled={false}
      />,
    );
    expectPlainPill(screen.getByText("66 / 45 Kinder"), "66 / 45 Kinder");
    expectPlainPill(screen.getByText("Überbucht"), "Überbucht");
  });
});

describe("RoomStatusBadge without decorative dot", () => {
  it.each([
    [true, "Belegt"],
    [false, "Frei"],
  ])("renders occupied=%s as a plain pill", (isOccupied, label) => {
    render(<RoomStatusBadge isOccupied={isOccupied} />);
    expectPlainPill(screen.getByText(label), label);
  });
});

describe("VisibilityBadge without decorative dot", () => {
  it("carries its eye icon and label, and no dot", () => {
    render(<VisibilityBadge hint="Hinweis" />);
    const pill = screen.getByText("Für Eltern sichtbar");
    expect(pill.textContent).toBe("Für Eltern sichtbar");
    expect(Array.from(pill.children).map((child) => child.tagName)).toEqual([
      "svg",
    ]);
  });
});
