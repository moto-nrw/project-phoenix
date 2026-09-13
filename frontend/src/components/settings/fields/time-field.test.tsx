import { describe, it, expect, vi } from "vitest";
import { render, fireEvent, screen } from "@testing-library/react";
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
    fireEvent.change(input, { target: { value: "183" } });
    expect(input.value).toBe("18:3");
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
