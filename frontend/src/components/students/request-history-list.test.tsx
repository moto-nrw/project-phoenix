import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  MasterDataHistoryList,
  RequestHistoryCard,
} from "./request-history-list";
import type { StaffMasterDataHistoryEntry } from "~/lib/master-data-review-api";

const { mockListHistory } = vi.hoisted(() => ({
  mockListHistory: vi.fn(),
}));

vi.mock("~/lib/master-data-review-api", () => ({
  listMasterDataChangeRequestHistory: (
    cursor?: string,
  ): ReturnType<typeof mockListHistory> => mockListHistory(cursor),
}));

function entry(
  id: string,
  overrides: Partial<StaffMasterDataHistoryEntry> = {},
): StaffMasterDataHistoryEntry {
  return {
    id,
    student_id: "42",
    first_name: "Lara",
    last_name: "Lehmann",
    target: "person",
    field_key: "first_name",
    old_value: "Lara",
    new_value: "Clara",
    status: "rejected",
    created_at: "2026-08-17T09:00:00Z",
    decided_at: "2026-08-18T10:00:00Z",
    decided_by_name: "Rieke Reviewer",
    review_reason: "zu kurz",
    ...overrides,
  };
}

describe("MasterDataHistoryList", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("lädt die erste Seite und zeigt Status, Person und Begründung", async () => {
    mockListHistory.mockResolvedValue({ items: [entry("1")] });

    render(<MasterDataHistoryList />);

    expect(await screen.findByText("Lara Lehmann")).toBeInTheDocument();
    expect(screen.getByText("Abgelehnt")).toBeInTheDocument();
    expect(
      screen.getByText(/Entschieden am 18\.08\.2026 von Rieke Reviewer/),
    ).toBeInTheDocument();
    expect(screen.getByText("„zu kurz“")).toBeInTheDocument();
    expect(screen.getByText(/Lara → Clara/)).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Weitere Einträge laden" }),
    ).toBeNull();
  });

  it("lädt weitere Seiten über den Cursor nach und hängt sie an", async () => {
    mockListHistory
      .mockResolvedValueOnce({ items: [entry("1")], next_cursor: "abc" })
      .mockResolvedValueOnce({
        items: [entry("2", { first_name: "Ben", last_name: "Berger" })],
      });

    render(<MasterDataHistoryList />);

    const button = await screen.findByRole("button", {
      name: "Weitere Einträge laden",
    });
    fireEvent.click(button);

    expect(await screen.findByText("Ben Berger")).toBeInTheDocument();
    expect(screen.getByText("Lara Lehmann")).toBeInTheDocument();
    expect(mockListHistory).toHaveBeenLastCalledWith("abc");
    await waitFor(() =>
      expect(
        screen.queryByRole("button", { name: "Weitere Einträge laden" }),
      ).toBeNull(),
    );
  });

  it("zeigt den leeren Zustand ohne entschiedene Anfragen", async () => {
    mockListHistory.mockResolvedValue({ items: [] });

    render(<MasterDataHistoryList />);

    expect(
      await screen.findByText("Noch keine entschiedenen Anfragen."),
    ).toBeInTheDocument();
  });

  it("zeigt den Fehlerzustand, wenn das Laden scheitert", async () => {
    mockListHistory.mockRejectedValue(
      new Error("Historie konnte nicht geladen werden"),
    );

    render(<MasterDataHistoryList />);

    expect(
      await screen.findByText("Historie konnte nicht geladen werden"),
    ).toBeInTheDocument();
  });
});

describe("RequestHistoryCard", () => {
  it("lässt bei automatisch übernommenen Änderungen die Person weg", () => {
    render(
      <RequestHistoryCard
        childName="Lara Lehmann"
        status="auto_applied"
        decidedAt="2026-08-18T10:00:00Z"
      />,
    );

    expect(screen.getByText("Automatisch übernommen")).toBeInTheDocument();
    expect(
      screen.getByText(/Entschieden am 18\.08\.2026$/),
    ).toBeInTheDocument();
  });
});
