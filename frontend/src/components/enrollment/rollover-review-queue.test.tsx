import { describe, it, expect, vi, beforeEach } from "vitest";
import {
  act,
  render,
  screen,
  fireEvent,
  waitFor,
} from "@testing-library/react";
import "@testing-library/jest-dom/vitest";

// ============================================================================
// Mocks
// ============================================================================

const { mockListReview, mockDecideReview, mockTenantMutate } = vi.hoisted(
  () => ({
    mockListReview: vi.fn(),
    mockDecideReview: vi.fn(),
    mockTenantMutate: vi.fn(() => Promise.resolve()),
  }),
);

vi.mock("~/lib/swr", () => ({
  useTenantMutate: () => mockTenantMutate,
}));

vi.mock("~/lib/enrollment-phase-api", async (importOriginal) => {
  const actual = (await importOriginal()) as Record<string, unknown>;
  return {
    ...actual,
    listRolloverReview: mockListReview,
    decideRolloverReview: mockDecideReview,
  };
});

vi.mock("~/components/ui/back-link", () => ({
  BackLink: () => null,
}));

import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { deferred } from "~/test/helpers/form-modal-pending";
import { catalogText } from "~/test/error-catalog-text";
import { SAVED_BUT_STALE_SUFFIX } from "~/lib/hooks/use-refresh-after-write";
import { RolloverReviewQueue as Queue } from "./rollover-review-queue";

