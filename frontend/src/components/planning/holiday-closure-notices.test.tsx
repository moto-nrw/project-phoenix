import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type { HolidayClosureNotice } from "~/lib/holiday-closure-notices";
import { catalogText } from "~/test/error-catalog-text";

const { store, mockNotify, mockReload, session } = vi.hoisted(() => ({
  store: { current: [] as HolidayClosureNotice[] },
  mockNotify: vi.fn(),
  mockReload: vi.fn(),
  session: { permissions: ["schedules:update"] as string[] },
}));

vi.mock("~/lib/holiday-closure-notices", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  notifyHolidayClosureNotice: mockNotify,
}));

// Ein kleiner Zwischenspeicher: `mutate` liest den Stand des Servers neu.
vi.mock("~/lib/swr", async () => {
  const React = await import("react");
  return {
    useSWRAuth: (key: string | null) => {
      const [data, setData] = React.useState(store.current);
      const mutate = React.useCallback(() => {
        mockReload();
        setData(store.current);
        return Promise.resolve(store.current);
      }, []);
      return {
        data: key === null ? undefined : data,
        error: undefined,
        mutate,
      };
    },
  };
});

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { id: "1", permissions: session.permissions } },
    status: "authenticated",
  }),
}));

import { HolidayClosureNotices } from "./holiday-closure-notices";

function notice(
  overrides: Partial<HolidayClosureNotice> = {},
): HolidayClosureNotice {
  return {
    id: "7",
    schoolId: "1",
    schoolName: "OGS Am Park",
    closingDayId: "3",
    reason: "Pädagogischer Tag",
    previous: null,
    current: { startDate: "2026-10-12", endDate: "2026-10-14" },
    revision: 2,
    status: "suggested",
    changed: false,
    canNotify: true,
    families: 12,
    confirmedAt: null,
    updatedAt: "2026-09-08T10:00:00Z",
    ...overrides,
  };
}

function renderNotices() {
  return render(
    <ToastProvider>
      <HolidayClosureNotices version={0} />
    </ToastProvider>,
  );
}

const button = (title = "„Pädagogischer Tag“") =>
  screen.queryByRole("button", { name: `Eltern informieren: ${title}` });

