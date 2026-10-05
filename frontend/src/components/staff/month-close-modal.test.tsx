import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

import { MonthCloseReasonModal } from "./month-close-modal";

vi.mock("~/components/ui/modal", () => ({
  Modal: ({
    isOpen,
    title,
    children,
    footer,
  }: {
    isOpen: boolean;
    title: string;
    children: React.ReactNode;
    footer?: React.ReactNode;
  }) =>
    isOpen ? (
      <div role="dialog" aria-label={title}>
        {children}
        {footer}
      </div>
    ) : null,
}));

const success = vi.fn();
vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({ success, error: vi.fn(), info: vi.fn() }),
}));

function renderModal(onSubmit: (reason: string) => Promise<void>) {
  const onClose = vi.fn();
  render(
    <MonthCloseReasonModal
      title="Monat wieder öffnen"
      description="Hebt den Abschluss auf."
      submitLabel="Monat wieder öffnen"
      successMessage="Der Monat ist wieder geöffnet."
      errorObject="das Öffnen des Monats"
      onSubmit={onSubmit}
      onClose={onClose}
    />,
  );
  fireEvent.change(screen.getByLabelText("Begründung (Pflicht)"), {
    target: { value: "Korrektur" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Monat wieder öffnen" }));
  return onClose;
}

describe("MonthCloseReasonModal", () => {
  it("keeps a refused action in the dialog with the catalog text", async () => {
    const onClose = renderModal(() =>
      Promise.reject(
        new ApiError("later month closed", 409, {
          code: "workforce.later_month_closed",
        }),
      ),
    );

    expect(
      await screen.findByText(
        catalogText("workforce.later_month_closed", "das Öffnen des Monats"),
      ),
    ).toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
    expect(success).not.toHaveBeenCalled();
  });

  it("retries with the current reason after a server error", async () => {
    const onSubmit = vi
      .fn<(reason: string) => Promise<void>>()
      .mockRejectedValueOnce(
        new ApiError("boom", 500, { code: "general.server" }),
      )
      .mockResolvedValueOnce(undefined);
    const onClose = renderModal(onSubmit);

    await screen.findByText(
      catalogText("general.server", "das Öffnen des Monats"),
    );
    fireEvent.change(screen.getByLabelText("Begründung (Pflicht)"), {
      target: { value: "Korrektur Juli" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    await waitFor(() =>
      expect(onSubmit).toHaveBeenLastCalledWith("Korrektur Juli"),
    );
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(success).toHaveBeenCalledWith("Der Monat ist wieder geöffnet.");
  });
});
