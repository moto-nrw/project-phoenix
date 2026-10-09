import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import type { StaffAbsenceRequestRow } from "~/lib/staff-api";
import { catalogText } from "~/test/error-catalog-text";

const mocks = vi.hoisted(() => ({
  deny: vi.fn(),
  question: vi.fn(),
  success: vi.fn(),
  error: vi.fn(),
}));

vi.mock("~/lib/staff-api", () => ({
  staffAbsenceService: { deny: mocks.deny, question: mocks.question },
}));

vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({ success: mocks.success, error: mocks.error }),
}));

import { DenyAbsenceModal } from "./absence-decision-modals";

function row(): StaffAbsenceRequestRow {
  return {
    id: "17",
    staff_id: "42",
    staff_name: "Mira Muster",
    absence_type: "vacation",
    date_start: "2027-07-10",
    date_end: "2027-07-11",
    half_day: false,
    note: "",
    status: "requested",
    working_days: 2,
    requested_at: "2027-06-02T08:00:00Z",
  };
}

function renderDeny() {
  const onDenied = vi.fn();
  const onClose = vi.fn();
  render(
    <DenyAbsenceModal absence={row()} onClose={onClose} onDenied={onDenied} />,
  );
  return { onDenied, onClose };
}

describe("DenyAbsenceModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("marks a missing reason at the field without sending", async () => {
    renderDeny();

    fireEvent.click(screen.getByRole("button", { name: "Ablehnen" }));

    expect(
      await screen.findByText("Bitte prüfen Sie die markierten Felder."),
    ).toBeInTheDocument();
    const field = screen.getByLabelText("Begründung");
    expect(field).toHaveAttribute("aria-invalid", "true");
    await waitFor(() => expect(field).toHaveFocus());
    expect(mocks.deny).not.toHaveBeenCalled();
  });

  it("keeps a refused decision in the dialog and offers no success", async () => {
    mocks.deny.mockRejectedValue(
      new ApiError("already decided", 409, {
        code: "workforce.absence_already_decided",
      }),
    );
    const { onDenied } = renderDeny();

    fireEvent.change(screen.getByLabelText("Begründung"), {
      target: { value: "Zu viele Abwesenheiten" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Ablehnen" }));

    const dialog = await screen.findByRole("dialog");
    await waitFor(() =>
      expect(dialog).toHaveTextContent(
        catalogText("workforce.absence_already_decided", "die Anfrage"),
      ),
    );
    expect(mocks.deny).toHaveBeenCalledWith("17", "Zu viele Abwesenheiten");
    expect(mocks.success).not.toHaveBeenCalled();
    expect(mocks.error).not.toHaveBeenCalled();
    expect(onDenied).not.toHaveBeenCalled();
  });

  it("retries with the current note after a server error", async () => {
    mocks.deny
      .mockRejectedValueOnce(
        new ApiError("boom", 500, { code: "general.server", instance: "r-1" }),
      )
      .mockResolvedValueOnce(undefined);
    const { onDenied } = renderDeny();

    fireEvent.change(screen.getByLabelText("Begründung"), {
      target: { value: "Erster Text" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Ablehnen" }));
    expect(
      await screen.findByText(catalogText("general.server", "die Anfrage")),
    ).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Begründung"), {
      target: { value: "Zweiter Text" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    await waitFor(() => expect(onDenied).toHaveBeenCalled());
    expect(mocks.deny).toHaveBeenLastCalledWith("17", "Zweiter Text");
    expect(mocks.success).toHaveBeenCalledWith("Der Antrag ist abgelehnt.");
  });
});
