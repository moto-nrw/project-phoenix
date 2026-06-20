import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { type AdminRequestSchemaField } from "~/lib/enrollment-admin-api";
import { ChildOfferings, formatCustomValue } from "./admin-enrollment-detail";

function field(
  type: string,
  overrides: Partial<AdminRequestSchemaField> = {},
): AdminRequestSchemaField {
  return {
    key: "student.pickup_status",
    label: "Abholregelung",
    type,
    applies_to_child: true,
    ...overrides,
  };
}

describe("formatCustomValue", () => {
  // Regression: weekday_boolean values ({mon: true, tue: false}) used to be
  // filtered out entirely (the object branch only kept non-empty strings), so
  // Abholregelung and Buskind vanished from the admin review page. They must
  // now render the selected days, like the backend export (formatWeekdayBoolean).
  it("renders selected days for a weekday_boolean value", () => {
    const value = {
      mon: true,
      tue: false,
      wed: true,
      thu: false,
      fri: true,
    };
    expect(formatCustomValue(value, field("weekday_boolean"))).toBe(
      "Mo, Mi, Fr",
    );
  });

  it("renders weekday_boolean even when no field metadata is passed", () => {
    const value = { mon: true, tue: true };
    expect(formatCustomValue(value)).toBe("Mo, Di");
  });

  it("returns null when no weekday_boolean day is selected", () => {
    const value = { mon: false, tue: false, wed: false };
    expect(formatCustomValue(value, field("weekday_boolean"))).toBeNull();
  });

  // The reserved student.pickup_status target treats an empty map as a real
  // answer ("Geht alleine nach Hause"), not a missing field. The admin must be
  // able to tell "goes alone every day" apart from an absent field, so the row
  // is rendered with an explicit label instead of being dropped.
  it("renders 'Geht alleine nach Hause' for an all-false pickup_status", () => {
    const value = {
      mon: false,
      tue: false,
      wed: false,
      thu: false,
      fri: false,
    };
    expect(
      formatCustomValue(
        value,
        field("weekday_boolean", { target: "student.pickup_status" }),
      ),
    ).toBe("Geht alleine nach Hause");
  });

  it("renders 'Geht alleine nach Hause' for an explicit empty pickup_status map", () => {
    expect(
      formatCustomValue(
        {},
        field("weekday_boolean", { target: "student.pickup_status" }),
      ),
    ).toBe("Geht alleine nach Hause");
  });

  it("still renders selected days for pickup_status when days are chosen", () => {
    const value = { mon: true, fri: true };
    expect(
      formatCustomValue(
        value,
        field("weekday_boolean", { target: "student.pickup_status" }),
      ),
    ).toBe("Mo, Fr");
  });

  // Buskind is also weekday_boolean but has no special empty-map meaning: no
  // bus days means the child simply is not a bus kid, so the row drops out.
  it("returns null for an all-false non-pickup weekday_boolean (Buskind)", () => {
    const value = { mon: false, tue: false };
    expect(
      formatCustomValue(
        value,
        field("weekday_boolean", {
          key: "student.bus",
          target: "student.bus",
          label: "Buskind",
        }),
      ),
    ).toBeNull();
  });

  it("still renders weekday_schedule time values per day", () => {
    const value = { mon: "07:30", wed: "08:00" };
    const { container } = render(
      <>{formatCustomValue(value, field("weekday_schedule"))}</>,
    );
    expect(container.textContent).toContain("Mo:");
    expect(container.textContent).toContain("07:30");
    expect(container.textContent).toContain("Mi:");
    expect(container.textContent).toContain("08:00");
    // Days without a time must not appear.
    expect(container.textContent).not.toContain("Di:");
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
});
