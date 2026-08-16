import { render, screen, waitFor, within } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import deMessages from "~/i18n/messages/de.json";
import {
  getChildFeatures,
  getChildToday,
  fetchParentProfile,
  listAnnouncements,
  listMessageThreads,
  listMyChildren,
  type ParentAnnouncement,
} from "~/lib/parent-api";
import {
  getParentCalendar,
  type CalendarEvent,
} from "~/lib/personal-calendar-api";
import { ParentStartPage } from "./parent-start-page";

vi.mock("~/lib/parent-url", () => ({
  parentPath: (path: string) => path,
}));

vi.mock("~/lib/hooks/use-berlin-today", () => ({
  useBerlinToday: () => "2026-05-14",
}));

vi.mock("~/lib/parent-api", () => ({
  listMyChildren: vi.fn(),
  listAnnouncements: vi.fn(),
  listMessageThreads: vi.fn(),
  getChildFeatures: vi.fn(),
  getChildToday: vi.fn(),
  fetchParentProfile: vi.fn(),
  UNKNOWN_CHILD_TODAY: { at_ogs: null, state: "unknown" },
}));

vi.mock("~/lib/personal-calendar-api", () => ({
  getParentCalendar: vi.fn(),
}));

vi.mock("~/lib/shell-auth-context", () => ({
  useShellAuth: () => ({ profile: { firstName: "Sabine" } }),
}));

vi.mock("~/components/parent/news/news-components", async (importOriginal) => {
  const actual =
    await importOriginal<
      typeof import("~/components/parent/news/news-components")
    >();
  return {
    ...actual,
    NewsDetailModal: () => <div data-testid="news-detail-modal" />,
  };
});

const mockedChildren = vi.mocked(listMyChildren);
const mockedAnnouncements = vi.mocked(listAnnouncements);
const mockedThreads = vi.mocked(listMessageThreads);
const mockedFeatures = vi.mocked(getChildFeatures);
const mockedToday = vi.mocked(getChildToday);
const mockedProfile = vi.mocked(fetchParentProfile);
const mockedCalendar = vi.mocked(getParentCalendar);

const felix = {
  student_id: "42",
  tenant_id: "7",
  first_name: "Felix",
  last_name: "Schneider",
  school_class: "1a",
  status: "active",
  school_name: "Schule am Berg",
  school_slug: "berg",
} as unknown as Awaited<ReturnType<typeof listMyChildren>>[number];

const mia = {
  ...felix,
  student_id: "43",
  first_name: "Mia",
  school_class: "3b",
};

function announcement(
  overrides: Partial<ParentAnnouncement>,
): ParentAnnouncement {
  return {
    id: "announcement-1",
    title: "Information der OGS",
    body: "Inhalt",
    priority: "info",
    requires_acknowledgement: false,
    school_name: "Schule am Berg",
    published_at: "2026-08-17T06:00:00Z",
    read: false,
    acknowledged: false,
    response_type: "none",
    ...overrides,
  };
}

function appointment(overrides: Partial<CalendarEvent>): CalendarEvent {
  return {
    id: "appointment-1",
    source: "appointment",
    title: "Elternabend",
    start_date: "2026-09-01",
    end_date: "2026-09-01",
    start_time: "18:00",
    end_time: "20:00",
    all_day: false,
    can_respond: true,
    can_edit: false,
    response_status: "pending",
    student_name: "Felix Schneider",
    ...overrides,
  };
}

function renderPage() {
  return render(
    <NextIntlClientProvider locale="de" messages={deMessages}>
      <ParentStartPage />
    </NextIntlClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.useFakeTimers({ shouldAdvanceTime: true });
  // 09:00 Berlin (Sommerzeit) — "Guten Morgen".
  vi.setSystemTime(new Date("2026-08-17T07:00:00Z"));
  mockedChildren.mockResolvedValue([felix]);
  mockedAnnouncements.mockResolvedValue([]);
  mockedThreads.mockResolvedValue([]);
  mockedCalendar.mockResolvedValue({ from: "", to: "", events: [] });
  mockedFeatures.mockResolvedValue({
    sick_note_enabled: false,
    pickup_change_enabled: false,
    notes_enabled: false,
  } as unknown as Awaited<ReturnType<typeof getChildFeatures>>);
  mockedToday.mockResolvedValue({ at_ogs: null, state: "unknown" });
  mockedProfile.mockResolvedValue({
    first_name: "Karin",
    last_name: "Klein",
    portal_locale: "de",
  });
});

afterEach(() => {
  vi.useRealTimers();
});

