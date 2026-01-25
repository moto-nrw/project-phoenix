import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { SettingInput, SettingLoadingSpinner } from "./setting-input";
import type { ResolvedSetting } from "~/lib/settings-helpers";

// Helper to create a mock setting
function createMockSetting(
  overrides: Partial<ResolvedSetting> = {},
): ResolvedSetting {
  return {
    key: "test.setting",
    value: "test-value",
    type: "string",
    category: "test",
    isDefault: true,
    isActive: true,
    canModify: true,
    ...overrides,
  };
}

describe("SettingInput", () => {
  describe("boolean type", () => {
    it("renders checkbox for boolean setting", () => {
      const onChange = vi.fn();
      const setting = createMockSetting({ type: "bool", value: false });

      render(
        <SettingInput
          setting={setting}
          isSaving={false}
          isDisabled={false}
          onChange={onChange}
        />,
      );

      const checkbox = screen.getByRole("checkbox");
      expect(checkbox).toBeInTheDocument();
      expect(checkbox).not.toBeChecked();
    });

    it("renders checked checkbox when value is true", () => {
      const onChange = vi.fn();
      const setting = createMockSetting({ type: "bool", value: true });

      render(
        <SettingInput
          setting={setting}
          isSaving={false}
          isDisabled={false}
          onChange={onChange}
        />,
      );

      expect(screen.getByRole("checkbox")).toBeChecked();
    });

    it("calls onChange with boolean value when toggled", () => {
      const onChange = vi.fn();
      const setting = createMockSetting({ type: "bool", value: false });

      render(
        <SettingInput
          setting={setting}
          isSaving={false}
          isDisabled={false}
          onChange={onChange}
        />,
      );

      fireEvent.click(screen.getByRole("checkbox"));

      expect(onChange).toHaveBeenCalledWith("test.setting", true);
    });

    it("shows saving indicator when isSaving is true", () => {
      const onChange = vi.fn();
      const setting = createMockSetting({ type: "bool", value: false });

      render(
        <SettingInput
          setting={setting}
          isSaving={true}
          isDisabled={false}
          onChange={onChange}
        />,
      );

      expect(screen.getByText("Speichern...")).toBeInTheDocument();
    });

    it("disables checkbox when isDisabled is true", () => {
      const onChange = vi.fn();
      const setting = createMockSetting({ type: "bool", value: false });

      render(
        <SettingInput
          setting={setting}
          isSaving={false}
          isDisabled={true}
          onChange={onChange}
        />,
      );

      expect(screen.getByRole("checkbox")).toBeDisabled();
    });
  });

  describe("int type", () => {
    it("renders number input for int setting", () => {
      const onChange = vi.fn();
      const setting = createMockSetting({ type: "int", value: 42 });

      render(
        <SettingInput
          setting={setting}
          isSaving={false}
          isDisabled={false}
          onChange={onChange}
        />,
      );

      const input = screen.getByRole("spinbutton");
      expect(input).toBeInTheDocument();
      expect(input).toHaveValue(42);
    });

    it("calls onChange with parsed integer value", () => {
      const onChange = vi.fn();
      const setting = createMockSetting({ type: "int", value: 10 });

      render(
        <SettingInput
          setting={setting}
          isSaving={false}
          isDisabled={false}
          onChange={onChange}
        />,
      );

      fireEvent.change(screen.getByRole("spinbutton"), {
        target: { value: "25" },
      });

      expect(onChange).toHaveBeenCalledWith("test.setting", 25);
    });

    it("does not call onChange for invalid number", () => {
      const onChange = vi.fn();
      const setting = createMockSetting({ type: "int", value: 10 });

      render(
        <SettingInput
          setting={setting}
          isSaving={false}
          isDisabled={false}
          onChange={onChange}
        />,
      );

      fireEvent.change(screen.getByRole("spinbutton"), {
        target: { value: "abc" },
      });

      expect(onChange).not.toHaveBeenCalled();
    });

    it("respects min and max validation", () => {
      const onChange = vi.fn();
      const setting = createMockSetting({
        type: "int",
        value: 50,
        validation: { min: 1, max: 100 },
      });

      render(
        <SettingInput
          setting={setting}
          isSaving={false}
          isDisabled={false}
          onChange={onChange}
        />,
      );

      const input = screen.getByRole("spinbutton");
      expect(input).toHaveAttribute("min", "1");
      expect(input).toHaveAttribute("max", "100");
    });

    it("shows saving indicator when isSaving is true", () => {
      const onChange = vi.fn();
      const setting = createMockSetting({ type: "int", value: 10 });

      render(
        <SettingInput
          setting={setting}
          isSaving={true}
          isDisabled={false}
          onChange={onChange}
        />,
      );

      expect(screen.getByText("Speichern...")).toBeInTheDocument();
    });

    it("disables input when isDisabled is true", () => {
      const onChange = vi.fn();
      const setting = createMockSetting({ type: "int", value: 10 });

      render(
        <SettingInput
          setting={setting}
          isSaving={false}
          isDisabled={true}
          onChange={onChange}
        />,
      );

      expect(screen.getByRole("spinbutton")).toBeDisabled();
    });
  });

  describe("enum type", () => {
    it("renders select for enum setting", () => {
      const onChange = vi.fn();
      const setting = createMockSetting({
        type: "enum",
        value: "option1",
        validation: { options: ["option1", "option2", "option3"] },
      });

      render(
        <SettingInput
          setting={setting}
          isSaving={false}
          isDisabled={false}
          onChange={onChange}
        />,
      );

      const select = screen.getByRole("combobox");
      expect(select).toBeInTheDocument();
      expect(select).toHaveValue("option1");
    });

    it("renders all options", () => {
      const onChange = vi.fn();
      const setting = createMockSetting({
        type: "enum",
        value: "option1",
        validation: { options: ["option1", "option2", "option3"] },
      });

      render(
        <SettingInput
          setting={setting}
          isSaving={false}
          isDisabled={false}
          onChange={onChange}
        />,
      );

      const options = screen.getAllByRole("option");
      expect(options).toHaveLength(3);
      expect(options[0]).toHaveTextContent("option1");
      expect(options[1]).toHaveTextContent("option2");
      expect(options[2]).toHaveTextContent("option3");
    });

    it("calls onChange with selected value", () => {
      const onChange = vi.fn();
      const setting = createMockSetting({
        type: "enum",
        value: "option1",
        validation: { options: ["option1", "option2", "option3"] },
      });

      render(
        <SettingInput
          setting={setting}
          isSaving={false}
          isDisabled={false}
          onChange={onChange}
        />,
      );

      fireEvent.change(screen.getByRole("combobox"), {
        target: { value: "option2" },
      });

      expect(onChange).toHaveBeenCalledWith("test.setting", "option2");
    });

    it("shows saving indicator when isSaving is true", () => {
      const onChange = vi.fn();
      const setting = createMockSetting({
        type: "enum",
        value: "option1",
        validation: { options: ["option1"] },
      });

      render(
        <SettingInput
          setting={setting}
          isSaving={true}
          isDisabled={false}
          onChange={onChange}
        />,
      );

      expect(screen.getByText("Speichern...")).toBeInTheDocument();
    });

    it("disables select when isDisabled is true", () => {
      const onChange = vi.fn();
      const setting = createMockSetting({
        type: "enum",
        value: "option1",
        validation: { options: ["option1"] },
      });

      render(
        <SettingInput
          setting={setting}
          isSaving={false}
          isDisabled={true}
          onChange={onChange}
        />,
      );

      expect(screen.getByRole("combobox")).toBeDisabled();
    });
  });

  describe("time type", () => {
    it("renders time input for time setting", () => {
      const onChange = vi.fn();
      const setting = createMockSetting({ type: "time", value: "14:30" });

      render(
        <SettingInput
          setting={setting}
          isSaving={false}
          isDisabled={false}
          onChange={onChange}
        />,
      );

      const input = screen.getByDisplayValue("14:30");
      expect(input).toBeInTheDocument();
      expect(input).toHaveAttribute("type", "time");
    });

    it("calls onChange with time value", () => {
      const onChange = vi.fn();
      const setting = createMockSetting({ type: "time", value: "14:30" });

      render(
        <SettingInput
          setting={setting}
          isSaving={false}
          isDisabled={false}
          onChange={onChange}
        />,
      );

      fireEvent.change(screen.getByDisplayValue("14:30"), {
        target: { value: "16:00" },
      });

      expect(onChange).toHaveBeenCalledWith("test.setting", "16:00");
    });

    it("shows saving indicator when isSaving is true", () => {
      const onChange = vi.fn();
      const setting = createMockSetting({ type: "time", value: "14:30" });

      render(
        <SettingInput
          setting={setting}
          isSaving={true}
          isDisabled={false}
          onChange={onChange}
        />,
      );

      expect(screen.getByText("Speichern...")).toBeInTheDocument();
    });

    it("disables input when isDisabled is true", () => {
      const onChange = vi.fn();
      const setting = createMockSetting({ type: "time", value: "14:30" });

      render(
        <SettingInput
          setting={setting}
          isSaving={false}
          isDisabled={true}
          onChange={onChange}
        />,
      );

      expect(screen.getByDisplayValue("14:30")).toBeDisabled();
    });
  });

  describe("string type (default)", () => {
    it("renders text input for string setting", () => {
      const onChange = vi.fn();
      const setting = createMockSetting({ type: "string", value: "hello" });

      render(
        <SettingInput
          setting={setting}
          isSaving={false}
          isDisabled={false}
          onChange={onChange}
        />,
      );

      const input = screen.getByRole("textbox");
      expect(input).toBeInTheDocument();
      expect(input).toHaveValue("hello");
    });

    it("calls onChange with string value", () => {
      const onChange = vi.fn();
      const setting = createMockSetting({ type: "string", value: "hello" });

      render(
        <SettingInput
          setting={setting}
          isSaving={false}
          isDisabled={false}
          onChange={onChange}
        />,
      );

      fireEvent.change(screen.getByRole("textbox"), {
        target: { value: "world" },
      });

      expect(onChange).toHaveBeenCalledWith("test.setting", "world");
    });

    it("uses pattern validation when provided", () => {
      const onChange = vi.fn();
      const setting = createMockSetting({
        type: "string",
        value: "test@example.com",
        validation: { pattern: "^[^@]+@[^@]+$" },
      });

      render(
        <SettingInput
          setting={setting}
          isSaving={false}
          isDisabled={false}
          onChange={onChange}
        />,
      );

      const input = screen.getByRole("textbox");
      expect(input).toHaveAttribute("pattern", "^[^@]+@[^@]+$");
    });

    it("shows saving indicator when isSaving is true", () => {
      const onChange = vi.fn();
      const setting = createMockSetting({ type: "string", value: "hello" });

      render(
        <SettingInput
          setting={setting}
          isSaving={true}
          isDisabled={false}
          onChange={onChange}
        />,
      );

      expect(screen.getByText("Speichern...")).toBeInTheDocument();
    });

    it("disables input when isDisabled is true", () => {
      const onChange = vi.fn();
      const setting = createMockSetting({ type: "string", value: "hello" });

      render(
        <SettingInput
          setting={setting}
          isSaving={false}
          isDisabled={true}
          onChange={onChange}
        />,
      );

      expect(screen.getByRole("textbox")).toBeDisabled();
    });

    it("handles unknown type as string", () => {
      const onChange = vi.fn();
      // Force an unknown type to test default case
      const setting = createMockSetting({
        type: "unknown" as ResolvedSetting["type"],
        value: "test",
      });

      render(
        <SettingInput
          setting={setting}
          isSaving={false}
          isDisabled={false}
          onChange={onChange}
        />,
      );

      expect(screen.getByRole("textbox")).toBeInTheDocument();
    });
  });

  describe("color variants", () => {
    it("applies gray variant styles by default", () => {
      const onChange = vi.fn();
      const setting = createMockSetting({ type: "bool", value: false });

      const { container } = render(
        <SettingInput
          setting={setting}
          isSaving={false}
          isDisabled={false}
          onChange={onChange}
        />,
      );

      // Check that gray variant classes are applied
      const toggle = container.querySelector(".peer-checked\\:bg-gray-900");
      expect(toggle).toBeInTheDocument();
    });

    it("applies green variant styles", () => {
      const onChange = vi.fn();
      const setting = createMockSetting({ type: "bool", value: false });

      const { container } = render(
        <SettingInput
          setting={setting}
          isSaving={false}
          isDisabled={false}
          variant="green"
          onChange={onChange}
        />,
      );

      const toggle = container.querySelector(".peer-checked\\:bg-green-600");
      expect(toggle).toBeInTheDocument();
    });

    it("applies purple variant styles", () => {
      const onChange = vi.fn();
      const setting = createMockSetting({ type: "bool", value: false });

      const { container } = render(
        <SettingInput
          setting={setting}
          isSaving={false}
          isDisabled={false}
          variant="purple"
          onChange={onChange}
        />,
      );

      const toggle = container.querySelector(".peer-checked\\:bg-purple-600");
      expect(toggle).toBeInTheDocument();
    });

    it("applies variant focus styles to int input", () => {
      const onChange = vi.fn();
      const setting = createMockSetting({ type: "int", value: 10 });

      const { container } = render(
        <SettingInput
          setting={setting}
          isSaving={false}
          isDisabled={false}
          variant="green"
          onChange={onChange}
        />,
      );

      const input = container.querySelector(".focus\\:ring-green-300");
      expect(input).toBeInTheDocument();
    });

    it("applies variant focus styles to enum select", () => {
      const onChange = vi.fn();
      const setting = createMockSetting({
        type: "enum",
        value: "opt1",
        validation: { options: ["opt1"] },
      });

      const { container } = render(
        <SettingInput
          setting={setting}
          isSaving={false}
          isDisabled={false}
          variant="purple"
          onChange={onChange}
        />,
      );

      const select = container.querySelector(".focus\\:ring-purple-300");
      expect(select).toBeInTheDocument();
    });
  });

  describe("edge cases", () => {
    it("handles null value in boolean setting", () => {
      const onChange = vi.fn();
      const setting = createMockSetting({ type: "bool", value: null });

      render(
        <SettingInput
          setting={setting}
          isSaving={false}
          isDisabled={false}
          onChange={onChange}
        />,
      );

      expect(screen.getByRole("checkbox")).not.toBeChecked();
    });

    it("handles undefined value in string setting", () => {
      const onChange = vi.fn();
      const setting = createMockSetting({ type: "string", value: undefined });

      render(
        <SettingInput
          setting={setting}
          isSaving={false}
          isDisabled={false}
          onChange={onChange}
        />,
      );

      expect(screen.getByRole("textbox")).toHaveValue("");
    });

    it("handles empty options array in enum", () => {
      const onChange = vi.fn();
      const setting = createMockSetting({
        type: "enum",
        value: "",
        validation: { options: [] },
      });

      render(
        <SettingInput
          setting={setting}
          isSaving={false}
          isDisabled={false}
          onChange={onChange}
        />,
      );

      const options = screen.queryAllByRole("option");
      expect(options).toHaveLength(0);
    });

    it("handles missing validation in enum", () => {
      const onChange = vi.fn();
      const setting = createMockSetting({
        type: "enum",
        value: "test",
        validation: undefined,
      });

      render(
        <SettingInput
          setting={setting}
          isSaving={false}
          isDisabled={false}
          onChange={onChange}
        />,
      );

      expect(screen.getByRole("combobox")).toBeInTheDocument();
    });
  });
});

describe("SettingLoadingSpinner", () => {
  it("renders spinner with default gray variant", () => {
    const { container } = render(<SettingLoadingSpinner />);

    const spinner = container.querySelector(".border-t-gray-900");
    expect(spinner).toBeInTheDocument();
  });

  it("renders spinner with green variant", () => {
    const { container } = render(<SettingLoadingSpinner variant="green" />);

    const spinner = container.querySelector(".border-t-green-600");
    expect(spinner).toBeInTheDocument();
  });

  it("renders spinner with purple variant", () => {
    const { container } = render(<SettingLoadingSpinner variant="purple" />);

    const spinner = container.querySelector(".border-t-purple-600");
    expect(spinner).toBeInTheDocument();
  });

  it("has spin animation class", () => {
    const { container } = render(<SettingLoadingSpinner />);

    const spinner = container.querySelector(".animate-spin");
    expect(spinner).toBeInTheDocument();
  });
});
