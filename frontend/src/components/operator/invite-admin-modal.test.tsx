import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";

/* eslint-disable @typescript-eslint/no-explicit-any */

const { mockInviteSchoolAdmin } = vi.hoisted(() => ({
  mockInviteSchoolAdmin: vi.fn(),
}));

vi.mock("~/lib/operator/provisioning-api", () => ({
  operatorProvisioningService: {
    inviteSchoolAdmin: mockInviteSchoolAdmin,
  },
}));

vi.mock("~/lib/operator/api-helpers", () => ({
  OperatorApiError: class OperatorApiError extends Error {
    status: number;
    constructor(message: string, status: number) {
      super(message);
      this.name = "OperatorApiError";
      this.status = status;
    }
  },
  isOperatorApiError: (error: any) =>
    error instanceof Error && error.name === "OperatorApiError",
}));

vi.mock("~/components/ui/modal", () => ({
  Modal: ({ isOpen, children, title, footer }: any) =>
    isOpen ? (
      <div data-testid="modal">
        <h2>{title}</h2>
        {children}
        <div data-testid="modal-footer">{footer}</div>
      </div>
    ) : null,
}));

import { InviteAdminModal } from "./invite-admin-modal";
import { OperatorApiError } from "~/lib/operator/api-helpers";
import type { School, Invitation } from "~/lib/operator/provisioning-helpers";

const mockSchool: School = {
  id: "5",
  createdAt: "2026-01-01",
  updatedAt: "2026-01-01",
  organizationId: "1",
  name: "GGS Musterberg",
  slug: "ggs-musterberg",
  subdomain: "ggs-musterberg",
  active: true,
};

const mockInvitation: Invitation = {
  id: "99",
  email: "admin@school.de",
  roleId: "5",
  roleName: "School Admin",
  expiresAt: "2026-03-01T00:00:00Z",
  firstName: "Max",
  lastName: "Mustermann",
  position: "Schulleitung",
  createdBy: "1",
  creator: "Operator",
  deliveryStatus: "sent",
  emailSentAt: "2026-02-15T10:00:00Z",
  emailRetryCount: 0,
};