describe("HolidayClosureNotices (ADR 0196)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    session.permissions = ["schedules:update"];
    store.current = [notice()];
  });

  it("proposes the notice with the number of families, never names", () => {
    renderNotices();

    const card = screen
      .getByText("Eltern der Ferienbetreuung informieren")
      .closest("section")!;
    expect(within(card).getByText("„Pädagogischer Tag“")).toBeInTheDocument();
    expect(
      within(card).getByText("Geschlossen: 12.10.2026 bis 14.10.2026"),
    ).toBeInTheDocument();
    expect(
      within(card).getByText(
        "12 Familien haben an diesen Tagen Ferienbetreuung gebucht.",
      ),
    ).toBeInTheDocument();
    expect(
      within(card).getByText("Die Eltern wurden noch nicht informiert."),
    ).toBeInTheDocument();
    expect(within(card).getByText("1 offen")).toBeInTheDocument();
    expect(button()).toBeInTheDocument();
  });

  it("says „1 Familie hat“ for a single family", () => {
    store.current = [notice({ families: 1 })];
    renderNotices();

    expect(
      screen.getByText(
        "1 Familie hat an diesen Tagen Ferienbetreuung gebucht.",
      ),
    ).toBeInTheDocument();
  });

  it("names a moved closing day with both ranges and a deleted one as open again", () => {
    store.current = [
      notice({
        previous: { startDate: "2026-10-05", endDate: "2026-10-05" },
      }),
      notice({
        id: "8",
        reason: "Teamtag",
        previous: { startDate: "2026-10-20", endDate: "2026-10-21" },
        current: null,
      }),
    ];
    renderNotices();

    expect(
      screen.getByText("Neu: 12.10.2026 bis 14.10.2026"),
    ).toBeInTheDocument();
    expect(screen.getByText("Vorher: Mo., 05.10.2026")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Gelöscht: 20.10.2026 bis 21.10.2026. Diese Tage sind wieder geöffnet.",
      ),
    ).toBeInTheDocument();
  });

  it("shows each state with its text and the button only where a click sends", () => {
    store.current = [
      notice({ id: "1", reason: "A", status: "sending", canNotify: false }),
      notice({ id: "2", reason: "B", status: "failed", canNotify: true }),
      notice({ id: "3", reason: "C", status: "unknown", canNotify: false }),
      notice({ id: "4", reason: "D", changed: true, revision: 5 }),
      notice({ id: "5", reason: "E", status: "sent", canNotify: false }),
      notice({ id: "6", reason: "F", status: "no_mail", canNotify: false }),
    ];
    renderNotices();

    expect(screen.getByText("Wird gesendet")).toBeInTheDocument();
    expect(screen.getByText("Bitte warten Sie.")).toBeInTheDocument();
    expect(button("„A“")).not.toBeInTheDocument();

    expect(screen.getByText("Nicht angekommen")).toBeInTheDocument();
    expect(
      screen.getByText("Bitte versuchen Sie es erneut."),
    ).toBeInTheDocument();
    expect(button("„B“")).toBeInTheDocument();

    expect(screen.getByText("Ergebnis unbekannt")).toBeInTheDocument();
    expect(
      screen.getByText("Bitte lassen Sie den Versand vom moto-Team prüfen."),
    ).toBeInTheDocument();
    expect(button("„C“")).not.toBeInTheDocument();

    expect(
      screen.getByText(
        "Der Schließtag wurde geändert. Bitte prüfen Sie den Hinweis.",
      ),
    ).toBeInTheDocument();
    expect(button("„D“")).toBeInTheDocument();

    // Erledigt: zugeklappt, ohne Knopf. Nur Vorschläge nennen Familien.
    expect(
      screen.getByRole("button", { name: /Erledigte Hinweise \(2\)/ }),
    ).toBeInTheDocument();
    expect(button("„E“")).not.toBeInTheDocument();
    expect(button("„F“")).not.toBeInTheDocument();
    expect(
      screen.getAllByText(
        "12 Familien haben an diesen Tagen Ferienbetreuung gebucht.",
      ),
    ).toHaveLength(1);
  });

  it("sends with one click at the revision it showed and shows „Wird gesendet“ at once", async () => {
    let resolve: (value: HolidayClosureNotice) => void = () => undefined;
    mockNotify.mockReturnValue(
      new Promise<HolidayClosureNotice>((done) => {
        resolve = done;
      }),
    );
    renderNotices();

    fireEvent.click(button()!);
    expect(mockNotify).toHaveBeenCalledWith("7", 2);
    expect(mockNotify).toHaveBeenCalledTimes(1);
    expect(screen.getByText("Wird gesendet")).toBeInTheDocument();
    expect(button()).not.toBeInTheDocument();

    const sending = notice({ status: "sending", canNotify: false });
    store.current = [sending];
    resolve(sending);
    expect(
      await screen.findByText("Die E-Mails an 12 Familien werden gesendet."),
    ).toBeInTheDocument();
    expect(mockReload).toHaveBeenCalled();
    expect(screen.getByText("Wird gesendet")).toBeInTheDocument();
  });

  it("shows a changed closing day through the error path and loads the new state", async () => {
    mockNotify.mockRejectedValue(
      new ApiError("conflict", 409, {
        code: "timetable.closing_day_notice_changed",
      }),
    );
    renderNotices();

    store.current = [notice({ revision: 3, families: 1 })];
    fireEvent.click(button()!);
    expect(
      await screen.findByText(
        catalogText(
          "timetable.closing_day_notice_changed",
          "die E-Mail an die Eltern",
        ),
      ),
    ).toBeInTheDocument();
    expect(
      await screen.findByText(
        "1 Familie hat an diesen Tagen Ferienbetreuung gebucht.",
      ),
    ).toBeInTheDocument();
    fireEvent.click(button()!);
    expect(mockNotify).toHaveBeenLastCalledWith("7", 3);
  });

  it("shows nothing without the right to change closing days", () => {
    session.permissions = ["schedules:read"];
    render(
      <ToastProvider>
        <p>Schließtage</p>
        <HolidayClosureNotices version={0} />
      </ToastProvider>,
    );

    expect(screen.getByText("Schließtage")).toBeInTheDocument();
    expect(
      screen.queryByText("Eltern der Ferienbetreuung informieren"),
    ).not.toBeInTheDocument();
  });
});
