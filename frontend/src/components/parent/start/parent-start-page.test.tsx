import { render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import deMessages from "~/i18n/messages/de.json";
import {
  getChildFeatures,
  getChildToday,
  listAnnouncements,
  listMessageThreads,
  listMyChildren,
} from "~/lib/parent-api";
import { getParentCalendar } from "~/lib/personal-calendar-api";
import { ParentStartPage } from "./parent-start-page";

vi.mock("~/lib/parent-url", () => ({
  parentPath: (path: string) => path,
}));

vi.mock("~/lib/parent-api", () => ({
  listMyChildren: vi.fn(),
  listAnnouncements: vi.fn(),
  listMessageThreads: vi.fn(),
  getChildFeatures: vi.fn(),
  getChildToday: vi.fn(),
  UNKNOWN_CHILD_TODAY: { at_ogs: null, state: "unknown" },
}));

vi.mock("~/lib/personal-calendar-api", () => ({
  getParentCalendar: vi.fn(),
}));

vi.mock("~/lib/shell-auth-context", () => ({
  useShellAuth: () => ({ profile: { firstName: "Sabine" } }),
}));

vi.mock("~/components/parent/news/news-components", () => ({
  NewsDetailModal: () => <div data-testid="news-detail-modal" />,
  isOpenPoll: (item: { poll_options?: unknown[]; poll_closed?: boolean }) =>
    Array.isArray(item.poll_options) &&
    item.poll_options.length > 0 &&
    !item.poll_closed,
}));

const mockedChildren = vi.mocked(listMyChildren);
const mockedAnnouncements = vi.mocked(listAnnouncements);
const mockedThreads = vi.mocked(listMessageThreads);
const mockedFeatures = vi.mocked(getChildFeatures);
const mockedToday = vi.mocked(getChildToday);
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
});

afterEach(() => {
  vi.useRealTimers();
});

describe("ParentStartPage", () => {
  describe("Begruessung", () => {
    it("gruesst morgens mit dem Vornamen", async () => {
      renderPage();
      expect(
        await screen.findByRole("heading", { name: "Guten Morgen, Sabine" }),
      ).toBeInTheDocument();
    });

    it("gruesst mittags mit 'Guten Tag'", async () => {
      vi.setSystemTime(new Date("2026-08-17T12:00:00Z")); // 14:00 Berlin
      renderPage();
      expect(
        await screen.findByRole("heading", { name: "Guten Tag, Sabine" }),
      ).toBeInTheDocument();
    });

    it("gruesst abends mit 'Guten Abend'", async () => {
      vi.setSystemTime(new Date("2026-08-17T18:00:00Z")); // 20:00 Berlin
      renderPage();
      expect(
        await screen.findByRole("heading", { name: "Guten Abend, Sabine" }),
      ).toBeInTheDocument();
    });
  });

  describe("Zu erledigen", () => {
    it("erscheint nicht ohne Inhalt und zeigt stattdessen den ruhigen Zustand", async () => {
      renderPage();
      expect(await screen.findByText("Alles erledigt")).toBeInTheDocument();
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
      expect(screen.getByText("Neue Nachricht der OGS")).toBeInTheDocument();
      expect(screen.queryByText("Alles erledigt")).not.toBeInTheDocument();
    });

    it("erscheint mit einer offenen Termineinladung", async () => {
      mockedCalendar.mockResolvedValue({
        from: "",
        to: "",
        events: [
          {
            id: "e1",
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
          },
        ],
      } as unknown as Awaited<ReturnType<typeof getParentCalendar>>);
      renderPage();
      expect(await screen.findByText("Zu erledigen")).toBeInTheDocument();
      expect(screen.getByText("Elternabend")).toBeInTheDocument();
    });
  });

  it("zeigt je Kind genau eine Tageskarte", async () => {
    mockedChildren.mockResolvedValue([felix, mia]);
    renderPage();
    await waitFor(() =>
      expect(screen.getAllByTestId("child-day-state-icon")).toHaveLength(2),
    );
    expect(screen.getByText("Felix Schneider")).toBeInTheDocument();
    expect(screen.getByText("Mia Schneider")).toBeInTheDocument();
  });

  it("bietet 'Neue Anmeldung' nicht als Hauptaktion an", async () => {
    renderPage();
    await screen.findByText("Felix Schneider");
    expect(screen.queryByText("Neue Anmeldung")).not.toBeInTheDocument();
  });
});
