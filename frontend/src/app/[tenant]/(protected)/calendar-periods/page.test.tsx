import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  useRequireAdmin: vi.fn(),
}));

vi.mock("~/lib/hooks/use-require-admin", () => ({
  useRequireAdmin: mocks.useRequireAdmin,
}));

vi.mock("~/components/planning/calendar-periods-editor", () => ({
  CalendarPeriodsEditor: () => <div data-testid="calendar-periods-editor" />,
}));

vi.mock("~/components/planning/closing-days-editor", () => ({
  ClosingDaysEditor: () => <div data-testid="closing-days-editor" />,
}));

vi.mock("~/components/ui/loading", () => ({
  Loading: () => <div data-testid="loading" />,
}));

vi.mock("~/components/ui/desktop-only-notice", () => ({
  DesktopOnlyNotice: () => null,
}));

import CalendarPeriodsPage from "./page";

describe("CalendarPeriodsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.useRequireAdmin.mockReturnValue({ isReady: true });
  });

  // Bewusst kein timetable.enabled-Gate: Kalenderzeiträume werden auch von
  // den Anmeldephasen genutzt und bleiben bei abgeschaltetem Planungsbereich
  // erreichbar.
  it("renders the editor without a timetable route gate", () => {
    render(<CalendarPeriodsPage />);

    expect(screen.getByTestId("calendar-periods-editor")).toBeInTheDocument();
  });

  it("shows the loading state until the admin gate resolves", () => {
    mocks.useRequireAdmin.mockReturnValue({ isReady: false });

    render(<CalendarPeriodsPage />);

    expect(screen.getByTestId("loading")).toBeInTheDocument();
    expect(
      screen.queryByTestId("calendar-periods-editor"),
    ).not.toBeInTheDocument();
  });
});
