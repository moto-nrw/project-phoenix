import { render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { beforeEach, describe, expect, it, vi } from "vitest";
import deMessages from "~/i18n/messages/de.json";
import { listMessageThreads, listMyChildren } from "~/lib/parent-api";
import { ParentMessagesPage } from "./parent-messages-page";

vi.mock("~/lib/parent-url", () => ({
  parentPath: (path: string) => path,
}));

vi.mock("~/lib/parent-api", () => ({
  listMyChildren: vi.fn(),
  listMessageThreads: vi.fn(),
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
});