describe("InviteAdminModal", () => {
  const mockOnClose = vi.fn();
  const mockOnInvited = vi.fn();

  const defaultProps = {
    isOpen: true,
    onClose: mockOnClose,
    school: mockSchool,
    onInvited: mockOnInvited,
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders nothing when closed", () => {
    render(<InviteAdminModal {...defaultProps} isOpen={false} />);
    expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
  });

  it("renders modal with school name in title", () => {
    render(<InviteAdminModal {...defaultProps} />);
    expect(
      screen.getByText("Admin einladen — GGS Musterberg"),
    ).toBeInTheDocument();
  });

  it("renders email input and optional fields", () => {
    render(<InviteAdminModal {...defaultProps} />);
    expect(screen.getByLabelText("E-Mail")).toBeInTheDocument();
    expect(screen.getByLabelText("Vorname")).toBeInTheDocument();
    expect(screen.getByLabelText("Nachname")).toBeInTheDocument();
    expect(screen.getByLabelText("Position")).toBeInTheDocument();
  });

  it("shows email error when submitting empty email", async () => {
    render(<InviteAdminModal {...defaultProps} />);
    const form = document.getElementById("invite-admin-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(screen.getByText("E-Mail ist erforderlich")).toBeInTheDocument();
    });
    expect(mockInviteSchoolAdmin).not.toHaveBeenCalled();
  });

  it("does not submit when school is null", async () => {
    render(<InviteAdminModal {...defaultProps} school={null} />);
    fireEvent.change(screen.getByLabelText("E-Mail"), {
      target: { value: "test@test.de" },
    });
    const form = document.getElementById("invite-admin-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(mockInviteSchoolAdmin).not.toHaveBeenCalled();
    });
  });

  it("submits with email only", async () => {
    mockInviteSchoolAdmin.mockResolvedValue(mockInvitation);

    render(<InviteAdminModal {...defaultProps} />);
    fireEvent.change(screen.getByLabelText("E-Mail"), {
      target: { value: "Admin@School.de" },
    });

    const form = document.getElementById("invite-admin-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(mockInviteSchoolAdmin).toHaveBeenCalledWith("5", {
        email: "admin@school.de",
      });
      expect(mockOnInvited).toHaveBeenCalled();
    });
  });

  it("submits with all optional fields", async () => {
    mockInviteSchoolAdmin.mockResolvedValue(mockInvitation);

    render(<InviteAdminModal {...defaultProps} />);
    fireEvent.change(screen.getByLabelText("E-Mail"), {
      target: { value: "admin@school.de" },
    });
    fireEvent.change(screen.getByLabelText("Vorname"), {
      target: { value: "Max" },
    });
    fireEvent.change(screen.getByLabelText("Nachname"), {
      target: { value: "Mustermann" },
    });
    fireEvent.change(screen.getByLabelText("Position"), {
      target: { value: "Schulleitung" },
    });

    const form = document.getElementById("invite-admin-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(mockInviteSchoolAdmin).toHaveBeenCalledWith("5", {
        email: "admin@school.de",
        first_name: "Max",
        last_name: "Mustermann",
        position: "Schulleitung",
      });
    });
  });

  it("shows success state after invitation", async () => {
    mockInviteSchoolAdmin.mockResolvedValue(mockInvitation);

    render(<InviteAdminModal {...defaultProps} />);
    fireEvent.change(screen.getByLabelText("E-Mail"), {
      target: { value: "admin@school.de" },
    });

    const form = document.getElementById("invite-admin-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(
        screen.getByText("Einladung erfolgreich erstellt"),
      ).toBeInTheDocument();
      expect(screen.getByText("admin@school.de")).toBeInTheDocument();
      expect(screen.getByText("Gesendet")).toBeInTheDocument();
      expect(screen.getByText("Einladung gesendet")).toBeInTheDocument();
    });
  });

  it("shows close button in success state", async () => {
    mockInviteSchoolAdmin.mockResolvedValue(mockInvitation);

    render(<InviteAdminModal {...defaultProps} />);
    fireEvent.change(screen.getByLabelText("E-Mail"), {
      target: { value: "admin@school.de" },
    });

    const form = document.getElementById("invite-admin-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(screen.getByText("Schließen")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Schließen"));
    expect(mockOnClose).toHaveBeenCalled();
  });

  it("shows expiry date in success state", async () => {
    mockInviteSchoolAdmin.mockResolvedValue(mockInvitation);

    render(<InviteAdminModal {...defaultProps} />);
    fireEvent.change(screen.getByLabelText("E-Mail"), {
      target: { value: "admin@school.de" },
    });

    const form = document.getElementById("invite-admin-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(screen.getByText("Gültig bis")).toBeInTheDocument();
    });
  });

  it("shows fallback style for unknown delivery status", async () => {
    const unknownStatusInvitation: Invitation = {
      ...mockInvitation,
      deliveryStatus: "unknown_status",
    };
    mockInviteSchoolAdmin.mockResolvedValue(unknownStatusInvitation);

    render(<InviteAdminModal {...defaultProps} />);
    fireEvent.change(screen.getByLabelText("E-Mail"), {
      target: { value: "admin@school.de" },
    });

    const form = document.getElementById("invite-admin-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(screen.getByText("unknown_status")).toBeInTheDocument();
    });
  });

  it("shows email conflict error on 409", async () => {
    mockInviteSchoolAdmin.mockRejectedValue(
      new OperatorApiError("email already invited", 409),
    );

    render(<InviteAdminModal {...defaultProps} />);
    fireEvent.change(screen.getByLabelText("E-Mail"), {
      target: { value: "admin@school.de" },
    });

    const form = document.getElementById("invite-admin-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(
        screen.getByText("Diese E-Mail-Adresse wurde bereits eingeladen"),
      ).toBeInTheDocument();
    });
  });

  it("shows generic 409 error when not email conflict", async () => {
    mockInviteSchoolAdmin.mockRejectedValue(
      new OperatorApiError("some conflict", 409),
    );

    render(<InviteAdminModal {...defaultProps} />);
    fireEvent.change(screen.getByLabelText("E-Mail"), {
      target: { value: "admin@school.de" },
    });

    const form = document.getElementById("invite-admin-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(screen.getByText("some conflict")).toBeInTheDocument();
    });
  });

  it("logs non-409 errors", async () => {
    mockInviteSchoolAdmin.mockRejectedValue(new Error("Server error"));
    const consoleError = vi
      .spyOn(console, "error")
      // eslint-disable-next-line @typescript-eslint/no-empty-function
      .mockImplementation(() => {});

    render(<InviteAdminModal {...defaultProps} />);
    fireEvent.change(screen.getByLabelText("E-Mail"), {
      target: { value: "admin@school.de" },
    });

    const form = document.getElementById("invite-admin-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(consoleError).toHaveBeenCalledWith(
        "invite_admin_failed",
        expect.objectContaining({ error: "Server error" }),
      );
    });
    consoleError.mockRestore();
  });

  it("logs non-Error exceptions as strings", async () => {
    mockInviteSchoolAdmin.mockRejectedValue("string error");
    const consoleError = vi
      .spyOn(console, "error")
      // eslint-disable-next-line @typescript-eslint/no-empty-function
      .mockImplementation(() => {});

    render(<InviteAdminModal {...defaultProps} />);
    fireEvent.change(screen.getByLabelText("E-Mail"), {
      target: { value: "admin@school.de" },
    });

    const form = document.getElementById("invite-admin-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(consoleError).toHaveBeenCalledWith(
        "invite_admin_failed",
        expect.objectContaining({ error: "string error" }),
      );
    });
    consoleError.mockRestore();
  });

  it("shows saving state", async () => {
    let resolveInvite: (value: unknown) => void;
    mockInviteSchoolAdmin.mockReturnValue(
      new Promise((resolve) => {
        resolveInvite = resolve;
      }),
    );

    render(<InviteAdminModal {...defaultProps} />);
    fireEvent.change(screen.getByLabelText("E-Mail"), {
      target: { value: "admin@school.de" },
    });

    const form = document.getElementById("invite-admin-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(screen.getByText("Wird gesendet...")).toBeInTheDocument();
    });

    resolveInvite!(mockInvitation);

    await waitFor(() => {
      expect(screen.queryByText("Wird gesendet...")).not.toBeInTheDocument();
    });
  });

  it("disables submit when email is empty", () => {
    render(<InviteAdminModal {...defaultProps} />);
    const footer = screen.getByTestId("modal-footer");
    const submitButton = footer.querySelector("button:last-child");
    expect(submitButton).toBeDisabled();
  });

  it("calls onClose when cancel is clicked", () => {
    render(<InviteAdminModal {...defaultProps} />);
    fireEvent.click(screen.getByText("Abbrechen"));
    expect(mockOnClose).toHaveBeenCalled();
  });

  it("resets form when modal reopens", () => {
    const { rerender } = render(
      <InviteAdminModal {...defaultProps} isOpen={true} />,
    );

    fireEvent.change(screen.getByLabelText("E-Mail"), {
      target: { value: "old@email.de" },
    });

    rerender(<InviteAdminModal {...defaultProps} isOpen={false} />);
    rerender(<InviteAdminModal {...defaultProps} isOpen={true} />);

    const emailInput = screen.getByLabelText("E-Mail");
    expect(emailInput.value).toBe("");
  });

  it("clears email error when typing", async () => {
    render(<InviteAdminModal {...defaultProps} />);
    const form = document.getElementById("invite-admin-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(screen.getByText("E-Mail ist erforderlich")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText("E-Mail"), {
      target: { value: "a" },
    });
    expect(
      screen.queryByText("E-Mail ist erforderlich"),
    ).not.toBeInTheDocument();
  });

  it("renders empty title when school is null", () => {
    render(<InviteAdminModal {...defaultProps} school={null} />);
    expect(screen.getByText("Admin einladen —")).toBeInTheDocument();
  });

  it("shows failed delivery status", async () => {
    const failedInvitation: Invitation = {
      ...mockInvitation,
      deliveryStatus: "failed",
    };
    mockInviteSchoolAdmin.mockResolvedValue(failedInvitation);

    render(<InviteAdminModal {...defaultProps} />);
    fireEvent.change(screen.getByLabelText("E-Mail"), {
      target: { value: "admin@school.de" },
    });

    const form = document.getElementById("invite-admin-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(screen.getByText("Fehlgeschlagen")).toBeInTheDocument();
    });
  });

  it("shows pending delivery status", async () => {
    const pendingInvitation: Invitation = {
      ...mockInvitation,
      deliveryStatus: "pending",
    };
    mockInviteSchoolAdmin.mockResolvedValue(pendingInvitation);

    render(<InviteAdminModal {...defaultProps} />);
    fireEvent.change(screen.getByLabelText("E-Mail"), {
      target: { value: "admin@school.de" },
    });

    const form = document.getElementById("invite-admin-form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(screen.getByText("Ausstehend")).toBeInTheDocument();
    });
  });
});
