import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  AggregatedRequestList,
  type AggregatedRequestFilters,
} from "./aggregated-request-list";
import {
  listAggregatedOpenRequests,
  listAggregatedRequestHistory,
  listEnrollmentChangeRequests,
} from "~/lib/change-request-list-api";

vi.mock("~/lib/change-request-list-api", () => ({
  listAggregatedOpenRequests: vi.fn(),
  listAggregatedRequestHistory: vi.fn(),
  listEnrollmentChangeRequests: vi.fn(),
}));

vi.mock("~/components/students/enrollment-request-item", () => ({
  EnrollmentRequestItem: ({ row }: { row: { id: string } }) => (
    <div>enrollment-item-{row.id}</div>
  ),
}));

// Die per-Art-Karten sind separat getestet; hier zählt nur, dass die Liste
// pro request_type die richtige Karte rendert und deren onDecided verarbeitet.
vi.mock("~/components/students/master-data-review-item", () => ({
  MasterDataReviewItem: ({
    row,
    onDecided,
  }: {
    row: { id: string };
    onDecided: (notice: string) => void;
  }) => (
    <button type="button" onClick={() => onDecided("Änderung übernommen")}>
      master-item-{row.id}
    </button>
  ),
}));
vi.mock("~/components/students/care-request-review-item", () => ({
  CareRequestReviewItem: ({ row }: { row: { id: string } }) => (
    <div>care-item-{row.id}</div>
  ),
}));
vi.mock("~/components/students/offering-request-review-item", () => ({
  OfferingRequestReviewItem: ({ row }: { row: { id: string } }) => (
    <div>offering-item-{row.id}</div>
  ),
}));
vi.mock("~/components/students/excused-request-review-item", () => ({
  ExcusedRequestReviewItem: ({ row }: { row: { id: string } }) => (
    <div>excused-item-{row.id}</div>
  ),
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

const mockListOpen = vi.mocked(listAggregatedOpenRequests);
const mockListHistory = vi.mocked(listAggregatedRequestHistory);
const mockListEnrollment = vi.mocked(listEnrollmentChangeRequests);

const NO_FILTERS: AggregatedRequestFilters = {
  search: "",
  types: [],
  statuses: [],
};

function openItem(type: string, id: string) {
  return { request_type: type, data: { id } } as never;
}

/** Wie openItem, aber mit dem Zeitpunkt, nach dem Quellen verschränkt werden. */
function stampedItem(type: string, id: string, occurredAt: string) {
  return { request_type: type, occurred_at: occurredAt, data: { id } } as never;
}

describe("AggregatedRequestList", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockListOpen.mockResolvedValue({ items: [] });
    mockListHistory.mockResolvedValue({ items: [] });
    mockListEnrollment.mockResolvedValue({ items: [] });
  });

  it("verschränkt Anmeldungsänderungen nach Zeitpunkt in die Liste", async () => {
    mockListOpen.mockResolvedValue({
      items: [
        stampedItem("excused", "4", "2026-08-20T10:00:00Z"),
        stampedItem("master_data", "1", "2026-08-18T10:00:00Z"),
      ],
    });
    mockListEnrollment.mockResolvedValue({
      items: [stampedItem("enrollment", "9", "2026-08-19T10:00:00Z")],
    });

    render(
      <AggregatedRequestList
        view="open"
        filters={{ ...NO_FILTERS, includeEnrollment: true }}
      />,
    );

    await screen.findByText("enrollment-item-9");
    const rendered = screen
      .getAllByText(/-item-/)
      .map((node) => node.textContent);
    expect(rendered).toEqual([
      "excused-item-4",
      "enrollment-item-9",
      "master-item-1",
    ]);
  });

  it("fragt bei Art-Filter Anmeldung nur diese Quelle ab", async () => {
    mockListEnrollment.mockResolvedValue({
      items: [stampedItem("enrollment", "9", "2026-08-19T10:00:00Z")],
    });

    render(
      <AggregatedRequestList
        view="history"
        filters={{
          ...NO_FILTERS,
          types: ["enrollment"],
          includeEnrollment: true,
        }}
      />,
    );

    expect(await screen.findByText("enrollment-item-9")).toBeInTheDocument();
    // Der Aggregator kennt die Art nicht und würde sie mit 400 abweisen.
    expect(mockListHistory).not.toHaveBeenCalled();
  });

  it("reicht die unbekannte Art nie an den Aggregator durch", async () => {
    render(
      <AggregatedRequestList
        view="open"
        filters={{
          ...NO_FILTERS,
          types: ["excused", "enrollment"],
          includeEnrollment: true,
        }}
      />,
    );

    await waitFor(() => expect(mockListOpen).toHaveBeenCalledTimes(1));
    expect(mockListOpen).toHaveBeenCalledWith(
      expect.objectContaining({ types: ["excused"] }),
    );
  });

  it("fragt Anmeldungsänderungen ohne Berechtigung gar nicht erst ab", async () => {
    render(<AggregatedRequestList view="open" filters={NO_FILTERS} />);

    await waitFor(() => expect(mockListOpen).toHaveBeenCalledTimes(1));
    expect(mockListEnrollment).not.toHaveBeenCalled();
  });

  it("lässt den Aggregator weg, wenn nur Anmeldungsänderungen erlaubt sind", async () => {
    mockListEnrollment.mockResolvedValue({
      items: [stampedItem("enrollment", "9", "2026-08-19T10:00:00Z")],
    });

    render(
      <AggregatedRequestList
        view="open"
        filters={{
          ...NO_FILTERS,
          includeAggregated: false,
          includeEnrollment: true,
        }}
      />,
    );

    expect(await screen.findByText("enrollment-item-9")).toBeInTheDocument();
    expect(mockListOpen).not.toHaveBeenCalled();
  });

  it("rendert offene Anfragen aller Arten in Server-Reihenfolge", async () => {
    mockListOpen.mockResolvedValue({
      items: [
        openItem("offering", "3"),
        openItem("master_data", "1"),
        openItem("excused", "4"),
        openItem("care_schedule", "2"),
      ],
    });

    render(<AggregatedRequestList view="open" filters={NO_FILTERS} />);

    expect(await screen.findByText("offering-item-3")).toBeInTheDocument();
    expect(screen.getByText("master-item-1")).toBeInTheDocument();
    expect(screen.getByText("excused-item-4")).toBeInTheDocument();
    expect(screen.getByText("care-item-2")).toBeInTheDocument();
    expect(mockListOpen).toHaveBeenCalledWith(
      expect.objectContaining({ search: "", types: [] }),
    );
  });

  it("entfernt eine entschiedene Zeile, zeigt den Hinweis und stößt das Badge an", async () => {
    mockListOpen.mockResolvedValue({
      items: [openItem("master_data", "1"), openItem("excused", "4")],
    });
    const refreshListener = vi.fn();
    window.addEventListener("change-requests-refresh", refreshListener);

    render(<AggregatedRequestList view="open" filters={NO_FILTERS} />);
    const decideButton = await screen.findByText("master-item-1");

    fireEvent.click(decideButton);

    expect(screen.queryByText("master-item-1")).toBeNull();
    expect(screen.getByText("excused-item-4")).toBeInTheDocument();
    expect(screen.getByText("Änderung übernommen")).toBeInTheDocument();
    expect(refreshListener).toHaveBeenCalledTimes(1);
    // Der eigene Listener ist unterdrückt: kein zweiter Fetch durch das
    // selbst ausgelöste Event.
    expect(mockListOpen).toHaveBeenCalledTimes(1);
    window.removeEventListener("change-requests-refresh", refreshListener);
  });

  it("lädt bei fremdem change-requests-refresh in place nach", async () => {
    mockListOpen.mockResolvedValue({ items: [openItem("excused", "4")] });

    render(<AggregatedRequestList view="open" filters={NO_FILTERS} />);
    await screen.findByText("excused-item-4");

    mockListOpen.mockResolvedValue({ items: [] });
    act(() => {
      window.dispatchEvent(new Event("change-requests-refresh"));
    });

    await waitFor(() => expect(mockListOpen).toHaveBeenCalledTimes(2));
    await waitFor(() =>
      expect(screen.queryByText("excused-item-4")).toBeNull(),
    );
  });

  it("lädt weitere Einträge über den Cursor nach", async () => {
    // Eine volle Seite plus Cursor: erst dann bleibt etwas zum Nachladen übrig.
    // (Eine kurze Seite mit Cursor zieht die Liste selbst nach, damit niemand
    // blind auf den Knopf drücken muss.)
    mockListOpen.mockResolvedValueOnce({
      items: Array.from({ length: 25 }, (_, index) =>
        openItem("excused", String(index + 1)),
      ),
      next_cursor: "cursor-1",
    });
    mockListOpen.mockResolvedValueOnce({ items: [openItem("excused", "26")] });

    render(<AggregatedRequestList view="open" filters={NO_FILTERS} />);
    await screen.findByText("excused-item-1");

    fireEvent.click(
      screen.getByRole("button", { name: "Weitere Einträge laden" }),
    );

    expect(await screen.findByText("excused-item-26")).toBeInTheDocument();
    expect(screen.getByText("excused-item-1")).toBeInTheDocument();
    expect(mockListOpen).toHaveBeenLastCalledWith(
      expect.objectContaining({ cursor: "cursor-1" }),
    );
    expect(
      screen.queryByRole("button", { name: "Weitere Einträge laden" }),
    ).toBeNull();
  });

  it("lädt bei geänderten Filtern neu und reicht sie serverseitig durch", async () => {
    const { rerender } = render(
      <AggregatedRequestList view="open" filters={NO_FILTERS} />,
    );
    await waitFor(() => expect(mockListOpen).toHaveBeenCalledTimes(1));

    rerender(
      <AggregatedRequestList
        view="open"
        filters={{ search: "Emma", types: ["excused"], statuses: [] }}
      />,
    );

    await waitFor(() => expect(mockListOpen).toHaveBeenCalledTimes(2));
    expect(mockListOpen).toHaveBeenLastCalledWith(
      expect.objectContaining({ search: "Emma", types: ["excused"] }),
    );
  });

  it("rendert die Historie mit Status- und Zeitraum-Filtern", async () => {
    mockListHistory.mockResolvedValue({
      items: [openItem("care_schedule", "7"), openItem("master_data", "8")],
    });

    render(
      <AggregatedRequestList
        view="history"
        filters={{
          search: "",
          types: [],
          statuses: ["approved"],
          from: "2026-08-01",
          to: "2026-08-19",
        }}
      />,
    );

    expect(
      await screen.findByText("history-item-care_schedule-7"),
    ).toBeInTheDocument();
    expect(screen.getByText("history-item-master_data-8")).toBeInTheDocument();
    expect(mockListHistory).toHaveBeenCalledWith(
      expect.objectContaining({
        statuses: ["approved"],
        from: "2026-08-01",
        to: "2026-08-19",
      }),
    );
    expect(mockListOpen).not.toHaveBeenCalled();
  });

  it("zeigt passende Leer-Zustände für Offen und Historie", async () => {
    const { unmount } = render(
      <AggregatedRequestList view="open" filters={NO_FILTERS} />,
    );
    expect(
      await screen.findByText("Keine offenen Anfragen."),
    ).toBeInTheDocument();
    unmount();

    render(<AggregatedRequestList view="history" filters={NO_FILTERS} />);
    expect(
      await screen.findByText("Noch keine entschiedenen Anfragen."),
    ).toBeInTheDocument();
  });

  it("erklärt einen leeren Treffer bei aktiver Suche", async () => {
    render(
      <AggregatedRequestList
        view="open"
        filters={{ search: "Zebra", types: [], statuses: [] }}
      />,
    );

    expect(
      await screen.findByText(
        "Für die aktuelle Suche und Filter gibt es keine Treffer.",
      ),
    ).toBeInTheDocument();
  });

  it("zeigt eine Fehlermeldung, wenn das Laden scheitert", async () => {
    mockListOpen.mockRejectedValue(new Error("kaputt"));

    render(<AggregatedRequestList view="open" filters={NO_FILTERS} />);

    expect(
      await screen.findByText("Anfragen konnten nicht geladen werden."),
    ).toBeInTheDocument();
  });
});
