import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import { FormErrorAlert } from "~/components/ui/form-error-alert";
import type { FormErrorInput } from "~/components/ui/form-error";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

import { StaffNoticeModal } from "./staff-notice-modal";

vi.mock("~/components/ui/form-modal", () => ({
  // Wie der echte FormModal: der Fehler steht oben im Bearbeitungsbereich.
  FormModal: ({
    isOpen,
    children,
    footer,
    error,
  }: {
    isOpen: boolean;
    children: ReactNode;
    footer?: ReactNode;
    error?: FormErrorInput;
  }) =>
    isOpen ? (
      <div>
        <FormErrorAlert message={error} />
        {children}
        {footer}
      </div>
    ) : null,
}));

function renderModal(onSubmit = vi.fn()) {
  const onClose = vi.fn();
  render(
    <StaffNoticeModal
      isOpen
      notice={null}
      onClose={onClose}
      onSubmit={onSubmit}
    />,
  );
  return { onSubmit, onClose };
}

describe("StaffNoticeModal", () => {
  it("marks a missing title at the field before sending", async () => {
    const { onSubmit } = renderModal();

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText("Bitte prüfen Sie die markierten Felder."),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Titel")).toHaveAttribute(
      "aria-invalid",
      "true",
    );
    expect(
      screen.getByText("Bitte geben Sie einen Titel an."),
    ).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  // #2517: Katalogtext statt Serversatz; der Fehler bleibt im Dialog und
  // „Wiederholen“ sendet den aktuellen Stand.
  it("keeps a failed save in the dialog and retries with the current title", async () => {
    const onSubmit = vi
      .fn()
      .mockRejectedValueOnce(
        new ApiError("notice kaputt", 500, { code: "general.server" }),
      )
      .mockResolvedValueOnce(undefined);
    const { onClose } = renderModal(onSubmit);

    fireEvent.change(screen.getByLabelText("Titel"), {
      target: { value: "Dienstbesprechung" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(
        catalogText("general.server", "das Speichern der Tagesinformation"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/kaputt/)).not.toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText("Titel"), {
      target: { value: "Teamsitzung" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    await waitFor(() => expect(onClose).toHaveBeenCalledOnce());
    expect(onSubmit).toHaveBeenLastCalledWith(
      expect.objectContaining({ title: "Teamsitzung" }),
    );
  });
});
