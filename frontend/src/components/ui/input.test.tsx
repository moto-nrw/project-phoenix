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

  it("keeps a hint after Speichern quiet and links it to the field", () => {
    // The form's FormErrorAlert announces the failure once (#5217).
    render(<Input name="email" label="E-Mail" error="Invalid email address" />);

    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    const input = screen.getByRole("textbox", { name: "E-Mail" });
    expect(input).toHaveAttribute("aria-invalid", "true");
    expect(input).toHaveAccessibleDescription("Invalid email address");
  });

  it("announces the error as an alert for a check while typing", () => {
    render(<Input name="email" error="Invalid email address" errorLive />);

    expect(screen.getByRole("alert")).toHaveTextContent(
      "Invalid email address",
    );
  });

  it("does not render error when not provided", () => {
    render(<Input name="email" />);

    // Anker: das Eingabefeld steht da, nur ohne Fehler.
    expect(screen.getByRole("textbox")).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
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

  it("associates label with input when only id is given", () => {
    render(<Input label="Start" id="edit-start" type="time" />);

    expect(screen.getByText("Start")).toHaveAttribute("for", "edit-start");
    expect(screen.getByLabelText("Start")).toHaveAttribute("id", "edit-start");
  });

  it("prefers an explicit id over name for the association", () => {
    render(<Input label="Ende" id="edit-end" name="endTime" />);

    expect(screen.getByText("Ende")).toHaveAttribute("for", "edit-end");
    const input = screen.getByLabelText("Ende");
    expect(input).toHaveAttribute("id", "edit-end");
    expect(input).toHaveAttribute("name", "endTime");
  });
});

describe("Input required mark (#5217)", () => {
  it("marks a required field with a star that stays out of its name", () => {
    render(<Input label="Vorname" name="first_name" required />);

    const input = screen.getByRole("textbox", { name: "Vorname" });
    expect(input).toBeRequired();
    expect(screen.getByText("*")).toHaveAttribute("aria-hidden", "true");
  });

  it("shows no star on an optional field", () => {
    render(<Input label="Vorname" name="first_name" />);

    expect(screen.getByRole("textbox", { name: "Vorname" })).not.toBeRequired();
    expect(screen.queryByText("*")).not.toBeInTheDocument();
  });
});
