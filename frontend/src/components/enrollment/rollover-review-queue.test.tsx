import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";

// ============================================================================
// Mocks
// ============================================================================

const { mockListReview, mockDecideReview } = vi.hoisted(() => ({
  mockListReview: vi.fn(),
  mockDecideReview: vi.fn(),
}));

vi.mock("~/lib/enrollment-phase-api", async (importOriginal) => {
  const actual = (await importOriginal()) as Record<string, unknown>;
  return {
    ...actual,
    listRolloverReview: mockListReview,
    decideRolloverReview: mockDecideReview,
  };
});

vi.mock("~/components/ui/mobile-back-button", () => ({
  MobileBackButton: () => null,
}));

import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";
import { RolloverReviewQueue as Queue } from "./rollover-review-queue";

function RolloverReviewQueue(props: { phaseID: string }) {
  return (
    <ToastProvider>
      <Queue {...props} />
    </ToastProvider>
  );
}
import type { ReviewQueueItem } from "~/lib/enrollment-phase-api";

// ============================================================================
// Test data
// ============================================================================

function makeItem(overrides: Partial<ReviewQueueItem> = {}): ReviewQueueItem {
  return {
    child_id: "999",
    request_id: "555",
    first_name: "Lina",
    last_name: "Beispiel",
    target_grade_level: 5,
    review_reason: "grade_above_max",
    source_grade_level: 4,
    source_first_name: "Lina",
    source_last_name: "Beispiel",
    guardian_first_name: "Anna",
    guardian_last_name: "Beispiel",
    guardian_email: "anna@example.com",
    status_token: "tok-abc",
    ...overrides,
  };
}

// ============================================================================
// Tests
// ============================================================================

