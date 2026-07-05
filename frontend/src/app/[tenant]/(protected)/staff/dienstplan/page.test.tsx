import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  replace: vi.fn(),
  useSession: vi.fn(),
  useSWRAuth: vi.fn(),
  isAdmin: vi.fn(),
  useBerlinToday: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
}));

vi.mock("next-auth/react", () => ({
  useSession: mocks.useSession,
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ replace: mocks.replace }),
}));

vi.mock("~/lib/auth-utils", () => ({
  isAdmin: mocks.isAdmin,
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: mocks.useSWRAuth,
}));

vi.mock("~/lib/hooks/use-berlin-today", () => ({
  useBerlinToday: mocks.useBerlinToday,
}));

vi.mock("~/components/staff/dienstplan-week-grid", () => ({
  DienstplanWeekGrid: ({
    weekDays,
    todayIso,
  }: {
    weekDays: string[];
    todayIso: string;
  }) => (
    <div data-testid="dienstplan-grid">
      <span data-testid="dienstplan-week-days">{weekDays.join(",")}</span>
      <span data-testid="dienstplan-today">{todayIso}</span>
    </div>
  ),
}));

vi.mock("~/components/staff/shift-edit-modal", () => ({
  ShiftEditModal: () => <div data-testid="shift-modal" />,
}));

vi.mock("~/components/ui/loading", () => ({
  Loading: () => <div data-testid="loading" />,
}));

import DienstplanPage from "./page";

describe("DienstplanPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.useSession.mockReturnValue({
      data: { user: { id: "1", isAdmin: true, token: "token" } },
      status: "authenticated",
    });
    mocks.isAdmin.mockReturnValue(true);
    mocks.useBerlinToday.mockReturnValue("2026-07-05");
  });

  it("shows a blocking load error instead of the editable grid", () => {
    const mutateStaff = vi.fn();
    const mutateShifts = vi.fn();
    mocks.useSWRAuth.mockImplementation((key: string) => {
      if (key === "dienstplan-staff") {
        return {
          data: [{ id: "7", firstName: "Ada", lastName: "Lovelace" }],
          error: undefined,
          isLoading: false,
          mutate: mutateStaff,
        };
      }
      return {
        data: undefined,
        error: new Error("server down"),
        isLoading: false,
        mutate: mutateShifts,
      };
    });

    render(<DienstplanPage />);

    expect(
      screen.getByText(
        /Der Dienstplan konnte nicht vollständig geladen werden/,
      ),
    ).toBeInTheDocument();
    expect(screen.queryByTestId("dienstplan-grid")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Erneut laden" }));
    expect(mutateStaff).toHaveBeenCalledTimes(1);
    expect(mutateShifts).toHaveBeenCalledTimes(1);
  });

  it("anchors the current week and today marker to the Berlin calendar date", () => {
    mocks.useBerlinToday.mockReturnValue("2026-07-06");
    mocks.useSWRAuth.mockImplementation((key: string) => {
      if (key === "dienstplan-staff") {
        return {
          data: [{ id: "7", firstName: "Ada", lastName: "Lovelace" }],
          error: undefined,
          isLoading: false,
          mutate: vi.fn(),
        };
      }
      return {
        data: [],
        error: undefined,
        isLoading: false,
        mutate: vi.fn(),
      };
    });

    render(<DienstplanPage />);

    expect(screen.getByTestId("dienstplan-week-days")).toHaveTextContent(
      "2026-07-06,2026-07-07,2026-07-08,2026-07-09,2026-07-10",
    );
    expect(screen.getByTestId("dienstplan-today")).toHaveTextContent(
      "2026-07-06",
    );
    expect(mocks.useSWRAuth).toHaveBeenCalledWith(
      "dienstplan-shifts-2026-07-06-2026-07-10",
      expect.any(Function),
    );
  });
});
