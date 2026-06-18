import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { PasskeySettingsSection } from "./passkey-settings-section";

const {
  mockIsPasskeySupported,
  mockListPasskeys,
  mockRegisterPasskey,
  mockRevokePasskey,
  mockStartPasskeyEnrollment,
  mockSuggestCurrentDeviceLabel,
} = vi.hoisted(() => ({
  mockIsPasskeySupported: vi.fn(),
  mockListPasskeys: vi.fn(),
  mockRegisterPasskey: vi.fn(),
  mockRevokePasskey: vi.fn(),
  mockStartPasskeyEnrollment: vi.fn(),
  mockSuggestCurrentDeviceLabel: vi.fn(),
}));

vi.mock("~/lib/passkey-api", () => ({
  isPasskeySupported: mockIsPasskeySupported,
  listPasskeys: mockListPasskeys,
  registerPasskey: mockRegisterPasskey,
  revokePasskey: mockRevokePasskey,
  startPasskeyEnrollment: mockStartPasskeyEnrollment,
}));

vi.mock("~/lib/device-label", () => ({
  suggestCurrentDeviceLabel: mockSuggestCurrentDeviceLabel,
}));

describe("PasskeySettingsSection", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockIsPasskeySupported.mockReturnValue(true);
    mockSuggestCurrentDeviceLabel.mockReturnValue("Chrome auf macOS");
    mockListPasskeys.mockResolvedValue([]);
    mockStartPasskeyEnrollment.mockResolvedValue({
      challenge_token: "challenge-token",
      masked_email: "m***@example.test",
    });
    mockRegisterPasskey.mockResolvedValue({
      id: "2",
      name: "Chrome auf macOS",
      created_at: "2026-06-15T10:00:00Z",
    });
    mockRevokePasskey.mockResolvedValue(undefined);
  });

  it("shows an unsupported browser message", async () => {
    mockIsPasskeySupported.mockReturnValue(false);

    render(<PasskeySettingsSection />);

    expect(
      screen.getByText("Passkeys werden von diesem Browser nicht unterstützt."),
    ).toBeInTheDocument();
    await waitFor(() => {
      expect(
        screen.getByText("Keine Passkeys hinterlegt."),
      ).toBeInTheDocument();
    });
    expect(screen.queryByRole("button", { name: "Hinzufügen" })).toBeNull();
  });

  it("renders existing passkeys and formatted dates", async () => {
    mockListPasskeys.mockResolvedValueOnce([
      {
        id: "1",
        name: "Laptop",
        created_at: "2026-06-15T10:00:00Z",
      },
      {
        id: "2",
        name: "Phone",
        created_at: "2026-06-01T10:00:00Z",
        last_used_at: "2026-06-14T10:00:00Z",
      },
    ]);

    render(<PasskeySettingsSection scope="operator" />);

    await waitFor(() => {
      expect(screen.getByText("Laptop")).toBeInTheDocument();
      expect(screen.getByText("Phone")).toBeInTheDocument();
    });
    expect(screen.getByText("Erstellt: 15.06.2026")).toBeInTheDocument();
    expect(
      screen.getByText("Zuletzt verwendet: 14.06.2026"),
    ).toBeInTheDocument();
    expect(mockListPasskeys).toHaveBeenCalledWith("operator");
  });

  it("starts enrollment, registers a passkey, and reloads credentials", async () => {
    mockListPasskeys.mockResolvedValueOnce([]).mockResolvedValueOnce([
      {
        id: "3",
        name: "MacBook",
        created_at: "2026-06-15T10:00:00Z",
      },
    ]);

    render(<PasskeySettingsSection />);

    fireEvent.click(await screen.findByRole("button", { name: "Hinzufügen" }));

    expect(mockStartPasskeyEnrollment).not.toHaveBeenCalled();
    expect(
      screen.getByText("Sicherheitscode per E-Mail senden"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Öffnen Sie danach Ihr E-Mail-Postfach/),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "E-Mail senden" }));

    await waitFor(() => {
      expect(mockStartPasskeyEnrollment).toHaveBeenCalledWith("tenant");
    });
    expect(
      screen.getByText("Code gesendet an m***@example.test"),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Name")).toHaveValue("Chrome auf macOS");

    fireEvent.change(screen.getByLabelText("Code"), {
      target: { value: "123456" },
    });
    fireEvent.change(screen.getByLabelText("Name"), {
      target: { value: "MacBook" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(mockRegisterPasskey).toHaveBeenCalledWith("tenant", {
        code: "123456",
        name: "MacBook",
      });
    });
    await waitFor(() => {
      expect(
        screen.getByText("Passkey wurde hinzugefügt."),
      ).toBeInTheDocument();
      expect(screen.getByText("MacBook")).toBeInTheDocument();
    });
    expect(mockListPasskeys).toHaveBeenCalledTimes(2);
  });

  it("revokes a passkey and reloads the list", async () => {
    mockListPasskeys
      .mockResolvedValueOnce([
        {
          id: "5",
          name: "Old phone",
          created_at: "2026-06-15T10:00:00Z",
        },
      ])
      .mockResolvedValueOnce([]);

    render(<PasskeySettingsSection scope="operator" />);

    fireEvent.click(await screen.findByLabelText("Passkey entfernen"));

    await waitFor(() => {
      expect(mockRevokePasskey).toHaveBeenCalledWith("operator", "5");
    });
    await waitFor(() => {
      expect(screen.getByText("Passkey wurde entfernt.")).toBeInTheDocument();
      expect(
        screen.getByText("Keine Passkeys hinterlegt."),
      ).toBeInTheDocument();
    });
  });

  it("shows load and action errors", async () => {
    mockListPasskeys.mockRejectedValueOnce(new Error("Liste kaputt"));
    render(<PasskeySettingsSection />);

    expect(await screen.findByRole("alert")).toHaveTextContent("Liste kaputt");

    mockStartPasskeyEnrollment.mockRejectedValueOnce(
      new Error("Code konnte nicht gesendet werden"),
    );
    fireEvent.click(screen.getByRole("button", { name: "Hinzufügen" }));
    fireEvent.click(screen.getByRole("button", { name: "E-Mail senden" }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent(
        "Code konnte nicht gesendet werden",
      );
    });
  });
});
