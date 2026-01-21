import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { Input } from "./input";

describe("Input", () => {
  it("renders input without label", () => {
    render(<Input name="test" placeholder="Enter text" />);

    expect(screen.getByPlaceholderText("Enter text")).toBeInTheDocument();
  });

  it("renders input with label", () => {
    render(<Input label="Username" name="username" />);

    expect(screen.getByText("Username")).toBeInTheDocument();
    expect(screen.getByLabelText("Username")).toBeInTheDocument();
  });

  it("renders error message when provided", () => {
    render(<Input name="email" error="Invalid email address" />);

    expect(screen.getByText("Invalid email address")).toBeInTheDocument();
  });

  it("does not render error when not provided", () => {
    render(<Input name="email" />);

    expect(screen.queryByRole("paragraph")).not.toBeInTheDocument();
  });

  it("passes custom className", () => {
    render(<Input name="test" className="custom-class" />);

    const input = screen.getByRole("textbox");
    expect(input.className).toContain("custom-class");
  });

  it("passes additional props to input", () => {
    render(
      <Input
        name="password"
        type="password"
        required
        maxLength={50}
        placeholder="Enter password"
      />,
    );

    const input = screen.getByPlaceholderText("Enter password");
    expect(input).toHaveAttribute("type", "password");
    expect(input).toHaveAttribute("required");
    expect(input).toHaveAttribute("maxLength", "50");
  });

  it("calls onChange handler when value changes", () => {
    const handleChange = vi.fn();
    render(<Input name="test" onChange={handleChange} />);

    const input = screen.getByRole("textbox");
    fireEvent.change(input, { target: { value: "new value" } });

    expect(handleChange).toHaveBeenCalled();
  });

  it("renders with disabled state", () => {
    render(<Input name="test" disabled />);

    expect(screen.getByRole("textbox")).toBeDisabled();
  });

  it("sets id and name attributes correctly", () => {
    render(<Input name="fieldName" />);

    const input = screen.getByRole("textbox");
    expect(input).toHaveAttribute("id", "fieldName");
    expect(input).toHaveAttribute("name", "fieldName");
  });

  it("associates label with input via htmlFor", () => {
    render(<Input label="Email" name="email" />);

    const label = screen.getByText("Email");
    expect(label).toHaveAttribute("for", "email");
  });
});
