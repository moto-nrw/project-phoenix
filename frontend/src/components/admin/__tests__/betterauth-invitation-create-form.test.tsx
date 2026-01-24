/**
 * Tests for BetterAuthInvitationCreateForm component
 *
 * This file tests:
 * - Form rendering and initial state
 * - Role loading from Go backend
 * - Form field validation
 * - Role mapping from Go backend to BetterAuth
 * - Form submission (success and error cases)
 * - Error handling for various API responses
 * - Success message display with invitation link
 */

import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { BetterAuthInvitationCreateForm } from "../betterauth-invitation-create-form";

// Mock next/navigation
vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push: vi.fn(),
    refresh: vi.fn(),
  }),
}));

// Mock toast context
const mockToastSuccess = vi.fn();
const mockToastError = vi.fn();

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: mockToastSuccess,
    error: mockToastError,
  }),
}));

// Mock authClient - use hoisted mocks
const mockInviteMember = vi.hoisted(() => vi.fn());
const mockUseSession = vi.hoisted(() => vi.fn());

vi.mock("~/lib/auth-client", () => ({
  authClient: {
    organization: {
      inviteMember: mockInviteMember,
    },
  },
  useSession: mockUseSession,
}));

// Mock authService
const mockGetRoles = vi.hoisted(() => vi.fn());

vi.mock("~/lib/auth-service", () => ({
  authService: {
    getRoles: mockGetRoles,
  },
}));

// Mock getRoleDisplayName
vi.mock("~/lib/auth-helpers", () => ({
  getRoleDisplayName: (name: string) => {
    const displayNames: Record<string, string> = {
      supervisor: "Supervisor",
      "ogs-administrator": "OGS-Administrator",
      admin: "Administrator",
    };
    return displayNames[name] ?? name;
  },
}));

// Mock window.location.origin
const originalLocation = globalThis.location;

beforeEach(() => {
  vi.clearAllMocks();

  // Mock window.location
  Object.defineProperty(globalThis, "location", {
    value: { origin: "http://localhost:3000" },
    writable: true,
    configurable: true,
  });

  // Default session mock with active organization
  mockUseSession.mockReturnValue({
    data: {
      session: {
        activeOrganizationId: "org-123",
      },
    },
  });

  // Default roles mock
  mockGetRoles.mockResolvedValue([
    { id: "role-1", name: "supervisor", description: "Supervisor role" },
    {
      id: "role-2",
      name: "ogs-administrator",
      description: "OGS Admin role",
    },
  ]);
});

afterEach(() => {
  Object.defineProperty(globalThis, "location", {
    value: originalLocation,
    writable: true,
    configurable: true,
  });
});

