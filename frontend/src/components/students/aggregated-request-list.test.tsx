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
  bulkApproveParentRequests,
  getFamilyProtection,
  listAggregatedOpenRequests,
  listAggregatedRequestHistory,
  listEnrollmentChangeRequests,
  setFamilyProtection,
} from "~/lib/change-request-list-api";
import { fetchCareWithdrawals } from "~/lib/care-exit-api";

vi.mock("~/lib/change-request-list-api", () => ({
  bulkApproveParentRequests: vi.fn(),
  getFamilyProtection: vi.fn(),
  listAggregatedOpenRequests: vi.fn(),
  listAggregatedRequestHistory: vi.fn(),
  listEnrollmentChangeRequests: vi.fn(),
  setFamilyProtection: vi.fn(),
}));

vi.mock("~/lib/care-exit-api", () => ({
  fetchCareWithdrawals: vi.fn(),
}));

vi.mock("~/components/students/care-exit-modal", () => ({
  CareExitModal: ({
    isOpen,
    onFinished,
  }: {
    isOpen: boolean;
    onFinished: () => void;
  }) =>
    isOpen ? (
      <button type="button" onClick={onFinished}>
        withdrawal-finished
      </button>
    ) : null,
}));

vi.mock("~/components/students/student-deletion-modal", () => ({
  StudentDeletionModal: () => null,
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
const mockBulkApprove = vi.mocked(bulkApproveParentRequests);
const mockGetFamilyProtection = vi.mocked(getFamilyProtection);
const mockListHistory = vi.mocked(listAggregatedRequestHistory);
const mockListEnrollment = vi.mocked(listEnrollmentChangeRequests);
const mockListWithdrawals = vi.mocked(fetchCareWithdrawals);
const mockSetFamilyProtection = vi.mocked(setFamilyProtection);

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
    mockListWithdrawals.mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      pageSize: 25,
    });
    mockBulkApprove.mockResolvedValue(2);
    mockGetFamilyProtection.mockImplementation(async (studentId) => ({
      student_id: studentId,
      enabled: false,
    }));
    mockSetFamilyProtection.mockResolvedValue();
  });

  it("bündelt offene Anfragen pro Kind und zeigt heute betroffene Fälle zuerst", async () => {
    mockListOpen.mockResolvedValue({
      items: [
        {
          request_type: "master_data",
          occurred_at: "2026-08-29T10:00:00Z",
          student_id: "20",
          student_name: "Später Kind",
          expected_version: "v1",
          urgent_today: false,
          bulk_eligible: true,
          data: { id: "1" },
        },
        {
          request_type: "excused",
          occurred_at: "2026-08-29T09:00:00Z",
          student_id: "10",
          student_name: "Heute Kind",
          group_name: "Füchse",
          expected_version: "v2",
          urgent_today: true,
          bulk_eligible: true,
          data: { id: "2", dates: ["2026-08-29"] },
        },
        {
          request_type: "care_schedule",
          occurred_at: "2026-08-29T08:00:00Z",
          student_id: "10",
          student_name: "Heute Kind",
          expected_version: "v3",
          urgent_today: false,
          bulk_eligible: false,
          bulk_ineligible_reason:
            "Betreuungszeiten müssen einzeln geprüft werden.",
          data: { id: "3" },
        },
      ] as never,
    });

    render(<AggregatedRequestList view="open" filters={NO_FILTERS} />);

    const urgentHeading = await screen.findByRole("heading", {
      name: "Heute wichtig",
    });
    const laterHeading = screen.getByRole("heading", {
      name: "Weitere Anfragen",
    });
    expect(
      urgentHeading.compareDocumentPosition(laterHeading) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(screen.getAllByText("Heute Kind")).toHaveLength(1);
    expect(screen.getByText("2 offene Anfragen")).toBeVisible();
    expect(screen.getByText("Füchse")).toBeVisible();
    expect(screen.getByText("Betrifft: 29.08.2026")).toBeVisible();
    expect(screen.queryByText(/Keine Widersprüche/)).not.toBeInTheDocument();
    expect(screen.getByText("Anfrage 1 von 2")).toBeVisible();
    expect(screen.getByText("Anfrage 2 von 2")).toBeVisible();
    expect(screen.getByText("Nur einzeln freigeben")).toBeVisible();
    expect(screen.getByText("Später Kind")).toBeVisible();
  });

  it("kennzeichnet widersprüchliche Wünsche im Fall", async () => {
    mockListOpen.mockResolvedValue({
      items: ["alleine", "bus"].map((mode, index) => ({
        request_type: "master_data",
        occurred_at: `2026-08-29T0${9 - index}:00:00Z`,
        student_id: "10",
        student_name: "Mia Muster",
        expected_version: `v${index}`,
        urgent_today: false,
        bulk_eligible: true,
        data: {
          id: `${index + 1}`,
          target: "student",
          field_key: "departure_mode_mon",
          new_value: mode,
        },
      })) as never,
    });

    render(<AggregatedRequestList view="open" filters={NO_FILTERS} />);

    expect(
      await screen.findByText("Diese Anfragen widersprechen sich"),
    ).toBeVisible();
  });

  it("ordnet eine Mehrkind-Anmeldung jedem betroffenen Kind zu", async () => {
    mockListOpen.mockResolvedValue({
      items: [
        {
          request_type: "master_data",
          occurred_at: "2026-08-29T10:00:00Z",
          student_id: "10",
          student_name: "Mia Muster",
          expected_version: "v1",
          urgent_today: false,
          bulk_eligible: true,
          data: { id: "1" },
        },
      ] as never,
    });
    mockListEnrollment.mockResolvedValue({
      items: [
        {
          request_type: "enrollment",
          occurred_at: "2026-08-29T09:00:00Z",
          data: {
            id: "9",
            child_ids: ["10", "20"],
            child_names: ["Mia Muster", "Noah Muster"],
          },
        },
      ] as never,
    });

    render(
      <AggregatedRequestList
        view="open"
        filters={{ ...NO_FILTERS, includeEnrollment: true }}
      />,
    );

    expect(await screen.findByText("2 offene Anfragen")).toBeVisible();
    expect(screen.getByText("Noah Muster")).toBeVisible();
    expect(screen.getAllByText("enrollment-item-9")).toHaveLength(2);
  });

  it("bestätigt eine Sammelfreigabe mit gemeinsamer Begründung", async () => {
    mockListOpen.mockResolvedValue({
      items: [
        {
          request_type: "master_data",
          occurred_at: "2026-08-29T10:00:00Z",
          student_id: "10",
          student_name: "Mia Muster",
          expected_version: "v1",
          urgent_today: false,
          bulk_eligible: true,
          data: { id: "1" },
        },
        {
          request_type: "excused",
          occurred_at: "2026-08-29T09:00:00Z",
          student_id: "20",
          student_name: "Noah Beispiel",
          expected_version: "v2",
          urgent_today: false,
          bulk_eligible: true,
          data: { id: "2" },
        },
      ] as never,
    });

    render(<AggregatedRequestList view="open" filters={NO_FILTERS} />);

    expect(screen.queryByText("Gemeinsam freigeben")).not.toBeInTheDocument();

    const selections = await screen.findAllByRole("checkbox", {
      name: /für gemeinsame Freigabe auswählen/,
    });
    fireEvent.click(selections[0]!);
    expect(
      screen.getByRole("heading", { name: "Gemeinsam freigeben" }),
    ).toBeVisible();
    expect(
      screen.getByText("Wählen Sie noch eine passende Anfrage aus."),
    ).toBeVisible();
    fireEvent.click(selections[1]!);
    fireEvent.change(screen.getByLabelText("Gemeinsame Begründung"), {
      target: { value: "Alles geprüft" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "2 Anfragen freigeben" }),
    );

    expect(
      screen.getByText(/Alle 2 Anfragen werden gemeinsam freigegeben/),
    ).toBeVisible();
    const approvalButtons = screen.getAllByRole("button", {
      name: "2 Anfragen freigeben",
    });
    fireEvent.click(approvalButtons.at(-1)!);

    await waitFor(() =>
      expect(mockBulkApprove).toHaveBeenCalledWith(
        [
          { kind: "master_data", id: "1", expected_version: "v1" },
          { kind: "excused", id: "2", expected_version: "v2" },
        ],
        "Alles geprüft",
      ),
    );
    expect(
      await screen.findByText("2 Anfragen wurden freigegeben."),
    ).toBeVisible();
  });

  it("erklärt private Angaben und schaltet Familienschutz mit Begründung ein", async () => {
    mockListOpen.mockResolvedValue({
      items: [
        {
          request_type: "master_data",
          occurred_at: "2026-08-29T10:00:00Z",
          student_id: "10",
          student_name: "Mia Muster",
          expected_version: "v1",
          urgent_today: false,
          bulk_eligible: true,
          family_protected: false,
          data: { id: "1" },
        },
      ] as never,
    });

    render(
      <AggregatedRequestList
        view="open"
        filters={{ ...NO_FILTERS, canManageFamilyProtection: true }}
      />,
    );

    expect(
      await screen.findByRole("button", { name: "Angaben schützen" }),
    ).toBeVisible();
    fireEvent.click(screen.getByRole("button", { name: "Angaben schützen" }));
    expect(
      screen.getByText(
        "Andere Sorgeberechtigte sehen dann keine geteilten Anfragen und Begründungen mehr.",
      ),
    ).toBeVisible();
    fireEvent.change(screen.getByLabelText("Grund für die Änderung"), {
      target: { value: "Schutz nötig" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Schutz einschalten" }));

    await waitFor(() =>
      expect(mockSetFamilyProtection).toHaveBeenCalledWith(
        "10",
        true,
        "Schutz nötig",
      ),
    );
    expect(await screen.findByText("Familienschutz")).toBeVisible();
    expect(
      screen.getByRole("button", { name: "Schutz aufheben" }),
    ).toBeVisible();
  });

  it("loads complete withdrawals into the shared open task list", async () => {
    mockListWithdrawals.mockResolvedValue({
      items: [
        {
          id: "withdrawal-1",
          studentId: "10",
          firstName: "Mia",
          lastName: "Muster",
          schoolClass: "2a",
          firstBookinglessDay: "2026-09-01",
          urgency: "planned",
          state: "pending",
        },
      ],
      total: 1,
      page: 1,
      pageSize: 25,
    });

    render(
      <AggregatedRequestList
        view="open"
        filters={{ ...NO_FILTERS, includeCareWithdrawals: true }}
      />,
    );

    const task = await screen.findByRole("button", {
      name: /Anfrage für Mia Muster/,
    });
    expect(mockListWithdrawals).toHaveBeenCalledWith({
      search: "",
      studentId: undefined,
      page: 1,
      pageSize: 25,
    });
    fireEvent.click(task);
    fireEvent.click(
      await screen.findByRole("button", { name: "Betreuung beenden" }),
    );
    fireEvent.click(await screen.findByText("withdrawal-finished"));
    expect(screen.queryByText("Mia Muster")).toBeNull();
    expect(screen.getByText("Die Betreuung wurde beendet.")).toBeVisible();
  });

  it("loads every open withdrawal progressively for complete child cases", async () => {
    const firstPage = Array.from({ length: 25 }, (_, index) => ({
      id: `withdrawal-${index + 1}`,
      studentId: `${index + 1}`,
      firstName: "Mia",
      lastName: "Muster",
      schoolClass: "2a",
      firstBookinglessDay: "2026-09-01",
      urgency: "planned" as const,
      state: "pending" as const,
    }));
    mockListWithdrawals
      .mockResolvedValueOnce({
        items: firstPage,
        total: 26,
        page: 1,
        pageSize: 25,
      })
      .mockResolvedValueOnce({
        items: [{ ...firstPage[0]!, id: "withdrawal-26" }],
        total: 26,
        page: 2,
        pageSize: 25,
      });

    render(
      <AggregatedRequestList
        view="open"
        filters={{ ...NO_FILTERS, includeCareWithdrawals: true }}
      />,
    );

    await waitFor(() =>
      expect(
        screen.getAllByRole("button", { name: /Anfrage für Mia Muster/ }),
      ).toHaveLength(26),
    );
    expect(mockListWithdrawals).toHaveBeenNthCalledWith(2, {
      search: "",
      studentId: undefined,
      page: 2,
      pageSize: 25,
    });
  });

  it("ignores a late withdrawal page after the child filter changes", async () => {
    const oldItems = Array.from({ length: 25 }, (_, index) => ({
      id: `old-${index + 1}`,
      studentId: "10",
      firstName: "Altes",
      lastName: "Kind",
      schoolClass: "2a",
      firstBookinglessDay: "2026-09-01",
      urgency: "planned" as const,
      state: "pending" as const,
    }));
    let resolveOldPage!: (value: {
      items: typeof oldItems;
      total: number;
      page: number;
      pageSize: number;
    }) => void;
    const oldPage = new Promise<{
      items: typeof oldItems;
      total: number;
      page: number;
      pageSize: number;
    }>((resolve) => {
      resolveOldPage = resolve;
    });
    mockListWithdrawals
      .mockResolvedValueOnce({
        items: oldItems,
        total: 26,
        page: 1,
        pageSize: 25,
      })
      .mockReturnValueOnce(oldPage)
      .mockResolvedValueOnce({
        items: [
          {
            ...oldItems[0]!,
            id: "new-1",
            studentId: "20",
            firstName: "Neues",
          },
        ],
        total: 1,
        page: 1,
        pageSize: 25,
      });

    const { rerender } = render(
      <AggregatedRequestList
        view="open"
        filters={{
          ...NO_FILTERS,
          studentId: "10",
          includeCareWithdrawals: true,
        }}
      />,
    );
    await waitFor(() => expect(mockListWithdrawals).toHaveBeenCalledTimes(2));

    rerender(
      <AggregatedRequestList
        view="open"
        filters={{
          ...NO_FILTERS,
          studentId: "20",
          includeCareWithdrawals: true,
        }}
      />,
    );
    expect(
      await screen.findByRole("button", { name: /Anfrage für Neues Kind/ }),
    ).toBeVisible();

    await act(async () => {
      resolveOldPage({
        items: [{ ...oldItems[0]!, id: "late-old" }],
        total: 26,
        page: 2,
        pageSize: 25,
      });
      await oldPage;
    });

    expect(
      screen.queryByRole("button", { name: /Anfrage für Altes Kind/ }),
    ).toBeNull();
    expect(mockListWithdrawals).toHaveBeenLastCalledWith(
      expect.objectContaining({ studentId: "20", page: 1 }),
    );
  });

  it("warns before opening the detailed deletion preview", async () => {
    mockListWithdrawals.mockResolvedValue({
      items: [
        {
          id: "withdrawal-delete",
          studentId: "10",
          firstName: "Mia",
          lastName: "Muster",
          schoolClass: "2a",
          firstBookinglessDay: "2026-09-01",
          urgency: "planned",
          state: "pending",
        },
      ],
      total: 1,
      page: 1,
      pageSize: 100,
    });
    render(
      <AggregatedRequestList
        view="open"
        filters={{ ...NO_FILTERS, includeCareWithdrawals: true }}
      />,
    );

    fireEvent.click(
      await screen.findByRole("button", { name: /Anfrage für Mia Muster/ }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Kind sofort löschen" }),
    );

    expect(
      screen.getByRole("heading", { name: "Kind sofort löschen" }),
    ).toBeVisible();
    expect(
      screen.getByText(
        "Das Kind wird sofort gelöscht. Auch ein späterer letzter Betreuungstag wird nicht abgewartet.",
      ),
    ).toBeVisible();
    expect(
      screen.getByRole("button", { name: "Löschen prüfen" }),
    ).toBeVisible();
  });

  it("shows a deleted completion without child data in history", async () => {
    mockListWithdrawals.mockResolvedValue({
      items: [
        {
          id: "withdrawal-deleted",
          studentId: "",
          firstName: "",
          lastName: "",
          schoolClass: "",
          firstBookinglessDay: "2026-09-01",
          urgency: "overdue",
          state: "resolved",
          outcome: "deleted",
          resolvedAt: "2026-09-02T09:00:00Z",
        },
      ],
      total: 1,
      page: 1,
      pageSize: 100,
    });

    render(
      <AggregatedRequestList
        view="history"
        filters={{ ...NO_FILTERS, includeCareWithdrawals: true }}
      />,
    );

    expect(await screen.findByText("Gelöschtes Kind")).toBeVisible();
    expect(screen.getByText("Kind sofort gelöscht")).toBeVisible();
    expect(mockListWithdrawals).toHaveBeenCalledWith({
      search: "",
      studentId: undefined,
      page: 1,
      pageSize: 25,
      state: "resolved",
    });
  });

  it("preserves a care-ended outcome after the child data is redacted", async () => {
    mockListWithdrawals.mockResolvedValue({
      items: [
        {
          id: "withdrawal-care-ended-redacted",
          studentId: "",
          firstName: "",
          lastName: "",
          schoolClass: "",
          firstBookinglessDay: "2026-09-01",
          urgency: "overdue",
          state: "resolved",
          outcome: "care_ended",
          resolvedAt: "2026-09-02T09:00:00Z",
        },
      ],
      total: 1,
      page: 1,
      pageSize: 100,
    });

    render(
      <AggregatedRequestList
        view="history"
        filters={{ ...NO_FILTERS, includeCareWithdrawals: true }}
      />,
    );

    expect(await screen.findByText("Gelöschtes Kind")).toBeVisible();
    expect(screen.getByText("Betreuung beendet")).toBeVisible();
    expect(screen.queryByText("Kind sofort gelöscht")).not.toBeInTheDocument();
  });

  it("does not show withdrawals when another request type is selected", async () => {
    mockListWithdrawals.mockResolvedValue({
      items: [
        {
          id: "withdrawal-filtered",
          studentId: "10",
          firstName: "Mia",
          lastName: "Muster",
          schoolClass: "2a",
          firstBookinglessDay: "2026-09-01",
          urgency: "planned",
          state: "pending",
        },
      ],
      total: 1,
      page: 1,
      pageSize: 25,
    });

    render(
      <AggregatedRequestList
        view="open"
        filters={{
          ...NO_FILTERS,
          includeCareWithdrawals: true,
          types: ["excused"],
        }}
      />,
    );

    await waitFor(() => expect(mockListOpen).toHaveBeenCalled());
    expect(mockListWithdrawals).not.toHaveBeenCalled();
    expect(
      screen.queryByText("withdrawal-item-withdrawal-filtered"),
    ).toBeNull();
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

  it("ignoriert eine verspätete erste Ladung nach einem Refresh", async () => {
    let resolveFirst: (value: { items: ReturnType<typeof openItem>[] }) => void;
    const firstPage = new Promise<{ items: ReturnType<typeof openItem>[] }>(
      (resolve) => {
        resolveFirst = resolve;
      },
    );
    mockListOpen
      .mockReturnValueOnce(firstPage as never)
      .mockResolvedValueOnce({ items: [openItem("excused", "neu")] });

    render(<AggregatedRequestList view="open" filters={NO_FILTERS} />);
    await waitFor(() => expect(mockListOpen).toHaveBeenCalledTimes(1));

    await act(async () => {});
    act(() => {
      window.dispatchEvent(new Event("change-requests-refresh"));
    });
    await waitFor(() => expect(mockListOpen).toHaveBeenCalledTimes(2));

    await act(async () => {
      resolveFirst!({ items: [openItem("excused", "alt")] });
      await firstPage;
    });

    expect(screen.queryByText("excused-item-alt")).toBeNull();
    expect(await screen.findByText("excused-item-neu")).toBeInTheDocument();
  });

  it("ergänzt offene Seiten im Hintergrund zu vollständigen Fällen", async () => {
    mockListOpen
      .mockResolvedValueOnce({
        items: Array.from(
          { length: 25 },
          (_, index) =>
            ({
              request_type: "excused",
              occurred_at: `2026-08-29T09:${String(index).padStart(2, "0")}:00Z`,
              student_id: "10",
              student_name: "Mia Muster",
              expected_version: `v${index + 1}`,
              urgent_today: false,
              bulk_eligible: true,
              family_protected: false,
              data: { id: String(index + 1), dates: ["2026-09-01"] },
            }) as never,
        ),
        next_cursor: "cursor-1",
      })
      .mockResolvedValueOnce({
        items: [
          {
            request_type: "excused",
            occurred_at: "2026-08-28T09:00:00Z",
            student_id: "10",
            student_name: "Mia Muster",
            expected_version: "v26",
            urgent_today: false,
            bulk_eligible: true,
            family_protected: false,
            data: { id: "26", dates: ["2026-09-01"] },
          } as never,
        ],
      });

    render(<AggregatedRequestList view="open" filters={NO_FILTERS} />);

    await screen.findByText("excused-item-1");
    expect(await screen.findByText("excused-item-26")).toBeInTheDocument();
    expect(screen.getByText("26 offene Anfragen")).toBeVisible();
    expect(mockListOpen).toHaveBeenLastCalledWith(
      expect.objectContaining({ cursor: "cursor-1" }),
    );
    expect(
      screen.queryByRole("button", { name: "Weitere Einträge laden" }),
    ).not.toBeInTheDocument();
  });

  it("stops automatic paging after an error and retries on request", async () => {
    const firstPage = Array.from(
      { length: 25 },
      (_, index) =>
        ({
          request_type: "excused",
          occurred_at: `2026-08-29T09:${String(index).padStart(2, "0")}:00Z`,
          student_id: "10",
          student_name: "Mia Muster",
          expected_version: `v${index + 1}`,
          urgent_today: false,
          bulk_eligible: true,
          data: { id: String(index + 1), dates: ["2026-09-01"] },
        }) as never,
    );
    mockListOpen
      .mockResolvedValueOnce({ items: firstPage, next_cursor: "cursor-1" })
      .mockRejectedValueOnce(new Error("page unavailable"))
      .mockResolvedValueOnce({
        items: [
          {
            request_type: "excused",
            occurred_at: "2026-08-28T09:00:00Z",
            student_id: "10",
            student_name: "Mia Muster",
            expected_version: "v26",
            urgent_today: false,
            bulk_eligible: true,
            data: { id: "26", dates: ["2026-09-01"] },
          } as never,
        ],
      });

    render(<AggregatedRequestList view="open" filters={NO_FILTERS} />);

    expect(
      await screen.findByText("Weitere Anfragen konnten nicht geladen werden."),
    ).toBeVisible();
    await act(async () => {});
    expect(mockListOpen).toHaveBeenCalledTimes(2);

    fireEvent.click(
      screen.getByRole("button", { name: "Weitere Einträge laden" }),
    );

    expect(await screen.findByText("excused-item-26")).toBeVisible();
    expect(mockListOpen).toHaveBeenCalledTimes(3);
    expect(
      screen.queryByText("Weitere Anfragen konnten nicht geladen werden."),
    ).toBeNull();
  });

  it("lädt weitere Historien-Einträge über den Cursor nach", async () => {
    // Eine volle Seite plus Cursor: erst dann bleibt etwas zum Nachladen übrig.
    // (Eine kurze Seite mit Cursor zieht die Liste selbst nach, damit niemand
    // blind auf den Knopf drücken muss.)
    mockListHistory.mockResolvedValueOnce({
      items: Array.from({ length: 25 }, (_, index) =>
        openItem("excused", String(index + 1)),
      ),
      next_cursor: "cursor-1",
    });
    mockListHistory.mockResolvedValueOnce({
      items: [openItem("excused", "26")],
    });

    render(<AggregatedRequestList view="history" filters={NO_FILTERS} />);
    await screen.findByText("history-item-excused-1");

    fireEvent.click(
      screen.getByRole("button", { name: "Weitere Einträge laden" }),
    );

    expect(
      await screen.findByText("history-item-excused-26"),
    ).toBeInTheDocument();
    expect(screen.getByText("history-item-excused-1")).toBeInTheDocument();
    expect(mockListHistory).toHaveBeenLastCalledWith(
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
