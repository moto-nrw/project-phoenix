import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  confirmRenewal,
  fetchStatus,
  patchStatus,
  withdrawStatus,
  type StatusResponse,
} from "~/lib/enrollment-submission-api";
import { EnrollmentStatusView } from "./enrollment-status-view";

vi.mock("~/lib/enrollment-submission-api", () => ({
  confirmRenewal: vi.fn(),
  fetchStatus: vi.fn(),
  patchStatus: vi.fn(),
  withdrawStatus: vi.fn(),
}));

const mockFetchStatus = vi.mocked(fetchStatus);
const mockPatchStatus = vi.mocked(patchStatus);
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

describe("EnrollmentStatusView", () => {
  beforeEach(() => {
    mockFetchStatus.mockReset();
    mockPatchStatus.mockReset();
    mockWithdrawStatus.mockReset();
    mockConfirmRenewal.mockReset();
    Object.defineProperty(window, "confirm", {
      value: vi.fn(() => true),
      configurable: true,
      writable: true,
    });
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

    await waitFor(() => {
      expect(window.confirm).toHaveBeenCalledWith(
        "Möchtest du diese Anmeldung wirklich zurückziehen?",
      );
      expect(mockWithdrawStatus).toHaveBeenCalledWith("tok", "7");
    });
    expect(
      await screen.findByText("Anmeldung für dieses Kind zurückgezogen."),
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

    expect(await screen.findByText("Nicht möglich")).toBeInTheDocument();
  });
});
