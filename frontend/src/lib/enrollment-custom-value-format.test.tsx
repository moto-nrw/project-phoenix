import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

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
});
