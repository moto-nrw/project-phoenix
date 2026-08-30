/**
 * Die Zwei-Spalten-Arbeitsliste (#2267): Auswahl, Rückweg auf schmalen
 * Geräten, Leer-Zustand ohne Entscheidungsrecht, Sammelfreigabe über alle
 * Arten und der Umgang mit einer zwischenzeitlich geänderten Anfrage.
 */

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  AggregatedRequestList,
  type AggregatedRequestFilters,
} from "~/components/students/aggregated-request-list";
import {
  bulkApproveParentRequests,
  ChangeRequestStaleError,
  listAggregatedOpenRequests,
  listAggregatedRequestHistory,
  listEnrollmentChangeRequests,
} from "~/lib/change-request-list-api";
import { fetchCareWithdrawals } from "~/lib/care-exit-api";

vi.mock("~/lib/change-request-list-api", async () => {
  const actual = await vi.importActual<
    typeof import("~/lib/change-request-list-api")
  >("~/lib/change-request-list-api");
  return {
    ...actual,
    bulkApproveParentRequests: vi.fn(),
    getFamilyProtection: vi.fn(),
    listAggregatedOpenRequests: vi.fn(),
    listAggregatedRequestHistory: vi.fn(),
    listEnrollmentChangeRequests: vi.fn(),
    setFamilyProtection: vi.fn(),
  };
});

vi.mock("~/lib/care-exit-api", async () => {
  const actual = await vi.importActual<typeof import("~/lib/care-exit-api")>(
    "~/lib/care-exit-api",
  );
  return { ...actual, fetchCareWithdrawals: vi.fn() };
});

vi.mock("~/components/students/care-request-review-item", () => ({
  CareRequestReviewItem: ({ row }: { row: { id: string } }) => (
    <div>care-item-{row.id}</div>
  ),
}));
vi.mock("~/components/students/excused-request-review-item", () => ({
  ExcusedRequestReviewItem: ({ row }: { row: { id: string } }) => (
    <div>excused-item-{row.id}</div>
  ),
}));

const mockListOpen = vi.mocked(listAggregatedOpenRequests);
const mockListHistory = vi.mocked(listAggregatedRequestHistory);
const mockListEnrollment = vi.mocked(listEnrollmentChangeRequests);
const mockListWithdrawals = vi.mocked(fetchCareWithdrawals);
const mockBulkApprove = vi.mocked(bulkApproveParentRequests);

const NO_FILTERS: AggregatedRequestFilters = {
  search: "",
  types: [],
  statuses: [],
};

function item(
  id: string,
  studentID: string,
  studentName: string,
  overrides: Record<string, unknown> = {},
) {
  return {
    request_type: "excused",
    occurred_at: "2026-08-29T09:00:00Z",
    student_id: studentID,
    student_name: studentName,
    expected_version: `v${id}`,
    urgent_today: false,
    bulk_eligible: true,
    family_protected: false,
    data: { id, dates: ["2026-08-29"], absence_status: "sick" },
    ...overrides,
  } as never;
}

function setViewportWidth(width: number) {
  Object.defineProperty(window, "innerWidth", {
    configurable: true,
    writable: true,
    value: width,
  });
  window.dispatchEvent(new Event("resize"));
}

