import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ExcusedRequestReviewItem } from "./excused-request-review-item";
import {
  ExcusedRequestApiError,
  decideExcusedAbsenceRequest,
  type StaffExcusedRequest,
} from "~/lib/excused-request-review-api";

// Mock only the network function; keep the real ExcusedRequestApiError so the
// component's `err instanceof ExcusedRequestApiError` code-branch resolves
// against the actual class instead of an undefined stub.
vi.mock("~/lib/excused-request-review-api", async (importActual) => {
  const actual =
    await importActual<typeof import("~/lib/excused-request-review-api")>();
  return {
    ...actual,
    decideExcusedAbsenceRequest: vi.fn(),
  };
});

const mockDecide = vi.mocked(decideExcusedAbsenceRequest);

// Cards render collapsed (compact queue); expand via the header button so the
// details and the Freigeben/Ablehnen actions render. The header button's
// accessible name contains the child name.
function expand() {
  fireEvent.click(screen.getByRole("button", { name: /Lara Beispiel/ }));
}

function row(
  overrides: Partial<StaffExcusedRequest> = {},
): StaffExcusedRequest {
  return {
    id: "300",
    student_id: "42",
    first_name: "Lara",
    last_name: "Beispiel",
    status: "pending",
    dates: ["2026-07-01", "2026-07-02", "2026-07-03"],
    note: "Familienfeier",
    created_at: "2026-06-28T12:00:00Z",
    ...overrides,
  };
}

describe("ExcusedRequestReviewItem", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockDecide.mockReset();
  });

  it("collapses contiguous days to a range and shows the parent's note", () => {
    render(<ExcusedRequestReviewItem row={row()} onDecided={vi.fn()} />);

    // Summary appears in the collapsed header and again in the expanded panel.
    expect(
      screen.getAllByText(/01\.07\.2026 – 03\.07\.2026/).length,
    ).toBeGreaterThan(0);
    expand();
    expect(screen.getByText("Notiz der Eltern:")).toBeInTheDocument();
    expect(screen.getByText("Familienfeier")).toBeInTheDocument();
  });

  it("lists non-contiguous days individually instead of implying a range", () => {
    // A Mon+Wed request must never render as "Mon – Wed" — that would wrongly
    // imply Tuesday is included too.
    render(
      <ExcusedRequestReviewItem
        row={row({ dates: ["2026-07-01", "2026-07-03"] })}
        onDecided={vi.fn()}
      />,
    );

    expect(
      screen.getAllByText(/01\.07\.2026, 03\.07\.2026/).length,
    ).toBeGreaterThan(0);
  });

  it("requires a reason before rejecting", async () => {
    mockDecide.mockResolvedValue(row({ status: "rejected" }));
    const onDecided = vi.fn();

    render(<ExcusedRequestReviewItem row={row()} onDecided={onDecided} />);

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
      { target: { value: " keine Kapazität " } },
    );
    fireEvent.click(screen.getByRole("button", { name: "Ablehnen" }));

    await waitFor(() =>
      expect(mockDecide).toHaveBeenCalledWith("300", false, "keine Kapazität"),
    );
    expect(onDecided).toHaveBeenCalledWith("Entschuldigungs-Anfrage abgelehnt");
  });

  it("approves without a reason and reports the notice", async () => {
    mockDecide.mockResolvedValue(row({ status: "approved" }));
    const onDecided = vi.fn();

    render(<ExcusedRequestReviewItem row={row()} onDecided={onDecided} />);

    expand();
    fireEvent.click(screen.getByRole("button", { name: "Freigeben" }));

    await waitFor(() =>
      expect(mockDecide).toHaveBeenCalledWith("300", true, undefined),
    );
    expect(onDecided).toHaveBeenCalledWith("Abmeldung bestätigt");
  });

  it("surfaces the recovery action on the excused_request_status_conflict 409", async () => {
    mockDecide.mockRejectedValueOnce(
      new ExcusedRequestApiError(
        "students: excused request status conflict",
        "excused_request_status_conflict",
      ),
    );
    const onDecided = vi.fn();

    render(<ExcusedRequestReviewItem row={row()} onDecided={onDecided} />);

    expand();
    fireEvent.click(screen.getByRole("button", { name: "Freigeben" }));

    expect(
      await screen.findByText(
        "Für einen dieser Tage wurde inzwischen ein neuerer Status gesetzt (z. B. krank oder Ausflug). Die Freigabe würde ihn überschreiben. Bitte die Anfrage ablehnen oder den neueren Status zuerst entfernen.",
      ),
    ).toBeInTheDocument();
    expect(onDecided).not.toHaveBeenCalled();
    expect(screen.getByText("Lara Beispiel")).toBeInTheDocument();
  });
});
