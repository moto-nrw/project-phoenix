import { describe, it, expect, vi } from "vitest";
import { render, fireEvent, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SettingsTimeField } from "./time-field";

describe("SettingsTimeField", () => {
  it("renders with value", () => {
    render(<SettingsTimeField value="18:00" onChange={vi.fn()} />);
    const input = screen.getByPlaceholderText("HH:MM") as HTMLInputElement;
    expect(input.value).toBe("18:00");
  });

  it("shows emptyLabel pill when value is empty and emptyLabel set", () => {
    render(
      <SettingsTimeField value="" onChange={vi.fn()} emptyLabel="Jederzeit" />,
    );
    expect(screen.getByText("Jederzeit")).toBeInTheDocument();
  });

  it("shows HH:MM input when value is empty and no emptyLabel", () => {
    render(<SettingsTimeField value="" onChange={vi.fn()} />);
    const input = screen.getByPlaceholderText("HH:MM") as HTMLInputElement;
    expect(input.value).toBe("");
  });

  it("switches to input when emptyLabel pill is clicked", () => {
    render(
      <SettingsTimeField value="" onChange={vi.fn()} emptyLabel="Jederzeit" />,
    );
    fireEvent.click(screen.getByText("Jederzeit"));
    expect(screen.getByPlaceholderText("HH:MM")).toBeInTheDocument();
  });

  it("calls onChange with valid HH:MM", () => {
    const onChange = vi.fn();
    render(<SettingsTimeField value="18:00" onChange={onChange} />);
    const input = screen.getByPlaceholderText("HH:MM");
    fireEvent.change(input, { target: { value: "1630" } });
    expect(onChange).toHaveBeenCalledWith("16:30");
  });

  it("calls onChange with an empty value for optional times", () => {
    const onChange = vi.fn();
    render(
      <SettingsTimeField
        value="18:00"
        onChange={onChange}
        emptyLabel="Jederzeit"
      />,
    );
    const input = screen.getByPlaceholderText("HH:MM");
    fireEvent.change(input, { target: { value: "" } });
    expect(onChange).toHaveBeenCalledWith("");
  });

  it("does not call onChange with incomplete input", () => {
    const onChange = vi.fn();
    render(<SettingsTimeField value="18:00" onChange={onChange} />);
    const input = screen.getByPlaceholderText("HH:MM");
    fireEvent.change(input, { target: { value: "16" } });
    expect(onChange).not.toHaveBeenCalled();
  });

  it("does not call onChange with invalid time", () => {
    const onChange = vi.fn();
    render(<SettingsTimeField value="18:00" onChange={onChange} />);
    const input = screen.getByPlaceholderText("HH:MM");
    fireEvent.change(input, { target: { value: "2560" } });
    expect(onChange).not.toHaveBeenCalled();
  });

  it("reverts to last valid value on blur when incomplete", () => {
    render(<SettingsTimeField value="18:00" onChange={vi.fn()} />);
    const input = screen.getByPlaceholderText("HH:MM") as HTMLInputElement;
    fireEvent.change(input, { target: { value: "16" } });
    fireEvent.blur(input);
    expect(input.value).toBe("18:00");
  });

  it("calls onBlur", () => {
    const onBlur = vi.fn();
    render(
      <SettingsTimeField value="18:00" onChange={vi.fn()} onBlur={onBlur} />,
    );
    const input = screen.getByPlaceholderText("HH:MM");
    fireEvent.blur(input);
    expect(onBlur).toHaveBeenCalled();
  });

  it("disables emptyLabel pill when disabled", () => {
    render(
      <SettingsTimeField
        value=""
        onChange={vi.fn()}
        emptyLabel="Jederzeit"
        disabled
      />,
    );
    expect(screen.getByText("Jederzeit")).toBeDisabled();
  });

  it("auto-inserts colon when typing digits", () => {
    render(<SettingsTimeField value="12:00" onChange={vi.fn()} />);
    const input = screen.getByPlaceholderText("HH:MM") as HTMLInputElement;
    fireEvent.change(input, { target: { value: "1" } });
    fireEvent.change(input, { target: { value: "18" } });
    fireEvent.change(input, { target: { value: "183" } });
    expect(input.value).toBe("18:3");
  });

  it.each([
    ["100", "01:00"],
    ["130", "01:30"],
    ["230", "02:30"],
    ["1530", "15:30"],
  ])("completes sequential digits %s as %s", async (digits, expected) => {
    const user = userEvent.setup();
    render(<SettingsTimeField value="" onChange={vi.fn()} />);

    const input = screen.getByPlaceholderText("HH:MM");
    await user.type(input, digits);

    expect(input).toHaveValue(expected);
  });

  it("keeps a completed time when another digit is typed", async () => {
    const user = userEvent.setup();
    render(<SettingsTimeField value="12:34" onChange={vi.fn()} />);

    const input = screen.getByPlaceholderText("HH:MM");
    await user.type(input, "5");

    expect(input).toHaveValue("12:34");
  });

  it("does not continue a one-digit-hour entry after leaving the field", async () => {
    const user = userEvent.setup();
    render(<SettingsTimeField value="" onChange={vi.fn()} />);

    const input = screen.getByPlaceholderText("HH:MM") as HTMLInputElement;
    await user.type(input, "130");
    expect(input).toHaveValue("01:30");

    fireEvent.blur(input);
    input.focus();
    input.setSelectionRange(input.value.length, input.value.length);
    await user.type(input, "5");

    expect(input).toHaveValue("01:30");
  });

  it("strips non-digit characters", () => {
    render(<SettingsTimeField value="12:00" onChange={vi.fn()} />);
    const input = screen.getByPlaceholderText("HH:MM") as HTMLInputElement;
    fireEvent.change(input, { target: { value: "1a8b:3c0" } });
    expect(input.value).toBe("18:30");
  });

  it("blurs input on Enter key", () => {
    const onBlur = vi.fn();
    render(
      <SettingsTimeField value="18:00" onChange={vi.fn()} onBlur={onBlur} />,
    );
    const input = screen.getByPlaceholderText("HH:MM") as HTMLInputElement;
    input.focus();
    fireEvent.keyDown(input, { key: "Enter" });
    expect(onBlur).toHaveBeenCalled();
  });

  it("reverts to value and exits editing when blur with invalid time on empty value", () => {
    const onBlur = vi.fn();
    render(
      <SettingsTimeField
        value=""
        onChange={vi.fn()}
        onBlur={onBlur}
        emptyLabel="Jederzeit"
      />,
    );
    // Switch to input mode by clicking the pill
    fireEvent.click(screen.getByText("Jederzeit"));
    const input = screen.getByPlaceholderText("HH:MM") as HTMLInputElement;
    // Type an invalid time (hours > 23)
    fireEvent.change(input, { target: { value: "2500" } });
    fireEvent.blur(input);
    // Should revert to empty and exit editing (show pill again)
    expect(screen.getByText("Jederzeit")).toBeInTheDocument();
  });

  it("reverts display to value on blur with out-of-range time", () => {
    render(<SettingsTimeField value="18:00" onChange={vi.fn()} />);
    const input = screen.getByPlaceholderText("HH:MM") as HTMLInputElement;
    // Type a time with hours > 23
    fireEvent.change(input, { target: { value: "2500" } });
    fireEvent.blur(input);
    expect(input.value).toBe("18:00");
  });

  it("hides edit icon when disabled with emptyLabel", () => {
    const { container } = render(
      <SettingsTimeField
        value=""
        onChange={vi.fn()}
        emptyLabel="Jederzeit"
        disabled
      />,
    );
    // Should not have the SVG pencil icon when disabled
    expect(container.querySelector("svg")).toBeNull();
  });
});
