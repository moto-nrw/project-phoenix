import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  confirmRenewal,
  fetchStatus,
  listEnrollmentChangeRequests,
  patchStatus,
  replyEnrollmentChangeRequest,
  withdrawStatus,
  type EnrollmentChangeRequest,
  type StatusResponse,
} from "~/lib/enrollment-submission-api";
import { EnrollmentStatusView } from "./enrollment-status-view";

vi.mock("~/lib/enrollment-submission-api", () => ({
  confirmRenewal: vi.fn(),
  fetchStatus: vi.fn(),
  listEnrollmentChangeRequests: vi.fn(),
  patchStatus: vi.fn(),
  replyEnrollmentChangeRequest: vi.fn(),
  withdrawStatus: vi.fn(),
}));

const mockFetchStatus = vi.mocked(fetchStatus);
const mockListEnrollmentChangeRequests = vi.mocked(
  listEnrollmentChangeRequests,
);
const mockPatchStatus = vi.mocked(patchStatus);
const mockReplyEnrollmentChangeRequest = vi.mocked(
  replyEnrollmentChangeRequest,
);
const mockWithdrawStatus = vi.mocked(withdrawStatus);
const mockConfirmRenewal = vi.mocked(confirmRenewal);

function status(overrides: Partial<StatusResponse> = {}): StatusResponse {
  return {
    request_id: "99",
    guardian_first_name: "Mara",
    guardian_last_name: "Muster",
    guardian_email: "mara@example.test",
    guardian_phone: "+49 221 1234567",
    submitted_at: "2026-01-15T10:00:00Z",
    withdrawn_at: null,
    edit_mode: "direct_edit",
    children: [
      {
        id: "7",
        first_name: "Lina",
        last_name: "Muster",
        status: "submitted",
      },
    ],
    ...overrides,
  };
}

function changeRequest(
  overrides: Partial<EnrollmentChangeRequest> = {},
): EnrollmentChangeRequest {
  return {
    id: "42",
    request_id: "99",
    origin: "parent",
    status: "pending_review",
    parent_note: "Name korrigieren",
    admin_decision_note: null,
    base_snapshot: { guardian_first_name: "Daniela" },
    proposed_snapshot: { guardian_first_name: "Danielaaaa" },
    diff: { changed: ["guardian_first_name"] },
    created_at: "2026-06-28T11:09:00.000Z",
    updated_at: "2026-06-28T11:09:00.000Z",
    reviewed_at: null,
    reviewed_by_account_id: null,
    messages: [],
    ...overrides,
  };
}

