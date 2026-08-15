import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { listAnnouncements, type ParentAnnouncement } from "~/lib/parent-api";
import { ParentNewsPage } from "./parent-news-page";

vi.mock("~/lib/parent-api", () => ({
  listAnnouncements: vi.fn(),
  markAnnouncementRead: vi.fn(),
  acknowledgeAnnouncement: vi.fn(),
  respondToAnnouncement: vi.fn(),
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
  mocked.mockResolvedValue([]);
});

describe("ParentNewsPage", () => {
  it("heisst 'Aus der OGS', nicht 'Neuigkeiten'", async () => {
    render(<ParentNewsPage />);
    expect(
      await screen.findByRole("heading", { name: "Aus der OGS" }),
    ).toBeInTheDocument();
    expect(screen.queryByText("Neuigkeiten")).not.toBeInTheDocument();
  });

  it("hebt eine ungelesene Meldung im Titelgewicht hervor", async () => {
    mocked.mockResolvedValue([announcement({ read: false })]);
    render(<ParentNewsPage />);
    const card = await screen.findByRole("button", { name: /Sommerfest/ });
    expect(screen.getByText("Sommerfest").className).toContain("font-semibold");
    expect(card.querySelector('[data-moto-duotone-tone="blue"]')).not.toBeNull();
  });

  it("markiert eine offene Umfrage als Ankuendigung und bietet Antworten", async () => {
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
      card.querySelector('[data-moto-duotone-tone="amber"]'),
    ).not.toBeNull();
    expect(screen.getByText("Antworten")).toBeInTheDocument();
  });

  it("bietet bei bestaetigungspflichtigen Meldungen 'Gelesen bestätigen'", async () => {
    mocked.mockResolvedValue([
      announcement({ requires_acknowledgement: true, acknowledged: false }),
    ]);
    render(<ParentNewsPage />);
    expect(await screen.findByText("Gelesen bestätigen")).toBeInTheDocument();
  });

  it("sagt in Alltagssprache, wenn nichts anliegt", async () => {
    render(<ParentNewsPage />);
    expect(
      await screen.findByText("Derzeit gibt es nichts Neues aus der OGS."),
    ).toBeInTheDocument();
  });
});
