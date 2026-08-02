import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { OfferingRequestReviewList } from "./offering-request-review-list";
import {
  OfferingRequestApiError,
  decideOfferingChangeRequest,
  listOfferingChangeRequests,
  type StaffOfferingRequest,
} from "~/lib/offering-request-review-api";

// Mock only the two network functions; keep the real error class so the
// component's `err instanceof OfferingRequestApiError` branch resolves.
vi.mock("~/lib/offering-request-review-api", async (importActual) => {
  const actual =
    await importActual<typeof import("~/lib/offering-request-review-api")>();
  return {
    ...actual,
    listOfferingChangeRequests: vi.fn(),
    decideOfferingChangeRequest: vi.fn(),
  };
});

const mockList = vi.mocked(listOfferingChangeRequests);
const mockDecide = vi.mocked(decideOfferingChangeRequest);

function request(
  overrides: Partial<StaffOfferingRequest> = {},
): StaffOfferingRequest {
  return {
    id: "77",
    student_id: "42",
    student_name: "Lara Beispiel",
    status: "pending",
    effective_from: "2027-02-01",
    diff: [{ label: "Regelbetreuung", old: "Mo, Di, Mi", new: "Mo, Di" }],
    created_at: "2026-07-30T09:00:00Z",
    ...overrides,
  };
}

// Cards are collapsed by default; expand so the diff and the actions render.
function expandAll() {
  for (const btn of screen.getAllByRole("button", { name: /Lara Beispiel/ })) {
    fireEvent.click(btn);
  }
}

describe("OfferingRequestReviewList", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockList.mockResolvedValue([request()]);
  });

  it("lists a pending request with its effective date and diff", async () => {
    render(<OfferingRequestReviewList />);

    expect(await screen.findByText(/Lara Beispiel/)).toBeInTheDocument();
    expect(screen.getByText(/ab 01\.02\.2027/)).toBeInTheDocument();
    expandAll();
    expect(screen.getByText("Mo, Di, Mi")).toBeInTheDocument();
    expect(screen.getByText("Mo, Di")).toBeInTheDocument();
  });

  it("shows the empty state without a queue", async () => {
    mockList.mockResolvedValue([]);
    render(<OfferingRequestReviewList />);

    expect(
      await screen.findByText("Keine offenen Anfragen zu Betreuungsangeboten."),
    ).toBeInTheDocument();
  });

  it("approves a request and removes the row", async () => {
    mockDecide.mockResolvedValue(undefined);
    render(<OfferingRequestReviewList />);
    await screen.findByText(/Lara Beispiel/);
    expandAll();

    fireEvent.click(screen.getByRole("button", { name: /Freigeben/ }));

    await waitFor(() =>
      expect(mockDecide).toHaveBeenCalledWith("77", true, undefined),
    );
    expect(
      await screen.findByText(/gültig ab 01\.02\.2027/),
    ).toBeInTheDocument();
  });

  it("requires a reason before rejecting", async () => {
    render(<OfferingRequestReviewList />);
    await screen.findByText(/Lara Beispiel/);
    expandAll();

    fireEvent.click(screen.getByRole("button", { name: /Ablehnen/ }));

    expect(
      await screen.findByText(
        "Für eine Ablehnung ist eine Begründung erforderlich.",
      ),
    ).toBeInTheDocument();
    expect(mockDecide).not.toHaveBeenCalled();
  });

  it("names the capacity conflict and keeps the row pending", async () => {
    mockDecide.mockRejectedValue(
      new OfferingRequestApiError("full", "offering_change_capacity_full"),
    );
    render(<OfferingRequestReviewList />);
    await screen.findByText(/Lara Beispiel/);
    expandAll();

    fireEvent.click(screen.getByRole("button", { name: /Freigeben/ }));

    expect(await screen.findByText(/kein Platz mehr frei/)).toBeInTheDocument();
    // The row survives a failed approval: the switch was not applied.
    expect(screen.getByText(/Lara Beispiel/)).toBeInTheDocument();
  });

  it("shows the parent note when one was added", async () => {
    mockList.mockResolvedValue([request({ note: "Neuer Arbeitsbeginn" })]);
    render(<OfferingRequestReviewList />);
    await screen.findByText(/Lara Beispiel/);
    expandAll();

    expect(
      screen.getByText(/Nachricht der Eltern: Neuer Arbeitsbeginn/),
    ).toBeInTheDocument();
  });
});
