import "@testing-library/jest-dom/vitest";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { AnnouncementRecipient } from "~/lib/parent-announcements-api";
import { ReachSummary, RecipientList } from "./announcement-detail";

const recipients: AnnouncementRecipient[] = [
  {
    account_id: "1",
    first_name: "Mira",
    last_name: "Muster",
    status: "pending",
  },
];

describe("RecipientList", () => {
  it("announces the active recipient status filter", () => {
    render(
      <RecipientList
        recipients={recipients}
        showStatus
        statusFilter="pending"
        onStatusFilter={vi.fn()}
        nameFilter=""
        onNameFilter={vi.fn()}
      />,
    );

    expect(screen.getByRole("button", { name: "Alle (1)" })).toHaveAttribute(
      "aria-pressed",
      "false",
    );
    expect(
      screen.getByRole("button", { name: "Ausstehend (1)" }),
    ).toHaveAttribute("aria-pressed", "true");
  });
});

describe("ReachSummary", () => {
  it("counts the parents the publication reached and names its day", () => {
    render(
      <ReachSummary
        stats={{
          target_count: 12,
          read_count: 5,
          acknowledged_count: 2,
          recorded: true,
        }}
        requiresAcknowledgement
        publishedAt="2026-09-07T08:30:00Z"
      />,
    );

    expect(
      screen.getByRole("progressbar", { name: "5 von 12 gelesen" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("progressbar", { name: "2 von 12 bestätigt" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Gezählt werden die Eltern der Kinder, die die Mitteilung am 07.09.2026 erreicht hat.",
      ),
    ).toBeInTheDocument();
  });

  it("claims no publication day for counts of the audience of today", () => {
    render(
      <ReachSummary
        stats={{ target_count: 3, read_count: 1, acknowledged_count: 0 }}
        requiresAcknowledgement={false}
        publishedAt="2026-09-07T08:30:00Z"
      />,
    );

    expect(
      screen.getByRole("progressbar", { name: "1 von 3 gelesen" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("progressbar", { name: /bestätigt/ })).toBeNull();
    expect(screen.queryByText(/Gezählt werden/)).toBeNull();
  });
});
