import { fireEvent, render, screen } from "@testing-library/react";
import { ApiError } from "~/lib/api-error";
import { NextIntlClientProvider } from "next-intl";
import { beforeEach, describe, expect, it, vi } from "vitest";
import deMessages from "~/i18n/messages/de.json";
import {
  listGroupThreads,
  listMessageThreads,
  listMyChildren,
} from "~/lib/parent-api";
import { catalogText } from "~/test/error-catalog-text";
import { ParentMessagesPage } from "./parent-messages-page";

vi.mock("~/lib/parent-url", () => ({
  parentPath: (path: string) => path,
}));

vi.mock("~/lib/parent-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/parent-api")>()),
  listMyChildren: vi.fn(),
  listMessageThreads: vi.fn(),
  listGroupThreads: vi.fn(),
}));

vi.mock("~/lib/hooks/use-messages-activity", () => ({
  useMessagesActivity: () => undefined,
}));

vi.mock("~/components/parent/ogs-conversation", () => ({
  OgsConversation: ({ studentId }: { studentId: string }) => (
    <div data-testid="conversation">{studentId}</div>
  ),
}));

const mockedChildren = vi.mocked(listMyChildren);
const mockedThreads = vi.mocked(listMessageThreads);
const mockedGroupThreads = vi.mocked(listGroupThreads);

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

const mia = { ...felix, student_id: "43", first_name: "Mia" };

function renderPage() {
  return render(
    <NextIntlClientProvider locale="de" messages={deMessages}>
      <ParentMessagesPage />
    </NextIntlClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  mockedThreads.mockResolvedValue([]);
  mockedGroupThreads.mockResolvedValue([]);
});

const sommerfest = {
  id: "9",
  title: "Sommerfest am Freitag",
  target_label: "Gruppe Sonnenblumen",
  school_name: "Schule am Berg",
  counterpart_name: "OGS Schule am Berg",
  closed: false,
  last_message_at: "2026-09-09T08:00:00Z",
  last_message_body: "Wer kann einen Kuchen mitbringen?",
  unread: 1,
  children: [{ student_id: "42", first_name: "Felix", last_name: "Schneider" }],
};

describe("ParentMessagesPage – Gruppennachrichten (#4784)", () => {
  it("zeigt bei einem Kind die Gruppennachrichten ueber dem Chat", async () => {
    mockedChildren.mockResolvedValue([felix]);
    mockedGroupThreads.mockResolvedValue([sommerfest]);

    renderPage();

    expect(await screen.findByTestId("conversation")).toHaveTextContent("42");
    expect(
      screen.getByRole("heading", { name: "Gruppennachrichten der OGS" }),
    ).toBeInTheDocument();
    const link = screen.getByRole("link", { name: /Sommerfest am Freitag/ });
    expect(link).toHaveAttribute("href", "/parents/messages/group/9");
    expect(link).toHaveTextContent("Gruppe Sonnenblumen");
    expect(link).toHaveTextContent("Wer kann einen Kuchen mitbringen?");
    expect(link).toHaveTextContent("1");
  });

  it("markiert eine geschlossene Gruppennachricht bei mehreren Kindern", async () => {
    mockedChildren.mockResolvedValue([felix, mia]);
    mockedGroupThreads.mockResolvedValue([
      { ...sommerfest, closed: true, unread: 0 },
    ]);

    renderPage();

    const link = await screen.findByRole("link", {
      name: /Sommerfest am Freitag/,
    });
    expect(link).toHaveTextContent("Geschlossen");
  });

  it("zeigt ohne Gruppennachricht keinen leeren Abschnitt", async () => {
    mockedChildren.mockResolvedValue([felix]);

    renderPage();

    expect(await screen.findByTestId("conversation")).toBeInTheDocument();
    expect(screen.getByTestId("conversation")).toHaveTextContent("42");
    expect(
      screen.queryByRole("heading", { name: "Gruppennachrichten der OGS" }),
    ).not.toBeInTheDocument();
  });

  // Die neue, optionale Gruppennachrichten-Abfrage darf die bestehende
  // Unterhaltungsliste nicht verdecken, wenn ihr Endpunkt vorübergehend fehlt.
  it("zeigt die Unterhaltungen trotz fehlender Gruppennachrichten", async () => {
    mockedChildren.mockResolvedValue([felix, mia]);
    mockedThreads.mockResolvedValue([
      {
        thread_id: "t1",
        student_id: "42",
        student_name: "Felix Schneider",
        school_name: "Schule am Berg",
        counterpart_name: "OGS Schule am Berg",
        last_message_kind: "message",
        last_message_read_by_staff: false,
        unread: 0,
      },
    ]);
    mockedGroupThreads.mockRejectedValue(new Error("nicht erreichbar"));

    renderPage();

    // Anker: die bestehende Unterhaltungsliste ist geladen.
    expect(
      await screen.findByRole("link", { name: /Felix Schneider/ }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "Gruppennachrichten der OGS" }),
    ).not.toBeInTheDocument();
  });
});

