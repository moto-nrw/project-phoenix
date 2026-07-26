import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { StammdatenTab } from "./stammdaten-tab";

const mutate = vi.hoisted(() => vi.fn());
const useSWRAuth = vi.hoisted(() => vi.fn());
const swrResult = vi.hoisted(() => ({
  current: {
    data: undefined as string | null | undefined,
    error: undefined as Error | undefined,
    isLoading: false,
    isValidating: false,
    mutate,
  },
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth,
}));

vi.mock("~/lib/staff-api", () => ({
  staffPayrollNumberService: {
    get: vi.fn(),
    update: vi.fn(),
  },
}));

describe("StammdatenTab", () => {
  beforeEach(() => {
    mutate.mockReset();
    useSWRAuth.mockReset();
    useSWRAuth.mockImplementation(() => swrResult.current);
    swrResult.current = {
      data: undefined,
      error: undefined,
      isLoading: false,
      isValidating: false,
      mutate,
    };
  });

  it("zeigt bei einem Ladefehler keinen vermeintlich leeren Wert", () => {
    swrResult.current = {
      data: undefined,
      error: new Error("request failed"),
      isLoading: false,
      isValidating: false,
      mutate,
    };

    render(<StammdatenTab staffId="42" canManagePayroll />);

    expect(
      screen.getByText("Die Personalnummer konnte nicht geladen werden."),
    ).toBeInTheDocument();
    expect(screen.queryByText("Nicht gesetzt")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Bearbeiten" }),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Erneut laden" }));
    expect(mutate).toHaveBeenCalledTimes(1);
  });

  it("zeigt einen erfolgreich geladenen Nullwert als nicht gesetzt", () => {
    swrResult.current = {
      data: null,
      error: undefined,
      isLoading: false,
      isValidating: false,
      mutate,
    };

    render(<StammdatenTab staffId="42" canManagePayroll />);

    expect(screen.getByText("Nicht gesetzt")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Bearbeiten" }),
    ).toBeInTheDocument();
  });
});
