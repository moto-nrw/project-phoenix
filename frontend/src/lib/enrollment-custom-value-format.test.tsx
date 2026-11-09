import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { AdminRequestSchemaField } from "~/lib/enrollment-admin-api";
import { formatCustomValue } from "./enrollment-custom-value-format";

describe("formatCustomValue", () => {
  it("drops empty values and formats primitive answers", () => {
    expect(formatCustomValue(null)).toBeNull();
    expect(formatCustomValue(undefined)).toBeNull();
    expect(formatCustomValue("   ")).toBeNull();
    expect(formatCustomValue(true)).toBe("Ja");
    expect(formatCustomValue(false)).toBe("Nein");
    expect(formatCustomValue(128)).toBe("128");
    expect(formatCustomValue("Kann schwimmen")).toBe("Kann schwimmen");
  });

  it("uses select option labels when available", () => {
    expect(
      formatCustomValue("yes", {
        type: "select",
        options: [{ label: "Ja", value: "yes" }],
      }),
    ).toBe("Ja");
    expect(
      formatCustomValue("unknown", {
        type: "select",
        options: [{ label: "Ja", value: "yes" }],
      }),
    ).toBe("unknown");
  });

  it("formats weekday mode and multi-mode maps", () => {
    expect(
      formatCustomValue(
        { mon: "alone", tue: "bus", wed: "pickup" },
        { type: "weekday_mode" },
      ),
    ).toBe("Di: fährt Bus, Mi: wird abgeholt");
    expect(formatCustomValue({}, { type: "weekday_mode" })).toBe(
      "Geht immer alleine",
    );
    expect(
      formatCustomValue(
        { mon: ["alone", "bus"], tue: ["pickup"] },
        { type: "weekday_multi_mode" },
      ),
    ).toBe("Mo: geht zu Fuß, fährt Bus; Di: wird abgeholt");
  });

  it("drops inferred weekday mode maps when only alone values are present", () => {
    expect(formatCustomValue({ mon: "alone", tue: "alone" })).toBeNull();
  });

  it("formats weekday boolean maps including pickup-status empty answers", () => {
    expect(
      formatCustomValue(
        { mon: true, tue: false, fri: true },
        { type: "weekday_boolean" },
      ),
    ).toBe("Mo, Fr");
    expect(
      formatCustomValue(
        { mon: false, tue: false },
        { type: "weekday_boolean", target: "student.pickup_status" },
      ),
    ).toBe("Geht alleine nach Hause");
    expect(
      formatCustomValue(
        { mon: false, tue: false },
        { type: "weekday_boolean" },
      ),
    ).toBeNull();
  });

  it("renders generic weekday string maps as inline day cells", () => {
    const { container } = render(
      <>{formatCustomValue({ mon: "07:30", fri: "08:15" })}</>,
    );

    expect(container.textContent).toContain("Mo:");
    expect(container.textContent).toContain("07:30");
    expect(container.textContent).toContain("Fr:");
    expect(container.textContent).toContain("08:15");
    expect(container.textContent).not.toContain("Di:");
  });

  it("formats array entries for pickup contacts and phone rows", () => {
    const { container } = render(
      <>
        {formatCustomValue([
          {
            first_name: "Laura",
            last_name: "Beispiel",
            relationship_type: "Mutter",
            email: "laura@example.com",
            phone_numbers: [{ phone_number: "+49 170 1234567" }],
            can_pickup: true,
            is_emergency_contact: true,
          },
          {
            phone_number: "+49 221 123",
            phone_type: "home",
            is_primary: true,
          },
          { custom: "value" },
          "Freitags früher abholen",
        ])}
      </>,
    );

    expect(container.textContent).toContain("Laura Beispiel (Mutter)");
    expect(container.textContent).toContain("laura@example.com");
    expect(container.textContent).toContain("Abholberechtigt");
    expect(container.textContent).toContain("Notfallkontakt");
    expect(container.textContent).toContain("+49 221 123 (Privat)");
    expect(container.textContent).toContain("(Hauptnummer)");
    expect(container.textContent).toContain('{"custom":"value"}');
    expect(container.textContent).toContain("Freitags früher abholen");
  });

  it("returns null for empty arrays and empty unknown objects", () => {
    expect(formatCustomValue([])).toBeNull();
    expect(formatCustomValue({ custom: true })).toBeNull();
  });

  it("falls back to String for uncommon primitive values", () => {
    expect(formatCustomValue(Symbol("answer"))).toBe("Symbol(answer)");
  });

  it("lists the file names of a file answer instead of raw entries", () => {
    const value = [
      {
        upload_id: "11",
        filename: "steuerbescheid.pdf",
        size_bytes: 1200,
        content_type: "application/pdf",
      },
      {
        token: "tok",
        filename: "seite-2.png",
        size_bytes: 900,
        content_type: "image/png",
      },
    ];
    const { container } = render(
      <>{formatCustomValue(value, { type: "file" })}</>,
    );
    const items = [...container.querySelectorAll("li")].map(
      (item) => item.textContent,
    );
    expect(items).toEqual(["steuerbescheid.pdf", "seite-2.png"]);
    expect(container.textContent).not.toContain("upload_id");

    // Without the field definition (student detail) the shape decides.
    const { container: bare } = render(<>{formatCustomValue(value)}</>);
    expect(bare.textContent).toContain("steuerbescheid.pdf");
    expect(bare.textContent).not.toContain("{");
  });

  it("treats an empty file answer as no answer", () => {
    expect(formatCustomValue([], { type: "file" })).toBeNull();
  });
});

describe("formatCustomValue on the admin review page", () => {
  const field = (
    type: string,
    overrides: Partial<AdminRequestSchemaField> = {},
  ): AdminRequestSchemaField => ({
    key: "student.pickup_status",
    label: "Abholregelung",
    type,
    applies_to_child: true,
    ...overrides,
  });

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
