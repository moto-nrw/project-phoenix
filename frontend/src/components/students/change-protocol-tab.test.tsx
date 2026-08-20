import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { StudentChangeProtocolTab } from "./change-protocol-tab";
import { listAggregatedRequestHistory } from "~/lib/change-request-list-api";

vi.mock("~/lib/change-request-list-api", () => ({
  listAggregatedOpenRequests: vi.fn(),
  listAggregatedRequestHistory: vi.fn(),
  listEnrollmentChangeRequests: vi.fn(),
}));

vi.mock("~/components/students/request-history-item", () => ({
  RequestHistoryItem: ({
    item,
  }: {
    item: { request_type: string; data: { id: string } };
  }) => (
    <div>
      history-item-{item.request_type}-{item.data.id}
    </div>
  ),
}));

const mockListHistory = vi.mocked(listAggregatedRequestHistory);

describe("StudentChangeProtocolTab", () => {
  it("lädt die Historie dieses Kindes und zeigt sie chronologisch", async () => {
    mockListHistory.mockResolvedValue({
      items: [
        {
          request_type: "care_schedule",
          occurred_at: "2026-08-19T10:00:00Z",
          data: { id: "7" },
        },
        {
          request_type: "direct_correction",
          occurred_at: "2026-08-18T10:00:00Z",
          data: { id: "3" },
        },
      ] as never,
    });

    render(<StudentChangeProtocolTab studentId="42" />);

    await screen.findByText("history-item-care_schedule-7");
    expect(
      screen.getAllByText(/history-item-/).map((node) => node.textContent),
    ).toEqual([
      "history-item-care_schedule-7",
      "history-item-direct_correction-3",
    ]);
    expect(mockListHistory).toHaveBeenCalledWith(
      expect.objectContaining({ studentId: "42" }),
    );
  });

  it("behauptet nicht, es gäbe nichts, solange ältere Seiten offen sind", async () => {
    mockListHistory.mockResolvedValue({ items: [], next_cursor: "weiter" });

    render(<StudentChangeProtocolTab studentId="42" />);

    expect(
      await screen.findByText("Hier ist noch nichts gefunden."),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Noch keine entschiedenen Anfragen."),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Weitere Einträge laden" }),
    ).toBeInTheDocument();
  });

  it("zeigt einen leeren Zustand, wenn nichts geändert wurde", async () => {
    mockListHistory.mockResolvedValue({ items: [] });

    render(<StudentChangeProtocolTab studentId="42" />);

    expect(
      await screen.findByText("Noch keine entschiedenen Anfragen."),
    ).toBeInTheDocument();
  });
});
