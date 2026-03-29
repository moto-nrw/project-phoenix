import { describe, it, expect, vi } from "vitest";
import { render, fireEvent } from "@testing-library/react";
import { SelectField } from "./select-field";

const options = [
  { label: "Deutsch", value: "de" },
  { label: "English", value: "en" },
  { label: "Türkçe", value: "tr" },
];

describe("SelectField", () => {
  it("renders options", () => {
    const { container } = render(
      <SelectField value="de" onChange={vi.fn()} options={options} />,
    );
    const select = container.querySelector("select") as HTMLSelectElement;
    expect(select.options).toHaveLength(3);
    expect(select.value).toBe("de");
  });

  it("calls onChange with selected value", () => {
    const onChange = vi.fn();
    const { container } = render(
      <SelectField value="de" onChange={onChange} options={options} />,
    );
    const select = container.querySelector("select") as HTMLSelectElement;
    fireEvent.change(select, { target: { value: "en" } });
    expect(onChange).toHaveBeenCalledWith("en");
  });

  it("is disabled when disabled prop is set", () => {
    const { container } = render(
      <SelectField value="de" onChange={vi.fn()} options={options} disabled />,
    );
    expect(
      (container.querySelector("select") as HTMLSelectElement).disabled,
    ).toBe(true);
  });
});
