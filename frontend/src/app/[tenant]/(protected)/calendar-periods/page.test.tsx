import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  useRequireAdmin: vi.fn(),
  useSWRAuth: vi.fn(),
}));

vi.mock("~/lib/hooks/use-require-admin", () => ({
  useRequireAdmin: mocks.useRequireAdmin,
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: mocks.useSWRAuth,
}));

vi.mock("~/components/planning/calendar-periods-editor", () => ({
  CalendarPeriodsEditor: () => <div data-testid="calendar-periods-editor" />,
}));

vi.mock("~/components/ui/loading", () => ({
  Loading: () => <div data-testid="loading" />,
}));

vi.mock("~/components/ui/desktop-only-notice", () => ({
  DesktopOnlyNotice: () => null,
}));

import CalendarPeriodsPage from "./page";

function mockSchema(timetableEnabled: boolean | undefined) {
  mocks.useSWRAuth.mockReturnValue({
    data:
      timetableEnabled === undefined
        ? null
        : {
            tabs: [
              {
                categories: [
                  {
                    items: [
                      { key: "timetable.enabled", value: timetableEnabled },
                    ],
                  },
                ],
              },
            ],
          },
    error: undefined,
    isLoading: false,
    mutate: vi.fn(),
  });
}

describe("CalendarPeriodsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.useRequireAdmin.mockReturnValue({ isReady: true });
  });

  it("renders the editor when timetable.enabled is not false", () => {
    mockSchema(true);

    render(<CalendarPeriodsPage />);

    expect(screen.getByTestId("calendar-periods-editor")).toBeInTheDocument();
    expect(
      screen.queryByTestId("calendar-periods-disabled-state"),
    ).not.toBeInTheDocument();
  });

  it("renders normally when the settings schema is unreadable", () => {
    // fetchSettingsSchema liefert null ohne Leserecht — Graceful Default wie
    // Sidebar und die übrigen Planungsseiten.
    mockSchema(undefined);

    render(<CalendarPeriodsPage />);

    expect(screen.getByTestId("calendar-periods-editor")).toBeInTheDocument();
  });

  it("renders the disabled state when timetable.enabled is false", () => {
    mockSchema(false);

    render(<CalendarPeriodsPage />);

    expect(
      screen.getByTestId("calendar-periods-disabled-state"),
    ).toBeInTheDocument();
    expect(
      screen.queryByTestId("calendar-periods-editor"),
    ).not.toBeInTheDocument();
  });

  it("shows the loading state until admin gate and schema resolve", () => {
    mocks.useRequireAdmin.mockReturnValue({ isReady: false });
    mocks.useSWRAuth.mockReturnValue({
      data: undefined,
      error: undefined,
      isLoading: false,
      mutate: vi.fn(),
    });

    render(<CalendarPeriodsPage />);

    expect(screen.getByTestId("loading")).toBeInTheDocument();
  });
});
