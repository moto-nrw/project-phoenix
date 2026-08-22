import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";

const {
  mockUseSession,
  mockUpdateSession,
  mockSessionFetch,
  mockToastSuccess,
  mockToastError,
} = vi.hoisted(() => ({
  mockUseSession: vi.fn(),
  mockUpdateSession: vi.fn(),
  mockSessionFetch: vi.fn(),
  mockToastSuccess: vi.fn(),
  mockToastError: vi.fn(),
}));

vi.mock("next-auth/react", () => ({
  useSession: mockUseSession,
}));

vi.mock("~/lib/session-cache", () => ({
  sessionFetch: (...args: unknown[]) => mockSessionFetch(...args),
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: mockToastSuccess,
    error: mockToastError,
  }),
}));

vi.mock("~/components/ui/page-header/PageHeaderWithSearch", () => ({
  PageHeaderWithSearch: ({ title }: { title: string }) => (
    <div data-testid="page-header">{title}</div>
  ),
}));

vi.mock("~/components/ui/password-change-modal", () => ({
  PasswordChangeModal: () => null,
}));

// TrustedDevicesSection pulls in useToast/useTrustedDevices and is exercised
// by its own component tests — stub it out here to keep this page-level test
// focused on profile editing and avoid wiring ToastProvider/Tanstack Query.
vi.mock("~/components/settings/trusted-devices-section", () => ({
  TrustedDevicesSection: () => null,
}));

import OperatorSettingsPage from "./page";

const defaultProfile = {
  id: 1,
  email: "mail@yannickwenger.de",
  display_name: "yonnock",
};

