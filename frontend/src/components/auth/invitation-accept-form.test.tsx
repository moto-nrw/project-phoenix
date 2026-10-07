/**
 * Tests for InvitationAcceptForm Component
 * Tests the rendering and basic functionality of invitation acceptance form
 */
import {
  act,
  render,
  screen,
  waitFor,
  fireEvent,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { ApiError, unavailableApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";
import { InvitationAcceptForm } from "./invitation-accept-form";
import type { InvitationValidation } from "~/lib/invitation-helpers";

// Mock next-auth/react
vi.mock("next-auth/react", () => ({
  signOut: vi.fn(() => Promise.resolve({ redirect: false })),
}));

// Mock next/navigation
const mockPush = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push: mockPush,
  }),
}));

// Mock ToastContext
const mockToastSuccess = vi.fn();
vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({
    success: mockToastSuccess,
  }),
}));

// Mock invitation-api
const { mockAcceptInvitation } = vi.hoisted(() => ({
  mockAcceptInvitation: vi
    .fn()
    .mockResolvedValue({ tenantSubdomain: "burbach" }),
}));
vi.mock("~/lib/invitation-api", () => ({
  acceptInvitation: mockAcceptInvitation,
}));

// Mock auth-helpers
vi.mock("~/lib/auth-helpers", () => ({
  getRoleDisplayName: (role: string) => role,
}));

