import { describe, it, expect, vi } from "vitest";
import { render, fireEvent, screen } from "@testing-library/react";
import { TimeField } from "./time-field";

describe("TimeField", () => {
  it("renders with value", () => {
    render(<TimeField value="18:00" onChange={vi.fn()} />);
    const input = screen.getByPlaceholderText("HH:MM") as HTMLInputElement;
    expect(input.value).toBe("18:00");
  });

  it("shows Jederzeit label when value is empty", () => {
    render(<TimeField value="" onChange={vi.fn()} />);
    expect(screen.getByText("Jederzeit")).toBeInTheDocument();
    expect(screen.getByText("Ändern")).toBeInTheDocument();
  });

  it("switches to input when Ändern is clicked", () => {
    render(<TimeField value="" onChange={vi.fn()} />);
    fireEvent.click(screen.getByText("Ändern"));
    expect(screen.getByPlaceholderText("HH:MM")).toBeInTheDocument();
  });

  it("calls onChange with valid HH:MM", () => {
    const onChange = vi.fn();
    render(<TimeField value="18:00" onChange={onChange} />);
    const input = screen.getByPlaceholderText("HH:MM");
    fireEvent.change(input, { target: { value: "1630" } });
    expect(onChange).toHaveBeenCalledWith("16:30");
  });

  it("does not call onChange with incomplete input", () => {
    const onChange = vi.fn();
    render(<TimeField value="18:00" onChange={onChange} />);
    const input = screen.getByPlaceholderText("HH:MM");
    fireEvent.change(input, { target: { value: "16" } });
    expect(onChange).not.toHaveBeenCalled();
  });

  it("does not call onChange with invalid time", () => {
    const onChange = vi.fn();
    render(<TimeField value="18:00" onChange={onChange} />);
    const input = screen.getByPlaceholderText("HH:MM");
    fireEvent.change(input, { target: { value: "2560" } });
    expect(onChange).not.toHaveBeenCalled();
  });

  it("reverts to last valid value on blur when incomplete", () => {
    render(<TimeField value="18:00" onChange={vi.fn()} />);
    const input = screen.getByPlaceholderText("HH:MM") as HTMLInputElement;
    fireEvent.change(input, { target: { value: "16" } });
    fireEvent.blur(input);
    expect(input.value).toBe("18:00");
  });

  it("calls onBlur", () => {
    const onBlur = vi.fn();
    render(<TimeField value="18:00" onChange={vi.fn()} onBlur={onBlur} />);
    const input = screen.getByPlaceholderText("HH:MM");
    fireEvent.blur(input);
    expect(onBlur).toHaveBeenCalled();
  });

  it("shows Entfernen button when value is set", () => {
    render(<TimeField value="18:00" onChange={vi.fn()} />);
    expect(screen.getByText("Entfernen")).toBeInTheDocument();
  });

  it("clears value and shows Jederzeit when Entfernen is clicked", () => {
    const onChange = vi.fn();
    render(<TimeField value="18:00" onChange={onChange} />);
    fireEvent.click(screen.getByText("Entfernen"));
    expect(onChange).toHaveBeenCalledWith("");
  });

  it("hides Ändern button when disabled", () => {
    render(<TimeField value="" onChange={vi.fn()} disabled />);
    expect(screen.getByText("Jederzeit")).toBeInTheDocument();
    expect(screen.queryByText("Ändern")).not.toBeInTheDocument();
  });

  it("auto-inserts colon when typing digits", () => {
    render(<TimeField value="12:00" onChange={vi.fn()} />);
    const input = screen.getByPlaceholderText("HH:MM") as HTMLInputElement;
    fireEvent.change(input, { target: { value: "183" } });
    expect(input.value).toBe("18:3");
  });

  it("strips non-digit characters", () => {
    render(<TimeField value="12:00" onChange={vi.fn()} />);
    const input = screen.getByPlaceholderText("HH:MM") as HTMLInputElement;
    fireEvent.change(input, { target: { value: "1a8b:3c0" } });
    expect(input.value).toBe("18:30");
  });
});