describe("OperatorSettingsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUpdateSession.mockResolvedValue(undefined);
    mockUseSession.mockReturnValue({
      data: {
        user: { name: "John Doe", email: "john@example.com" },
      },
      status: "authenticated",
      update: mockUpdateSession,
    });
    mockSessionFetch.mockImplementation(
      async (url: string, init?: RequestInit) => {
        if (url === "/api/operator/profile" && init?.method === "GET") {
          return {
            ok: true,
            json: async () => ({ data: defaultProfile }),
          } as Response;
        }
        if (url === "/api/operator/profile" && init?.method === "PUT") {
          const body = JSON.parse(String(init.body ?? "{}")) as {
            display_name?: string;
          };
          return {
            ok: true,
            json: async () => ({
              data: {
                ...defaultProfile,
                display_name: body.display_name ?? defaultProfile.display_name,
              },
            }),
          } as Response;
        }
        if (url === "/api/operator/profile/email-change") {
          return {
            ok: true,
            json: async () => ({ message: "Verification email sent" }),
          } as Response;
        }
        return {
          ok: true,
          json: async () => ({ data: defaultProfile }),
        } as Response;
      },
    );
  });

  it("shows loading state when auth is loading", async () => {
    mockUseSession.mockReturnValue({
      data: null,
      status: "loading",
      update: mockUpdateSession,
    });

    render(<OperatorSettingsPage />);

    await waitFor(() => {
      expect(
        screen.getByLabelText("Profileinstellungen werden geladen"),
      ).toBeInTheDocument();
    });
  });

  it("renders settings form with operator data", async () => {
    render(<OperatorSettingsPage />);

    await waitFor(() => {
      const displayNameInput = screen.getByLabelText("Anzeigename");
      expect(displayNameInput).toHaveValue("yonnock");
      expect(screen.getByLabelText("E-Mail")).toHaveValue(
        "mail@yannickwenger.de",
      );
    });
  });

  it("loads canonical backend email instead of a platform token subject", async () => {
    mockUseSession.mockReturnValue({
      data: {
        user: { name: "yonnock", email: "operator:2" },
      },
      status: "authenticated",
      update: mockUpdateSession,
    });

    render(<OperatorSettingsPage />);

    await waitFor(() => {
      expect(screen.getByLabelText("E-Mail")).toHaveValue(
        "mail@yannickwenger.de",
      );
    });
    expect(screen.getByLabelText("E-Mail")).not.toHaveValue("operator:2");
  });

  it("does not show a non-email session fallback when profile loading fails", async () => {
    mockUseSession.mockReturnValue({
      data: {
        user: { name: "yonnock", email: "operator:2" },
      },
      status: "authenticated",
      update: mockUpdateSession,
    });
    mockSessionFetch.mockRejectedValueOnce(new Error("Network error"));

    render(<OperatorSettingsPage />);

    await waitFor(() => {
      expect(mockSessionFetch).toHaveBeenCalledWith("/api/operator/profile", {
        method: "GET",
      });
    });
    expect(screen.getByLabelText("E-Mail")).toHaveValue("");
  });

  it("updates profile on save", async () => {
    render(<OperatorSettingsPage />);

    fireEvent.click(await screen.findByText("Bearbeiten"));

    const displayNameInput = screen.getByLabelText("Anzeigename");
    fireEvent.change(displayNameInput, { target: { value: "Jane Doe" } });

    const saveButton = screen.getByText("Speichern");
    fireEvent.click(saveButton);

    await waitFor(() => {
      expect(mockSessionFetch).toHaveBeenCalledWith(
        "/api/operator/profile",
        expect.objectContaining({
          method: "PUT",
          body: JSON.stringify({ display_name: "Jane Doe" }),
        }),
      );
    });

    await waitFor(() => {
      expect(mockUpdateSession).toHaveBeenCalled();
    });
    expect(mockToastSuccess).toHaveBeenCalledWith(
      "Profil erfolgreich aktualisiert",
      { duration: 3000 },
    );
  });

  it("handles save error gracefully", async () => {
    mockSessionFetch.mockImplementation(
      async (url: string, init?: RequestInit) => {
        if (url === "/api/operator/profile" && init?.method === "GET") {
          return {
            ok: true,
            json: async () => ({ data: defaultProfile }),
          } as Response;
        }
        return {
          ok: false,
          json: async () => ({ error: "Something went wrong" }),
        } as Response;
      },
    );

    render(<OperatorSettingsPage />);

    fireEvent.click(await screen.findByText("Bearbeiten"));

    const saveButton = screen.getByText("Speichern");
    fireEvent.click(saveButton);

    await waitFor(() => {
      expect(mockUpdateSession).not.toHaveBeenCalled();
    });
    expect(mockToastError).toHaveBeenCalledWith(
      "Fehler beim Speichern des Profils",
      { duration: 3000 },
    );
  });

  it("cancels editing and restores original values", async () => {
    render(<OperatorSettingsPage />);

    fireEvent.click(await screen.findByText("Bearbeiten"));

    const displayNameInput = screen.getByLabelText("Anzeigename");
    fireEvent.change(displayNameInput, { target: { value: "Changed Name" } });

    fireEvent.click(screen.getByText("Abbrechen"));

    await waitFor(() => {
      expect(screen.getByLabelText("Anzeigename")).toHaveValue("yonnock");
    });
  });

  it("displays initials correctly", async () => {
    render(<OperatorSettingsPage />);

    await waitFor(() => {
      expect(screen.getByText("Y")).toBeInTheDocument();
    });
  });

  it("handles single word name initials", async () => {
    mockUseSession.mockReturnValue({
      data: {
        user: { name: "Admin", email: "admin@example.com" },
      },
      status: "authenticated",
      update: mockUpdateSession,
    });
    mockSessionFetch.mockImplementation(async () => ({
      ok: true,
      json: async () => ({
        data: { id: 1, email: "admin@example.com", display_name: "Admin" },
      }),
    }));

    render(<OperatorSettingsPage />);

    await waitFor(() => {
      expect(screen.getByText("A")).toBeInTheDocument();
    });
  });

  describe("email change dialog", () => {
    beforeEach(() => {
      HTMLDialogElement.prototype.showModal = vi.fn();
      HTMLDialogElement.prototype.close = vi.fn();
    });

    it("shows email change button when editing", async () => {
      render(<OperatorSettingsPage />);

      fireEvent.click(await screen.findByText("Bearbeiten"));

      expect(screen.getByText("E-Mail ändern")).toBeInTheDocument();
    });

    it("does not show email change button when not editing", async () => {
      render(<OperatorSettingsPage />);

      await waitFor(() => {
        expect(screen.getByText("Bearbeiten")).toBeInTheDocument();
      });

      expect(screen.queryByText("E-Mail ändern")).not.toBeInTheDocument();
    });

    it("opens email change dialog with form fields", async () => {
      render(<OperatorSettingsPage />);

      fireEvent.click(await screen.findByText("Bearbeiten"));

      fireEvent.click(screen.getByText("E-Mail ändern"));

      await waitFor(() => {
        expect(screen.getByText("E-Mail-Adresse ändern")).toBeInTheDocument();
        expect(
          screen.getByLabelText("Neue E-Mail-Adresse"),
        ).toBeInTheDocument();
        expect(screen.getByLabelText("Aktuelles Passwort")).toBeInTheDocument();
        expect(
          screen.getByText("E-Mail-Änderung anfordern"),
        ).toBeInTheDocument();
      });
    });

    it("submits email change request successfully", async () => {
      render(<OperatorSettingsPage />);

      fireEvent.click(await screen.findByText("Bearbeiten"));

      fireEvent.click(screen.getByText("E-Mail ändern"));

      await waitFor(() => {
        expect(
          screen.getByLabelText("Neue E-Mail-Adresse"),
        ).toBeInTheDocument();
      });

      fireEvent.change(screen.getByLabelText("Neue E-Mail-Adresse"), {
        target: { value: "new@example.com" },
      });
      fireEvent.change(screen.getByLabelText("Aktuelles Passwort"), {
        target: { value: "mypassword" },
      });

      fireEvent.click(screen.getByText("E-Mail-Änderung anfordern"));

      await waitFor(() => {
        expect(mockSessionFetch).toHaveBeenCalledWith(
          "/api/operator/profile/email-change",
          expect.objectContaining({
            method: "POST",
            body: JSON.stringify({
              new_email: "new@example.com",
              current_password: "mypassword",
            }),
          }),
        );
      });
      expect(mockToastSuccess).toHaveBeenCalledWith(
        "Eine Bestätigungs-E-Mail wird an new@example.com gesendet. Bitte überprüfe dein Postfach.",
        { duration: 3000 },
      );
    });

    it("closes dialog after successful submission", async () => {
      render(<OperatorSettingsPage />);

      fireEvent.click(await screen.findByText("Bearbeiten"));

      fireEvent.click(screen.getByText("E-Mail ändern"));

      await waitFor(() => {
        expect(
          screen.getByLabelText("Neue E-Mail-Adresse"),
        ).toBeInTheDocument();
      });

      fireEvent.change(screen.getByLabelText("Neue E-Mail-Adresse"), {
        target: { value: "new@example.com" },
      });
      fireEvent.change(screen.getByLabelText("Aktuelles Passwort"), {
        target: { value: "pass" },
      });

      fireEvent.click(screen.getByText("E-Mail-Änderung anfordern"));

      await waitFor(() => {
        expect(
          screen.queryByText("E-Mail-Adresse ändern"),
        ).not.toBeInTheDocument();
      });
    });

    it("sends error response without closing dialog on wrong password", async () => {
      mockSessionFetch.mockImplementation(
        async (url: string, init?: RequestInit) => {
          if (url === "/api/operator/profile" && init?.method === "GET") {
            return {
              ok: true,
              json: async () => ({ data: defaultProfile }),
            } as Response;
          }
          return {
            ok: false,
            json: async () => ({
              error: "Das aktuelle Passwort ist falsch",
            }),
          } as Response;
        },
      );

      render(<OperatorSettingsPage />);

      fireEvent.click(await screen.findByText("Bearbeiten"));

      fireEvent.click(screen.getByText("E-Mail ändern"));

      await waitFor(() => {
        expect(
          screen.getByLabelText("Neue E-Mail-Adresse"),
        ).toBeInTheDocument();
      });

      fireEvent.change(screen.getByLabelText("Neue E-Mail-Adresse"), {
        target: { value: "new@example.com" },
      });
      fireEvent.change(screen.getByLabelText("Aktuelles Passwort"), {
        target: { value: "wrong" },
      });

      fireEvent.click(screen.getByText("E-Mail-Änderung anfordern"));

      await waitFor(() => {
        expect(mockSessionFetch).toHaveBeenCalledWith(
          "/api/operator/profile/email-change",
          expect.objectContaining({
            method: "POST",
            body: JSON.stringify({
              new_email: "new@example.com",
              current_password: "wrong",
            }),
          }),
        );
      });

      // Dialog stays open — error path does not close it
      expect(screen.getByText("E-Mail-Adresse ändern")).toBeInTheDocument();
    });

    it("keeps dialog open on other API errors", async () => {
      mockSessionFetch.mockImplementation(
        async (url: string, init?: RequestInit) => {
          if (url === "/api/operator/profile" && init?.method === "GET") {
            return {
              ok: true,
              json: async () => ({ data: defaultProfile }),
            } as Response;
          }
          return {
            ok: false,
            json: async () => ({ error: "E-Mail bereits verwendet" }),
          } as Response;
        },
      );

      render(<OperatorSettingsPage />);

      fireEvent.click(await screen.findByText("Bearbeiten"));

      fireEvent.click(screen.getByText("E-Mail ändern"));

      await waitFor(() => {
        expect(
          screen.getByLabelText("Neue E-Mail-Adresse"),
        ).toBeInTheDocument();
      });

      fireEvent.change(screen.getByLabelText("Neue E-Mail-Adresse"), {
        target: { value: "taken@example.com" },
      });
      fireEvent.change(screen.getByLabelText("Aktuelles Passwort"), {
        target: { value: "pass" },
      });

      fireEvent.click(screen.getByText("E-Mail-Änderung anfordern"));

      await waitFor(() => {
        expect(mockSessionFetch).toHaveBeenCalledWith(
          "/api/operator/profile/email-change",
          expect.objectContaining({ method: "POST" }),
        );
      });

      // Dialog stays open on error
      expect(screen.getByText("E-Mail-Adresse ändern")).toBeInTheDocument();
    });

    it("keeps dialog open on network error", async () => {
      mockSessionFetch.mockImplementation(
        async (url: string, init?: RequestInit) => {
          if (url === "/api/operator/profile" && init?.method === "GET") {
            return {
              ok: true,
              json: async () => ({ data: defaultProfile }),
            } as Response;
          }
          throw new Error("Network error");
        },
      );

      render(<OperatorSettingsPage />);

      fireEvent.click(await screen.findByText("Bearbeiten"));

      fireEvent.click(screen.getByText("E-Mail ändern"));

      await waitFor(() => {
        expect(
          screen.getByLabelText("Neue E-Mail-Adresse"),
        ).toBeInTheDocument();
      });

      fireEvent.change(screen.getByLabelText("Neue E-Mail-Adresse"), {
        target: { value: "new@example.com" },
      });
      fireEvent.change(screen.getByLabelText("Aktuelles Passwort"), {
        target: { value: "pass" },
      });

      fireEvent.click(screen.getByText("E-Mail-Änderung anfordern"));

      await waitFor(() => {
        expect(mockSessionFetch).toHaveBeenCalledWith(
          "/api/operator/profile/email-change",
          expect.objectContaining({ method: "POST" }),
        );
      });

      // Dialog stays open on network error
      expect(screen.getByText("E-Mail-Adresse ändern")).toBeInTheDocument();
    });

    it("closes dialog and resets fields on cancel", async () => {
      render(<OperatorSettingsPage />);

      fireEvent.click(await screen.findByText("Bearbeiten"));

      fireEvent.click(screen.getByText("E-Mail ändern"));

      await waitFor(() => {
        expect(
          screen.getByLabelText("Neue E-Mail-Adresse"),
        ).toBeInTheDocument();
      });

      fireEvent.change(screen.getByLabelText("Neue E-Mail-Adresse"), {
        target: { value: "typed@example.com" },
      });

      // Dialog's Abbrechen is distinct from the profile edit Abbrechen
      const cancelButtons = screen.getAllByText("Abbrechen");
      // The dialog cancel button — find the one inside the dialog
      const dialogCancel = cancelButtons.find(
        (btn) => btn.closest("dialog") !== null,
      );
      fireEvent.click(dialogCancel ?? cancelButtons[0]!);

      await waitFor(() => {
        expect(
          screen.queryByText("E-Mail-Adresse ändern"),
        ).not.toBeInTheDocument();
      });
    });
  });
});
