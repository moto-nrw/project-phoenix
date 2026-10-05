import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
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
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";
import { EnrollmentStatusView } from "./enrollment-status-view";

const mockPathname = vi.hoisted(() => ({ value: "/anmeldung/status/tok" }));

const toastSuccess = vi.hoisted(() => vi.fn());
const toastError = vi.hoisted(() => vi.fn());
const showActionError = vi.hoisted(() => vi.fn());
vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({ success: toastSuccess, error: toastError }),
  useApiErrorDisplay: () => ({ show: showActionError }),
}));

vi.mock("next/navigation", () => ({
  usePathname: () => mockPathname.value,
}));

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
    mockPathname.value = "/anmeldung/status/tok";
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
    await waitFor(() => {
      expect(toastSuccess).toHaveBeenCalledWith(
        "Ihre Änderungen sind gespeichert.",
      );
    });
    expect(mockFetchStatus).toHaveBeenCalledTimes(2);
  });

  it("uses the renamed parent route for direct edits", async () => {
    mockPathname.value = "/parents/anmeldung/status/tok";
    mockFetchStatus.mockResolvedValueOnce(status());

    render(<EnrollmentStatusView token="tok" />);

    expect(
      await screen.findByRole("link", { name: "Anmeldung bearbeiten" }),
    ).toHaveAttribute("href", "/parents/anmeldung/status/tok/edit");
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

  it("points a fully taken-over enrollment at the parents app (ADR 0003)", async () => {
    mockFetchStatus.mockResolvedValueOnce(
      status({
        edit_mode: "none",
        children: [
          {
            id: "7",
            first_name: "Lina",
            last_name: "Muster",
            status: "approved",
            locked: true,
          },
        ],
      }),
    );

    render(<EnrollmentStatusView token="tok" />);

    expect(
      await screen.findByText("Änderungen laufen über die Eltern-App"),
    ).toBeInTheDocument();
    expect(
      screen.getAllByRole("link", { name: "Zur Eltern-App" })[0],
    ).toHaveAttribute("href", "/parents");
    expect(
      screen.queryByRole("link", { name: "Änderung anfragen" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Anmeldung zurückziehen" }),
    ).not.toBeInTheDocument();
  });

  it("sends a family without an account to the invitation, not the login (#3742)", async () => {
    mockFetchStatus.mockResolvedValueOnce(
      status({
        edit_mode: "none",
        has_parent_account: false,
        children: [
          {
            id: "7",
            first_name: "Lina",
            last_name: "Muster",
            status: "approved",
            locked: true,
          },
        ],
      }),
    );

    render(<EnrollmentStatusView token="tok" />);

    expect(
      await screen.findByText("Änderungen laufen über die Eltern-App"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Noch kein Zugang zur Eltern-App"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Öffnen Sie die E-Mail „Einladung zum Eltern-Portal“."),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Bitte melden Sie sich bei der OGS\.$/),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Zur Eltern-App" }),
    ).not.toBeInTheDocument();
  });

  it("keeps the login link for a family with an account (#3742)", async () => {
    mockFetchStatus.mockResolvedValueOnce(
      status({
        edit_mode: "none",
        has_parent_account: true,
        children: [
          {
            id: "7",
            first_name: "Lina",
            last_name: "Muster",
            status: "approved",
            locked: true,
          },
        ],
      }),
    );

    render(<EnrollmentStatusView token="tok" />);

    expect(
      (await screen.findAllByRole("link", { name: "Zur Eltern-App" }))[0],
    ).toHaveAttribute("href", "/parents");
    expect(
      screen.queryByText("Noch kein Zugang zur Eltern-App"),
    ).not.toBeInTheDocument();
    expect(screen.queryByText(/^Noch kein Zugang\?/)).not.toBeInTheDocument();
  });

  it("directs an existing but unavailable account to the OGS", async () => {
    mockFetchStatus.mockResolvedValueOnce(
      status({
        edit_mode: "none",
        has_parent_account: false,
        parent_portal_access: "contact_ogs",
        children: [
          {
            id: "7",
            first_name: "Lina",
            last_name: "Muster",
            status: "approved",
            locked: true,
          },
        ],
      }),
    );

    render(<EnrollmentStatusView token="tok" />);

    expect(
      await screen.findByText("Zugang zur Eltern-App nicht möglich"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Bitte melden Sie sich bei der OGS. Die OGS hilft Ihnen weiter.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(
        "Öffnen Sie die E-Mail „Einladung zum Eltern-Portal“.",
      ),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Zur Eltern-App" }),
    ).not.toBeInTheDocument();
  });

  it("names the invitation next to the login when the account is unknown", async () => {
    mockFetchStatus.mockResolvedValueOnce(
      status({
        edit_mode: "none",
        children: [
          {
            id: "7",
            first_name: "Lina",
            last_name: "Muster",
            status: "approved",
            locked: true,
          },
        ],
      }),
    );

    render(<EnrollmentStatusView token="tok" />);

    expect(
      (await screen.findAllByRole("link", { name: "Zur Eltern-App" }))[0],
    ).toHaveAttribute("href", "/parents");
    expect(screen.getByText(/^Noch kein Zugang\?/)).toBeInTheDocument();
  });

  it("drops the login link of a single taken-over child without an account", async () => {
    mockFetchStatus.mockResolvedValueOnce(
      status({
        edit_mode: "change_request",
        has_parent_account: false,
        children: [
          {
            id: "7",
            first_name: "Lina",
            last_name: "Muster",
            status: "approved",
            locked: true,
          },
          {
            id: "8",
            first_name: "Timo",
            last_name: "Muster",
            status: "waitlisted",
          },
        ],
      }),
    );

    render(<EnrollmentStatusView token="tok" />);

    expect(await screen.findByText("Timo Muster")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Änderungen für dieses Kind laufen über die Eltern-App.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Noch kein Zugang zur Eltern-App"),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Zur Eltern-App" }),
    ).not.toBeInTheDocument();
  });

  it("hides the login link of a single taken-over child with unavailable access", async () => {
    mockFetchStatus.mockResolvedValueOnce(
      status({
        edit_mode: "change_request",
        has_parent_account: false,
        parent_portal_access: "contact_ogs",
        children: [
          {
            id: "7",
            first_name: "Lina",
            last_name: "Muster",
            status: "approved",
            locked: true,
          },
          {
            id: "8",
            first_name: "Timo",
            last_name: "Muster",
            status: "waitlisted",
          },
        ],
      }),
    );

    render(<EnrollmentStatusView token="tok" />);

    expect(
      await screen.findByText("Zugang zur Eltern-App nicht möglich"),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Zur Eltern-App" }),
    ).not.toBeInTheDocument();
  });

  it("shows invitation instructions on the public parents status route", async () => {
    mockPathname.value = "/parents/anmeldung/status/tok";
    mockFetchStatus.mockResolvedValueOnce(
      status({
        edit_mode: "none",
        has_parent_account: false,
        children: [
          {
            id: "7",
            first_name: "Lina",
            last_name: "Muster",
            status: "approved",
            locked: true,
          },
        ],
      }),
    );

    render(<EnrollmentStatusView token="tok" />);

    expect(
      await screen.findByText("Noch kein Zugang zur Eltern-App"),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Zur Eltern-App" }),
    ).not.toBeInTheDocument();
  });

  it("keeps the change form for a sibling and marks only the locked child", async () => {
    mockFetchStatus.mockResolvedValueOnce(
      status({
        edit_mode: "change_request",
        children: [
          {
            id: "7",
            first_name: "Lina",
            last_name: "Muster",
            status: "approved",
            locked: true,
          },
          {
            id: "8",
            first_name: "Timo",
            last_name: "Muster",
            status: "waitlisted",
          },
        ],
      }),
    );

    render(<EnrollmentStatusView token="tok" />);

    expect(await screen.findByText("Timo Muster")).toBeInTheDocument();
    expect(
      screen.getAllByText(
        "Änderungen für dieses Kind laufen über die Eltern-App.",
      ),
    ).toHaveLength(1);
    expect(
      screen.queryByText("Änderungen laufen über die Eltern-App"),
    ).not.toBeInTheDocument();
    expect(
      screen.getAllByRole("link", { name: "Änderung anfragen" }).length,
    ).toBeGreaterThan(0);
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
    await waitFor(() => {
      expect(toastSuccess).toHaveBeenCalledWith(
        expect.stringContaining("Anmeldung bestätigt"),
      );
    });
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
    await waitFor(() => {
      expect(toastSuccess).toHaveBeenCalledWith(
        "Anmeldung für dieses Kind zurückgezogen.",
      );
    });
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
    await waitFor(() => {
      expect(toastSuccess).toHaveBeenCalledWith("Anmeldung zurückgezogen.");
    });
  });

  it("shows a failed load with retry where the status would be", async () => {
    mockFetchStatus
      .mockRejectedValueOnce(new ApiError("Status", 503))
      .mockResolvedValueOnce(status());
    render(<EnrollmentStatusView token="tok" />);

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Anmeldung"),
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    expect(await screen.findByText("Lina Muster")).toBeInTheDocument();
    expect(mockFetchStatus).toHaveBeenCalledTimes(2);
  });

  it("keeps a failed withdrawal in the open confirmation dialog", async () => {
    mockFetchStatus.mockResolvedValue(status());
    const failure = new ApiError("Nicht möglich", 409);
    mockWithdrawStatus
      .mockRejectedValueOnce(failure)
      .mockResolvedValueOnce(undefined);
    render(<EnrollmentStatusView token="tok" />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Anmeldung zurückziehen" }),
    );
    fireEvent.click(
      await screen.findByRole("button", { name: "Endgültig zurückziehen" }),
    );

    const dialog = await screen.findByRole("dialog");
    expect(
      await within(dialog).findByText(
        catalogText("general.business_rejection", "die Anmeldung"),
      ),
    ).toBeInTheDocument();
    expect(showActionError).not.toHaveBeenCalled();
    expect(screen.queryByText("Nicht möglich")).not.toBeInTheDocument();
    expect(
      within(dialog).queryByRole("button", { name: "Wiederholen" }),
    ).not.toBeInTheDocument();

    // Trying again goes through the dialog's confirm button.
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Endgültig zurückziehen" }),
    );
    await waitFor(() => expect(mockWithdrawStatus).toHaveBeenCalledTimes(2));
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
  });

  it("keeps a failed contact change in the form and marks the field", async () => {
    mockFetchStatus.mockResolvedValueOnce(status());
    mockPatchStatus.mockRejectedValueOnce(
      new ApiError("bad", 400, {
        code: "general.input",
        errors: [{ field: "guardian_phone", reason: "invalid" }],
      }),
    );
    render(<EnrollmentStatusView token="tok" />);

    fireEvent.click(await screen.findByRole("button", { name: "Bearbeiten" }));
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(catalogText("general.input", "die Änderung")),
    ).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByLabelText("Telefon, optional")).toHaveAttribute(
        "aria-invalid",
        "true",
      );
    });
    expect(
      screen.getByRole("button", { name: "Speichern" }),
    ).toBeInTheDocument();
  });

  it("shows a failed change-request list instead of its empty text", async () => {
    mockFetchStatus.mockResolvedValueOnce(
      status({ edit_mode: "change_request" }),
    );
    mockListEnrollmentChangeRequests.mockReset();
    mockListEnrollmentChangeRequests.mockRejectedValueOnce(
      new ApiError("list", 500),
    );
    render(<EnrollmentStatusView token="tok" />);

    expect(
      await screen.findByText(
        catalogText("general.server", "die Liste der Änderungsanfragen"),
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Noch keine Änderungsanfrage gesendet."),
    ).not.toBeInTheDocument();
  });
});

