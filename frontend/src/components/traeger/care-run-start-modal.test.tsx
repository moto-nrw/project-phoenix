import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";

// The start of a monthly run (#5916): every free school is chosen, a
// school in an open run of the month cannot be chosen, and the start sends
// month, schools and invoice date.

const { review } = vi.hoisted(() => ({
  review: { fetchCareRunSchools: vi.fn() },
}));
vi.mock("~/lib/traeger-care-review-api", async (original) => ({
  ...(await original<typeof import("~/lib/traeger-care-review-api")>()),
  ...review,
}));

import { CareRunStartModal } from "./care-run-start-modal";

const MONTHS = [{ value: "2026-10-01", label: "Oktober 2026" }];

function renderModal(onStart = vi.fn().mockResolvedValue(undefined)) {
  render(
    <ToastProvider>
      <CareRunStartModal
        isOpen
        months={MONTHS}
        initialMonth="2026-10-01"
        onClose={vi.fn()}
        onStart={onStart}
      />
    </ToastProvider>,
  );
  return onStart;
}

describe("CareRunStartModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    review.fetchCareRunSchools.mockResolvedValue([
      { id: "1", name: "Grundschule Birkenfeld", openRunId: null },
      { id: "2", name: "Grundschule Ispringen", openRunId: null },
      { id: "3", name: "Ludwig-Uhland-Schule", openRunId: "9" },
    ]);
  });

  it("chooses every free school and starts with them", async () => {
    const onStart = renderModal();
    const taken = await screen.findByRole("checkbox", {
      name: /Ludwig-Uhland-Schule/,
    });
    expect(taken).toBeDisabled();
    expect(taken).not.toBeChecked();
    expect(
      screen.getByText("Schon in einem offenen Lauf für Oktober 2026"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("checkbox", { name: /Grundschule Birkenfeld/ }),
    ).toBeChecked();
    expect(review.fetchCareRunSchools).toHaveBeenCalledWith("2026-10-01");

    fireEvent.click(screen.getByRole("button", { name: "Vorschau öffnen" }));
    await waitFor(() =>
      expect(onStart).toHaveBeenCalledWith({
        billingMonth: "2026-10-01",
        schoolIds: ["1", "2"],
        invoiceDate: null,
      }),
    );
  });

  it("needs at least one school", async () => {
    const onStart = renderModal();
    fireEvent.click(
      await screen.findByRole("button", { name: "Keine wählen" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Vorschau öffnen" }));
    expect(
      await screen.findByText("Bitte wählen Sie mindestens eine Schule."),
    ).toBeInTheDocument();
    expect(onStart).not.toHaveBeenCalled();
  });
});
