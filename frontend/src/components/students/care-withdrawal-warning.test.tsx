import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { fetchStudentCareWithdrawal } from "~/lib/care-exit-api";
import { CareWithdrawalWarning } from "./care-withdrawal-warning";

vi.mock("~/lib/care-exit-api", () => ({
  fetchStudentCareWithdrawal: vi.fn(),
}));

vi.mock("~/components/students/care-exit-modal", () => ({
  CareExitModal: ({
    isOpen,
    completionId,
    onFinished,
  }: {
    isOpen: boolean;
    completionId?: string;
    onFinished: () => void;
  }) =>
    isOpen ? (
      <button type="button" onClick={onFinished}>
        modal-{completionId}
      </button>
    ) : null,
}));

const mockFetch = vi.mocked(fetchStudentCareWithdrawal);

describe("CareWithdrawalWarning", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockFetch.mockResolvedValue({
      id: "completion-1",
      studentId: "10",
      firstName: "Mia",
      lastName: "Muster",
      schoolClass: "2a",
      firstBookinglessDay: "2026-09-01",
      urgency: "planned",
    });
  });

  it("shows the child warning and opens the completion flow", async () => {
    render(<CareWithdrawalWarning enabled studentId="10" />);

    expect(
      await screen.findByText(/Abmeldung noch abschließen.*01\.09\.2026/),
    ).toBeVisible();
    fireEvent.click(screen.getByRole("button", { name: "Betreuung beenden" }));
    expect(screen.getByText("modal-completion-1")).toBeVisible();
  });

  it("does not query or reveal the warning without users:delete", () => {
    render(<CareWithdrawalWarning enabled={false} studentId="10" />);
    expect(mockFetch).not.toHaveBeenCalled();
    expect(screen.queryByText(/Abmeldung noch abschließen/)).toBeNull();
  });

  it("does not hide a failed warning lookup", async () => {
    mockFetch.mockRejectedValue(new Error("network"));
    render(<CareWithdrawalWarning enabled studentId="10" />);

    await waitFor(() =>
      expect(
        screen.getByText("Die offene Abmeldung konnte nicht geladen werden."),
      ).toBeVisible(),
    );
  });

  it("appears immediately when the booking editor creates the task", async () => {
    mockFetch.mockResolvedValueOnce(null);
    render(<CareWithdrawalWarning enabled studentId="10" />);

    await waitFor(() => expect(mockFetch).toHaveBeenCalledTimes(1));
    expect(screen.queryByText(/Abmeldung noch abschließen/)).toBeNull();

    mockFetch.mockResolvedValueOnce({
      id: "completion-2",
      studentId: "10",
      firstName: "Mia",
      lastName: "Muster",
      schoolClass: "2a",
      firstBookinglessDay: "2026-09-02",
      urgency: "planned",
    });
    window.dispatchEvent(new Event("change-requests-refresh"));

    expect(
      await screen.findByText(/Abmeldung noch abschließen.*02\.09\.2026/),
    ).toBeVisible();
    expect(mockFetch).toHaveBeenCalledTimes(2);
  });
});
