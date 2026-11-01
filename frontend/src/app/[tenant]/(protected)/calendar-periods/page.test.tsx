import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  useRequirePermission: vi.fn(),
}));

vi.mock("~/lib/hooks/use-require-permission", () => ({
  useRequirePermission: mocks.useRequirePermission,
}));

vi.mock("~/components/planning/calendar-periods-editor", () => ({
  // Der Zustand liegt seit dem Gerüst-Umbau im Hook, den die Seite aufruft;
  // Kopf-Aktionen und Inhaltsblock bekommen ihn als Prop.
  useCalendarPeriods: () => ({
    statusLine: "1 Zeitraum · 1 aktiv",
    loading: false,
  }),
  CalendarPeriodsActions: () => <div data-testid="calendar-periods-actions" />,
  CalendarPeriodsEditor: () => <div data-testid="calendar-periods-editor" />,
}));

vi.mock("~/components/planning/closing-days-editor", () => ({
  ClosingDaysEditor: () => <div data-testid="closing-days-editor" />,
}));

vi.mock("~/components/planning/school-holidays-list", () => ({
  SchoolHolidaysList: () => <div data-testid="school-holidays-list" />,
}));

vi.mock("~/components/planning/school-vacations-list", () => ({
  SchoolVacationsList: () => <div data-testid="school-vacations-list" />,
}));

vi.mock("~/components/timetable/school-vacation-hints", () => ({
  SchoolVacationHints: ({
    showClosingDaysLink,
  }: {
    showClosingDaysLink?: boolean;
  }) => (
    <div
      data-testid="school-vacation-hints"
      data-closing-days-link={String(showClosingDaysLink)}
    />
  ),
}));

vi.mock("~/components/ui/loading", () => ({
  Loading: () => <div data-testid="loading" />,
}));

import CalendarPeriodsPage from "./page";

describe("CalendarPeriodsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.useRequirePermission.mockReturnValue({ isReady: true });
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
    // #4775: die gesetzlichen Feiertage stehen unter den Schließtagen.
    expect(
      screen.getByTestId("school-holidays-list").closest(".hidden"),
    ).toBeNull();
    expect(
      screen.queryByText(/Bitte am Computer öffnen/),
    ).not.toBeInTheDocument();
  });

  it("shows the loading state until the permission gate resolves", () => {
    mocks.useRequirePermission.mockReturnValue({ isReady: false });

    render(<CalendarPeriodsPage />);

    expect(screen.getByTestId("loading")).toBeInTheDocument();
    expect(
      screen.queryByTestId("calendar-periods-editor"),
    ).not.toBeInTheDocument();
  });

  // #5345: die Schulferien und der Ferienhinweis stehen auf der Seite; der
  // Hinweis verlinkt nicht auf die Seite, auf der er steht.
  it("shows the school vacations and the vacation hint without a link to itself", () => {
    render(<CalendarPeriodsPage />);

    expect(screen.getByTestId("school-vacations-list")).toBeInTheDocument();
    expect(screen.getByTestId("school-vacation-hints")).toHaveAttribute(
      "data-closing-days-link",
      "false",
    );
  });
});