function RolloverReviewQueue(props: { phaseID: string }) {
  return (
    <ToastProvider>
      <Queue {...props} />
    </ToastProvider>
  );
}
import type { ReviewQueueItem } from "~/lib/enrollment-phase-api";
import {
  expectPageReloadedAfterRefusal,
  resetPageReloads,
} from "~/test/helpers/refused-refresh";

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
    // A decision can resolve a phase-end hint, so the hints reload too.
    expect(mockTenantMutate).toHaveBeenCalledWith(
      "enrollment-phase-expiry-warnings",
    );

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

  // #6652: a write holds once it is through; a failed reload keeps the list
  // and says the change is saved, and its retry only reloads.
  describe("reload after a write (#6652)", () => {
    const reloadText = `${catalogText("general.unavailable", "die Anzeige der Prüfliste")} ${SAVED_BUT_STALE_SUFFIX}`;

    it.each([403, 503])(
      "handles a %s reload after a confirmed decision without exposing refused data",
      async (status) => {
        mockListReview
          .mockResolvedValueOnce([
            makeItem(),
            makeItem({
              child_id: "998",
              first_name: "Ben",
              guardian_email: "ben-parent@example.test",
            }),
          ])
          .mockRejectedValueOnce(new ApiError("failed", status));
        mockDecideReview.mockResolvedValue(undefined);
        resetPageReloads();
        render(<RolloverReviewQueue phaseID="77" />);
        fireEvent.click(
          (await screen.findAllByRole("button", { name: /Behalten/ }))[0]!,
        );
        if (status === 403) {
          // #6801: the page loads anew; its first load shows the refusal.
          await expectPageReloadedAfterRefusal();
          return;
        }
        expect(await screen.findByText(reloadText)).toBeVisible();
        expect(screen.getByText("Ben Beispiel")).toBeVisible();
        expect(screen.getByRole("button", { name: /Behalten/ })).toBeEnabled();
      },
    );

    it("ends loading when a decision refresh supersedes the new phase's initial read", async () => {
      const write = deferred<void>();
      const older = deferred<ReturnType<typeof makeItem>[]>();
      mockListReview
        .mockResolvedValueOnce([makeItem()])
        .mockReturnValueOnce(older.promise)
        .mockResolvedValueOnce([makeItem({ first_name: "Ben" })]);
      mockDecideReview.mockReturnValueOnce(write.promise);
      const view = render(<RolloverReviewQueue phaseID="77" />);
      fireEvent.click(await screen.findByRole("button", { name: /Behalten/ }));
      view.rerender(<RolloverReviewQueue phaseID="78" />);
      await waitFor(() => expect(mockListReview).toHaveBeenCalledTimes(2));
      await act(async () => write.resolve());
      expect(await screen.findByText("Ben Beispiel")).toBeVisible();
      await act(async () =>
        older.resolve([makeItem({ first_name: "Veraltet" })]),
      );
      expect(screen.getByText("Ben Beispiel")).toBeVisible();
      expect(screen.queryByText("Veraltet Beispiel")).not.toBeInTheDocument();
    });

    it("locks a confirmed decision when its reload fails", async () => {
      mockListReview
        .mockResolvedValueOnce([makeItem()])
        .mockRejectedValueOnce(new ApiError("down", 503));
      mockDecideReview.mockResolvedValue(undefined);
      render(<RolloverReviewQueue phaseID="77" />);
      fireEvent.click(await screen.findByRole("button", { name: /Behalten/ }));
      expect(await screen.findByText(reloadText)).toBeVisible();
      expect(
        screen.queryByRole("button", { name: /Behalten/ }),
      ).not.toBeInTheDocument();
      expect(
        screen.queryByRole("button", { name: /Abschließen/ }),
      ).not.toBeInTheDocument();
    });

    it("ignores queue responses that precede confirmed decisions", async () => {
      const older = deferred<ReturnType<typeof makeItem>[]>();
      mockListReview
        .mockResolvedValueOnce([
          makeItem(),
          makeItem({ child_id: "998", first_name: "Ben" }),
        ])
        .mockReturnValueOnce(older.promise)
        .mockResolvedValueOnce([]);
      mockDecideReview.mockResolvedValue(undefined);
      render(<RolloverReviewQueue phaseID="77" />);
      const keep = await screen.findAllByRole("button", { name: /Behalten/ });
      fireEvent.click(keep[0]!);
      await waitFor(() => expect(mockListReview).toHaveBeenCalledTimes(2));
      fireEvent.click(
        (await screen.findAllByRole("button", { name: /Behalten/ }))[0]!,
      );
      expect(await screen.findByText(/Keine offenen Einträge/)).toBeVisible();
      await act(async () => older.resolve([makeItem()]));
      expect(screen.getByText(/Keine offenen Einträge/)).toBeVisible();
      expect(screen.queryByText("Lina Beispiel")).not.toBeInTheDocument();
    });

    it("keeps a decided entry decided when the reload fails", async () => {
      mockListReview
        .mockResolvedValueOnce([makeItem()])
        .mockRejectedValueOnce(
          new ApiError("down", 503, { code: "general.unavailable" }),
        )
        .mockResolvedValueOnce([]);
      mockDecideReview.mockResolvedValueOnce(undefined);
      render(<RolloverReviewQueue phaseID="77" />);

      expect(await screen.findByText(/Lina Beispiel/)).toBeInTheDocument();
      fireEvent.click(screen.getByRole("button", { name: /Behalten/ }));

      expect(await screen.findByText(reloadText)).toBeInTheDocument();
      // Die Entscheidung ist bestätigt, der Eintrag ist nicht mehr offen.
      expect(screen.queryByText(/Lina Beispiel/)).not.toBeInTheDocument();
      expect(
        screen.queryByText(
          catalogText("general.unavailable", "die Entscheidung"),
        ),
      ).not.toBeInTheDocument();
      expect(
        screen.queryByText(catalogText("general.unavailable", "die Prüfliste")),
      ).not.toBeInTheDocument();
      expect(mockDecideReview).toHaveBeenCalledTimes(1);

      fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
      expect(
        await screen.findByText(/Keine offenen Einträge/),
      ).toBeInTheDocument();
      expect(mockListReview).toHaveBeenCalledTimes(3);
      expect(mockDecideReview).toHaveBeenCalledTimes(1);
    });
  });
});