describe("Anfragen: Liste und Detail", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockListOpen.mockResolvedValue({ items: [] });
    mockListHistory.mockResolvedValue({ items: [] });
    mockListEnrollment.mockResolvedValue({ items: [] });
    mockListWithdrawals.mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      pageSize: 25,
    });
    mockBulkApprove.mockResolvedValue(2);
  });

  afterEach(() => setViewportWidth(1024));

  it("ersetzt auf schmalen Geräten die Liste und bringt den Fokus zurück", async () => {
    setViewportWidth(390);
    mockListOpen.mockResolvedValue({
      items: [
        item("1", "10", "Mia Muster"),
        item("2", "20", "Noah Beispiel"),
      ] as never,
    });

    render(<AggregatedRequestList view="open" filters={NO_FILTERS} />);

    const row = await screen.findByRole("button", { name: /^Mia Muster/ });
    fireEvent.click(row);

    // Die Liste ist ersetzt: das zweite Kind steht nicht mehr da.
    expect(
      screen.queryByRole("button", { name: /^Noah Beispiel/ }),
    ).not.toBeInTheDocument();
    expect(screen.getByText("excused-item-1")).toBeVisible();

    fireEvent.click(screen.getByRole("button", { name: "Zur Liste" }));
    expect(
      await screen.findByRole("button", { name: /^Noah Beispiel/ }),
    ).toBeVisible();
  });

  it("zeigt in der breiten Ansicht beide Bereiche und markiert die Auswahl", async () => {
    mockListOpen.mockResolvedValue({
      items: [
        item("1", "10", "Mia Muster"),
        item("2", "20", "Noah Beispiel"),
      ] as never,
    });

    render(<AggregatedRequestList view="open" filters={NO_FILTERS} />);

    const rows = await screen.findAllByRole("button", {
      name: /^(Mia Muster|Noah Beispiel)/,
    });
    expect(rows).toHaveLength(2);
    // Ohne eigene Auswahl steht rechts das erste Kind.
    expect(screen.getByText("excused-item-1")).toBeVisible();
    fireEvent.click(rows[1]!);
    expect(rows[1]).toHaveAttribute("aria-current", "true");
    expect(screen.getByText("excused-item-2")).toBeVisible();
  });

  it("erklärt eine leere Liste, wenn die Person nicht entscheiden darf", async () => {
    mockListOpen.mockResolvedValue({ items: [], review_access: "none" });

    render(<AggregatedRequestList view="open" filters={NO_FILTERS} />);

    expect(
      await screen.findByText(
        "Sie dürfen Elternanfragen zurzeit nicht entscheiden.",
      ),
    ).toBeVisible();
    expect(
      screen.getByText(
        "Die Leitung der OGS kann das erlauben. Der Schalter steht in den Einstellungen im Bereich Elternportal.",
      ),
    ).toBeVisible();
  });

  it("zeigt einer Gruppenleitung den gewohnten Leer-Zustand", async () => {
    mockListOpen.mockResolvedValue({
      items: [],
      review_access: "group_leader",
    });

    render(<AggregatedRequestList view="open" filters={NO_FILTERS} />);

    expect(await screen.findByText("Keine offenen Anfragen.")).toBeVisible();
  });

  it("nimmt jede Art in die Sammelfreigabe auf", async () => {
    mockListOpen.mockResolvedValue({
      items: [
        item("1", "10", "Mia Muster"),
        item("2", "10", "Mia Muster", {
          request_type: "care_schedule",
          data: { id: "2", diff: [] },
        }),
      ] as never,
    });

    render(<AggregatedRequestList view="open" filters={NO_FILTERS} />);

    const boxes = await screen.findAllByRole("checkbox", {
      name: "Gemeinsam freigeben: Mia Muster",
    });
    fireEvent.click(boxes[0]!);
    fireEvent.click(boxes[1]!);
    fireEvent.change(screen.getByLabelText(/Gemeinsame Begründung/), {
      target: { value: "Alles geprüft" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "2 Anfragen freigeben" }),
    );
    fireEvent.click(
      screen.getAllByRole("button", { name: "2 Anfragen freigeben" }).at(-1)!,
    );

    await waitFor(() =>
      expect(mockBulkApprove).toHaveBeenCalledWith(
        [
          { kind: "excused", id: "1", expected_version: "v1" },
          { kind: "care_schedule", id: "2", expected_version: "v2" },
        ],
        "Alles geprüft",
      ),
    );
  });

  it("lädt genau einmal neu, wenn eine Anfrage inzwischen geändert wurde", async () => {
    mockListOpen.mockResolvedValue({
      items: [
        item("1", "10", "Mia Muster"),
        item("2", "10", "Mia Muster"),
      ] as never,
    });
    mockBulkApprove.mockRejectedValue(new ChangeRequestStaleError());

    render(<AggregatedRequestList view="open" filters={NO_FILTERS} />);

    const boxes = await screen.findAllByRole("checkbox", {
      name: "Gemeinsam freigeben: Mia Muster",
    });
    fireEvent.click(boxes[0]!);
    fireEvent.click(boxes[1]!);
    fireEvent.change(screen.getByLabelText(/Gemeinsame Begründung/), {
      target: { value: "Alles geprüft" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "2 Anfragen freigeben" }),
    );
    fireEvent.click(
      screen.getAllByRole("button", { name: "2 Anfragen freigeben" }).at(-1)!,
    );

    expect(
      await screen.findByText(
        "Die Anfrage wurde inzwischen geändert. Die neue Fassung wird geladen.",
      ),
    ).toBeVisible();
    // Erstabruf plus genau ein Nachladen.
    await waitFor(() => expect(mockListOpen).toHaveBeenCalledTimes(2));
    expect(screen.getByText("excused-item-1")).toBeVisible();
  });
});
