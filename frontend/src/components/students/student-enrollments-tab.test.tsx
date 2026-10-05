import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { StudentEnrollmentsTab } from "./student-enrollments-tab";

const exportRequests = vi.fn();
const mutate = vi.fn(async () => undefined);
const swrResult: { data: unknown; isLoading: boolean; error: unknown } = {
  data: undefined,
  isLoading: false,
  error: undefined,
};

vi.mock("~/lib/swr", () => ({
  useSWRAuth: () => ({ ...swrResult, mutate }),
}));

vi.mock("~/components/enrollment/admin-enrollment-detail", () => ({
  ChildExtraFields: () => null,
  ChildOfferingAdjustment: () => null,
  ChildOfferings: () => null,
  RequestExtraSection: () => null,
  formatDateTime: () => "09.09.2026, 12:00",
  formatPlainDate: () => "01.01.2018",
}));

vi.mock("~/lib/enrollment-admin-api", () => ({
  exportStudentEnrollmentRequests: (...args: unknown[]) =>
    exportRequests(...args) as unknown,
  listStudentEnrollmentRequests: vi.fn(),
}));

function renderTab() {
  return render(
    <ToastProvider>
      <StudentEnrollmentsTab studentId="7" />
    </ToastProvider>,
  );
}

describe("StudentEnrollmentsTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    swrResult.error = undefined;
    swrResult.data = undefined;
  });

  it("shows a failed load in place with a retry", async () => {
    swrResult.error = new ApiError("boom", 500, { code: "general.server" });
    renderTab();

    expect(
      await screen.findByText(
        "Die Liste der Anmeldungen konnte nicht bearbeitet werden. Bitte versuchen Sie es später erneut.",
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(mutate).toHaveBeenCalledOnce();
  });

  it("reports a failed export as a toast with retry", async () => {
    swrResult.data = [
      {
        id: "r1",
        phase_name: "Halbjahr",
        submitted_at: "2026-09-01T10:00:00Z",
        children: [],
      },
    ];
    exportRequests.mockRejectedValue(
      new ApiError("down", 503, { code: "general.unavailable" }),
    );
    renderTab();

    fireEvent.click(screen.getByRole("button", { name: "PDF" }));

    expect(
      await screen.findByText(
        "Die Exportdatei ist gerade nicht erreichbar. Bitte versuchen Sie es erneut.",
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Wiederholen/ }));
    expect(exportRequests).toHaveBeenCalledTimes(2);
    expect(exportRequests).toHaveBeenLastCalledWith("7", "pdf");
  });
});