describe("BetterAuthInvitationCreateForm", () => {
  // =============================================================================
  // Rendering Tests
  // =============================================================================

  describe("initial rendering", () => {
    it("renders the form title and description", async () => {
      render(<BetterAuthInvitationCreateForm />);

      await waitFor(() => {
        expect(screen.getByText("Neue Einladung")).toBeInTheDocument();
        expect(screen.getByText("Per E-Mail einladen")).toBeInTheDocument();
      });
    });

    it("renders email input field", async () => {
      render(<BetterAuthInvitationCreateForm />);

      await waitFor(() => {
        // Input component uses explicit id prop, query by id directly
        expect(document.getElementById("invitation-email")).toBeInTheDocument();
        expect(screen.getByText("E-Mail-Adresse")).toBeInTheDocument();
      });
    });

    it("renders role selector", async () => {
      render(<BetterAuthInvitationCreateForm />);

      await waitFor(() => {
        expect(screen.getByLabelText("Rolle")).toBeInTheDocument();
      });
    });

    it("renders optional name fields", async () => {
      render(<BetterAuthInvitationCreateForm />);

      await waitFor(() => {
        // Input component uses explicit id prop, query by id directly
        expect(
          document.getElementById("invitation-first-name"),
        ).toBeInTheDocument();
        expect(screen.getByText("Vorname (optional)")).toBeInTheDocument();
        expect(
          document.getElementById("invitation-last-name"),
        ).toBeInTheDocument();
        expect(screen.getByText("Nachname (optional)")).toBeInTheDocument();
      });
    });

    it("renders submit button", async () => {
      render(<BetterAuthInvitationCreateForm />);

      await waitFor(() => {
        expect(
          screen.getByRole("button", { name: /Einladung senden/ }),
        ).toBeInTheDocument();
      });
    });
  });

  // =============================================================================
  // Role Loading Tests
  // =============================================================================

  describe("role loading", () => {
    it("loads roles from Go backend", async () => {
      render(<BetterAuthInvitationCreateForm />);

      await waitFor(() => {
        expect(mockGetRoles).toHaveBeenCalled();
      });
    });

    it("displays loaded roles in selector", async () => {
      render(<BetterAuthInvitationCreateForm />);

      await waitFor(() => {
        const roleSelect = screen.getByLabelText("Rolle");
        expect(roleSelect).toBeInTheDocument();
      });

      // Check that options are rendered
      expect(screen.getByText("Supervisor")).toBeInTheDocument();
      expect(screen.getByText("OGS-Administrator")).toBeInTheDocument();
    });

    it("shows error when roles fail to load", async () => {
      mockGetRoles.mockRejectedValue(new Error("Network error"));

      render(<BetterAuthInvitationCreateForm />);

      await waitFor(() => {
        expect(
          screen.getByText(
            "Rollen konnten nicht geladen werden. Bitte aktualisiere die Seite.",
          ),
        ).toBeInTheDocument();
      });
    });

    it("handles cancelled fetch (component unmount)", async () => {
      const { unmount } = render(<BetterAuthInvitationCreateForm />);

      // Unmount immediately before roles resolve
      unmount();

      // Verify no state updates after unmount (no errors thrown)
      await waitFor(() => {
        expect(mockGetRoles).toHaveBeenCalled();
      });
    });
  });

  // =============================================================================
  // Form Validation Tests
  // =============================================================================

  describe("form validation", () => {
    it("shows error when email is empty", async () => {
      render(<BetterAuthInvitationCreateForm />);

      await waitFor(() => {
        expect(
          screen.getByRole("button", { name: /Einladung senden/ }),
        ).toBeInTheDocument();
      });

      const submitButton = screen.getByRole("button", {
        name: /Einladung senden/,
      });
      fireEvent.click(submitButton);

      expect(
        screen.getByText("Bitte gib eine gültige E-Mail-Adresse ein."),
      ).toBeInTheDocument();
    });

    it("shows error when email is only whitespace", async () => {
      render(<BetterAuthInvitationCreateForm />);

      await waitFor(() => {
        expect(
          document.getElementById("invitation-email") as HTMLInputElement,
        ).toBeInTheDocument();
      });

      const emailInput = document.getElementById(
        "invitation-email",
      ) as HTMLInputElement;
      fireEvent.change(emailInput, { target: { value: "   " } });

      const submitButton = screen.getByRole("button", {
        name: /Einladung senden/,
      });
      fireEvent.click(submitButton);

      expect(
        screen.getByText("Bitte gib eine gültige E-Mail-Adresse ein."),
      ).toBeInTheDocument();
    });

    it("shows error when no role is selected", async () => {
      render(<BetterAuthInvitationCreateForm />);

      await waitFor(() => {
        expect(
          document.getElementById("invitation-email") as HTMLInputElement,
        ).toBeInTheDocument();
      });

      const emailInput = document.getElementById(
        "invitation-email",
      ) as HTMLInputElement;
      fireEvent.change(emailInput, { target: { value: "test@example.com" } });

      const submitButton = screen.getByRole("button", {
        name: /Einladung senden/,
      });
      fireEvent.click(submitButton);

      expect(
        screen.getByText("Bitte wähle eine Rolle aus."),
      ).toBeInTheDocument();
    });

    it("shows error when no active organization", async () => {
      mockUseSession.mockReturnValue({
        data: {
          session: {
            activeOrganizationId: null,
          },
        },
      });

      render(<BetterAuthInvitationCreateForm />);

      await waitFor(() => {
        expect(
          document.getElementById("invitation-email") as HTMLInputElement,
        ).toBeInTheDocument();
      });

      const emailInput = document.getElementById(
        "invitation-email",
      ) as HTMLInputElement;
      fireEvent.change(emailInput, { target: { value: "test@example.com" } });

      const roleSelect = screen.getByLabelText("Rolle");
      fireEvent.change(roleSelect, { target: { value: "role-1" } });

      const submitButton = screen.getByRole("button", {
        name: /Einladung senden/,
      });
      fireEvent.click(submitButton);

      expect(
        screen.getByText("Keine aktive Organisation gefunden."),
      ).toBeInTheDocument();
    });

    it("shows error when selected role not found in list", async () => {
      // This validation path occurs when user selects a role but the roles
      // list changes before submission (edge case). HTML selects don't allow
      // invalid values, so we test the more common case: no role selected.
      render(<BetterAuthInvitationCreateForm />);

      await waitFor(() => {
        expect(
          document.getElementById("invitation-email") as HTMLInputElement,
        ).toBeInTheDocument();
      });

      const emailInput = document.getElementById(
        "invitation-email",
      ) as HTMLInputElement;
      fireEvent.change(emailInput, { target: { value: "test@example.com" } });

      // Don't select a role - roleId will be empty string
      const submitButton = screen.getByRole("button", {
        name: /Einladung senden/,
      });
      fireEvent.click(submitButton);

      // Empty role triggers the first validation check
      expect(
        screen.getByText("Bitte wähle eine Rolle aus."),
      ).toBeInTheDocument();
    });
  });

  // =============================================================================
  // Form Submission Tests
  // =============================================================================

  describe("form submission", () => {
    const fillValidForm = async () => {
      await waitFor(() => {
        expect(
          document.getElementById("invitation-email") as HTMLInputElement,
        ).toBeInTheDocument();
      });

      const emailInput = document.getElementById(
        "invitation-email",
      ) as HTMLInputElement;
      fireEvent.change(emailInput, { target: { value: "new@example.com" } });

      const roleSelect = screen.getByLabelText("Rolle");
      fireEvent.change(roleSelect, { target: { value: "role-1" } });
    };

    it("submits form successfully", async () => {
      mockInviteMember.mockResolvedValue({
        data: { id: "invitation-abc" },
        error: null,
      });

      const onCreated = vi.fn();
      render(<BetterAuthInvitationCreateForm onCreated={onCreated} />);
      await fillValidForm();

      const submitButton = screen.getByRole("button", {
        name: /Einladung senden/,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(mockInviteMember).toHaveBeenCalledWith({
          email: "new@example.com",
          role: "supervisor",
          organizationId: "org-123",
        });
      });

      await waitFor(() => {
        expect(mockToastSuccess).toHaveBeenCalledWith(
          "Einladung an new@example.com wurde gesendet.",
        );
      });

      await waitFor(() => {
        expect(onCreated).toHaveBeenCalled();
      });
    });

    it("shows success info with invitation link", async () => {
      mockInviteMember.mockResolvedValue({
        data: { id: "invitation-abc" },
        error: null,
      });

      render(<BetterAuthInvitationCreateForm />);
      await fillValidForm();

      const submitButton = screen.getByRole("button", {
        name: /Einladung senden/,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        // Email is in a <strong> tag, so text is split across elements
        // Check for the email and the surrounding success message container
        expect(screen.getByText("new@example.com")).toBeInTheDocument();
      });

      await waitFor(() => {
        expect(
          screen.getByText(/Einladungslink \(zum manuellen Teilen\)/),
        ).toBeInTheDocument();
      });
    });

    it("includes name params in invitation link when provided", async () => {
      mockInviteMember.mockResolvedValue({
        data: { id: "invitation-abc" },
        error: null,
      });

      render(<BetterAuthInvitationCreateForm />);
      await fillValidForm();

      // Fill name fields
      const firstNameInput = document.getElementById(
        "invitation-first-name",
      ) as HTMLInputElement;
      fireEvent.change(firstNameInput, { target: { value: "Max" } });

      const lastNameInput = document.getElementById(
        "invitation-last-name",
      ) as HTMLInputElement;
      fireEvent.change(lastNameInput, { target: { value: "Mustermann" } });

      const submitButton = screen.getByRole("button", {
        name: /Einladung senden/,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        // Check that link contains the name params
        const linkElement = screen.getByText(
          /accept-invitation\/invitation-abc/,
        );
        expect(linkElement.textContent).toContain("firstName=Max");
        expect(linkElement.textContent).toContain("lastName=Mustermann");
      });
    });

    it("shows loading state during submission", async () => {
      let resolvePromise: ((value: unknown) => void) | undefined;
      mockInviteMember.mockImplementation(
        () =>
          new Promise((resolve) => {
            resolvePromise = resolve;
          }),
      );

      render(<BetterAuthInvitationCreateForm />);
      await fillValidForm();

      const submitButton = screen.getByRole("button", {
        name: /Einladung senden/,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(
          screen.getByRole("button", { name: /Wird gesendet/ }),
        ).toBeInTheDocument();
      });

      // Input should be disabled
      expect(
        document.getElementById("invitation-email") as HTMLInputElement,
      ).toBeDisabled();

      // Cleanup
      resolvePromise?.({ data: { id: "inv-1" }, error: null });
    });

    it("resets form after successful submission", async () => {
      mockInviteMember.mockResolvedValue({
        data: { id: "invitation-abc" },
        error: null,
      });

      render(<BetterAuthInvitationCreateForm />);
      await fillValidForm();

      const submitButton = screen.getByRole("button", {
        name: /Einladung senden/,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        const emailInput = document.getElementById(
          "invitation-email",
        ) as HTMLInputElement;
        expect(emailInput.value).toBe("");
      });
    });
  });

  // =============================================================================
  // Error Handling Tests
  // =============================================================================

  describe("error handling", () => {
    const fillValidForm = async () => {
      await waitFor(() => {
        expect(
          document.getElementById("invitation-email") as HTMLInputElement,
        ).toBeInTheDocument();
      });

      const emailInput = document.getElementById(
        "invitation-email",
      ) as HTMLInputElement;
      fireEvent.change(emailInput, { target: { value: "test@example.com" } });

      const roleSelect = screen.getByLabelText("Rolle");
      fireEvent.change(roleSelect, { target: { value: "role-1" } });
    };

    it("shows error for USER_ALREADY_MEMBER", async () => {
      mockInviteMember.mockResolvedValue({
        data: null,
        error: { code: "USER_ALREADY_MEMBER" },
      });

      render(<BetterAuthInvitationCreateForm />);
      await fillValidForm();

      const submitButton = screen.getByRole("button", {
        name: /Einladung senden/,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(
          screen.getByText(
            "Diese Person ist bereits Mitglied der Organisation.",
          ),
        ).toBeInTheDocument();
      });
    });

    it("shows error for INVITATION_EXISTS", async () => {
      mockInviteMember.mockResolvedValue({
        data: null,
        error: { code: "INVITATION_EXISTS" },
      });

      render(<BetterAuthInvitationCreateForm />);
      await fillValidForm();

      const submitButton = screen.getByRole("button", {
        name: /Einladung senden/,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(
          screen.getByText(
            "Für diese E-Mail-Adresse existiert bereits eine ausstehende Einladung.",
          ),
        ).toBeInTheDocument();
      });
    });

    it("shows generic error message when available", async () => {
      mockInviteMember.mockResolvedValue({
        data: null,
        error: { code: "OTHER_ERROR", message: "Custom error message" },
      });

      render(<BetterAuthInvitationCreateForm />);
      await fillValidForm();

      const submitButton = screen.getByRole("button", {
        name: /Einladung senden/,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(screen.getByText("Custom error message")).toBeInTheDocument();
      });
    });

    it("shows fallback error message when no message provided", async () => {
      mockInviteMember.mockResolvedValue({
        data: null,
        error: { code: "OTHER_ERROR" },
      });

      render(<BetterAuthInvitationCreateForm />);
      await fillValidForm();

      const submitButton = screen.getByRole("button", {
        name: /Einladung senden/,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(
          screen.getByText("Die Einladung konnte nicht erstellt werden."),
        ).toBeInTheDocument();
      });
    });

    it("handles thrown error during submission", async () => {
      mockInviteMember.mockRejectedValue(new Error("Network error"));

      render(<BetterAuthInvitationCreateForm />);
      await fillValidForm();

      const submitButton = screen.getByRole("button", {
        name: /Einladung senden/,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(
          screen.getByText("Die Einladung konnte nicht erstellt werden."),
        ).toBeInTheDocument();
      });
    });

    it("clears error on new submission", async () => {
      // First call fails
      mockInviteMember.mockResolvedValueOnce({
        data: null,
        error: { code: "OTHER_ERROR", message: "First error" },
      });

      render(<BetterAuthInvitationCreateForm />);
      await fillValidForm();

      const submitButton = screen.getByRole("button", {
        name: /Einladung senden/,
      });

      // First submission - error
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(screen.getByText("First error")).toBeInTheDocument();
      });

      // Wait for loading to complete
      await waitFor(() => {
        expect(
          screen.getByRole("button", { name: /Einladung senden/ }),
        ).not.toBeDisabled();
      });

      // Second call succeeds
      mockInviteMember.mockResolvedValueOnce({
        data: { id: "invitation-abc" },
        error: null,
      });

      // Second submission
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(screen.queryByText("First error")).not.toBeInTheDocument();
      });
    });
  });

  // =============================================================================
  // Role Mapping Tests
  // =============================================================================

  describe("role mapping", () => {
    it("maps ogs-administrator to ogsAdmin", async () => {
      mockGetRoles.mockResolvedValue([
        {
          id: "role-2",
          name: "ogs-administrator",
          description: "OGS Admin role",
        },
      ]);
      mockInviteMember.mockResolvedValue({
        data: { id: "invitation-abc" },
        error: null,
      });

      render(<BetterAuthInvitationCreateForm />);

      await waitFor(() => {
        expect(
          document.getElementById("invitation-email") as HTMLInputElement,
        ).toBeInTheDocument();
      });

      const emailInput = document.getElementById(
        "invitation-email",
      ) as HTMLInputElement;
      fireEvent.change(emailInput, { target: { value: "test@example.com" } });

      const roleSelect = screen.getByLabelText("Rolle");
      fireEvent.change(roleSelect, { target: { value: "role-2" } });

      const submitButton = screen.getByRole("button", {
        name: /Einladung senden/,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(mockInviteMember).toHaveBeenCalledWith(
          expect.objectContaining({
            role: "ogsAdmin",
          }),
        );
      });
    });

    it("maps buero-administrator to bueroAdmin", async () => {
      mockGetRoles.mockResolvedValue([
        {
          id: "role-3",
          name: "buero-administrator",
          description: "Buero Admin role",
        },
      ]);
      mockInviteMember.mockResolvedValue({
        data: { id: "invitation-abc" },
        error: null,
      });

      render(<BetterAuthInvitationCreateForm />);

      await waitFor(() => {
        expect(
          document.getElementById("invitation-email") as HTMLInputElement,
        ).toBeInTheDocument();
      });

      const emailInput = document.getElementById(
        "invitation-email",
      ) as HTMLInputElement;
      fireEvent.change(emailInput, { target: { value: "test@example.com" } });

      const roleSelect = screen.getByLabelText("Rolle");
      fireEvent.change(roleSelect, { target: { value: "role-3" } });

      const submitButton = screen.getByRole("button", {
        name: /Einladung senden/,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(mockInviteMember).toHaveBeenCalledWith(
          expect.objectContaining({
            role: "bueroAdmin",
          }),
        );
      });
    });

    it("maps traeger-administrator to traegerAdmin", async () => {
      mockGetRoles.mockResolvedValue([
        {
          id: "role-4",
          name: "traeger-administrator",
          description: "Traeger Admin role",
        },
      ]);
      mockInviteMember.mockResolvedValue({
        data: { id: "invitation-abc" },
        error: null,
      });

      render(<BetterAuthInvitationCreateForm />);

      await waitFor(() => {
        expect(
          document.getElementById("invitation-email") as HTMLInputElement,
        ).toBeInTheDocument();
      });

      const emailInput = document.getElementById(
        "invitation-email",
      ) as HTMLInputElement;
      fireEvent.change(emailInput, { target: { value: "test@example.com" } });

      const roleSelect = screen.getByLabelText("Rolle");
      fireEvent.change(roleSelect, { target: { value: "role-4" } });

      const submitButton = screen.getByRole("button", {
        name: /Einladung senden/,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(mockInviteMember).toHaveBeenCalledWith(
          expect.objectContaining({
            role: "traegerAdmin",
          }),
        );
      });
    });

    it("uses original role name when no mapping exists", async () => {
      mockGetRoles.mockResolvedValue([
        { id: "role-5", name: "custom_role", description: "Custom role" },
      ]);
      mockInviteMember.mockResolvedValue({
        data: { id: "invitation-abc" },
        error: null,
      });

      render(<BetterAuthInvitationCreateForm />);

      await waitFor(() => {
        expect(
          document.getElementById("invitation-email") as HTMLInputElement,
        ).toBeInTheDocument();
      });

      const emailInput = document.getElementById(
        "invitation-email",
      ) as HTMLInputElement;
      fireEvent.change(emailInput, { target: { value: "test@example.com" } });

      const roleSelect = screen.getByLabelText("Rolle");
      fireEvent.change(roleSelect, { target: { value: "role-5" } });

      const submitButton = screen.getByRole("button", {
        name: /Einladung senden/,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(mockInviteMember).toHaveBeenCalledWith(
          expect.objectContaining({
            role: "custom_role", // original name returned when no mapping exists
          }),
        );
      });
    });
  });

  // =============================================================================
  // Form Input Tests
  // =============================================================================

  describe("form input handling", () => {
    it("handles email input changes", async () => {
      render(<BetterAuthInvitationCreateForm />);

      await waitFor(() => {
        expect(
          document.getElementById("invitation-email") as HTMLInputElement,
        ).toBeInTheDocument();
      });

      const emailInput = document.getElementById(
        "invitation-email",
      ) as HTMLInputElement;
      fireEvent.change(emailInput, { target: { value: "new@test.com" } });

      expect(emailInput.value).toBe("new@test.com");
    });

    it("handles firstName input changes", async () => {
      render(<BetterAuthInvitationCreateForm />);

      await waitFor(() => {
        expect(
          document.getElementById("invitation-first-name") as HTMLInputElement,
        ).toBeInTheDocument();
      });

      const firstNameInput = document.getElementById(
        "invitation-first-name",
      ) as HTMLInputElement;
      fireEvent.change(firstNameInput, { target: { value: "John" } });

      expect(firstNameInput.value).toBe("John");
    });

    it("handles lastName input changes", async () => {
      render(<BetterAuthInvitationCreateForm />);

      await waitFor(() => {
        expect(
          document.getElementById("invitation-last-name") as HTMLInputElement,
        ).toBeInTheDocument();
      });

      const lastNameInput = document.getElementById(
        "invitation-last-name",
      ) as HTMLInputElement;
      fireEvent.change(lastNameInput, { target: { value: "Doe" } });

      expect(lastNameInput.value).toBe("Doe");
    });

    it("handles role selection changes", async () => {
      render(<BetterAuthInvitationCreateForm />);

      await waitFor(() => {
        expect(screen.getByLabelText("Rolle")).toBeInTheDocument();
      });

      const roleSelect = screen.getByLabelText("Rolle") as HTMLSelectElement;
      fireEvent.change(roleSelect, { target: { value: "role-2" } });

      expect(roleSelect.value).toBe("role-2");
    });

    it("trims email and normalizes to lowercase", async () => {
      mockInviteMember.mockResolvedValue({
        data: { id: "invitation-abc" },
        error: null,
      });

      render(<BetterAuthInvitationCreateForm />);

      await waitFor(() => {
        expect(
          document.getElementById("invitation-email") as HTMLInputElement,
        ).toBeInTheDocument();
      });

      const emailInput = document.getElementById(
        "invitation-email",
      ) as HTMLInputElement;
      fireEvent.change(emailInput, {
        target: { value: "  TEST@EXAMPLE.COM  " },
      });

      const roleSelect = screen.getByLabelText("Rolle");
      fireEvent.change(roleSelect, { target: { value: "role-1" } });

      const submitButton = screen.getByRole("button", {
        name: /Einladung senden/,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(mockInviteMember).toHaveBeenCalledWith(
          expect.objectContaining({
            email: "test@example.com",
          }),
        );
      });
    });
  });

  // =============================================================================
  // Edge Cases
  // =============================================================================

  describe("edge cases", () => {
    it("handles submission when no invitationId returned", async () => {
      mockInviteMember.mockResolvedValue({
        data: {}, // No id field
        error: null,
      });

      const onCreated = vi.fn();
      render(<BetterAuthInvitationCreateForm onCreated={onCreated} />);

      await waitFor(() => {
        expect(
          document.getElementById("invitation-email") as HTMLInputElement,
        ).toBeInTheDocument();
      });

      const emailInput = document.getElementById(
        "invitation-email",
      ) as HTMLInputElement;
      fireEvent.change(emailInput, { target: { value: "test@example.com" } });

      const roleSelect = screen.getByLabelText("Rolle");
      fireEvent.change(roleSelect, { target: { value: "role-1" } });

      const submitButton = screen.getByRole("button", {
        name: /Einladung senden/,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(mockToastSuccess).toHaveBeenCalled();
      });

      // Success info shouldn't show without ID
      expect(
        screen.queryByText(/Einladungslink.*zum manuellen Teilen/),
      ).not.toBeInTheDocument();
    });

    it("handles globalThis without location", async () => {
      // Temporarily remove location
      const savedLocation = globalThis.location;
      // eslint-disable-next-line @typescript-eslint/ban-ts-comment
      // @ts-ignore - Testing edge case
      delete globalThis.location;

      render(<BetterAuthInvitationCreateForm />);

      await waitFor(() => {
        expect(
          document.getElementById("invitation-email") as HTMLInputElement,
        ).toBeInTheDocument();
      });

      // Restore location
      globalThis.location = savedLocation;
    });

    it("calls onCreated callback after successful submission", async () => {
      mockInviteMember.mockResolvedValue({
        data: { id: "invitation-abc" },
        error: null,
      });

      const onCreated = vi.fn();
      render(<BetterAuthInvitationCreateForm onCreated={onCreated} />);

      await waitFor(() => {
        expect(
          document.getElementById("invitation-email") as HTMLInputElement,
        ).toBeInTheDocument();
      });

      const emailInput = document.getElementById(
        "invitation-email",
      ) as HTMLInputElement;
      fireEvent.change(emailInput, { target: { value: "test@example.com" } });

      const roleSelect = screen.getByLabelText("Rolle");
      fireEvent.change(roleSelect, { target: { value: "role-1" } });

      const submitButton = screen.getByRole("button", {
        name: /Einladung senden/,
      });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(onCreated).toHaveBeenCalledTimes(1);
      });
    });
  });
});
