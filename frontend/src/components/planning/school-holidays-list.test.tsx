import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { mockUseSWRAuth, mockMutate } = vi.hoisted(() => ({
  mockUseSWRAuth: vi.fn(),
  mockMutate: vi.fn(),
}));

vi.mock("~/lib/swr", () => ({ useSWRAuth: mockUseSWRAuth }));

import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type { CalendarPeriod } from "~/lib/calendar-period-helpers";
import { catalogText } from "~/test/error-catalog-text";

import { SchoolHolidaysList, holidayWindow } from "./school-holidays-list";

function period(overrides: Partial<CalendarPeriod>): CalendarPeriod {
  return {
    id: "1",
    tenantId: "1",
    name: "Schuljahr 2026/27",
    periodType: "school_year",
    startDate: "2026-08-01",
    endDate: "2027-07-31",
    weekCycleLength: 1,
    weekCycleAnchor: null,
    isActive: true,
    createdAt: "",
    updatedAt: "",
    ...overrides,
  };
}

const holidays = {
  federalState: "DE-NW",
  federalStateName: "Nordrhein-Westfalen",
  holidays: [
    { date: "2026-10-03", name: "Tag der Deutschen Einheit" },
    { date: "2026-11-01", name: "Allerheiligen" },
  ],
};

describe("holidayWindow", () => {
  it("covers the running active school year", () => {
    expect(
      holidayWindow(
        [
          period({ id: "2", periodType: "holiday", name: "Herbstferien" }),
          period({}),
        ],
        "2026-09-09",
      ),
    ).toEqual({
      from: "2026-08-01",
      to: "2027-07-31",
      schoolYearName: "Schuljahr 2026/27",
    });
  });

  it("falls back to the next twelve months without a running school year", () => {
    expect(holidayWindow([period({ isActive: false })], "2026-09-09")).toEqual({
      from: "2026-09-09",
      to: "2027-09-08",
      schoolYearName: null,
    });
  });
});

describe("SchoolHolidaysList", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockMutate.mockResolvedValue(undefined);
  });

  it("lists the holidays of the school year read-only and names the state", () => {
    mockUseSWRAuth.mockReturnValue({
      data: holidays,
      error: undefined,
      isLoading: false,
      mutate: mockMutate,
    });

    render(
      <SchoolHolidaysList periods={[period({})]} periodsLoading={false} />,
      {
        wrapper: ToastProvider,
      },
    );

    expect(mockUseSWRAuth).toHaveBeenCalledWith(
      "school-holidays-2026-08-01-2027-07-31",
      expect.any(Function),
    );
    const card = screen
      .getByRole("heading", { name: "Gesetzliche Feiertage" })
      .closest("section");
    expect(card).not.toBeNull();
    const list = within(card as HTMLElement);
    expect(
      list.getByText(
        /in Nordrhein-Westfalen im Schuljahr „Schuljahr 2026\/27“/,
      ),
    ).toBeInTheDocument();
    expect(list.getByText("Tag der Deutschen Einheit")).toBeInTheDocument();
    expect(list.getByText("Allerheiligen")).toBeInTheDocument();
    expect(list.getByText(/Sa\.?, 03\.10\.2026/)).toBeInTheDocument();
    // Nur zur Information: außer dem Sortieren keine Aktion an der Liste.
    expect(
      list.queryByRole("button", {
        name: /anlegen|bearbeiten|löschen|ändern|aktionen/i,
      }),
    ).not.toBeInTheDocument();
  });

  it("waits for the periods before it asks for holidays", () => {
    mockUseSWRAuth.mockReturnValue({
      data: undefined,
      error: undefined,
      isLoading: false,
      mutate: mockMutate,
    });

    render(<SchoolHolidaysList periods={[]} periodsLoading />, {
      wrapper: ToastProvider,
    });

    expect(mockUseSWRAuth).toHaveBeenCalledWith(null, expect.any(Function));
  });

  it("shows a failed load with retry instead of an empty list", async () => {
    mockUseSWRAuth.mockReturnValue({
      data: undefined,
      error: new ApiError("down", 503, { code: "general.unavailable" }),
      isLoading: false,
      mutate: mockMutate,
    });

    render(
      <SchoolHolidaysList periods={[period({})]} periodsLoading={false} />,
      {
        wrapper: ToastProvider,
      },
    );

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Liste der Feiertage"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("Keine Feiertage")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(mockMutate).toHaveBeenCalledOnce();
  });
});
