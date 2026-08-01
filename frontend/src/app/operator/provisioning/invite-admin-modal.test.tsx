import type { ReactNode } from "react";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const { mockInviteSchoolAdmin, mockLoggerError } = vi.hoisted(() => ({
  mockInviteSchoolAdmin: vi.fn(),
  mockLoggerError: vi.fn(),
}));

vi.mock("~/components/ui/modal", () => ({
  Modal: ({
    isOpen,
    title,
    children,
    footer,
  }: {
    isOpen: boolean;
    title: string;
    children: ReactNode;
    footer?: ReactNode;
  }) =>
    isOpen ? (
      <div data-testid="modal">
        <h2>{title}</h2>
        {children}
        <div>{footer}</div>
      </div>
    ) : null,
}));

vi.mock("~/lib/operator/provisioning-api", () => ({
  operatorProvisioningService: {
    inviteSchoolAdmin: mockInviteSchoolAdmin,
  },
}));

vi.mock("~/lib/hooks/use-scroll-to-error", () => ({
  useScrollToError: () => vi.fn(),
}));

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({
    error: mockLoggerError,
  }),
}));

import { InviteAdminModal } from "./invite-admin-modal";

function selectCustomOption(
  labelMatcher: RegExp | string,
  optionName: RegExp | string,
) {
  fireEvent.click(screen.getByLabelText(labelMatcher));
  fireEvent.click(screen.getByRole("option", { name: optionName }));
}

const invitationResult = {
  id: "55",
  email: "admin@example.com",
  roleId: "21",
  roleName: "admin",
  expiresAt: "2026-06-06T12:00:00Z",
  firstName: "Ada",
  lastName: "Lovelace",
  position: null,
  caregiverEnabled: false,
  createdBy: "31",
  creator: "operator@example.com",
  deliveryStatus: "sent",
  emailSentAt: "2026-06-05T12:00:00Z",
  emailError: null,
  emailRetryCount: 0,
};

function renderModal(
  props: Partial<Parameters<typeof InviteAdminModal>[0]> = {},
) {
  const defaultProps = {
    isOpen: true,
    onClose: vi.fn(),
    schoolId: "10",
    schoolName: "Test Schule",
    ...props,
  };
  return {
    ...render(<InviteAdminModal {...defaultProps} />),
    ...defaultProps,
  };
}

function fillInviteForm() {
  fireEvent.change(screen.getByLabelText(/E-Mail/), {
    target: { value: "admin@example.com" },
  });
  fireEvent.change(screen.getByLabelText(/Vorname/), {
    target: { value: "Ada" },
  });
  fireEvent.change(screen.getByLabelText(/Nachname/), {
    target: { value: "Lovelace" },
  });
}

describe("InviteAdminModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockInviteSchoolAdmin.mockResolvedValue(invitationResult);
  });

  it("does not render when closed", () => {
    renderModal({ isOpen: false });
    expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
  });

  it("renders caregiver toggle unchecked and hides position by default", () => {
    renderModal();

    expect(
      screen.getByText("Admin einladen — Test Schule"),
    ).toBeInTheDocument();
    expect(
      screen.getByLabelText("Auch als Betreuer einsetzen"),
    ).not.toBeChecked();
    expect(screen.queryByLabelText("Position")).not.toBeInTheDocument();
  });

  it("shows position only when caregiver capability is enabled", () => {
    renderModal();

    fireEvent.click(screen.getByLabelText("Auch als Betreuer einsetzen"));

    expect(screen.getByLabelText("Position")).toBeInTheDocument();
    fireEvent.click(screen.getByLabelText("Position"));
    expect(
      screen.getByRole("option", { name: "Pädagogische Fachkraft" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("option", { name: "OGS-Büro" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Extern" })).toBeInTheDocument();
  });

  it("submits a plain admin invitation without caregiver fields", async () => {
    renderModal();

    fillInviteForm();
    fireEvent.click(screen.getByText("Einladung senden"));

    await waitFor(() => {
      expect(mockInviteSchoolAdmin).toHaveBeenCalledWith("10", {
        email: "admin@example.com",
        first_name: "Ada",
        last_name: "Lovelace",
      });
    });
  });

  it("submits caregiver capability and position when enabled", async () => {
    mockInviteSchoolAdmin.mockResolvedValue({
      ...invitationResult,
      caregiverEnabled: true,
      position: "OGS-Büro",
    });
    renderModal();

    fillInviteForm();
    fireEvent.click(screen.getByLabelText("Auch als Betreuer einsetzen"));
    selectCustomOption("Position", "OGS-Büro");
    fireEvent.click(screen.getByText("Einladung senden"));

    await waitFor(() => {
      expect(mockInviteSchoolAdmin).toHaveBeenCalledWith("10", {
        email: "admin@example.com",
        first_name: "Ada",
        last_name: "Lovelace",
        caregiver_enabled: true,
        position: "OGS-Büro",
      });
    });

    await waitFor(() => {
      expect(screen.getByText("Verwaltung + Betreuung")).toBeInTheDocument();
    });
  });

  it("resets caregiver fields when reopened", () => {
    const { rerender, onClose } = renderModal();

    fireEvent.click(screen.getByLabelText("Auch als Betreuer einsetzen"));
    selectCustomOption("Position", "Extern");

    rerender(
      <InviteAdminModal
        isOpen={false}
        onClose={onClose}
        schoolId="10"
        schoolName="Test Schule"
      />,
    );
    rerender(
      <InviteAdminModal
        isOpen={true}
        onClose={onClose}
        schoolId="10"
        schoolName="Test Schule"
      />,
    );

    expect(
      screen.getByLabelText("Auch als Betreuer einsetzen"),
    ).not.toBeChecked();
    expect(screen.queryByLabelText("Position")).not.toBeInTheDocument();
  });

  it("shows API errors and logs them", async () => {
    mockInviteSchoolAdmin.mockRejectedValue(
      new Error("E-Mail bereits vergeben"),
    );
    renderModal();

    fillInviteForm();
    fireEvent.click(screen.getByText("Einladung senden"));

    await waitFor(() => {
      expect(screen.getByText("E-Mail bereits vergeben")).toBeInTheDocument();
      expect(mockLoggerError).toHaveBeenCalledWith(
        "admin_invite_failed",
        expect.objectContaining({ error: "E-Mail bereits vergeben" }),
      );
    });
  });
});
