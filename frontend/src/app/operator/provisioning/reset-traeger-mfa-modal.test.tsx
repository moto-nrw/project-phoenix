import type { ReactNode } from "react";
import {
  render as renderPlain,
  screen,
  fireEvent,
  waitFor,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

function render(ui: React.ReactElement) {
  return renderPlain(ui, { wrapper: ToastProvider });
}

const { mockReset } = vi.hoisted(() => ({ mockReset: vi.fn() }));

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
  operatorProvisioningService: { resetTraegerMemberMfa: mockReset },
}));

import { ResetTraegerMfaModal } from "./reset-traeger-mfa-modal";

const member = {
  accountId: "42",
  email: "buero@traeger.example",
  status: "active" as const,
  roles: ["administration" as const],
  activatedAt: null,
};

describe("ResetTraegerMfaModal", () => {
  beforeEach(() => {
    mockReset.mockReset();
  });

  it("resets the second factor with the reason for the audit", async () => {
    mockReset.mockResolvedValue(undefined);
    const onClose = vi.fn();
    const onReset = vi.fn().mockResolvedValue(undefined);
    render(
      <ResetTraegerMfaModal
        member={member}
        onClose={onClose}
        organizationId="7"
        onReset={onReset}
      />,
    );

    const submit = screen.getByRole("button", { name: "Zurücksetzen" });
    expect(submit).toBeDisabled();
    fireEvent.change(screen.getByLabelText(/Grund/), {
      target: { value: "  Postfach verloren  " },
    });
    fireEvent.click(submit);

    await waitFor(() =>
      expect(mockReset).toHaveBeenCalledWith("7", "42", "Postfach verloren"),
    );
    await waitFor(() => expect(onReset).toHaveBeenCalled());
    expect(onClose).toHaveBeenCalled();
  });

  it("keeps a refused reset in the dialog", async () => {
    mockReset.mockRejectedValue(
      new ApiError("Account is no member of this Träger-Büro", 404, {
        code: "identity.account_not_found",
      }),
    );
    const onClose = vi.fn();
    render(
      <ResetTraegerMfaModal
        member={member}
        onClose={onClose}
        organizationId="7"
        onReset={vi.fn()}
      />,
    );

    fireEvent.change(screen.getByLabelText(/Grund/), {
      target: { value: "Gesperrt" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Zurücksetzen" }));

    expect(
      await screen.findByText(
        catalogText(
          "identity.account_not_found",
          "das Zurücksetzen der Zwei-Faktor-Anmeldung",
        ),
      ),
    ).toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
  });
});
