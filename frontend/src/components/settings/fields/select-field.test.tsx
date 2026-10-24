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

  describe("multiple", () => {
    it("shows one checkbox per option, ticked for each chosen value", () => {
      render(
        <SelectField
          ariaLabel="Freigaben"
          value={["en"]}
          onChange={vi.fn()}
          options={options}
          multiple
        />,
      );
      const group = screen.getByRole("group", { name: "Freigaben" });
      expect(group).toBeInTheDocument();
      expect(screen.getAllByRole("checkbox")).toHaveLength(3);
      expect(screen.getByRole("checkbox", { name: "English" })).toBeChecked();
      expect(
        screen.getByRole("checkbox", { name: "Deutsch" }),
      ).not.toBeChecked();
      expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
    });

    it("adds a ticked option in option order", () => {
      const onChange = vi.fn();
      render(
        <SelectField
          value={["tr"]}
          onChange={onChange}
          options={options}
          multiple
        />,
      );
      fireEvent.click(screen.getByRole("checkbox", { name: "Deutsch" }));
      expect(onChange).toHaveBeenCalledExactlyOnceWith(["de", "tr"]);
    });

    it("removes an unticked option and allows an empty list", () => {
      const onChange = vi.fn();
      render(
        <SelectField
          value={["de"]}
          onChange={onChange}
          options={options}
          multiple
        />,
      );
      fireEvent.click(screen.getByRole("checkbox", { name: "Deutsch" }));
      expect(onChange).toHaveBeenCalledExactlyOnceWith([]);
    });

    it("reads a missing value as nothing chosen", () => {
      render(
        <SelectField
          value={null}
          onChange={vi.fn()}
          options={options}
          multiple
        />,
      );
      for (const checkbox of screen.getAllByRole("checkbox")) {
        expect(checkbox).not.toBeChecked();
      }
    });

    it("disables every checkbox when disabled", () => {
      render(
        <SelectField
          value={["de"]}
          onChange={vi.fn()}
          options={options}
          multiple
          disabled
        />,
      );
      for (const checkbox of screen.getAllByRole("checkbox")) {
        expect(checkbox).toBeDisabled();
      }
    });
  });
});
