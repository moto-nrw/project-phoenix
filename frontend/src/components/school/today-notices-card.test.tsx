import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { StaffNotice } from "~/lib/staff-notices-api";
import { TodayNoticesCard } from "./today-notices-card";

const mutate = vi.hoisted(() => vi.fn());
const useSWRAuth = vi.hoisted(() => vi.fn());
const swrResult = vi.hoisted(() => ({
  current: {
    data: undefined as StaffNotice[] | undefined,
    error: undefined as Error | undefined,
    isValidating: false,
    mutate,
  },
}));

vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: { user: { id: "1" } } }),
}));

vi.mock("~/lib/swr", () => ({ useSWRAuth }));

vi.mock("~/lib/school-staff-notices-api", () => ({
  SCHOOL_NOTICES_TODAY_KEY: "school-staff-notices-today",
  schoolStaffNoticesApi: { fetchTodaysNotices: vi.fn() },
}));

vi.mock("~/lib/school-url", () => ({
  schoolPath: (path: string) => path,
}));

describe("TodayNoticesCard", () => {
  beforeEach(() => {
    mutate.mockReset();
    useSWRAuth.mockReset();
    useSWRAuth.mockImplementation(() => swrResult.current);
    swrResult.current = {
      data: undefined,
      error: undefined,
      isValidating: false,
      mutate,
    };
  });

  it("zeigt einen Ladefehler mit Wiederholen statt keine Hinweise", () => {
    swrResult.current = {
      data: undefined,
      error: new Error("request failed"),
      isValidating: false,
      mutate,
    };

    render(<TodayNoticesCard />);

    expect(screen.getByRole("alert")).toHaveTextContent(
      "Die Tagesinformationen konnten nicht geladen werden.",
    );

    fireEvent.click(screen.getByRole("button", { name: "Erneut laden" }));
    expect(mutate).toHaveBeenCalledTimes(1);
  });

  it("behält zuletzt geladene Hinweise bei einem Aktualisierungsfehler sichtbar", () => {
    swrResult.current = {
      data: [
        {
          id: "1",
          title: "Raumwechsel",
          body: "Heute im Mehrzweckraum.",
          priority: "info",
          audience: "lehrkraft",
          valid_from: "2026-09-09",
          weekdays: [],
          week_pattern: 0,
          requires_acknowledgement: false,
          active: true,
        },
      ],
      error: new Error("request failed"),
      isValidating: false,
      mutate,
    };

    render(<TodayNoticesCard />);

    expect(screen.getByText("Raumwechsel")).toBeVisible();
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Die zuletzt geladenen Hinweise bleiben sichtbar.",
    );
  });
});
