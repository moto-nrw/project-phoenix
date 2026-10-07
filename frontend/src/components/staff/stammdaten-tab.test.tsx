import { fireEvent, render as renderUi, screen } from "@testing-library/react";
import type { ReactElement } from "react";
import { ToastProvider } from "~/contexts/ToastContext";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

import { StammdatenTab } from "./stammdaten-tab";

// The tab reports a failed reveal as a toast (#2511); the app mounts the
// provider globally.
function render(ui: ReactElement) {
  return renderUi(ui, { wrapper: ToastProvider });
}

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

  it("zeigt bei einem Ladefehler keinen vermeintlich leeren Wert", async () => {
    swrResult.current = {
      data: undefined,
      error: new ApiError("boom", 503, { code: "general.unavailable" }),
      isLoading: false,
      isValidating: false,
      mutate,
    };

    render(
      <StammdatenTab
        staffId="42"
        canManagePayroll
        canManagePayrollSettings={false}
      />,
    );

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Personalnummer"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("Nicht gesetzt")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Bearbeiten" }),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
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

    render(
      <StammdatenTab staffId="42" canManagePayroll canManagePayrollSettings />,
    );

    expect(screen.getByText("Nicht gesetzt")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Bearbeiten" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Abrechnung" })).toHaveAttribute(
      "href",
      "/test-tenant/payroll",
    );
  });

  it("zeigt ohne Settings-Berechtigung keinen Link zur Abrechnung", () => {
    swrResult.current = {
      data: null,
      error: undefined,
      isLoading: false,
      isValidating: false,
      mutate,
    };

    render(
      <StammdatenTab
        staffId="42"
        canManagePayroll
        canManagePayrollSettings={false}
      />,
    );

    expect(
      screen.queryByRole("link", { name: "Abrechnung" }),
    ).not.toBeInTheDocument();
  });
});
