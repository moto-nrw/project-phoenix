import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";

const { mockUseSession, mockUpdateSession, mockFetch } = vi.hoisted(() => ({
  mockUseSession: vi.fn(),
  mockUpdateSession: vi.fn(),
  mockFetch: vi.fn(),
}));

global.fetch = mockFetch;

vi.mock("next-auth/react", () => ({
  useSession: mockUseSession,
}));

vi.mock("~/components/ui/loading", () => ({
  Loading: () => <div>Loading...</div>,
}));

vi.mock("~/components/shared/settings-layout", () => ({
  SettingsLayout: ({ profileTab }: { profileTab: React.ReactNode }) => (
    <div>{profileTab}</div>
  ),
}));

import OperatorSettingsPage from "./page";

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
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => ({
        data: { id: 1, email: "test@example.com", display_name: "Test User" },
      }),
    } as Response);
  });

  it("shows loading state when auth is loading", async () => {
    mockUseSession.mockReturnValue({
      data: null,
      status: "loading",
      update: mockUpdateSession,
    });

    render(<OperatorSettingsPage />);

    await waitFor(() => {
      expect(screen.getByText("Loading...")).toBeInTheDocument();
    });
  });

  it("renders settings form with operator data", async () => {
    render(<OperatorSettingsPage />);

    await waitFor(() => {
      const displayNameInput = screen.getByLabelText("Anzeigename");
      expect(displayNameInput).toHaveValue("John Doe");
    });
  });

  it("updates profile on save", async () => {
    render(<OperatorSettingsPage />);

    await waitFor(() => {
      const editButton = screen.getByText("Bearbeiten");
      fireEvent.click(editButton);
    });

    const displayNameInput = screen.getByLabelText("Anzeigename");
    fireEvent.change(displayNameInput, { target: { value: "Jane Doe" } });

    const saveButton = screen.getByText("Speichern");
    fireEvent.click(saveButton);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
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
  });

  it("handles save error gracefully", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      json: async () => ({ error: "Something went wrong" }),
    } as Response);

    render(<OperatorSettingsPage />);

    await waitFor(() => {
      fireEvent.click(screen.getByText("Bearbeiten"));
    });

    const saveButton = screen.getByText("Speichern");
    fireEvent.click(saveButton);

    await waitFor(() => {
      expect(mockUpdateSession).not.toHaveBeenCalled();
    });
  });

  it("cancels editing and restores original values", async () => {
    render(<OperatorSettingsPage />);

    await waitFor(() => {
      fireEvent.click(screen.getByText("Bearbeiten"));
    });

    const displayNameInput = screen.getByLabelText("Anzeigename");
    fireEvent.change(displayNameInput, { target: { value: "Changed Name" } });

    fireEvent.click(screen.getByText("Abbrechen"));

    await waitFor(() => {
      expect(screen.getByLabelText("Anzeigename")).toHaveValue("John Doe");
    });
  });

  it("displays initials correctly", async () => {
    render(<OperatorSettingsPage />);

    await waitFor(() => {
      expect(screen.getByText("JD")).toBeInTheDocument();
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

      await waitFor(() => {
        fireEvent.click(screen.getByText("Bearbeiten"));
      });

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

      await waitFor(() => {
        fireEvent.click(screen.getByText("Bearbeiten"));
      });

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
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ message: "Verification email sent" }),
      } as Response);

      render(<OperatorSettingsPage />);

      await waitFor(() => {
        fireEvent.click(screen.getByText("Bearbeiten"));
      });

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
        expect(mockFetch).toHaveBeenCalledWith(
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
    });

    it("closes dialog after successful submission", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ message: "OK" }),
      } as Response);

      render(<OperatorSettingsPage />);

      await waitFor(() => {
        fireEvent.click(screen.getByText("Bearbeiten"));
      });

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
      mockFetch.mockResolvedValueOnce({
        ok: false,
        json: async () => ({
          error: "Das aktuelle Passwort ist falsch",
        }),
      } as Response);

      render(<OperatorSettingsPage />);

      await waitFor(() => {
        fireEvent.click(screen.getByText("Bearbeiten"));
      });

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
        expect(mockFetch).toHaveBeenCalledWith(
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
      mockFetch.mockResolvedValueOnce({
        ok: false,
        json: async () => ({ error: "E-Mail bereits verwendet" }),
      } as Response);

      render(<OperatorSettingsPage />);

      await waitFor(() => {
        fireEvent.click(screen.getByText("Bearbeiten"));
      });

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
        expect(mockFetch).toHaveBeenCalledTimes(1);
      });

      // Dialog stays open on error
      expect(screen.getByText("E-Mail-Adresse ändern")).toBeInTheDocument();
    });

    it("keeps dialog open on network error", async () => {
      mockFetch.mockRejectedValueOnce(new Error("Network error"));

      render(<OperatorSettingsPage />);

      await waitFor(() => {
        fireEvent.click(screen.getByText("Bearbeiten"));
      });

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
        expect(mockFetch).toHaveBeenCalledTimes(1);
      });

      // Dialog stays open on network error
      expect(screen.getByText("E-Mail-Adresse ändern")).toBeInTheDocument();
    });

    it("closes dialog and resets fields on cancel", async () => {
      render(<OperatorSettingsPage />);

      await waitFor(() => {
        fireEvent.click(screen.getByText("Bearbeiten"));
      });

      fireEvent.click(screen.getByText("E-Mail ändern"));

      await waitFor(() => {
        expect(
          screen.getByLabelText("Neue E-Mail-Adresse"),
        ).toBeInTheDocument();
      });

      fireEvent.change(screen.getByLabelText("Neue E-Mail-Adresse"), {
        target: { value: "typed@example.com" },
      });

      // Dialog renders before SettingsLayout in DOM, so dialog's Abbrechen is first
      const cancelButtons = screen.getAllByText("Abbrechen");
      fireEvent.click(cancelButtons[0]!);

      await waitFor(() => {
        expect(
          screen.queryByText("E-Mail-Adresse ändern"),
        ).not.toBeInTheDocument();
      });
    });
  });
});