describe("ParentMessagesPage", () => {
  it("zeigt Seitenkopf und Zeilengeometrie schon waehrend des Ladens", () => {
    mockedChildren.mockReturnValue(new Promise(() => {}));

    renderPage();

    expect(
      screen.getByRole("heading", { name: "Nachrichten" }),
    ).toBeInTheDocument();
    expect(
      screen.getAllByTestId("parent-page-section-row-skeleton"),
    ).toHaveLength(3);
  });

  it("ist bei einem Kind selbst die Unterhaltung, ohne Zwischenschritt", async () => {
    mockedChildren.mockResolvedValue([felix]);
    renderPage();
    expect(await screen.findByTestId("conversation")).toHaveTextContent("42");
    expect(mockedThreads).not.toHaveBeenCalled();
  });

  it("zeigt bei mehreren Kindern eine Zeile je Kind mit Vorschau und Zaehler", async () => {
    mockedChildren.mockResolvedValue([felix, mia]);
    mockedThreads.mockResolvedValue([
      {
        thread_id: "t1",
        student_id: "43",
        student_name: "Mia Schneider",
        school_name: "Schule am Berg",
        counterpart_name: "OGS Schule am Berg",
        last_message_at: "2026-08-17T06:00:00Z",
        last_sender_kind: "staff",
        last_message_body: "Bitte an die Trinkflasche denken.",
        last_message_kind: "message",
        unread: 2,
      },
    ] as unknown as Awaited<ReturnType<typeof listMessageThreads>>);

    renderPage();

    expect(
      await screen.findByText("Austausch mit der OGS"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Nachrichten", level: 1 }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Lesen Sie Nachrichten und schreiben Sie direkt an die OGS.",
      ),
    ).toBeInTheDocument();

    const link = await screen.findByRole("link", { name: /Mia Schneider/ });
    expect(link).toHaveAttribute("href", "/parents/messages/43");
    expect(
      screen.getByText("Bitte an die Trinkflasche denken."),
    ).toBeInTheDocument();
    expect(screen.getByText("Für Mia Schneider")).toBeInTheDocument();
    expect(screen.getAllByText("OGS-Team der Schule am Berg")).toHaveLength(2);
    expect(screen.getByText("2")).toBeInTheDocument();
    // Ein Kind ohne Unterhaltung bekommt trotzdem seine Zeile.
    expect(
      screen.getByRole("link", { name: /Felix Schneider/ }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Tippen Sie, um zu schreiben."),
    ).toBeInTheDocument();
  });

  it("zeigt fuer eine eigene ungelesene Nachricht zwei graue Versandhaken", async () => {
    mockedChildren.mockResolvedValue([felix, mia]);
    mockedThreads.mockResolvedValue([
      {
        thread_id: "t1",
        student_id: "42",
        student_name: "Felix Schneider",
        school_name: "Schule am Berg",
        counterpart_name: "OGS Schule am Berg",
        last_message_at: "2026-08-17T05:00:00Z",
        last_sender_kind: "guardian",
        last_message_body: "Können Sie kurz zurückrufen?",
        last_message_kind: "message",
        last_message_read_by_staff: false,
        unread: 0,
      },
    ]);

    renderPage();

    const sent = await screen.findByLabelText("Gesendet", { selector: "span" });
    expect(sent.className).toContain("text-gray-500");
  });

  it("haelt die Zeilen als gut treffbare Chatvorschau", async () => {
    mockedChildren.mockResolvedValue([felix, mia]);
    renderPage();
    const link = await screen.findByRole("link", { name: /Felix Schneider/ });
    expect(link.className).toContain("min-h-[88px]");
  });

  it("zeigt die OGS als Gegenueber und den Lesestatus der letzten eigenen Nachricht", async () => {
    mockedChildren.mockResolvedValue([felix, mia]);
    mockedThreads.mockResolvedValue([
      {
        thread_id: "t1",
        student_id: "43",
        student_name: "Mia Schneider",
        school_name: "Schule am Berg",
        counterpart_name: "OGS Schule am Berg",
        last_message_at: "2026-08-17T06:00:00Z",
        last_sender_kind: "staff",
        last_message_body: "Bitte an die Trinkflasche denken.",
        last_message_kind: "message",
        last_message_read_by_staff: false,
        unread: 2,
      },
      {
        thread_id: "t2",
        student_id: "42",
        student_name: "Felix Schneider",
        school_name: "Schule am Berg",
        counterpart_name: "OGS Schule am Berg",
        last_message_at: "2026-08-17T05:00:00Z",
        last_sender_kind: "guardian",
        last_message_body: "Danke für die Rückmeldung.",
        last_message_kind: "message",
        last_message_read_by_staff: true,
        unread: 0,
      },
    ]);

    renderPage();

    const unreadLink = await screen.findByRole("link", {
      name: /Mia Schneider.*OGS-Team der Schule am Berg/,
    });
    expect(unreadLink).toHaveAttribute("href", "/parents/messages/43");
    expect(
      screen.getAllByText("OGS-Team der Schule am Berg")[0]?.className,
    ).toContain("font-bold");
    expect(
      screen.getByText("Bitte an die Trinkflasche denken."),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("17.08.2026, 08:00").className).toContain(
      "text-moto-blue",
    );

    const readLink = screen.getByRole("link", {
      name: /Felix Schneider.*OGS-Team der Schule am Berg/,
    });
    expect(readLink).toHaveAttribute("href", "/parents/messages/42");
    const readReceipt = screen.getByLabelText("Von der OGS gelesen", {
      selector: "span",
    });
    expect(readReceipt).toBeInTheDocument();
    expect(readReceipt.className).toContain("text-moto-blue");
    expect(screen.getByText("Sie: Danke für die Rückmeldung.")).toBeVisible();
    expect(screen.queryByText(/Zu .* ·/)).not.toBeInTheDocument();
  });

  // #2518: der Ladefehler steht mit Katalogtext und Wiederholen an der Stelle
  // der Liste, nie der Satz vom Server.
  it("zeigt einen Ladefehler mit Wiederholen statt einer leeren Seite", async () => {
    mockedChildren
      .mockRejectedValueOnce(
        new ApiError("db kaputt", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValueOnce([felix]);
    renderPage();

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Liste der Unterhaltungen"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/db kaputt/)).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    expect(await screen.findByTestId("conversation")).toHaveTextContent("42");
    expect(mockedChildren).toHaveBeenCalledTimes(2);
  });
});
