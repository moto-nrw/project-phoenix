import { describe, it, expect, vi } from "vitest";
import { render, fireEvent, screen } from "@testing-library/react";
import { PasswordField } from "./password-field";

const defaultProps = {
  settingKey: "security.ogs_device_pin",
  onChange: vi.fn(),
};

describe("PasswordField", () => {
  it("shows masked text when value exists", () => {
    render(<PasswordField {...defaultProps} hasValue={true} />);
    expect(screen.getByText("••••••")).toBeInTheDocument();
  });

  it("shows Nicht gesetzt when no value", () => {
    render(<PasswordField {...defaultProps} hasValue={false} />);
    expect(screen.getByText("Nicht gesetzt")).toBeInTheDocument();
  });

  it("shows 4-dot mask for PIN pattern", () => {
    render(
      <PasswordField {...defaultProps} hasValue={true} pattern="^\d{4}$" />,
    );
    expect(screen.getByText("••••")).toBeInTheDocument();
  });

  it("shows eye and edit buttons when value exists", () => {
    render(<PasswordField {...defaultProps} hasValue={true} />);
    expect(screen.getByLabelText("Wert anzeigen")).toBeInTheDocument();
    expect(screen.getByLabelText("Wert ändern")).toBeInTheDocument();
  });

  it("switches to edit mode on edit button click", () => {
    const { container } = render(
      <PasswordField {...defaultProps} hasValue={true} />,
    );
    fireEvent.click(screen.getByLabelText("Wert ändern"));
    expect(
      container.querySelector("input[type='password']"),
    ).toBeInTheDocument();
  });

  it("calls onChange and exits edit mode on save", async () => {
    const onChange = vi.fn().mockResolvedValue(undefined);
    const { container } = render(
      <PasswordField
        settingKey="security.ogs_device_pin"
        hasValue={false}
        onChange={onChange}
      />,
    );

    fireEvent.click(screen.getByLabelText("Wert ändern"));

    const input = container.querySelector(
      "input[type='password']",
    ) as HTMLInputElement;
    fireEvent.change(input, { target: { value: "secret" } });
    fireEvent.click(screen.getByText("Speichern"));

    expect(onChange).toHaveBeenCalledWith("secret");
  });

  it("cancels edit mode without saving", () => {
    const onChange = vi.fn();
    render(
      <PasswordField {...defaultProps} hasValue={true} onChange={onChange} />,
    );

    fireEvent.click(screen.getByLabelText("Wert ändern"));
    fireEvent.click(screen.getByText("Abbrechen"));

    expect(onChange).not.toHaveBeenCalled();
    expect(screen.getByText("••••••")).toBeInTheDocument();
  });
});
