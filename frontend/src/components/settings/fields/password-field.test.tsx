import { describe, it, expect, vi } from "vitest";
import { render, fireEvent, screen } from "@testing-library/react";
import { PasswordField } from "./password-field";

describe("PasswordField", () => {
  it("shows masked text when value exists", () => {
    render(<PasswordField hasValue={true} onChange={vi.fn()} />);
    expect(screen.getByText("••••••")).toBeInTheDocument();
  });

  it("shows Nicht gesetzt when no value", () => {
    render(<PasswordField hasValue={false} onChange={vi.fn()} />);
    expect(screen.getByText("Nicht gesetzt")).toBeInTheDocument();
  });

  it("shows 4-dot mask for PIN pattern", () => {
    render(
      <PasswordField hasValue={true} onChange={vi.fn()} pattern="^\d{4}$" />,
    );
    expect(screen.getByText("••••")).toBeInTheDocument();
  });

  it("switches to edit mode on pill click", () => {
    const { container } = render(
      <PasswordField hasValue={true} onChange={vi.fn()} />,
    );
    fireEvent.click(screen.getByText("••••••"));
    expect(
      container.querySelector("input[type='password']"),
    ).toBeInTheDocument();
  });

  it("calls onChange and exits edit mode on save", () => {
    const onChange = vi.fn();
    const { container } = render(
      <PasswordField hasValue={false} onChange={onChange} />,
    );

    fireEvent.click(screen.getByText("Nicht gesetzt"));

    const input = container.querySelector(
      "input[type='password']",
    ) as HTMLInputElement;
    fireEvent.change(input, { target: { value: "secret" } });

    fireEvent.click(screen.getByText("Speichern"));
    expect(onChange).toHaveBeenCalledWith("secret");
  });

  it("cancels edit mode without saving", () => {
    const onChange = vi.fn();
    render(<PasswordField hasValue={true} onChange={onChange} />);

    fireEvent.click(screen.getByText("••••••"));
    fireEvent.click(screen.getByText("Abbrechen"));

    expect(onChange).not.toHaveBeenCalled();
    expect(screen.getByText("••••••")).toBeInTheDocument();
  });

  it("shows eye toggle in edit mode", () => {
    render(<PasswordField hasValue={true} onChange={vi.fn()} />);
    fireEvent.click(screen.getByText("••••••"));
    expect(screen.getByLabelText("Wert anzeigen")).toBeInTheDocument();
  });
});
