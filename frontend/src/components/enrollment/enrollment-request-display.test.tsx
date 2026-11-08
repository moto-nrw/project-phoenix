import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type {
  AdminRequestChild,
  AdminRequestSchemaField,
} from "~/lib/enrollment-admin-api";
import {
  ChildExtraFields,
  ChildOfferings,
  formatPlainDate,
} from "./enrollment-request-display";

describe("formatPlainDate", () => {
  it("preserves a date-only birthday east of Berlin", () => {
    const originalTZ = process.env.TZ;
    process.env.TZ = "Asia/Tokyo";
    try {
      expect(formatPlainDate("2015-06-15")).toBe("15.06.2015");
    } finally {
      process.env.TZ = originalTZ;
    }
  });
});

describe("ChildOfferings", () => {
  it("renders mixed automatic and manual day provenance", () => {
    render(
      <ChildOfferings
        offerings={[
          {
            offering_id: "22",
            offering_name: "Randstunde",
            days_of_week_mode: "parent_choice",
            selected_days: ["mon", "tue", "wed", "thu", "fri"],
            manual_selected_days: ["fri"],
            automatic_selected_days: ["mon", "tue", "wed", "thu"],
            available_days: ["mon", "tue", "wed", "thu", "fri"],
          },
        ]}
      />,
    );

    expect(screen.getByText("Randstunde")).toBeVisible();
    expect(
      screen.getByText(
        "Mo, Di, Mi, Do automatisch mitgebucht; Fr von Eltern gewählt",
      ),
    ).toBeVisible();
  });

  // #2185: staff must read the same attributes the parents app shows for the
  // same booking, worded the same way, or a guardian's question is
  // unanswerable from the staff view.
  it("renders the offering attributes the parents app shows", () => {
    render(
      <ChildOfferings
        offerings={[
          {
            offering_id: "22",
            offering_name: "Ganztagsbetreuung",
            days_of_week_mode: "fixed",
            available_days: ["mon", "tue"],
            includes_lunch: true,
            includes_holiday_care: true,
            price_cents: 8550,
            // Wire value is already the inclusive last covered day.
            valid_until: "2027-07-30",
          },
        ]}
        phaseName="Schuljahr 2026/2027"
      />,
    );

    expect(screen.getByText("mit Mittagessen")).toBeVisible();
    expect(screen.getByText("mit Ferienbetreuung")).toBeVisible();
    expect(screen.getByText("85,50 € pro Monat")).toBeVisible();
    expect(screen.getByText("bis 30.07.2027")).toBeVisible();
  });

  it("omits attribute pills the offering does not carry", () => {
    render(
      <ChildOfferings
        offerings={[
          {
            offering_id: "22",
            offering_name: "Frühbetreuung",
            days_of_week_mode: "fixed",
            available_days: ["mon"],
            includes_lunch: false,
            includes_holiday_care: false,
          },
        ]}
      />,
    );

    // Anker: das Angebot steht als Zeile da, an der die Merkmale stünden.
    expect(screen.getByText("Frühbetreuung")).toBeVisible();
    expect(screen.queryByText("mit Mittagessen")).toBeNull();
    expect(screen.queryByText("mit Ferienbetreuung")).toBeNull();
    expect(screen.queryByText(/pro Monat/)).toBeNull();
  });

  it("marks a booking that only takes effect later", () => {
    render(
      <ChildOfferings
        offerings={[
          {
            offering_id: "22",
            offering_name: "Ganztagsbetreuung",
            days_of_week_mode: "fixed",
            available_days: ["mon"],
            valid_from: "2026-09-01",
            starts_later: true,
          },
        ]}
      />,
    );

    expect(screen.getByText("gilt ab 01.09.2026")).toBeVisible();
  });

  it("names the phase the attributes are configured in", () => {
    render(
      <ChildOfferings
        offerings={[
          {
            offering_id: "22",
            offering_name: "Ganztagsbetreuung",
            days_of_week_mode: "fixed",
            available_days: ["mon"],
          },
        ]}
        phaseName="Schuljahr 2026/2027"
      />,
    );

    expect(
      screen.getByText(/Anmeldephase „Schuljahr 2026\/2027“/),
    ).toBeVisible();
  });

  // #2185: staff decide on this screen. A failed lookup renders no block at
  // all, which reads as "the family booked nothing" — and the decision gets
  // made on that. The warning has to reach children of every status, not
  // only approved ones (where the correction panel lives).
  it("warns instead of staying silent when the lookup failed", () => {
    render(<ChildOfferings unavailable />);

    expect(screen.getByText(/konnten nicht geladen werden/)).toBeVisible();
  });

  it("renders nothing for a child that simply has no bookings", () => {
    const { container } = render(<ChildOfferings offerings={[]} />);

    expect(container).toBeEmptyDOMElement();
  });
});

describe("ChildExtraFields companion note (#1694)", () => {
  function child(custom: Record<string, unknown>): AdminRequestChild {
    return {
      id: "1",
      first_name: "Max",
      last_name: "Muster",
      date_of_birth: "2018-05-01",
      status: "submitted",
      activation_mode: "immediate",
      custom_data: custom,
    };
  }

  const departureField: AdminRequestSchemaField = {
    key: "student.allowed_departure_modes",
    label: "Heimweg",
    type: "weekday_multi_mode",
    applies_to_child: true,
    target: "student.allowed_departure_modes",
  };

  // The companion note rides on a reserved key (not a schema field), so the
  // schema-field loop never emits it. Staff must still see it before approving,
  // because the backend persists it onto the student on approval.
  it("renders the reserved companion note even without a matching schema field", () => {
    const { container } = render(
      <ChildExtraFields
        child={child({ "student.departure_companion_note": "Geschwisterkind" })}
        schemaFields={[departureField]}
      />,
    );
    expect(container.textContent).toContain("Mit welchem Kind?");
    expect(container.textContent).toContain("Geschwisterkind");
  });

  it("renders nothing when neither schema fields nor a companion note are present", () => {
    const { container } = render(
      <ChildExtraFields child={child({})} schemaFields={[departureField]} />,
    );
    expect(container.firstChild).toBeNull();
  });
});
