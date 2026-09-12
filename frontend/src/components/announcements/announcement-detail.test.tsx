import "@testing-library/jest-dom/vitest";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { AnnouncementRecipient } from "~/lib/parent-announcements-api";
import { RecipientList } from "./announcement-detail";

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
