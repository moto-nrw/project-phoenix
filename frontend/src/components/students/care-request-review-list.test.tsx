import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { CareRequestReviewList } from "./care-request-review-list";
import {
  CareRequestApiError,
  decideCareScheduleChangeRequest,
  listCareScheduleChangeRequests,
  type StaffCareRequest,
} from "~/lib/care-request-review-api";

// Mock only the two network functions; keep the real CareRequestApiError so the
// component's `err instanceof CareRequestApiError` code-branch resolves against
// the actual class instead of an undefined stub.
vi.mock("~/lib/care-request-review-api", async (importActual) => {
  const actual =
    await importActual<typeof import("~/lib/care-request-review-api")>();
  return {
    ...actual,
    listCareScheduleChangeRequests: vi.fn(),
    decideCareScheduleChangeRequest: vi.fn(),
  };
});

const mockList = vi.mocked(listCareScheduleChangeRequests);
const mockDecide = vi.mocked(decideCareScheduleChangeRequest);

// Cards are collapsed by default (compact queue); expand every card so the diff
// and the Freigeben/Ablehnen actions render. The header button's accessible
// name contains the child name.
function expandAll() {
  for (const btn of screen.getAllByRole("button", { name: /Lara Beispiel/ })) {
    fireEvent.click(btn);
  }
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

describe("CareRequestReviewList", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockList.mockReset();
    mockDecide.mockReset();
  });

  it("renders the weekly diff and approves without requiring a reason", async () => {
    mockList.mockResolvedValue([row()]);
    mockDecide.mockResolvedValue(row({ status: "approved" }));

    render(<CareRequestReviewList />);

    expect(await screen.findByText("Lara Beispiel")).toBeInTheDocument();
    expandAll();
    expect(screen.getByText("Montag · Abholzeit:")).toBeInTheDocument();
    expect(screen.getByText("15:00")).toBeInTheDocument();
    expect(screen.getByText("Geht alleine")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Freigeben" }));

    await waitFor(() =>
      expect(mockDecide).toHaveBeenCalledWith("200", true, undefined),
    );
    await waitFor(() =>
      expect(screen.queryByText("Lara Beispiel")).not.toBeInTheDocument(),
    );
    expect(screen.getByText("Betreuungszeiten übernommen")).toBeInTheDocument();
  });

  it("requires a reason before rejecting", async () => {
    mockList.mockResolvedValue([row()]);
    mockDecide.mockResolvedValue(row({ status: "rejected" }));

    render(<CareRequestReviewList />);

    expect(await screen.findByText("Lara Beispiel")).toBeInTheDocument();
    expandAll();
    fireEvent.click(screen.getByRole("button", { name: "Ablehnen" }));

    expect(
      await screen.findByText(
        "Für eine Ablehnung ist eine Begründung erforderlich.",
      ),
    ).toBeInTheDocument();
    expect(mockDecide).not.toHaveBeenCalled();

    fireEvent.change(
      screen.getByPlaceholderText("Begründung (Pflicht bei Ablehnung)"),
      { target: { value: " zu kurzfristig " } },
    );
    fireEvent.click(screen.getByRole("button", { name: "Ablehnen" }));

    await waitFor(() =>
      expect(mockDecide).toHaveBeenCalledWith("200", false, "zu kurzfristig"),
    );
    expect(
      await screen.findByText("Betreuungszeit-Anfrage abgelehnt"),
    ).toBeInTheDocument();
  });

  it("shows load and decision errors without removing the row", async () => {
    mockList.mockRejectedValueOnce(new Error("queue down"));

    const { unmount } = render(<CareRequestReviewList />);

    expect(await screen.findByText("queue down")).toBeInTheDocument();
    unmount();

    mockList.mockResolvedValueOnce([row()]);
    mockDecide.mockRejectedValueOnce(new Error("boom"));
    render(<CareRequestReviewList />);

    expect(await screen.findByText("Lara Beispiel")).toBeInTheDocument();
    expandAll();
    fireEvent.click(screen.getByRole("button", { name: "Freigeben" }));

    expect(
      await screen.findByText(
        "Die Entscheidung konnte nicht gespeichert werden.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Lara Beispiel")).toBeInTheDocument();
  });

  it("surfaces the recovery action when approval is blocked by a 409 code", async () => {
    mockList.mockResolvedValueOnce([row()]);
    mockDecide.mockRejectedValueOnce(
      new CareRequestApiError(
        "schedule: care request messaging disabled",
        "messaging_disabled",
      ),
    );
    render(<CareRequestReviewList />);

    expect(await screen.findByText("Lara Beispiel")).toBeInTheDocument();
    expandAll();
    fireEvent.click(screen.getByRole("button", { name: "Freigeben" }));

    // The blocking reason must tell the reviewer to reject instead — not a
    // generic failure that leaves the request silently pending.
    expect(
      await screen.findByText(/Bitte die Anfrage stattdessen ablehnen\./),
    ).toBeInTheDocument();
    expect(screen.getByText("Lara Beispiel")).toBeInTheDocument();
  });

  it("renders the empty state", async () => {
    mockList.mockResolvedValue([]);

    render(<CareRequestReviewList />);

    expect(
      await screen.findByText("Keine offenen Betreuungszeit-Anfragen."),
    ).toBeInTheDocument();
  });

  it("refetches in place when the sibling queue dispatches change-requests-refresh", async () => {
    // A master-data departure-mode decision changes what this queue's diffs are
    // computed against; the sibling list emits change-requests-refresh, and this
    // list must refetch so it doesn't show stale "current → requested" values.
    mockList
      .mockResolvedValueOnce([row()])
      .mockResolvedValueOnce([
        row({ id: "201", first_name: "Max", last_name: "Muster" }),
      ]);

    render(<CareRequestReviewList />);

    expect(await screen.findByText("Lara Beispiel")).toBeInTheDocument();
    expect(mockList).toHaveBeenCalledTimes(1);

    await act(async () => {
      window.dispatchEvent(new Event("change-requests-refresh"));
    });

    await waitFor(() => expect(mockList).toHaveBeenCalledTimes(2));
    expect(await screen.findByText("Max Muster")).toBeInTheDocument();
    expect(screen.queryByText("Lara Beispiel")).not.toBeInTheDocument();
  });

  it("refetches in place on the SSE-derived messages-unread-refresh event", async () => {
    // A decision made in another tab or by another staffer never fires
    // change-requests-refresh here; it only arrives as the parent-message pill
    // use-global-sse fans out as messages-unread-refresh. The open queue must
    // refetch so it drops a row the backend has already decided.
    mockList
      .mockResolvedValueOnce([row()])
      .mockResolvedValueOnce([
        row({ id: "201", first_name: "Max", last_name: "Muster" }),
      ]);

    render(<CareRequestReviewList />);

    expect(await screen.findByText("Lara Beispiel")).toBeInTheDocument();
    expect(mockList).toHaveBeenCalledTimes(1);

    await act(async () => {
      window.dispatchEvent(new Event("messages-unread-refresh"));
    });

    await waitFor(() => expect(mockList).toHaveBeenCalledTimes(2));
    expect(await screen.findByText("Max Muster")).toBeInTheDocument();
    expect(screen.queryByText("Lara Beispiel")).not.toBeInTheDocument();
  });
});