describe("EnrollmentStatusView renewal adjust (#2251)", () => {
  beforeEach(() => {
    mockPathname.value = "/anmeldung/status/tok";
    mockFetchStatus.mockReset();
    mockListEnrollmentChangeRequests.mockReset();
    mockListEnrollmentChangeRequests.mockResolvedValue([]);
    mockWithdrawStatus.mockReset();
    mockConfirmRenewal.mockReset();
  });

  it("offers the reduced adjust flow next to confirm and decline (opt-in)", async () => {
    mockFetchStatus.mockResolvedValueOnce(
      status({
        edit_mode: "change_request",
        children: [
          {
            id: "7",
            first_name: "Lina",
            last_name: "Muster",
            status: "pending_renewal",
          },
        ],
      }),
    );

    render(<EnrollmentStatusView token="tok" />);

    expect(
      await screen.findByRole("button", { name: "Anmeldung bestätigen" }),
    ).toBeInTheDocument();
    const adjustLink = screen.getByRole("link", {
      name: "Angebote und Wochentage anpassen",
    });
    expect(adjustLink.getAttribute("href")).toContain("/adjust");
    expect(
      screen.getByRole("button", { name: "Anmeldung ablehnen" }),
    ).toBeInTheDocument();
  });

  it("uses the renamed parent route for renewal adjustments", async () => {
    mockPathname.value = "/parents/anmeldung/status/tok";
    mockFetchStatus.mockResolvedValueOnce(
      status({
        edit_mode: "change_request",
        children: [
          {
            id: "7",
            first_name: "Lina",
            last_name: "Muster",
            status: "pending_renewal",
          },
        ],
      }),
    );

    render(<EnrollmentStatusView token="tok" />);

    expect(
      await screen.findByRole("link", {
        name: "Angebote und Wochentage anpassen",
      }),
    ).toHaveAttribute("href", "/parents/anmeldung/status/tok/adjust");
  });

  it("offers adjust and unsubscribe on the opt-out banner", async () => {
    mockFetchStatus.mockResolvedValueOnce(
      status({
        edit_mode: "change_request",
        children: [
          {
            id: "7",
            first_name: "Lina",
            last_name: "Muster",
            status: "auto_renewed",
          },
        ],
      }),
    );

    render(<EnrollmentStatusView token="tok" />);

    expect(
      await screen.findByRole("link", {
        name: "Angebote und Wochentage anpassen",
      }),
    ).toBeInTheDocument();
  });

  it("hides the adjust link while a change request is already open", async () => {
    mockFetchStatus.mockResolvedValueOnce(
      status({
        edit_mode: "change_request",
        children: [
          {
            id: "7",
            first_name: "Lina",
            last_name: "Muster",
            status: "pending_renewal",
          },
        ],
      }),
    );
    mockListEnrollmentChangeRequests.mockResolvedValue([changeRequest()]);

    render(<EnrollmentStatusView token="tok" />);

    expect(
      await screen.findByRole("button", { name: "Anmeldung bestätigen" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Angebote und Wochentage anpassen" }),
    ).not.toBeInTheDocument();
  });

  it("hides the adjust link when the renewal is not editable as a change request", async () => {
    mockFetchStatus.mockResolvedValueOnce(
      status({
        edit_mode: "direct_edit",
        children: [
          {
            id: "7",
            first_name: "Lina",
            last_name: "Muster",
            status: "pending_renewal",
          },
        ],
      }),
    );

    render(<EnrollmentStatusView token="tok" />);

    expect(
      await screen.findByRole("button", { name: "Anmeldung bestätigen" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Angebote und Wochentage anpassen" }),
    ).not.toBeInTheDocument();
  });
});
