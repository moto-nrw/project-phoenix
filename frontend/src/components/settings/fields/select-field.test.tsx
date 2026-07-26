import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { SelectField } from "./select-field";

const options = [
  { label: "Deutsch", value: "de" },
  { label: "English", value: "en" },
  { label: "Türkçe", value: "tr" },
];

describe("SelectField", () => {
  it("renders options", () => {
    render(<SelectField value="de" onChange={vi.fn()} options={options} />);
    const trigger = screen.getByRole("combobox");
    expect(trigger).toHaveTextContent("Deutsch");
    fireEvent.click(trigger);
    expect(screen.getAllByRole("option")).toHaveLength(3);
  });

  it("calls onChange with selected value", () => {
    const onChange = vi.fn();
    render(<SelectField value="de" onChange={onChange} options={options} />);
    fireEvent.click(screen.getByRole("combobox"));
    fireEvent.click(screen.getByRole("option", { name: "English" }));
    expect(onChange).toHaveBeenCalledWith("en");
  });

  it("is disabled when disabled prop is set", () => {
    render(
      <SelectField value="de" onChange={vi.fn()} options={options} disabled />,
    );
    expect(screen.getByRole("combobox")).toBeDisabled();
  });
});
