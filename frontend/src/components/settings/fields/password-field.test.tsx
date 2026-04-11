import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, fireEvent, screen, waitFor } from "@testing-library/react";
import { PasswordField } from "./password-field";

// Mock revealSettingValue
const mockRevealSettingValue = vi.fn();
vi.mock("~/lib/settings-api", () => ({
  revealSettingValue: (...args: unknown[]) => mockRevealSettingValue(...args),
}));

const defaultProps = {
  settingKey: "security.ogs_device_pin",
  onChange: vi.fn(),
};

describe("PasswordField", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

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

  it("hides eye and edit buttons when disabled", () => {
    render(<PasswordField {...defaultProps} hasValue={true} disabled />);
    expect(screen.queryByLabelText("Wert anzeigen")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Wert ändern")).not.toBeInTheDocument();
  });

  it("hides eye button when no value exists", () => {
    render(<PasswordField {...defaultProps} hasValue={false} />);
    expect(screen.queryByLabelText("Wert anzeigen")).not.toBeInTheDocument();
    // Edit button is still shown
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

  it("does not save when value is empty on Speichern click", async () => {
    const onChange = vi.fn().mockResolvedValue(undefined);
    render(
      <PasswordField
        settingKey="security.ogs_device_pin"
        hasValue={false}
        onChange={onChange}
      />,
    );

    fireEvent.click(screen.getByLabelText("Wert ändern"));
    // Don't type anything, click save
    fireEvent.click(screen.getByText("Speichern"));

    expect(onChange).not.toHaveBeenCalled();
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

  it("toggles show/hide in edit mode", () => {
    const { container } = render(
      <PasswordField {...defaultProps} hasValue={true} />,
    );
    fireEvent.click(screen.getByLabelText("Wert ändern"));

    // Initially password type
    const input = container.querySelector(
      "input[type='password']",
    ) as HTMLInputElement;
    expect(input).toBeInTheDocument();

    // Click show button in edit mode
    fireEvent.click(screen.getByLabelText("Wert anzeigen"));
    expect(container.querySelector("input[type='text']")).toBeInTheDocument();

    // Click hide button
    fireEvent.click(screen.getByLabelText("Wert verbergen"));
    expect(
      container.querySelector("input[type='password']"),
    ).toBeInTheDocument();
  });

  it("saves on Enter key press with value", async () => {
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
    fireEvent.change(input, { target: { value: "1234" } });
    fireEvent.keyDown(input, { key: "Enter" });

    await waitFor(() => {
      expect(onChange).toHaveBeenCalledWith("1234");
    });
  });

  it("does not save on Enter key press without value", () => {
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
    fireEvent.keyDown(input, { key: "Enter" });

    expect(onChange).not.toHaveBeenCalled();
  });

  it("cancels on Escape key press", () => {
    const onChange = vi.fn();
    const { container } = render(
      <PasswordField {...defaultProps} hasValue={true} onChange={onChange} />,
    );

    fireEvent.click(screen.getByLabelText("Wert ändern"));
    const input = container.querySelector(
      "input[type='password']",
    ) as HTMLInputElement;
    fireEvent.change(input, { target: { value: "partial" } });
    fireEvent.keyDown(input, { key: "Escape" });

    expect(onChange).not.toHaveBeenCalled();
    expect(screen.getByText("••••••")).toBeInTheDocument();
  });

  it("strips non-digit characters for PIN pattern", () => {
    const { container } = render(
      <PasswordField {...defaultProps} hasValue={false} pattern="^\d{4}$" />,
    );

    fireEvent.click(screen.getByLabelText("Wert ändern"));
    const input = container.querySelector(
      "input[type='password']",
    ) as HTMLInputElement;
    fireEvent.change(input, { target: { value: "12ab34" } });

    expect(input.value).toBe("1234");
  });

  it("sets numeric inputMode and maxLength for PIN pattern", () => {
    const { container } = render(
      <PasswordField {...defaultProps} hasValue={false} pattern="^\d{4}$" />,
    );

    fireEvent.click(screen.getByLabelText("Wert ändern"));
    const input = container.querySelector(
      "input[type='password']",
    ) as HTMLInputElement;

    expect(input.inputMode).toBe("numeric");
    expect(input.maxLength).toBe(4);
  });

  it("uses text inputMode without maxLength for non-PIN pattern", () => {
    const { container } = render(
      <PasswordField {...defaultProps} hasValue={false} />,
    );

    fireEvent.click(screen.getByLabelText("Wert ändern"));
    const input = container.querySelector(
      "input[type='password']",
    ) as HTMLInputElement;

    expect(input.inputMode).toBe("text");
  });

  it("reveals value via API on eye button click", async () => {
    mockRevealSettingValue.mockResolvedValue("5678");

    render(<PasswordField {...defaultProps} hasValue={true} />);

    fireEvent.click(screen.getByLabelText("Wert anzeigen"));

    await waitFor(() => {
      expect(screen.getByText("5678")).toBeInTheDocument();
    });
    expect(mockRevealSettingValue).toHaveBeenCalledWith(
      "security.ogs_device_pin",
    );
  });

  it("hides revealed value on second eye click", async () => {
    mockRevealSettingValue.mockResolvedValue("5678");

    render(<PasswordField {...defaultProps} hasValue={true} />);

    // Reveal
    fireEvent.click(screen.getByLabelText("Wert anzeigen"));
    await waitFor(() => {
      expect(screen.getByText("5678")).toBeInTheDocument();
    });

    // Hide
    fireEvent.click(screen.getByLabelText("Wert verbergen"));
    expect(screen.queryByText("5678")).not.toBeInTheDocument();
    expect(screen.getByText("••••••")).toBeInTheDocument();
  });

  it("clears revealed value when entering edit mode", async () => {
    mockRevealSettingValue.mockResolvedValue("5678");

    render(<PasswordField {...defaultProps} hasValue={true} />);

    // Reveal
    fireEvent.click(screen.getByLabelText("Wert anzeigen"));
    await waitFor(() => {
      expect(screen.getByText("5678")).toBeInTheDocument();
    });

    // Enter edit mode
    fireEvent.click(screen.getByLabelText("Wert ändern"));
    // Should not show revealed value anymore
    expect(screen.queryByText("5678")).not.toBeInTheDocument();
  });

  it("returns text hints when no pattern provided", () => {
    const { container } = render(
      <PasswordField {...defaultProps} hasValue={false} />,
    );
    fireEvent.click(screen.getByLabelText("Wert ändern"));
    const input = container.querySelector(
      "input[type='password']",
    ) as HTMLInputElement;
    expect(input.placeholder).toBe("Neuen Wert eingeben");
  });

  it("returns text hints for non-matching pattern", () => {
    const { container } = render(
      <PasswordField {...defaultProps} hasValue={false} pattern="^[a-z]+$" />,
    );
    fireEvent.click(screen.getByLabelText("Wert ändern"));
    const input = container.querySelector(
      "input[type='password']",
    ) as HTMLInputElement;
    expect(input.placeholder).toBe("Neuen Wert eingeben");
    expect(input.inputMode).toBe("text");
  });
});
