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

    const link = await screen.findByRole("link", { name: /Mia Schneider/ });
    expect(link).toHaveAttribute("href", "/parents/messages/43");
    expect(
      screen.getByText("OGS: Bitte an die Trinkflasche denken."),
    ).toBeInTheDocument();
    expect(screen.getByText("2")).toBeInTheDocument();
    // Ein Kind ohne Unterhaltung bekommt trotzdem seine Zeile.
    expect(
      screen.getByRole("link", { name: /Felix Schneider/ }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Noch keine Nachrichten. Tippen Sie, um zu schreiben."),
    ).toBeInTheDocument();
  });

  it("haelt die Zeilen auf mindestens 72 px", async () => {
    mockedChildren.mockResolvedValue([felix, mia]);
    renderPage();
    const link = await screen.findByRole("link", { name: /Felix Schneider/ });
    expect(link.className).toContain("min-h-[72px]");
  });
});
