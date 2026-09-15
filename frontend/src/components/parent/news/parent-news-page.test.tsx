import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { listAnnouncements, type ParentAnnouncement } from "~/lib/parent-api";
import { ParentNewsPage } from "./parent-news-page";

let searchParams = new URLSearchParams();
const replace = vi.fn();

vi.mock("next/navigation", () => ({
  usePathname: () => "/parents/news",
  useRouter: () => ({ replace }),
  useSearchParams: () => searchParams,
}));

vi.mock("~/lib/parent-api", () => ({
  listAnnouncements: vi.fn(),
  markAnnouncementRead: vi.fn(),
  acknowledgeAnnouncement: vi.fn(),
  respondToAnnouncement: vi.fn(),
  // Anhänge (#2890): the detail view asks for them on open. No attachments in
  // these fixtures, so the section renders nothing.
  listAnnouncementAttachments: vi.fn(() => Promise.resolve([])),
  announcementAttachmentDownloadUrl: vi.fn(() => "/download"),
  ParentApiError: class extends Error {},
}));

const mocked = vi.mocked(listAnnouncements);

function announcement(
  overrides: Partial<ParentAnnouncement> = {},
): ParentAnnouncement {
  return {
    id: "a1",
    title: "Sommerfest",
    body: "Wir feiern am Freitag.",
    priority: "info",
    requires_acknowledgement: false,
    school_name: "Schule am Berg",
    published_at: "2026-08-10T08:00:00Z",
    read: true,
    acknowledged: false,
    response_type: "none",
    ...overrides,
  } as ParentAnnouncement;
}

beforeEach(() => {
  vi.clearAllMocks();
  searchParams = new URLSearchParams();
  mocked.mockResolvedValue([]);
});

describe("ParentNewsPage", () => {
  it("bildet Gruppenueberschrift und Karten waehrend des Ladens ab", () => {
    mocked.mockReturnValue(new Promise(() => {}));

    render(<ParentNewsPage />);

    expect(
      screen.getByRole("heading", { name: "Elternbriefe" }),
    ).toBeInTheDocument();
    const skeleton = screen.getByTestId("parent-news-skeleton");
    expect(skeleton.querySelectorAll(".rounded-2xl.border")).toHaveLength(2);
    expect(skeleton.querySelectorAll(".animate-pulse").length).toBeGreaterThan(
      10,
    );
  });

  it("zeigt Elternbriefe im gemeinsamen Seitenkopf", async () => {
    render(<ParentNewsPage />);
    expect(
      await screen.findByRole("heading", { name: "Elternbriefe" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Post von der OGS")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Mitteilungen, Umfragen und wichtige Informationen Ihrer OGS.",
      ),
    ).toBeInTheDocument();
  });

  it("hebt eine ungelesene Meldung im Titelgewicht hervor", async () => {
    mocked.mockResolvedValue([announcement({ read: false })]);
    render(<ParentNewsPage />);
    const card = await screen.findByRole("button", { name: /Sommerfest/ });
    expect(screen.getByText("Sommerfest").className).toContain("font-semibold");
    expect(
      card.querySelector('[data-moto-duotone-tone="blue"]'),
    ).not.toBeNull();
  });

  it("zeigt eine offene Umfrage ohne Status-Pill und bietet Antworten", async () => {
    mocked.mockResolvedValue([
      announcement({
        response_type: "single_choice",
        options: [{ id: "o1", label: "Ja" }],
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
    render(<ParentNewsPage />);
    const card = await screen.findByRole("button", { name: /Sommerfest/ });
    expect(
      card.querySelector('[data-moto-duotone-tone="blue"]'),
    ).not.toBeNull();
    expect(screen.getByText("Umfrage")).toBeInTheDocument();
    expect(screen.getByText("Antworten")).toBeInTheDocument();
    expect(screen.queryByText("Antwort nötig")).not.toBeInTheDocument();
  });

  it("bietet bei bestaetigungspflichtigen Meldungen 'Gelesen bestätigen'", async () => {
    mocked.mockResolvedValue([
      announcement({ requires_acknowledgement: true, acknowledged: false }),
    ]);
    render(<ParentNewsPage />);
    expect(await screen.findByText("Gelesen bestätigen")).toBeInTheDocument();
  });

  it("öffnet den per Link angeforderten Elternbrief", async () => {
    searchParams = new URLSearchParams("brief=a1");
    mocked.mockResolvedValue([announcement()]);

    render(<ParentNewsPage />);

    expect(await screen.findByRole("dialog")).toHaveTextContent("Sommerfest");
  });

  it("gruppiert offene Elternbriefe vor erledigten Einträgen", async () => {
    mocked.mockResolvedValue([
      announcement({ id: "done", title: "Bereits gelesen", read: true }),
      announcement({
        id: "ack",
        title: "Bestätigung fehlt",
        read: true,
        requires_acknowledgement: true,
        acknowledged: false,
      }),
      announcement({ id: "new", title: "Noch ungelesen", read: false }),
    ]);

    render(<ParentNewsPage />);

    expect(
      await screen.findByRole("heading", { name: "Offen, 2" }),
    ).toBeVisible();
    expect(screen.getByRole("heading", { name: "Erledigt" })).toBeVisible();
    const cards = screen.getAllByRole("button");
    expect(cards[0]).toHaveTextContent("Bestätigung fehlt");
    expect(cards[1]).toHaveTextContent("Noch ungelesen");
    expect(cards[2]).toHaveTextContent("Bereits gelesen");
  });

  it("sagt in Alltagssprache, wenn nichts anliegt", async () => {
    render(<ParentNewsPage />);
    expect(
      await screen.findByText("Derzeit gibt es keine neuen Elternbriefe."),
    ).toBeInTheDocument();
  });
});