describe("InvitationAcceptForm", () => {
  const mockInvitation: InvitationValidation = {
    email: "test@example.com",
    roleName: "teacher",
    firstName: "John",
    lastName: "Doe",
    position: "Math Teacher",
    expiresAt: new Date(Date.now() + 86400000).toISOString(), // Tomorrow
  };

  beforeEach(() => {
    vi.clearAllMocks();
    // Divergent slug/subdomain school (#1977): the redirect must use the
    // subdomain ("burbach"), never the slug ("ogs-burbach").
    mockAcceptInvitation.mockResolvedValue({ tenantSubdomain: "burbach" });
  });

  it("renders the form with invitation details", async () => {
    render(
      <InvitationAcceptForm token="test-token" invitation={mockInvitation} />,
    );

    await waitFor(() => {
      expect(screen.getByText(/Einladung für/i)).toBeInTheDocument();
      expect(screen.getByText(mockInvitation.email)).toBeInTheDocument();
    });
  });

  it("displays invitation role and position", async () => {
    render(
      <InvitationAcceptForm token="test-token" invitation={mockInvitation} />,
    );

    await waitFor(() => {
      // Multiple elements may contain "teacher" text, use getAllByText
      expect(screen.getAllByText(/teacher/i).length).toBeGreaterThan(0);
      expect(screen.getByText(mockInvitation.position!)).toBeInTheDocument();
    });
  });

  it("renders all required form fields", async () => {
    render(
      <InvitationAcceptForm token="test-token" invitation={mockInvitation} />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("input-firstName")).toBeInTheDocument();
      expect(screen.getByTestId("input-lastName")).toBeInTheDocument();
      expect(screen.getByLabelText(/^Passwort$/)).toBeInTheDocument();
      expect(screen.getByLabelText(/Passwort bestätigen/)).toBeInTheDocument();
    });
  });

  it("pre-fills first name and last name from invitation", async () => {
    render(
      <InvitationAcceptForm token="test-token" invitation={mockInvitation} />,
    );

    await waitFor(() => {
      const firstNameInput =
        screen.getByTestId<HTMLInputElement>("input-firstName");
      const lastNameInput =
        screen.getByTestId<HTMLInputElement>("input-lastName");

      expect(firstNameInput.value).toBe(mockInvitation.firstName);
      expect(lastNameInput.value).toBe(mockInvitation.lastName);
    });
  });

  it("displays password requirements", async () => {
    render(
      <InvitationAcceptForm token="test-token" invitation={mockInvitation} />,
    );

    await waitFor(() => {
      expect(screen.getByText(/Passwortanforderungen/i)).toBeInTheDocument();
      expect(screen.getByText(/Mindestens 8 Zeichen/i)).toBeInTheDocument();
      expect(screen.getByText(/Ein Großbuchstabe/i)).toBeInTheDocument();
      expect(screen.getByText(/Ein Kleinbuchstabe/i)).toBeInTheDocument();
      expect(screen.getByText(/Eine Zahl/i)).toBeInTheDocument();
      expect(screen.getByText(/Ein Sonderzeichen/i)).toBeInTheDocument();
    });
  });

  it("renders submit button", async () => {
    render(
      <InvitationAcceptForm token="test-token" invitation={mockInvitation} />,
    );

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: /Einladung akzeptieren/i }),
      ).toBeInTheDocument();
    });
  });

  it("allows toggling password visibility", async () => {
    render(
      <InvitationAcceptForm token="test-token" invitation={mockInvitation} />,
    );

    await waitFor(() => {
      // Multiple password visibility toggles may exist, use getAllByLabelText
      const toggleButtons = screen.getAllByLabelText(/Passwort anzeigen/);
      expect(toggleButtons.length).toBeGreaterThan(0);
    });
  });

  it("displays expiration date", async () => {
    render(
      <InvitationAcceptForm token="test-token" invitation={mockInvitation} />,
    );

    await waitFor(() => {
      expect(screen.getByText(/Gültig bis/i)).toBeInTheDocument();
    });
  });

  it("handles form input changes", async () => {
    render(
      <InvitationAcceptForm token="test-token" invitation={mockInvitation} />,
    );

    const firstNameInput =
      await screen.findByTestId<HTMLInputElement>("input-firstName");
    fireEvent.change(firstNameInput, { target: { value: "Jane" } });
    expect(firstNameInput.value).toBe("Jane");
  });

  it("renders without crashing when position is not provided", async () => {
    const invitationWithoutPosition: InvitationValidation = {
      ...mockInvitation,
      position: undefined,
    };

    render(
      <InvitationAcceptForm
        token="test-token"
        invitation={invitationWithoutPosition}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText(/Einladung für/i)).toBeInTheDocument();
    });
  });

  it("renders without crashing when firstName is not provided", async () => {
    const invitationWithoutName: InvitationValidation = {
      ...mockInvitation,
      firstName: undefined,
      lastName: undefined,
    };

    render(
      <InvitationAcceptForm
        token="test-token"
        invitation={invitationWithoutName}
      />,
    );

    await waitFor(() => {
      const firstNameInput =
        screen.getByTestId<HTMLInputElement>("input-firstName");
      expect(firstNameInput.value).toBe("");
    });
  });

  it("validates missing name fields", async () => {
    const invitationNoName: InvitationValidation = {
      ...mockInvitation,
      firstName: undefined,
      lastName: undefined,
    };

    render(
      <InvitationAcceptForm token="test-token" invitation={invitationNoName} />,
    );

    const submitButton = await screen.findByRole("button", {
      name: /Einladung akzeptieren/i,
    });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(
        screen.getByText("Bitte prüfen Sie die markierten Felder."),
      ).toBeInTheDocument();
      expect(
        screen.getByText("Bitte geben Sie Ihren Vornamen an."),
      ).toBeInTheDocument();
      expect(
        screen.getByText("Bitte geben Sie Ihren Nachnamen an."),
      ).toBeInTheDocument();
    });
    expect(mockAcceptInvitation).not.toHaveBeenCalled();
  });

  it("validates password mismatch", async () => {
    render(
      <InvitationAcceptForm token="test-token" invitation={mockInvitation} />,
    );

    const passwordInput = await screen.findByLabelText(/^Passwort$/);
    const confirmInput = await screen.findByLabelText(/Passwort bestätigen/);
    fireEvent.change(passwordInput, { target: { value: "Test1234%" } });
    fireEvent.change(confirmInput, { target: { value: "Different1!" } });

    const submitButton = screen.getByRole("button", {
      name: /Einladung akzeptieren/i,
    });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(
        screen.getByText(/Passwörter stimmen nicht überein/i),
      ).toBeInTheDocument();
    });
  });

  it("validates weak password", async () => {
    render(
      <InvitationAcceptForm token="test-token" invitation={mockInvitation} />,
    );

    const passwordInput = await screen.findByLabelText(/^Passwort$/);
    const confirmInput = await screen.findByLabelText(/Passwort bestätigen/);
    fireEvent.change(passwordInput, { target: { value: "weak" } });
    fireEvent.change(confirmInput, { target: { value: "weak" } });

    const submitButton = screen.getByRole("button", {
      name: /Einladung akzeptieren/i,
    });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(
        screen.getByText("Das Passwort erfüllt noch nicht alle Anforderungen."),
      ).toBeInTheDocument();
    });
    expect(mockAcceptInvitation).not.toHaveBeenCalled();
  });

  it("shows success state after successful submission", async () => {
    render(
      <InvitationAcceptForm token="test-token" invitation={mockInvitation} />,
    );

    const passwordInput = await screen.findByLabelText(/^Passwort$/);
    const confirmInput = await screen.findByLabelText(/Passwort bestätigen/);
    fireEvent.change(passwordInput, { target: { value: "Test1234%" } });
    fireEvent.change(confirmInput, { target: { value: "Test1234%" } });

    const submitButton = screen.getByRole("button", {
      name: /Einladung akzeptieren/i,
    });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(screen.getByText(/Konto erstellt/i)).toBeInTheDocument();
      expect(
        screen.getByText(
          /Bitte melden Sie sich mit Ihren neuen Zugangsdaten an/i,
        ),
      ).toBeInTheDocument();
    });
  });

  it("shows error for 410 expired invitation", async () => {
    mockAcceptInvitation.mockRejectedValueOnce(
      new ApiError("Expired", 410, { code: "identity.invitation_expired" }),
    );

    render(
      <InvitationAcceptForm token="test-token" invitation={mockInvitation} />,
    );

    const passwordInput = await screen.findByLabelText(/^Passwort$/);
    const confirmInput = await screen.findByLabelText(/Passwort bestätigen/);
    fireEvent.change(passwordInput, { target: { value: "Test1234%" } });
    fireEvent.change(confirmInput, { target: { value: "Test1234%" } });

    fireEvent.click(
      screen.getByRole("button", { name: /Einladung akzeptieren/i }),
    );

    await waitFor(() => {
      expect(
        screen.getByText(
          catalogText("identity.invitation_expired", "die Einladung"),
        ),
      ).toBeInTheDocument();
    });
  });

  it("shows error for 409 email conflict", async () => {
    mockAcceptInvitation.mockRejectedValueOnce(new ApiError("Conflict", 409));

    render(
      <InvitationAcceptForm token="test-token" invitation={mockInvitation} />,
    );

    const passwordInput = await screen.findByLabelText(/^Passwort$/);
    const confirmInput = await screen.findByLabelText(/Passwort bestätigen/);
    fireEvent.change(passwordInput, { target: { value: "Test1234%" } });
    fireEvent.change(confirmInput, { target: { value: "Test1234%" } });

    fireEvent.click(
      screen.getByRole("button", { name: /Einladung akzeptieren/i }),
    );

    await waitFor(() => {
      expect(
        screen.getByText(
          catalogText("general.business_rejection", "die Einladung"),
        ),
      ).toBeInTheDocument();
    });
  });

  it("shows error for 404 not found", async () => {
    mockAcceptInvitation.mockRejectedValueOnce(
      new ApiError("Not found", 404, {
        code: "identity.invitation_not_found",
      }),
    );

    render(
      <InvitationAcceptForm token="test-token" invitation={mockInvitation} />,
    );

    const passwordInput = await screen.findByLabelText(/^Passwort$/);
    const confirmInput = await screen.findByLabelText(/Passwort bestätigen/);
    fireEvent.change(passwordInput, { target: { value: "Test1234%" } });
    fireEvent.change(confirmInput, { target: { value: "Test1234%" } });

    fireEvent.click(
      screen.getByRole("button", { name: /Einladung akzeptieren/i }),
    );

    await waitFor(() => {
      expect(
        screen.getByText(
          catalogText("identity.invitation_not_found", "die Einladung"),
        ),
      ).toBeInTheDocument();
    });
  });

  it("shows the catalog text of a weak password and marks the field", async () => {
    mockAcceptInvitation.mockRejectedValueOnce(
      new ApiError("password too weak", 400, {
        code: "identity.password_too_weak",
        errors: [{ field: "password", reason: "password too weak" }],
      }),
    );

    render(
      <InvitationAcceptForm token="test-token" invitation={mockInvitation} />,
    );

    const passwordInput = await screen.findByLabelText(/^Passwort$/);
    const confirmInput = await screen.findByLabelText(/Passwort bestätigen/);
    fireEvent.change(passwordInput, { target: { value: "Test1234%" } });
    fireEvent.change(confirmInput, { target: { value: "Test1234%" } });

    fireEvent.click(
      screen.getByRole("button", { name: /Einladung akzeptieren/i }),
    );

    await waitFor(() => {
      expect(
        screen.getByText(
          catalogText("identity.password_too_weak", "die Einladung"),
        ),
      ).toBeInTheDocument();
      // The backend names the field; the form marks it.
      expect(screen.getByLabelText(/^Passwort$/)).toHaveAttribute(
        "aria-invalid",
        "true",
      );
    });
  });

  it("shows generic error for unknown failure", async () => {
    mockAcceptInvitation.mockRejectedValueOnce({});

    render(
      <InvitationAcceptForm token="test-token" invitation={mockInvitation} />,
    );

    const passwordInput = await screen.findByLabelText(/^Passwort$/);
    const confirmInput = await screen.findByLabelText(/Passwort bestätigen/);
    fireEvent.change(passwordInput, { target: { value: "Test1234%" } });
    fireEvent.change(confirmInput, { target: { value: "Test1234%" } });

    fireEvent.click(
      screen.getByRole("button", { name: /Einladung akzeptieren/i }),
    );

    await waitFor(() => {
      // Not an API error: the general text, never a backend sentence.
      expect(
        screen.getByText(catalogText("general.server", "die Einladung")),
      ).toBeInTheDocument();
    });
  });

  it("redirects to tenant subdomain after successful accept", async () => {
    const originalEnv = process.env.NEXT_PUBLIC_TENANT_DOMAIN;
    const originalLocation = window.location;
    process.env.NEXT_PUBLIC_TENANT_DOMAIN = "localhost";

    const mockLocation = { href: "", protocol: "http:", port: "3000" };
    Object.defineProperty(window, "location", {
      value: mockLocation,
      writable: true,
      configurable: true,
    });

    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      render(
        <InvitationAcceptForm token="test-token" invitation={mockInvitation} />,
      );

      const passwordInput = await screen.findByLabelText(/^Passwort$/);
      const confirmInput = await screen.findByLabelText(/Passwort bestätigen/);
      fireEvent.change(passwordInput, { target: { value: "Test1234%" } });
      fireEvent.change(confirmInput, { target: { value: "Test1234%" } });

      fireEvent.click(
        screen.getByRole("button", { name: /Einladung akzeptieren/i }),
      );

      await waitFor(() => expect(mockAcceptInvitation).toHaveBeenCalled());
      // The redirect fires 1.5 s after the accept; skip the wait.
      await act(() => vi.advanceTimersByTimeAsync(1600));
      await waitFor(
        () => {
          expect(mockLocation.href).toBe("http://burbach.localhost:3000/");
        },
        { timeout: 3000 },
      );
    } finally {
      process.env.NEXT_PUBLIC_TENANT_DOMAIN = originalEnv;
      Object.defineProperty(window, "location", {
        value: originalLocation,
        writable: true,
        configurable: true,
      });
    }
  });

  it("falls back to router.push when no tenant subdomain", async () => {
    mockAcceptInvitation.mockResolvedValueOnce({ tenantSubdomain: undefined });
    vi.useFakeTimers({ shouldAdvanceTime: true });

    render(
      <InvitationAcceptForm token="test-token" invitation={mockInvitation} />,
    );

    const passwordInput = await screen.findByLabelText(/^Passwort$/);
    const confirmInput = await screen.findByLabelText(/Passwort bestätigen/);
    fireEvent.change(passwordInput, { target: { value: "Test1234%" } });
    fireEvent.change(confirmInput, { target: { value: "Test1234%" } });

    fireEvent.click(
      screen.getByRole("button", { name: /Einladung akzeptieren/i }),
    );

    await waitFor(() => expect(mockAcceptInvitation).toHaveBeenCalled());
    await act(() => vi.advanceTimersByTimeAsync(1600));
    await waitFor(
      () => {
        expect(mockPush).toHaveBeenCalledWith("/");
      },
      { timeout: 3000 },
    );
  });

  it("shows manual redirect button when signOut fails instead of auto-redirecting", async () => {
    const { signOut } = await import("next-auth/react");
    vi.mocked(signOut).mockRejectedValueOnce(new Error("signOut failed"));

    const originalEnv = process.env.NEXT_PUBLIC_TENANT_DOMAIN;
    const originalLocation = window.location;
    process.env.NEXT_PUBLIC_TENANT_DOMAIN = "localhost";

    const mockLocation = { href: "", protocol: "http:", port: "3000" };
    Object.defineProperty(window, "location", {
      value: mockLocation,
      writable: true,
      configurable: true,
    });

    try {
      render(
        <InvitationAcceptForm token="test-token" invitation={mockInvitation} />,
      );

      const passwordInput = await screen.findByLabelText(/^Passwort$/);
      const confirmInput = await screen.findByLabelText(/Passwort bestätigen/);
      fireEvent.change(passwordInput, { target: { value: "Test1234%" } });
      fireEvent.change(confirmInput, { target: { value: "Test1234%" } });

      fireEvent.click(
        screen.getByRole("button", { name: /Einladung akzeptieren/i }),
      );

      // Should show success state with manual button, NOT auto-redirect
      await waitFor(() => {
        expect(screen.getByText(/Konto erstellt/i)).toBeInTheDocument();
        expect(
          screen.getByText(/nicht automatisch beendet/i),
        ).toBeInTheDocument();
        expect(
          screen.getByRole("button", { name: /Zur Anmeldung/i }),
        ).toBeInTheDocument();
      });

      // Should NOT have auto-redirected
      expect(mockLocation.href).toBe("");

      // Clicking the manual button with signOut failing again should NOT redirect
      vi.mocked(signOut).mockRejectedValueOnce(
        new Error("signOut failed again"),
      );
      fireEvent.click(screen.getByRole("button", { name: /Zur Anmeldung/i }));

      await waitFor(() => {
        // Should show the final fallback message, not redirect
        expect(
          screen.getByText(/löschen Sie die Websitedaten/i),
        ).toBeInTheDocument();
        // Button should be gone
        expect(
          screen.queryByRole("button", { name: /Zur Anmeldung/i }),
        ).not.toBeInTheDocument();
      });

      // Should still NOT have redirected
      expect(mockLocation.href).toBe("");
    } finally {
      process.env.NEXT_PUBLIC_TENANT_DOMAIN = originalEnv;
      Object.defineProperty(window, "location", {
        value: originalLocation,
        writable: true,
        configurable: true,
      });
    }
  });

  it("redirects when signOut retry succeeds", async () => {
    const { signOut } = await import("next-auth/react");
    vi.mocked(signOut).mockRejectedValueOnce(new Error("signOut failed"));

    const originalEnv = process.env.NEXT_PUBLIC_TENANT_DOMAIN;
    const originalLocation = window.location;
    process.env.NEXT_PUBLIC_TENANT_DOMAIN = "localhost";

    const mockLocation = { href: "", protocol: "http:", port: "3000" };
    Object.defineProperty(window, "location", {
      value: mockLocation,
      writable: true,
      configurable: true,
    });

    try {
      render(
        <InvitationAcceptForm token="test-token" invitation={mockInvitation} />,
      );

      const passwordInput = await screen.findByLabelText(/^Passwort$/);
      const confirmInput = await screen.findByLabelText(/Passwort bestätigen/);
      fireEvent.change(passwordInput, { target: { value: "Test1234%" } });
      fireEvent.change(confirmInput, { target: { value: "Test1234%" } });

      fireEvent.click(
        screen.getByRole("button", { name: /Einladung akzeptieren/i }),
      );

      await waitFor(() => {
        expect(
          screen.getByRole("button", { name: /Zur Anmeldung/i }),
        ).toBeInTheDocument();
      });

      // Retry succeeds — should redirect
      vi.mocked(signOut).mockResolvedValueOnce({ url: "" });
      fireEvent.click(screen.getByRole("button", { name: /Zur Anmeldung/i }));

      await waitFor(() => {
        expect(mockLocation.href).toBe("http://burbach.localhost:3000/");
      });
    } finally {
      process.env.NEXT_PUBLIC_TENANT_DOMAIN = originalEnv;
      Object.defineProperty(window, "location", {
        value: originalLocation,
        writable: true,
        configurable: true,
      });
    }
  });

  it("shows the unavailable text when the request never reached the API", async () => {
    mockAcceptInvitation.mockRejectedValueOnce(
      unavailableApiError(new TypeError("Failed to fetch")),
    );

    render(
      <InvitationAcceptForm token="test-token" invitation={mockInvitation} />,
    );

    const passwordInput = await screen.findByLabelText(/^Passwort$/);
    const confirmInput = await screen.findByLabelText(/Passwort bestätigen/);
    fireEvent.change(passwordInput, { target: { value: "Test1234%" } });
    fireEvent.change(confirmInput, { target: { value: "Test1234%" } });

    fireEvent.click(
      screen.getByRole("button", { name: /Einladung akzeptieren/i }),
    );

    await waitFor(() => {
      expect(
        screen.getByText(catalogText("general.unavailable", "die Einladung")),
      ).toBeInTheDocument();
    });
    // Retryable: the alert offers "Wiederholen" with the current form.
    mockAcceptInvitation.mockResolvedValueOnce({ tenantSubdomain: "burbach" });
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    await waitFor(() => {
      expect(mockAcceptInvitation).toHaveBeenCalledTimes(2);
    });
  });

  describe("Scroll to error and field highlighting", () => {
    it("scrolls to error when validation fails on missing names", async () => {
      const scrollIntoViewMock = vi.fn();
      Element.prototype.scrollIntoView = scrollIntoViewMock;

      const invitationNoName: InvitationValidation = {
        ...mockInvitation,
        firstName: undefined,
        lastName: undefined,
      };

      render(
        <InvitationAcceptForm
          token="test-token"
          invitation={invitationNoName}
        />,
      );

      const submitButton = await screen.findByRole("button", {
        name: /Einladung akzeptieren/i,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(
          screen.getByText("Bitte prüfen Sie die markierten Felder."),
        ).toBeInTheDocument();
        expect(scrollIntoViewMock).toHaveBeenCalledWith({
          behavior: "smooth",
          block: "nearest",
        });
      });
    });

    it("highlights the password label when password is weak", async () => {
      Element.prototype.scrollIntoView = vi.fn();

      render(
        <InvitationAcceptForm token="test-token" invitation={mockInvitation} />,
      );

      const passwordInput = await screen.findByLabelText(/^Passwort$/);
      const confirmInput = await screen.findByLabelText(/Passwort bestätigen/);
      fireEvent.change(passwordInput, { target: { value: "weak" } });
      fireEvent.change(confirmInput, { target: { value: "weak" } });

      fireEvent.click(
        screen.getByRole("button", { name: /Einladung akzeptieren/i }),
      );

      await waitFor(() => {
        expect(
          screen.getByText(
            "Das Passwort erfüllt noch nicht alle Anforderungen.",
          ),
        ).toBeInTheDocument();
      });

      const passwordLabel = screen.getByText("Passwort");
      expect(passwordLabel.className).toContain("text-moto-red");
    });
  });
});
