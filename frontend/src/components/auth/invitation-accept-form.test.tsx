/**
 * Tests for InvitationAcceptForm Component
 * Tests the rendering and basic functionality of invitation acceptance form
 */
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
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
vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: mockToastSuccess,
  }),
}));

// Mock invitation-api
vi.mock("~/lib/invitation-api", () => ({
  acceptInvitation: vi.fn(() => Promise.resolve()),
}));

// Mock auth-helpers
vi.mock("~/lib/auth-helpers", () => ({
  getRoleDisplayName: (role: string) => role,
}));

// Mock UI components
vi.mock("~/components/ui", () => ({
  Input: ({
    id,
    value,
    onChange,
    disabled,
    label,
    ...props
  }: {
    id: string;
    value: string;
    onChange: (e: React.ChangeEvent<HTMLInputElement>) => void;
    disabled?: boolean;
    label?: string;
    type?: string;
    name?: string;
    autoComplete?: string;
    required?: boolean;
    className?: string;
  }) => (
    <div>
      {label && <label htmlFor={id}>{label}</label>}
      <input
        id={id}
        data-testid={`input-${id}`}
        value={value}
        onChange={onChange}
        disabled={disabled}
        {...props}
      />
    </div>
  ),
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

    await waitFor(() => {
      const firstNameInput =
        screen.getByTestId<HTMLInputElement>("input-firstName");
      fireEvent.change(firstNameInput, { target: { value: "Jane" } });
      expect(firstNameInput.value).toBe("Jane");
    });
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
});