describe("EnrollmentStatusView", () => {
  beforeEach(() => {
    mockFetchStatus.mockReset();
    mockListEnrollmentChangeRequests.mockReset();
    mockListEnrollmentChangeRequests.mockResolvedValue([]);
    mockPatchStatus.mockReset();
    mockReplyEnrollmentChangeRequest.mockReset();
    mockWithdrawStatus.mockReset();
    mockConfirmRenewal.mockReset();
  });

  it("shows loading and then a missing-link state", async () => {
    mockFetchStatus.mockResolvedValueOnce(null);

    render(<EnrollmentStatusView token="missing" />);

    expect(screen.getByText("Status wird geladen…")).toBeInTheDocument();
    expect(await screen.findByText("Status-Link ungültig")).toBeInTheDocument();
  });

  it("renders editable submitted requests and saves contact changes", async () => {
    mockFetchStatus
      .mockResolvedValueOnce(status())
      .mockResolvedValueOnce(
        status({ guardian_first_name: "Maria", guardian_phone: "+49 30" }),
      );
    mockPatchStatus.mockResolvedValueOnce();

    render(<EnrollmentStatusView token="tok" justSubmitted />);

    expect(
      await screen.findByText("Danke. Ihre Anmeldung wurde übermittelt."),
    ).toBeInTheDocument();
    expect(screen.getByText("Lina Muster")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    fireEvent.change(screen.getByLabelText("Vorname"), {
      target: { value: " Maria " },
    });
    fireEvent.change(screen.getByLabelText("Telefon, optional"), {
      target: { value: " +49 30 " },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(mockPatchStatus).toHaveBeenCalledWith("tok", {
        guardian_first_name: "Maria",
        guardian_last_name: "Muster",
        guardian_phone: "+49 30",
      });
    });
    expect(
      await screen.findByText("Änderungen gespeichert."),
    ).toBeInTheDocument();
    expect(mockFetchStatus).toHaveBeenCalledTimes(2);
  });

  it("shows submitted change-request diffs to parents", async () => {
    const submittedChangeRequest = changeRequest({
      base_snapshot: {
        guardian_first_name: "Daniela",
        additional_guardians: [{ first_name: "Coco", last_name: "Sommer" }],
      },
      proposed_snapshot: {
        guardian_first_name: "Danielaaaa",
        additional_guardians: [{ first_name: "Coco", last_name: "Sommerer" }],
      },
      diff: { changed: ["additional_guardians", "guardian_first_name"] },
    });
    mockFetchStatus.mockResolvedValueOnce(status());
    mockListEnrollmentChangeRequests.mockResolvedValueOnce([
      submittedChangeRequest,
    ]);

    render(<EnrollmentStatusView token="tok" />);

    expect(
      await screen.findByText("Angefragte Änderungen"),
    ).toBeInTheDocument();
    expect(screen.getAllByText("Vorname").length).toBeGreaterThan(0);
    expect(screen.getByText("Daniela")).toBeInTheDocument();
    expect(screen.getByText("Danielaaaa")).toBeInTheDocument();
    expect(screen.getByText("Weitere Person 1 · Nachname")).toBeInTheDocument();
    expect(screen.getByText("Sommer")).toBeInTheDocument();
    expect(screen.getByText("Sommerer")).toBeInTheDocument();
    expect(screen.queryByText("1 Eintrag")).not.toBeInTheDocument();
  });

  it("keeps open change requests visible but hides second creation", async () => {
    mockFetchStatus.mockResolvedValueOnce(
      status({
        edit_mode: "change_request",
        children: [
          {
            id: "7",
            first_name: "Lina",
            last_name: "Muster",
            status: "approved",
          },
        ],
      }),
    );
    mockListEnrollmentChangeRequests.mockResolvedValueOnce([
      changeRequest({
        status: "needs_parent_response",
        messages: [
          {
            id: "9",
            author_type: "staff",
            body: "Bitte Nachweis ergänzen.",
            internal_only: false,
            created_at: "2026-06-28T12:00:00.000Z",
          },
        ],
      }),
    ]);

    render(<EnrollmentStatusView token="tok" />);

    expect(
      await screen.findByText("Angefragte Änderungen"),
    ).toBeInTheDocument();
    expect(screen.getByText("Bitte Nachweis ergänzen.")).toBeInTheDocument();
    expect(screen.getByLabelText("Antwort")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Antwort senden" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Änderung anfragen" }),
    ).not.toBeInTheDocument();
  });

  it("labels completed staff corrections separately from parent requests", async () => {
    mockFetchStatus.mockResolvedValueOnce(status());
    mockListEnrollmentChangeRequests.mockResolvedValueOnce([
      changeRequest({ origin: "admin", status: "approved" }),
    ]);

    render(<EnrollmentStatusView token="tok" />);

    expect(
      await screen.findByText(/Von der OGS korrigiert am/),
    ).toBeInTheDocument();
  });

  it("hides edit and change-request CTAs when backend reports no edit mode", async () => {
    mockFetchStatus.mockResolvedValueOnce(
      status({
        edit_mode: "none",
        children: [
          {
            id: "7",
            first_name: "Lina",
            last_name: "Muster",
            status: "approved",
          },
        ],
      }),
    );

    render(<EnrollmentStatusView token="tok" />);

    expect(await screen.findByText("Lina Muster")).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Anmeldung bearbeiten" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Änderung anfragen" }),
    ).not.toBeInTheDocument();
  });

  it("shows change-request CTA only when backend reports change-request mode", async () => {
    mockFetchStatus.mockResolvedValueOnce(
      status({
        edit_mode: "change_request",
        children: [
          {
            id: "7",
            first_name: "Lina",
            last_name: "Muster",
            status: "approved",
          },
        ],
      }),
    );

    render(<EnrollmentStatusView token="tok" />);

    expect(
      await screen.findAllByRole("link", { name: "Änderung anfragen" }),
    ).toHaveLength(2);
    expect(
      screen.queryByRole("link", { name: "Anmeldung bearbeiten" }),
    ).not.toBeInTheDocument();
  });

  it("confirms pending renewals and reloads the status", async () => {
    mockFetchStatus
      .mockResolvedValueOnce(
        status({
          children: [
            {
              id: "7",
              first_name: "Lina",
              last_name: "Muster",
              status: "pending_renewal",
            },
          ],
        }),
      )
      .mockResolvedValueOnce(status());
    mockConfirmRenewal.mockResolvedValueOnce(1);

    render(<EnrollmentStatusView token="tok" />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Anmeldung bestätigen" }),
    );

    await waitFor(() => {
      expect(mockConfirmRenewal).toHaveBeenCalledWith("tok");
    });
    expect(await screen.findByText(/Anmeldung bestätigt/)).toBeInTheDocument();
  });

  it("withdraws a single child when multiple requests are still open", async () => {
    mockFetchStatus
      .mockResolvedValueOnce(
        status({
          children: [
            {
              id: "7",
              first_name: "Lina",
              last_name: "Muster",
              status: "submitted",
            },
            {
              id: "8",
              first_name: "Noah",
              last_name: "Muster",
              status: "under_review",
            },
          ],
        }),
      )
      .mockResolvedValueOnce(status());
    mockWithdrawStatus.mockResolvedValueOnce();

    render(<EnrollmentStatusView token="tok" />);

    const buttons = await screen.findAllByRole("button", {
      name: "Dieses Kind zurückziehen",
    });
    fireEvent.click(buttons[0]!);

    expect(
      await screen.findByText("Anmeldung für dieses Kind zurückziehen?"),
    ).toBeInTheDocument();
    expect(mockWithdrawStatus).not.toHaveBeenCalled();

    fireEvent.click(
      screen.getByRole("button", { name: "Endgültig zurückziehen" }),
    );

    await waitFor(() => {
      expect(mockWithdrawStatus).toHaveBeenCalledWith("tok", "7");
    });
    expect(
      await screen.findByText("Anmeldung für dieses Kind zurückgezogen."),
    ).toBeInTheDocument();
  });

  it("hides the withdraw section directly after submission", async () => {
    mockFetchStatus.mockResolvedValueOnce(status());

    render(<EnrollmentStatusView token="tok" justSubmitted />);

    expect(await screen.findByText("Lina Muster")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Anmeldung zurückziehen" }),
    ).not.toBeInTheDocument();
  });

  it("hides per-child withdraw buttons directly after submission", async () => {
    mockFetchStatus.mockResolvedValueOnce(
      status({
        children: [
          {
            id: "7",
            first_name: "Lina",
            last_name: "Muster",
            status: "submitted",
          },
          {
            id: "8",
            first_name: "Noah",
            last_name: "Muster",
            status: "submitted",
          },
        ],
      }),
    );

    render(<EnrollmentStatusView token="tok" justSubmitted />);

    expect(await screen.findByText("Noah Muster")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Dieses Kind zurückziehen" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Gesamte Anmeldung zurückziehen" }),
    ).not.toBeInTheDocument();
  });

  it("withdraws everything only after confirming in the modal", async () => {
    mockFetchStatus.mockResolvedValueOnce(status()).mockResolvedValueOnce(
      status({
        withdrawn_at: "2026-01-16T10:00:00Z",
        children: [
          {
            id: "7",
            first_name: "Lina",
            last_name: "Muster",
            status: "withdrawn",
          },
        ],
      }),
    );
    mockWithdrawStatus.mockResolvedValueOnce();

    render(<EnrollmentStatusView token="tok" />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Anmeldung zurückziehen" }),
    );
    expect(
      await screen.findByText("Gesamte Anmeldung zurückziehen?"),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));
    expect(mockWithdrawStatus).not.toHaveBeenCalled();

    fireEvent.click(
      screen.getByRole("button", { name: "Anmeldung zurückziehen" }),
    );
    fireEvent.click(
      await screen.findByRole("button", { name: "Endgültig zurückziehen" }),
    );

    await waitFor(() => {
      expect(mockWithdrawStatus).toHaveBeenCalledWith("tok", undefined);
    });
    expect(
      await screen.findByText("Anmeldung zurückgezogen."),
    ).toBeInTheDocument();
  });

  it("shows load and mutation errors", async () => {
    mockFetchStatus.mockRejectedValueOnce(new Error("Netz kaputt"));
    const { rerender } = render(<EnrollmentStatusView token="tok" />);

    expect(await screen.findByText("Netz kaputt")).toBeInTheDocument();

    mockFetchStatus.mockResolvedValueOnce(status());
    mockWithdrawStatus.mockRejectedValueOnce(new Error("Nicht möglich"));
    rerender(<EnrollmentStatusView token="tok2" />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Anmeldung zurückziehen" }),
    );
    fireEvent.click(
      await screen.findByRole("button", { name: "Endgültig zurückziehen" }),
    );

    expect(await screen.findByText("Nicht möglich")).toBeInTheDocument();
  });
});
