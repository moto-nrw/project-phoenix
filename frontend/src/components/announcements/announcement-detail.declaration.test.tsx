import "@testing-library/jest-dom/vitest";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { Announcement } from "~/lib/parent-announcements-api";
import { AnnouncementDetail } from "./announcement-detail";

vi.mock("./declaration-status-panel", () => ({
  DeclarationStatusPanel: () => <div data-testid="declaration-status" />,
}));

vi.mock("~/lib/parent-announcements-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/parent-announcements-api")>()),
  fetchAnnouncementStats: vi.fn(() =>
    Promise.resolve({ target_count: 0, read_count: 0, acknowledged_count: 0 }),
  ),
  fetchAnnouncementRecipients: vi.fn(() => Promise.resolve([])),
}));

const consent: Announcement = {
  id: "42",
  title: "Ausflug in den Zoo",
  body: "Wir fahren am Freitag in den Zoo.",
  priority: "info",
  requires_acknowledgement: false,
  send_email: false,
  status: "published",
  published_at: "2026-09-01T08:00:00Z",
  active: true,
  created_at: "2026-08-30T08:00:00Z",
  updated_at: "2026-09-01T08:00:00Z",
  targets: [{ target_type: "school_all" }],
  response_type: "none",
  options: [],
  delivery_mode: "declaration",
  email_audience: "portal_only",
  declaration_kind: "consent",
  declaration_signers: "any",
  declaration_revocable: true,
  declaration_requires_password: false,
};

describe("AnnouncementDetail for an Einverständnis (#3430)", () => {
  it("names the mode Einverständnis and shows no Art or read confirmation row", () => {
    render(
      <AnnouncementDetail
        announcement={consent}
        groups={[]}
        activities={[]}
        onReminded={vi.fn()}
      />,
    );

    expect(screen.getByText("Einverständnis")).toBeInTheDocument();
    expect(screen.getByText("Stand der Antworten")).toBeInTheDocument();
    expect(screen.getByTestId("declaration-status")).toBeInTheDocument();
    expect(
      screen.getByText("Erlaubt, auch nach der Frist"),
    ).toBeInTheDocument();
    expect(screen.queryByText("Art")).not.toBeInTheDocument();
    expect(screen.queryByText("Lesebestätigung")).not.toBeInTheDocument();
    expect(screen.queryByText(/Erklärung|Kenntnis/)).not.toBeInTheDocument();
  });

  it("says a scheduled reminder also reaches guardians who already answered", () => {
    render(
      <AnnouncementDetail
        announcement={{ ...consent, reminder_at: "2099-10-01T06:00:00Z" }}
        groups={[]}
        activities={[]}
        onReminded={vi.fn()}
      />,
    );

    expect(
      screen.getByText(
        "Geht an alle Empfänger der Mitteilung, auch wenn sie schon geantwortet haben.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/gelesen oder bestätigt/),
    ).not.toBeInTheDocument();
  });
});
