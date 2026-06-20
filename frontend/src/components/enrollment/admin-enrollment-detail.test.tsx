import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  type AdminRequestChild,
  type AdminRequestSchemaField,
} from "~/lib/enrollment-admin-api";
import { ChildExtraFields } from "./admin-enrollment-detail";
import { formatCustomValue } from "~/lib/enrollment-custom-value-format";

const mocks = vi.hoisted(() => ({
  listCareOfferings: vi.fn(),
  listAdminChildOfferingAdjustments: vi.fn(),
  updateAdminChildOfferings: vi.fn(),
}));

vi.mock("~/lib/care-offering-api", () => ({
  listCareOfferings: mocks.listCareOfferings,
}));

vi.mock("~/lib/enrollment-admin-api", async (importOriginal) => {
  const actual = (await importOriginal()) as Record<string, unknown>;
  return {
    ...actual,
    listAdminChildOfferingAdjustments: mocks.listAdminChildOfferingAdjustments,
    updateAdminChildOfferings: mocks.updateAdminChildOfferings,
  };
});

import {
  ChildOfferingAdjustment,
  ChildOfferings,
} from "./admin-enrollment-detail";

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

beforeEach(() => {
  mocks.listCareOfferings.mockReset();
  mocks.listAdminChildOfferingAdjustments.mockReset();
  mocks.updateAdminChildOfferings.mockReset();
  mocks.listAdminChildOfferingAdjustments.mockResolvedValue([]);
});

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

  // #1694: the accompanied ("Mit anderem Kind") mode was added to the parent
  // form but the admin formatter only knew alone/bus/pickup, so a multi_mode
  // answer rendered the raw "accompanied" token to staff.
  it("renders the accompanied mode label for a weekday_multi_mode value", () => {
    const value = { mon: ["accompanied"], wed: ["bus", "accompanied"] };
    expect(formatCustomValue(value, field("weekday_multi_mode"))).toBe(
      "Mo: geht mit anderem Kind; Mi: fährt Bus, geht mit anderem Kind",
    );
  });

  // #1694 regression: a weekday_mode answer of all-accompanied days used to be
  // swallowed by the bus/pickup-only filter and rendered as the "Geht immer
  // alleine" fallback — actively WRONG info (the child never goes alone).
  it("renders accompanied days for a weekday_mode value, not 'Geht immer alleine'", () => {
    const value = { mon: "accompanied", tue: "accompanied", wed: "bus" };
    expect(formatCustomValue(value, field("weekday_mode"))).toBe(
      "Mo: geht mit anderem Kind, Di: geht mit anderem Kind, Mi: fährt Bus",
    );
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

describe("ChildOfferingAdjustment", () => {
  it("blocks saving when the care-offering catalog failed to load", async () => {
    mocks.listCareOfferings.mockRejectedValue(new Error("Katalog kaputt"));

    render(
      <ChildOfferingAdjustment
        requestId="request-1"
        phaseId="phase-1"
        onSaved={vi.fn()}
        child={{
          id: "child-1",
          first_name: "Lina",
          last_name: "Kind",
          date_of_birth: "2018-01-01",
          status: "approved",
          activation_mode: "scheduled",
          offerings: [
            {
              offering_id: "offering-1",
              offering_name: "Ganztag",
              days_of_week_mode: "parent_choice",
              selected_days: ["mon"],
              manual_selected_days: ["mon"],
              available_days: ["mon", "tue"],
            },
          ],
        }}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Katalog kaputt",
    );
    const save = screen.getByRole("button", { name: "Speichern" });
    expect(save).toBeDisabled();

    fireEvent.click(save);
    await waitFor(() => {
      expect(mocks.updateAdminChildOfferings).not.toHaveBeenCalled();
    });
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
