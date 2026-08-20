import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { CareRequestReviewItem } from "./care-request-review-item";
import {
  CareRequestApiError,
  decideCareScheduleChangeRequest,
  type StaffCareRequest,
} from "~/lib/care-request-review-api";

// Mock only the network function; keep the real CareRequestApiError so the
// component's `err instanceof CareRequestApiError` code-branch resolves against
// the actual class instead of an undefined stub.
vi.mock("~/lib/care-request-review-api", async (importActual) => {
  const actual =
    await importActual<typeof import("~/lib/care-request-review-api")>();
  return {
    ...actual,
    decideCareScheduleChangeRequest: vi.fn(),
  };
});

const mockDecide = vi.mocked(decideCareScheduleChangeRequest);

// Cards render collapsed (compact queue); expand via the header button so the
// diff and the Freigeben/Ablehnen actions render. The header button's
// accessible name contains the child name.
function expand() {
  fireEvent.click(screen.getByRole("button", { name: /Lara Beispiel/ }));
}

function row(overrides: Partial<StaffCareRequest> = {}): StaffCareRequest {
  return {
    id: "200",
    student_id: "42",
    first_name: "Lara",
    last_name: "Beispiel",
    status: "pending",
    request_kind: "weekly_schedule",
    diff: [
      {
        label: "Montag · Abholzeit",
        old: "—",
        new: "15:00",
        weekday: 1,
        care_kind: "pickup",
      },
      {
        label: "Montag · Abholart",
        old: "Fährt Bus / Wird abgeholt",
        new: "Geht alleine",
        weekday: 1,
        care_kind: "departure_mode",
        old_modes: ["bus", "pickup"],
        new_mode: "alone",
      },
    ],
    created_at: "2026-07-01T12:00:00Z",
    ...overrides,
  };
}

describe("CareRequestReviewItem", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockDecide.mockReset();
  });

  it("renders the weekly diff plus summary and approves without requiring a reason", async () => {
    mockDecide.mockResolvedValue(row({ status: "approved" }));
    const onDecided = vi.fn();

    render(<CareRequestReviewItem row={row()} onDecided={onDecided} />);

    // Collapsed summary: the distinct change kinds from the diff labels.
    expect(
      screen.getByText("Betreuungszeiten · Abholzeit + Abholart"),
    ).toBeInTheDocument();
    expand();
    expect(screen.getByText("Montag · Abholzeit:")).toBeInTheDocument();
    expect(screen.getByText("15:00")).toBeInTheDocument();
    expect(screen.getByText("Geht alleine")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Freigeben" }));

    await waitFor(() =>
      expect(mockDecide).toHaveBeenCalledWith("200", true, undefined),
    );
    expect(onDecided).toHaveBeenCalledWith("Betreuungszeiten übernommen");
  });

  it("requires a reason before rejecting", async () => {
    mockDecide.mockResolvedValue(row({ status: "rejected" }));
    const onDecided = vi.fn();

    render(<CareRequestReviewItem row={row()} onDecided={onDecided} />);

    expand();
    fireEvent.click(screen.getByRole("button", { name: "Ablehnen" }));

    expect(
      await screen.findByText(
        "Für eine Ablehnung ist eine Begründung erforderlich.",
      ),
    ).toBeInTheDocument();
    expect(mockDecide).not.toHaveBeenCalled();
    expect(onDecided).not.toHaveBeenCalled();

    fireEvent.change(
      screen.getByPlaceholderText("Begründung (Pflicht bei Ablehnung)"),
      { target: { value: " zu kurzfristig " } },
    );
    fireEvent.click(screen.getByRole("button", { name: "Ablehnen" }));

    await waitFor(() =>
      expect(mockDecide).toHaveBeenCalledWith("200", false, "zu kurzfristig"),
    );
    expect(onDecided).toHaveBeenCalledWith("Betreuungszeit-Anfrage abgelehnt");
  });

  it("shows the parent's mandatory reason for a pickup change and reports the pickup notice", async () => {
    mockDecide.mockResolvedValue(row({ status: "approved" }));
    const onDecided = vi.fn();

    render(
      <CareRequestReviewItem
        row={row({
          request_kind: "pickup_change",
          request_reason: "Arzttermin",
          diff: [
            {
              label: "17.08.2026 · Abholzeit",
              old: "15:30",
              new: "16:30",
              care_kind: "pickup",
            },
          ],
        })}
        onDecided={onDecided}
      />,
    );

    expand();
    expect(screen.getByText("Grund der Eltern:")).toBeInTheDocument();
    expect(screen.getByText("Arzttermin")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Freigeben" }));

    await waitFor(() =>
      expect(onDecided).toHaveBeenCalledWith("Abholzeit übernommen"),
    );
  });

  it("shows a generic decision error without calling onDecided", async () => {
    mockDecide.mockRejectedValueOnce(new Error("boom"));
    const onDecided = vi.fn();

    render(<CareRequestReviewItem row={row()} onDecided={onDecided} />);

    expand();
    fireEvent.click(screen.getByRole("button", { name: "Freigeben" }));

    expect(
      await screen.findByText(
        "Die Entscheidung konnte nicht gespeichert werden.",
      ),
    ).toBeInTheDocument();
    expect(onDecided).not.toHaveBeenCalled();
    expect(screen.getByText("Lara Beispiel")).toBeInTheDocument();
  });

  it("surfaces the recovery action when approval is blocked by messaging_disabled", async () => {
    mockDecide.mockRejectedValueOnce(
      new CareRequestApiError(
        "schedule: care request messaging disabled",
        "messaging_disabled",
      ),
    );
    const onDecided = vi.fn();

    render(<CareRequestReviewItem row={row()} onDecided={onDecided} />);

    expand();
    fireEvent.click(screen.getByRole("button", { name: "Freigeben" }));

    // The blocking reason must tell the reviewer to reject instead — not a
    // generic failure that leaves the request silently pending.
    expect(
      await screen.findByText(/Bitte die Anfrage stattdessen ablehnen\./),
    ).toBeInTheDocument();
    expect(onDecided).not.toHaveBeenCalled();
  });

  it("surfaces the recovery action when approval is blocked by pickup_change_conflict", async () => {
    mockDecide.mockRejectedValueOnce(
      new CareRequestApiError(
        "schedule: pickup change conflict",
        "pickup_change_conflict",
      ),
    );
    const onDecided = vi.fn();

    render(
      <CareRequestReviewItem
        row={row({ request_kind: "pickup_change" })}
        onDecided={onDecided}
      />,
    );

    expand();
    fireEvent.click(screen.getByRole("button", { name: "Freigeben" }));

    expect(
      await screen.findByText(
        "Für diesen Tag wurde inzwischen bereits eine Änderung durch die OGS eingetragen. Bitte prüfen und die Anfrage gegebenenfalls ablehnen.",
      ),
    ).toBeInTheDocument();
    expect(onDecided).not.toHaveBeenCalled();
  });
});
