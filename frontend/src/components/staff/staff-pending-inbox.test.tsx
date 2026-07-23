import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { Staff, StaffAbsenceRow } from "~/lib/staff-api";

import { StaffPendingInbox } from "./staff-pending-inbox";

const mocks = vi.hoisted(() => ({
  approve: vi.fn(),
  swrMutate: vi.fn(),
  toastError: vi.fn(),
  toastSuccess: vi.fn(),
}));

vi.mock("swr", async (importOriginal) => {
  const actual = await importOriginal<typeof import("swr")>();
  return {
    ...actual,
    useSWRConfig: () => ({ mutate: mocks.swrMutate }),
  };
});

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    error: mocks.toastError,
    success: mocks.toastSuccess,
  }),
}));

vi.mock("~/lib/staff-api", () => ({
  staffAbsenceService: {
    approve: mocks.approve,
  },
}));

function requestedRow(): StaffAbsenceRow {
  return {
    id: 17,
    staff_id: 42,
    absence_type: "vacation",
    date_start: "2027-07-10",
    date_end: "2027-07-11",
    half_day: false,
    note: "Sommerurlaub",
    status: "requested",
    working_days: 2,
    requested_at: "2027-06-01T08:00:00Z",
  };
}

describe("StaffPendingInbox refresh", () => {
  beforeEach(() => {
    mocks.approve.mockReset();
    mocks.swrMutate.mockReset();
    mocks.toastError.mockReset();
    mocks.toastSuccess.mockReset();
    mocks.approve.mockResolvedValue(undefined);
  });

  it("revalidates the shared SWR key family once after a decision", async () => {
    const staffList = [
      {
        id: "42",
        firstName: "Mila",
        lastName: "Muster",
      } as Staff,
    ];

    render(<StaffPendingInbox rows={[requestedRow()]} staffList={staffList} />);

    fireEvent.click(screen.getByRole("button", { name: "Genehmigen" }));

    await waitFor(() => {
      expect(mocks.approve).toHaveBeenCalledWith(17);
      expect(mocks.swrMutate).toHaveBeenCalledTimes(1);
    });

    const predicate = mocks.swrMutate.mock.calls[0]?.[0] as (
      key: unknown,
    ) => boolean;
    expect(predicate("tenant:staff-pending-absences-all")).toBe(true);
    expect(predicate("tenant:staff-pending-absences-42")).toBe(true);
    expect(predicate("tenant:staff-list")).toBe(false);
  });
});