describe("RolloverReviewQueue", () => {
  beforeEach(() => {
    mockListReview.mockReset();
    mockDecideReview.mockReset();
  });

  it("renders items with localised review reason", async () => {
    mockListReview.mockResolvedValueOnce([makeItem()]);
    render(<RolloverReviewQueue phaseID="77" />);

    await waitFor(() => {
      expect(screen.getByText(/Lina Beispiel/)).toBeInTheDocument();
    });
    expect(mockListReview).toHaveBeenCalledWith("77");
    expect(
      screen.getByText(/Klassenstufe über der Höchstgrenze/),
    ).toBeInTheDocument();
    expect(screen.getByText(/anna@example\.com/)).toBeInTheDocument();
  });

  it("falls back to the raw review reason when no label exists", async () => {
    mockListReview.mockResolvedValueOnce([
      makeItem({ review_reason: "future_unmapped_code" }),
    ]);
    render(<RolloverReviewQueue phaseID="77" />);

    await waitFor(() => {
      expect(screen.getByText(/future_unmapped_code/)).toBeInTheDocument();
    });
  });

  it("renders the empty state when there are no items", async () => {
    mockListReview.mockResolvedValueOnce([]);
    render(<RolloverReviewQueue phaseID="77" />);

    await waitFor(() => {
      expect(screen.getByText(/Keine offenen Einträge/)).toBeInTheDocument();
    });
  });

  it("dispatches a keep decision and reloads", async () => {
    mockListReview.mockResolvedValueOnce([makeItem()]);
    mockDecideReview.mockResolvedValueOnce(undefined);
    mockListReview.mockResolvedValueOnce([]);

    render(<RolloverReviewQueue phaseID="77" />);

    await waitFor(() => {
      expect(screen.getByText(/Lina Beispiel/)).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: /Behalten/ }));

    await waitFor(() => {
      expect(mockDecideReview).toHaveBeenCalledTimes(1);
    });
    const [childID, input] = mockDecideReview.mock.calls[0] as [
      string,
      Record<string, unknown>,
    ];
    expect(childID).toBe("999");
    expect(input).toEqual({ decision: "keep", new_grade_level: null });

    // Empty state surfaces after the reload returns [].
    await waitFor(() => {
      expect(screen.getByText(/Keine offenen Einträge/)).toBeInTheDocument();
    });
  });

  it("passes a numeric class override on keep", async () => {
    mockListReview.mockResolvedValueOnce([makeItem()]);
    mockDecideReview.mockResolvedValueOnce(undefined);
    mockListReview.mockResolvedValueOnce([]);

    render(<RolloverReviewQueue phaseID="77" />);

    await waitFor(() => {
      expect(screen.getByText(/Lina Beispiel/)).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText(/Klassenstufe für nächste Phase/), {
      target: { value: "4" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Behalten/ }));

    await waitFor(() => {
      expect(mockDecideReview).toHaveBeenCalledTimes(1);
    });
    const input = mockDecideReview.mock.calls[0]?.[1] as Record<
      string,
      unknown
    >;
    expect(input).toEqual({ decision: "keep", new_grade_level: 4 });
  });

  it("rejects non-integer class overrides", async () => {
    mockListReview.mockResolvedValueOnce([makeItem()]);
    render(<RolloverReviewQueue phaseID="77" />);

    await waitFor(() => {
      expect(screen.getByText(/Lina Beispiel/)).toBeInTheDocument();
    });

    // The <input type="number"> accepts 4.5; Number.isInteger(4.5)
    // is false so the keep guard throws. (Bare "abc" gets stripped
    // to "" by the number input and would silently pass.)
    fireEvent.change(screen.getByLabelText(/Klassenstufe für nächste Phase/), {
      target: { value: "4.5" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Behalten/ }));

    const field = screen.getByLabelText(/Klassenstufe für nächste Phase/);
    await waitFor(() => {
      expect(
        screen.getByText(
          "Bitte geben Sie die Klassenstufe als ganze Zahl ein.",
        ),
      ).toBeInTheDocument();
    });
    expect(field).toHaveAttribute("aria-invalid", "true");
    await waitFor(() => expect(field).toHaveFocus());
    expect(mockDecideReview).not.toHaveBeenCalled();
  });

  it("dispatches drop without a class override", async () => {
    mockListReview.mockResolvedValueOnce([makeItem()]);
    mockDecideReview.mockResolvedValueOnce(undefined);
    mockListReview.mockResolvedValueOnce([]);

    render(<RolloverReviewQueue phaseID="77" />);

    await waitFor(() => {
      expect(screen.getByText(/Lina Beispiel/)).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: /Abschließen/ }));

    await waitFor(() => {
      expect(mockDecideReview).toHaveBeenCalledTimes(1);
    });
    const input = mockDecideReview.mock.calls[0]?.[1] as Record<
      string,
      unknown
    >;
    expect(input).toEqual({ decision: "drop", new_grade_level: null });
  });

  it("dispatches defer without a class override", async () => {
    mockListReview.mockResolvedValueOnce([makeItem()]);
    mockDecideReview.mockResolvedValueOnce(undefined);
    mockListReview.mockResolvedValueOnce([]);

    render(<RolloverReviewQueue phaseID="77" />);

    await waitFor(() => {
      expect(screen.getByText(/Lina Beispiel/)).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: /Zurückstellen/ }));

    await waitFor(() => {
      expect(mockDecideReview).toHaveBeenCalledTimes(1);
    });
    const input = mockDecideReview.mock.calls[0]?.[1] as Record<
      string,
      unknown
    >;
    expect(input.decision).toBe("defer");
  });

  it("reports a refused decision as a toast with the catalog text", async () => {
    mockListReview.mockResolvedValueOnce([makeItem()]);
    mockDecideReview.mockRejectedValueOnce(
      new ApiError("backend down", 409, {
        code: "rollover.review_invalid",
      }),
    );

    render(<RolloverReviewQueue phaseID="77" />);

    await waitFor(() => {
      expect(screen.getByText(/Lina Beispiel/)).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: /Behalten/ }));

    expect(
      await screen.findByText(
        catalogText("rollover.review_invalid", "die Entscheidung"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("backend down")).not.toBeInTheDocument();
    // Der Eintrag bleibt stehen, damit ein zweiter Versuch möglich ist.
    expect(screen.getByText(/Lina Beispiel/)).toBeInTheDocument();
  });

  it("confirms a decision in a success toast", async () => {
    mockListReview.mockResolvedValueOnce([makeItem()]);
    mockDecideReview.mockResolvedValueOnce(undefined);
    mockListReview.mockResolvedValueOnce([]);
    render(<RolloverReviewQueue phaseID="77" />);

    await waitFor(() => {
      expect(screen.getByText(/Lina Beispiel/)).toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole("button", { name: /Zurückstellen/ }));

    expect(
      await screen.findByText("Der Eintrag ist zurückgestellt."),
    ).toBeInTheDocument();
  });

  it("shows a failed load in place with a retry instead of the empty state", async () => {
    mockListReview.mockRejectedValueOnce(
      new ApiError("kaputt", 503, { code: "general.unavailable" }),
    );
    mockListReview.mockResolvedValueOnce([makeItem()]);
    render(<RolloverReviewQueue phaseID="77" />);

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Prüfliste"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("kaputt")).not.toBeInTheDocument();
    expect(
      screen.queryByText(/Keine offenen Einträge/),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(await screen.findByText(/Lina Beispiel/)).toBeInTheDocument();
  });
});
