import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { DataTableStatusBadge } from "./data-table";
import { LocationBadge } from "./location-badge";
import { PresenceBadge } from "./presence-badge";
import { StatusBadge } from "./status-badge";
import { StatusColorBadge } from "./status-color-badge";
import {
  getAccessibleTextColor,
  LOCATION_COLORS,
  LOCATION_STATUSES,
} from "~/lib/location-helper";

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
        "not_arrival_today",
        "data-not-arrival-indicator",
        LOCATION_STATUSES.UNPLANNED_PRESENT,
      ],
    ] as const)(
      "renders the %s overlay pill as plain text",
      (flag, attr, label) => {
        const { container } = render(
          <LocationBadge
            student={{ current_location: "Anwesend - Raum 101", [flag]: true }}
            displayMode="roomName"
            variant={variant}
          />,
        );
        expectPlainPill(container.querySelector(`[${attr}]`), label);
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
        "not_arrival_today",
        "data-not-arrival-indicator",
        LOCATION_STATUSES.UNPLANNED_PRESENT,
      ],
    ] as const)(
      "renders the %s overlay pill as plain text",
      (flag, attr, label) => {
        const { container } = render(
          <PresenceBadge
            student={{ current_location: "Anwesend", [flag]: true }}
            variant={variant}
          />,
        );
        expectPlainPill(container.querySelector(`[${attr}]`), label);
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
  it("StatusBadge renders every tone as plain text", () => {
    for (const tone of ["blue", "green", "orange", "red", "gray"] as const) {
      const view = render(<StatusBadge label={`Ton ${tone}`} tone={tone} />);
      expectPlainPill(screen.getByText(`Ton ${tone}`), `Ton ${tone}`);
      view.unmount();
    }
  });

  it("StatusColorBadge renders a palette hex and a custom room hex as plain text", () => {
    const palette = render(
      <StatusColorBadge label="Krank" color={LOCATION_COLORS.SICK} />,
    );
    expectPlainPill(screen.getByText("Krank"), "Krank");
    palette.unmount();

    render(<StatusColorBadge label="Bauraum" color="#A3D977" />);
    expectPlainPill(screen.getByText("Bauraum"), "Bauraum");
  });

  it.each([
    ["Aktiv", { active: true }, LOCATION_COLORS.GROUP_ROOM],
    ["Inaktiv", { active: false }, LOCATION_COLORS.DANGER],
    ["Unbekannt", { active: false, unknown: true }, LOCATION_COLORS.UNKNOWN],
  ] as const)(
    "DataTableStatusBadge renders %s as plain text with accessible color",
    (label, props, color) => {
      const view = render(<DataTableStatusBadge {...props} />);
      const pill = screen.getByText(label);
      expectPlainPill(pill, label);
      expect(pill.style.color).toBe(getAccessibleTextColor(color, "#F9FAFB"));
      view.unmount();
    },
  );
});
