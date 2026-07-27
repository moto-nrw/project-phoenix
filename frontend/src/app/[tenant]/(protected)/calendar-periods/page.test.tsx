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

  // #2033: die Seite war hinter DesktopOnlyNotice plus "hidden lg:block"
  // versteckt und zeigte mobil einen Anmeldungs-Text. Beide Editoren müssen
  // ungegated rendern.
  it("renders both editors without a desktop-only gate", () => {
    render(<CalendarPeriodsPage />);

    const editor = screen.getByTestId("calendar-periods-editor");
    expect(editor.closest(".hidden")).toBeNull();
    expect(
      screen.getByTestId("closing-days-editor").closest(".hidden"),
    ).toBeNull();
    expect(screen.queryByText(/Bitte am Computer öffnen/)).not.toBeInTheDocument();
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
