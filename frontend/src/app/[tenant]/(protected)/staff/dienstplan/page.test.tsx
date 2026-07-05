import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  replace: vi.fn(),
  useSession: vi.fn(),
  useSWRAuth: vi.fn(),
  isAdmin: vi.fn(),
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

vi.mock("~/components/staff/dienstplan-week-grid", () => ({
  DienstplanWeekGrid: () => <div data-testid="dienstplan-grid" />,
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
});
