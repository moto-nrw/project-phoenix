import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { PasskeySettingsSection } from "./passkey-settings-section";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

// The shared error path shows failures through the toast provider (#2517).
function renderWithToast(
  ui: Parameters<typeof render>[0],
  options?: Parameters<typeof render>[1],
) {
  return render(ui, { wrapper: ToastProvider, ...options });
}

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
  isPasskeyCeremonyIncompleteError: (error: unknown) =>
    error instanceof Error && error.name === "NotAllowedError",
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

    renderWithToast(<PasskeySettingsSection />);

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

    renderWithToast(<PasskeySettingsSection scope="operator" />);

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

    renderWithToast(<PasskeySettingsSection />);

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
        screen.getByText("Der Passkey ist hinzugefügt."),
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

    renderWithToast(<PasskeySettingsSection scope="operator" />);

    // #3109: the row menu opens the ConfirmDeleteModal; the passkey is only
    // revoked after the two-step confirmation inside the dialog.
    fireEvent.click(await screen.findByLabelText("Aktionen für Old phone"));
    fireEvent.click(screen.getByRole("menuitem", { name: "Entfernen" }));
    expect(mockRevokePasskey).not.toHaveBeenCalled();
    expect(
      within(
        screen.getByRole("dialog", { name: "Passkey entfernen?" }),
      ).getByText("Old phone"),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Ja, entfernen" }));
    expect(mockRevokePasskey).not.toHaveBeenCalled();
    fireEvent.click(
      screen.getByRole("button", { name: "Endgültig entfernen" }),
    );

    await waitFor(() => {
      expect(mockRevokePasskey).toHaveBeenCalledWith("operator", "5");
    });
    await waitFor(() => {
      expect(
        screen.queryByRole("heading", { name: "Passkey entfernen?" }),
      ).not.toBeInTheDocument();
    });
    await waitFor(() => {
      expect(screen.getByText("Der Passkey ist entfernt.")).toBeInTheDocument();
      expect(
        screen.getByText("Keine Passkeys hinterlegt."),
      ).toBeInTheDocument();
    });
  });

  it("keeps the passkey when the removal is cancelled", async () => {
    mockListPasskeys.mockResolvedValue([
      { id: "5", name: "Old phone", created_at: "2026-06-15T10:00:00Z" },
    ]);
    renderWithToast(<PasskeySettingsSection />);

    fireEvent.click(await screen.findByLabelText("Aktionen für Old phone"));
    fireEvent.click(screen.getByRole("menuitem", { name: "Entfernen" }));
    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));

    expect(mockRevokePasskey).not.toHaveBeenCalled();
    expect(
      screen.queryByRole("heading", { name: "Passkey entfernen?" }),
    ).not.toBeInTheDocument();
    expect(screen.getByText("Old phone")).toBeInTheDocument();
    expect(mockListPasskeys).toHaveBeenCalledTimes(1);
  });

  it("shows a failed removal inside the dialog", async () => {
    mockListPasskeys.mockResolvedValue([
      { id: "5", name: "Old phone", created_at: "2026-06-15T10:00:00Z" },
    ]);
    mockRevokePasskey.mockRejectedValueOnce(new ApiError("Nicht erlaubt", 403));
    renderWithToast(<PasskeySettingsSection />);

    fireEvent.click(await screen.findByLabelText("Aktionen für Old phone"));
    fireEvent.click(screen.getByRole("menuitem", { name: "Entfernen" }));
    fireEvent.click(screen.getByRole("button", { name: "Ja, entfernen" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Endgültig entfernen" }),
    );

    expect(
      await screen.findByText(
        catalogText("general.permission", "das Entfernen des Passkeys"),
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Passkey entfernen?" }),
    ).toBeInTheDocument();
    expect(screen.getAllByText("Old phone")).toHaveLength(2);
  });

  it("shows load and action errors", async () => {
    mockListPasskeys.mockRejectedValueOnce(new ApiError("Liste kaputt", 500));
    renderWithToast(<PasskeySettingsSection />);

    // #2517: catalog text in place instead of an empty list.
    expect(
      await screen.findByText(
        catalogText("general.server", "die Liste der Passkeys"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("Keine Passkeys hinterlegt.")).toBeNull();

    mockStartPasskeyEnrollment.mockRejectedValueOnce(
      new ApiError("Code konnte nicht gesendet werden", 503),
    );
    fireEvent.click(screen.getByRole("button", { name: "Hinzufügen" }));
    fireEvent.click(screen.getByRole("button", { name: "E-Mail senden" }));

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "das Senden des Sicherheitscodes"),
      ),
    ).toBeInTheDocument();
  });

  // A wrong e-mail code answers 401 with identity.mfa_code_invalid. That is
  // no expired session: the form says so instead of jumping to the login.
  it("shows a wrong code in the form without leaving the page", async () => {
    const assign = vi.fn();
    vi.stubGlobal("location", { ...window.location, assign });
    mockRegisterPasskey.mockRejectedValueOnce(
      new ApiError("invalid code", 401, { code: "identity.mfa_code_invalid" }),
    );
    renderWithToast(<PasskeySettingsSection />);

    fireEvent.click(await screen.findByRole("button", { name: "Hinzufügen" }));
    fireEvent.click(screen.getByRole("button", { name: "E-Mail senden" }));
    fireEvent.change(await screen.findByLabelText("Code"), {
      target: { value: "123456" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(
        catalogText("identity.mfa_code_invalid", "das Hinzufügen des Passkeys"),
      ),
    ).toBeInTheDocument();
    expect(assign).not.toHaveBeenCalled();
    vi.unstubAllGlobals();
  });

  it("treats a cancelled device prompt as no error", async () => {
    const cancelled = new Error("The operation was not allowed");
    cancelled.name = "NotAllowedError";
    mockRegisterPasskey.mockRejectedValueOnce(cancelled);
    renderWithToast(<PasskeySettingsSection />);

    fireEvent.click(await screen.findByRole("button", { name: "Hinzufügen" }));
    fireEvent.click(screen.getByRole("button", { name: "E-Mail senden" }));
    fireEvent.change(await screen.findByLabelText("Code"), {
      target: { value: "123456" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => expect(mockRegisterPasskey).toHaveBeenCalledTimes(1));
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.getByRole("button", { name: "Speichern" })).toBeEnabled();
  });
});