describe("ParentStartPage", () => {
  it("zeigt inhaltstreue Skeletons, solange die Startseitendaten laden", () => {
    mockedChildren.mockImplementation(() => new Promise(() => undefined));
    mockedAnnouncements.mockImplementation(() => new Promise(() => undefined));
    mockedThreads.mockImplementation(() => new Promise(() => undefined));
    mockedCalendar.mockImplementation(() => new Promise(() => undefined));

    renderPage();

    expect(
      screen.getByTestId("parent-start-todo-skeleton"),
    ).toBeInTheDocument();
    expect(
      screen.queryByTestId("parent-start-todo-skeleton-row"),
    ).not.toBeInTheDocument();
    expect(
      screen.getByTestId("parent-start-child-skeleton"),
    ).toBeInTheDocument();
    expect(
      screen.queryByTestId("parent-start-child-action-skeleton"),
    ).not.toBeInTheDocument();
  });

  describe("Begruessung", () => {
    it("gruesst morgens mit dem Vornamen", async () => {
      renderPage();
      expect(
        await screen.findByRole("heading", { name: "Guten Morgen, Karin" }),
      ).toBeInTheDocument();
      expect(screen.getByText("Elternportal")).toBeInTheDocument();
      expect(
        screen.getByText("Das Wichtigste für heute auf einen Blick."),
      ).toBeInTheDocument();
    });

    it("gruesst mittags mit 'Guten Tag'", async () => {
      vi.setSystemTime(new Date("2026-08-17T12:00:00Z")); // 14:00 Berlin
      renderPage();
      expect(
        await screen.findByRole("heading", { name: "Guten Tag, Karin" }),
      ).toBeInTheDocument();
    });

    it("gruesst abends mit 'Guten Abend'", async () => {
      vi.setSystemTime(new Date("2026-08-17T18:00:00Z")); // 20:00 Berlin
      renderPage();
      expect(
        await screen.findByRole("heading", { name: "Guten Abend, Karin" }),
      ).toBeInTheDocument();
    });
  });

  describe("Zu erledigen", () => {
    it("erscheint nicht ohne Inhalt und zeigt stattdessen den ruhigen Zustand", async () => {
      renderPage();
      expect(
        await screen.findByRole("heading", { name: "Alles erledigt" }),
      ).toBeInTheDocument();
      expect(
        screen.getByText("Es gibt gerade nichts zu tun."),
      ).toBeInTheDocument();
      expect(screen.queryByText("Zu erledigen")).not.toBeInTheDocument();
    });

    it("erscheint mit einer ungelesenen Nachricht", async () => {
      mockedThreads.mockResolvedValue([
        {
          thread_id: "t1",
          student_id: "42",
          student_name: "Felix Schneider",
          counterpart_name: "OGS Schule am Berg",
          unread: 2,
          last_message_at: "2026-08-17T06:00:00Z",
        },
      ] as unknown as Awaited<ReturnType<typeof listMessageThreads>>);
      renderPage();
      expect(await screen.findByText("Zu erledigen")).toBeInTheDocument();
      expect(
        screen.getByText(
          "Offene Nachrichten, Termine und Rückmeldungen auf einen Blick.",
        ),
      ).toBeInTheDocument();
      expect(screen.getByText("2 neue Nachrichten")).toBeInTheDocument();
      expect(screen.getByText("Ungelesen:")).toBeInTheDocument();
      expect(screen.getByText("Heute, 08:00")).toBeInTheDocument();
      expect(screen.queryByText("Alles erledigt")).not.toBeInTheDocument();
    });

    it("erscheint mit einer offenen Termineinladung", async () => {
      mockedCalendar.mockResolvedValue({
        from: "",
        to: "",
        events: [appointment({})],
      } as unknown as Awaited<ReturnType<typeof getParentCalendar>>);
      renderPage();
      expect(await screen.findByText("Zu erledigen")).toBeInTheDocument();
      expect(screen.getByText("Elternabend")).toBeInTheDocument();
    });

    it("zeigt alle offenen Typen in ihrer Dringlichkeitsreihenfolge", async () => {
      mockedAnnouncements.mockResolvedValue([
        announcement({
          id: "news",
          title: "Sommerfest",
          published_at: "2026-08-16T06:00:00Z",
        }),
        announcement({
          id: "ack",
          title: "Ausflug bestätigen",
          read: true,
          requires_acknowledgement: true,
        }),
        announcement({
          id: "poll",
          title: "Ferienbetreuung wählen",
          read: true,
          response_type: "single_choice",
          response_deadline: "2026-09-10T12:00:00Z",
          options: [
            { id: "yes", label: "Ja" },
            { id: "no", label: "Nein" },
          ],
          children: [
            {
              student_id: "42",
              first_name: "Felix",
              last_name: "Schneider",
              selected_options: [],
            },
          ],
        }),
      ]);
      mockedThreads.mockResolvedValue([
        {
          thread_id: "thread-1",
          student_id: "42",
          student_name: "Felix Schneider",
          counterpart_name: "OGS Schule am Berg",
          unread: 2,
          last_message_at: "2026-08-17T06:00:00Z",
        },
      ] as unknown as Awaited<ReturnType<typeof listMessageThreads>>);
      mockedCalendar.mockResolvedValue({
        from: "",
        to: "",
        events: [appointment({})],
      });

      renderPage();

      const heading = await screen.findByRole("heading", {
        name: "Zu erledigen",
      });
      const section = heading.closest("section");
      expect(section).not.toBeNull();
      const rows = within(section!).getAllByRole("listitem");
      expect(rows.map((row) => row.textContent)).toEqual([
        "Ferienbetreuung wählenUmfrage · Schule am BergHeute, 08:00",
        "ElternabendTermin für Felix Schneider01.09., 18:00",
        "Ungelesen: 2 neue NachrichtenOGS · Zu Felix SchneiderHeute, 08:00",
        "Ungelesen: SommerfestElternbrief · Schule am Berg16.08., 08:00",
        "Ausflug bestätigenBestätigung erforderlich · Schule am BergHeute, 08:00",
      ]);
      expect(
        Array.from(
          section!.querySelectorAll("[data-moto-duotone-tone]"),
          (icon) => icon.getAttribute("data-moto-duotone-tone"),
        ),
      ).toEqual(Array(rows.length).fill("blue"));
    });

    it("laesst erledigte oder nicht handlungsrelevante Eintraege weg", async () => {
      mockedAnnouncements.mockResolvedValue([
        announcement({ id: "read", read: true }),
        announcement({
          id: "answered-poll",
          read: true,
          response_type: "single_choice",
          response_deadline: "2026-09-10T12:00:00Z",
          children: [
            {
              student_id: "42",
              first_name: "Felix",
              last_name: "Schneider",
              selected_options: ["yes"],
            },
          ],
        }),
        announcement({
          id: "closed-poll",
          read: true,
          response_type: "single_choice",
          response_deadline: "2026-08-01T12:00:00Z",
          children: [
            {
              student_id: "42",
              first_name: "Felix",
              last_name: "Schneider",
              selected_options: [],
            },
          ],
        }),
      ]);
      mockedThreads.mockResolvedValue([
        {
          thread_id: "thread-read",
          student_id: "42",
          student_name: "Felix Schneider",
          counterpart_name: "OGS Schule am Berg",
          unread: 0,
        },
      ] as unknown as Awaited<ReturnType<typeof listMessageThreads>>);
      mockedCalendar.mockResolvedValue({
        from: "",
        to: "",
        events: [
          appointment({ id: "accepted", response_status: "accepted" }),
          appointment({ id: "cancelled", cancelled: true }),
          appointment({ id: "info", can_respond: false }),
        ],
      });

      renderPage();

      expect(
        await screen.findByRole("heading", { name: "Alles erledigt" }),
      ).toBeInTheDocument();
      expect(screen.queryByText("Zu erledigen")).not.toBeInTheDocument();
    });

    it("behauptet bei einer unvollstaendigen leeren Uebersicht nicht, dass alles erledigt ist", async () => {
      mockedAnnouncements.mockRejectedValue(new Error("nicht erreichbar"));

      renderPage();

      expect(
        await screen.findByText(
          "Einige Punkte konnten gerade nicht geladen werden.",
        ),
      ).toBeInTheDocument();
      expect(screen.queryByText("Alles erledigt")).not.toBeInTheDocument();
      expect(screen.queryByText("Zu erledigen")).not.toBeInTheDocument();
    });
  });

  it("zeigt je Kind genau eine Tageskarte", async () => {
    mockedChildren.mockResolvedValue([felix, mia]);
    renderPage();
    await waitFor(() =>
      expect(screen.getAllByTestId("child-day-card")).toHaveLength(2),
    );
    expect(screen.getByText("Felix Schneider")).toBeInTheDocument();
    expect(screen.getByText("Mia Schneider")).toBeInTheDocument();
    expect(screen.getAllByTestId("child-card-profile-icon")).toHaveLength(2);
    expect(screen.getAllByText("Donnerstag, 14. Mai 2026")).toHaveLength(2);
  });

  it("zeigt die heutige Abholzeit nur fuer ein Kind in der OGS", async () => {
    mockedToday.mockResolvedValue({
      at_ogs: true,
      state: "present",
      since: "12:38",
      pickup_time: "15:30",
    });

    const { unmount } = renderPage();

    expect(
      await screen.findByText("Abholung heute um 15:30 Uhr"),
    ).toBeInTheDocument();

    unmount();
    mockedToday.mockResolvedValue({
      at_ogs: false,
      state: "left",
      until: "14:45",
      pickup_time: "15:30",
    });
    renderPage();

    await waitFor(() => {
      expect(screen.getByText("Nicht in der OGS")).toBeInTheDocument();
    });
    expect(
      screen.queryByText("Abholung heute um 15:30 Uhr"),
    ).not.toBeInTheDocument();
  });

  it("zeigt auf der Startseite kompakte Aktionen", async () => {
    mockedFeatures.mockResolvedValue({
      sick_note_enabled: true,
      pickup_change_enabled: true,
      notes_enabled: true,
    } as unknown as Awaited<ReturnType<typeof getChildFeatures>>);

    renderPage();

    expect(
      await screen.findByRole("link", { name: "Felix abmelden" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Abholzeit ändern" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Nachricht schreiben" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Krank melden oder entschuldigen"),
    ).not.toBeInTheDocument();
  });

  it("bietet 'Neue Anmeldung' nicht als Hauptaktion an", async () => {
    renderPage();
    await screen.findByText("Felix Schneider");
    expect(screen.queryByText("Neue Anmeldung")).not.toBeInTheDocument();
  });
});
