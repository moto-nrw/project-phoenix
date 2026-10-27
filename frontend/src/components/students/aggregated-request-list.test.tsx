import {
  act,
  fireEvent,
  render as renderComponent,
  renderHook,
  screen,
  waitFor,
  within,
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
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { mutate, SWRConfig, type Cache } from "~/lib/swr";

import {
  useMergedRequestFeed,
  useRequestSources,
  useWithdrawalFeed,
} from "./requests/use-request-feed";

import {
  fetchCareWithdrawals,
  fetchStudentCareWithdrawalHistory,
} from "~/lib/care-exit-api";
import { itemKey } from "./requests/case-model";

// Die Liste hält ihre Seiten im SWR-Cache (#4988): echtes SWR, ein eigener
// Cache je Render und eine angemeldete Sitzung für useSWRAuth.
vi.unmock("swr");

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    status: "authenticated",
    data: { user: { id: "1", token: "test-token" } },
  }),
}));

// Nur die Netzaufrufe ersetzen, den Rest des Moduls stehen lassen: die Liste
// braucht von dort auch Fehlerklassen und reine Hilfsfunktionen, und ein
// vollständig ersetztes Modul lässt sie beim nächsten Zusatz still abstürzen.
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

vi.mock("~/lib/care-exit-api", () => ({
  fetchCareWithdrawals: vi.fn(),
  fetchStudentCareWithdrawalHistory: vi.fn(),
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

// Der Löschdialog ist separat getestet; hier zählt nur, dass die Abmeldung
// ihn mit dem Sofort-Hinweis öffnet (#3110).
vi.mock("~/components/students/student-deletion-modal", () => ({
  StudentDeletionModal: ({
    displayName,
    skipsLastCareDay,
  }: {
    displayName: string;
    skipsLastCareDay?: boolean;
  }) => (
    <div role="dialog" aria-label={`${displayName} löschen`}>
      {skipsLastCareDay
        ? "Das Kind wird sofort gelöscht. Auch ein späterer letzter Betreuungstag wird nicht abgewartet."
        : null}
    </div>
  ),
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

function providersWith(cache: Cache) {
  return function Providers({
    children,
  }: Readonly<{ children: React.ReactNode }>) {
    return (
      <ToastProvider>
        <SWRConfig value={{ provider: () => cache, dedupingInterval: 0 }}>
          {children}
        </SWRConfig>
      </ToastProvider>
    );
  };
}

const render = (ui: React.ReactNode) =>
  renderComponent(ui, { wrapper: providersWith(new Map()) });

/** Der Standard-Cache von SWR, wie ihn die App nutzt. */
function DefaultCacheProviders({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <ToastProvider>
      <SWRConfig value={{ dedupingInterval: 0 }}>{children}</SWRConfig>
    </ToastProvider>
  );
}

const clearDefaultCache = () =>
  act(async () => {
    await mutate(() => true, undefined, { revalidate: false });
  });

/** Rendert mit einem Cache, den ein späterer Render wiederfindet. */
const renderWithCache = (ui: React.ReactNode, cache: Cache) =>
  renderComponent(ui, { wrapper: providersWith(cache) });

const mockListOpen = vi.mocked(listAggregatedOpenRequests);
const mockBulkApprove = vi.mocked(bulkApproveParentRequests);
const mockGetFamilyProtection = vi.mocked(getFamilyProtection);
const mockListHistory = vi.mocked(listAggregatedRequestHistory);
const mockListEnrollment = vi.mocked(listEnrollmentChangeRequests);
const mockListWithdrawals = vi.mocked(fetchCareWithdrawals);
const mockListStudentWithdrawals = vi.mocked(fetchStudentCareWithdrawalHistory);
const mockSetFamilyProtection = vi.mocked(setFamilyProtection);

const NO_FILTERS: AggregatedRequestFilters = {
  search: "",
  types: [],
  statuses: [],
};

/**
 * Eine offene Anfrage desselben Kindes. Seit #2267 zeigt der Detailbereich
 * die Anfragen EINES Kindes; für Reihenfolge-Prüfungen müssen die Zeilen
 * deshalb zu einem Fall gehören.
 */
function openItem(type: string, id: string) {
  return {
    request_type: type,
    student_id: "10",
    student_name: "Mia Muster",
    expected_version: `v${id}`,
    urgent_today: false,
    bulk_eligible: true,
    data: { id },
  } as never;
}

/** Wie openItem, aber mit dem Zeitpunkt, nach dem Quellen verschränkt werden. */
function stampedItem(
  type: string,
  id: string,
  occurredAt: string,
  urgentToday = false,
) {
  return {
    request_type: type,
    occurred_at: occurredAt,
    student_id: "10",
    student_name: "Mia Muster",
    expected_version: `v${id}`,
    urgent_today: urgentToday,
    bulk_eligible: true,
    data: { id },
  } as never;
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
          bulk_ineligible_reason: "single_only",
          bulk_ineligible_text:
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
    // Ein Fall je Kind: eine Listenzeile, egal wie viele Anfragen darin
    // stecken. Der Name steht zusätzlich über dem Detailbereich (#2267).
    expect(screen.getAllByRole("button", { name: /^Heute Kind/ })).toHaveLength(
      1,
    );
    expect(screen.getAllByText("2 offene Anfragen").length).toBeGreaterThan(0);
    expect(screen.getByText("Füchse")).toBeVisible();
    expect(screen.getByText("Betrifft: 29.08.2026")).toBeVisible();
    expect(screen.queryByText(/Keine Widersprüche/)).not.toBeInTheDocument();
    expect(screen.getByText("Anfrage 1 von 2")).toBeVisible();
    expect(screen.getByText("Anfrage 2 von 2")).toBeVisible();
    expect(
      screen.getByText(
        "Nur einzeln entscheiden: Diese Art wird immer einzeln geprüft.",
      ),
    ).toBeVisible();
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
        // Widerspruch und Gruppengröße kommen seit #2267 vom Backend.
        conflict_key: "md:student:departure_mode_mon",
        conflict_group_size: 2,
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

    // Die Anmeldung erzeugt für jedes betroffene Kind einen eigenen Fall:
    // Mia mit zwei Anfragen, Noah mit einer.
    expect(
      (await screen.findAllByText("2 offene Anfragen")).length,
    ).toBeGreaterThan(0);
    expect(screen.getByText("Noah Muster")).toBeVisible();
    expect(
      screen.getAllByRole("button", { name: /^Noah Muster/ }),
    ).toHaveLength(1);
    expect(screen.getAllByText("enrollment-item-9").length).toBeGreaterThan(0);
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
          // Seit #2267 werden Anfragen im Detailbereich EINES Kindes
          // ausgewählt; die Sammelfreigabe wirkt also innerhalb eines Falls.
          student_id: "10",
          student_name: "Mia Muster",
          expected_version: "v2",
          urgent_today: false,
          bulk_eligible: true,
          data: { id: "2" },
        },
      ] as never,
    });

    render(<AggregatedRequestList view="open" filters={NO_FILTERS} />);

    expect(
      screen.queryByRole("heading", { name: "Gemeinsam freigeben" }),
    ).not.toBeInTheDocument();

    const selections = await screen.findAllByRole("checkbox", {
      name: /^Gemeinsam freigeben:/,
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
    expect(await screen.findByRole("status")).toHaveTextContent(
      "2 Anfragen wurden freigegeben.",
    );
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
    // Zweimal: als Zustand in der Listenzeile und am Schalter im Detail.
    expect((await screen.findAllByText("Familienschutz")).length).toBe(2);
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
        // Eigenes Kind: sonst fiele die 26. Abmeldung in den Fall von Kind 1
        // und die Liste zeigte weiterhin 25 Zeilen.
        items: [{ ...firstPage[0]!, id: "withdrawal-26", studentId: "26" }],
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

    // 26 Kinder, 26 Fälle: seit #2267 zeigt die Liste einen Eintrag je Kind
    // und die Abmeldekarte selbst erst im Detailbereich.
    await waitFor(() =>
      expect(
        screen.getAllByRole("button", { name: /^Mia Muster/ }),
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

    // Kein vorgeschalteter Warn-Dialog mehr: der Löschdialog öffnet direkt
    // und trägt den Sofort-Hinweis selbst (#3110).
    expect(
      screen.getByRole("dialog", { name: "Mia Muster löschen" }),
    ).toBeVisible();
    expect(
      screen.getByText(
        "Das Kind wird sofort gelöscht. Auch ein späterer letzter Betreuungstag wird nicht abgewartet.",
      ),
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

  // Im Verlauf steht eine abgeschlossene Abmeldung im Raster der übrigen
  // Einträge: Datum in der Spalte „Entschieden“, nicht der Name des Kindes.
  it("puts a completed withdrawal into the history columns", async () => {
    mockListWithdrawals.mockResolvedValue({
      items: [
        {
          id: "withdrawal-dated",
          studentId: "10",
          firstName: "Mia",
          lastName: "Muster",
          schoolClass: "2a",
          firstBookinglessDay: "2026-09-01",
          urgency: "overdue",
          state: "resolved",
          outcome: "care_ended",
          resolvedAt: "2026-09-02T09:00:00Z",
        },
      ],
      total: 1,
      page: 1,
      pageSize: 25,
    });

    render(
      <AggregatedRequestList
        view="history"
        filters={{ ...NO_FILTERS, includeCareWithdrawals: true }}
      />,
    );

    const row = (await screen.findByText("Betreuung beendet")).closest(
      "[data-request-row]",
    );
    expect(row).not.toBeNull();
    const cells = within(row as HTMLElement);
    expect(cells.getByText("02.09.2026")).toBeInTheDocument();
    expect(cells.getByText("Mia Muster")).toBeVisible();
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

    // Anker: die gefilterte Liste ist geladen und zeigt ihren Leerzustand.
    expect(
      await screen.findByText(
        "Für die aktuelle Suche und Filter gibt es keine Treffer.",
      ),
    ).toBeInTheDocument();
    expect(mockListOpen).toHaveBeenCalled();
    expect(mockListWithdrawals).not.toHaveBeenCalled();
    expect(
      screen.queryByRole("button", { name: /Anfrage für Mia Muster/ }),
    ).not.toBeInTheDocument();
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

  it("entfernt eine entschiedene Zeile, zeigt einen schließbaren Toast und stößt das Badge an", async () => {
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
    expect(screen.getByRole("status")).toHaveTextContent("Änderung übernommen");
    expect(
      within(screen.getByRole("status")).getByRole("button", {
        name: "Schließen",
      }),
    ).toBeVisible();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
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
    // Die nachgeladene, leere Liste steht da (nicht nur ein Ladeskelett).
    expect(await screen.findByText("Keine offenen Anfragen.")).toBeVisible();
    expect(screen.queryByText("excused-item-4")).toBeNull();
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
              urgent_today: true,
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
            urgent_today: true,
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
          urgent_today: true,
          bulk_eligible: true,
          data: { id: String(index + 1), dates: ["2026-09-01"] },
        }) as never,
    );
    mockListOpen
      .mockResolvedValueOnce({ items: firstPage, next_cursor: "cursor-1" })
      .mockRejectedValueOnce(
        new ApiError("page unavailable", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValueOnce({
        items: [
          {
            request_type: "excused",
            occurred_at: "2026-08-28T09:00:00Z",
            student_id: "10",
            student_name: "Mia Muster",
            expected_version: "v26",
            urgent_today: true,
            bulk_eligible: true,
            data: { id: "26", dates: ["2026-09-01"] },
          } as never,
        ],
      });

    render(<AggregatedRequestList view="open" filters={NO_FILTERS} />);

    expect(
      await screen.findByText(
        "Die Liste der Anfragen ist gerade nicht erreichbar. Bitte versuchen Sie es erneut.",
      ),
    ).toBeVisible();
    await act(async () => {});
    expect(mockListOpen).toHaveBeenCalledTimes(2);

    fireEvent.click(
      screen.getByRole("button", { name: "Weitere Einträge laden" }),
    );

    expect(await screen.findByText("excused-item-26")).toBeVisible();
    expect(mockListOpen).toHaveBeenCalledTimes(3);
    expect(
      screen.queryByText(/Die Liste der Anfragen ist gerade nicht erreichbar/),
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
    expect(
      screen.queryByText("Neue Anfragen von Eltern erscheinen hier."),
    ).toBeNull();
    expect(
      screen.queryByText("Entschiedene Anfragen erscheinen hier."),
    ).toBeNull();
  });

  it("sagt ohne Filter, was im leeren Offen und in der leeren Historie erscheint", async () => {
    const { unmount } = render(
      <AggregatedRequestList view="open" filters={NO_FILTERS} />,
    );
    expect(
      await screen.findByText("Neue Anfragen von Eltern erscheinen hier."),
    ).toBeInTheDocument();
    unmount();

    render(<AggregatedRequestList view="history" filters={NO_FILTERS} />);
    expect(
      await screen.findByText("Entschiedene Anfragen erscheinen hier."),
    ).toBeInTheDocument();
  });

  it("legt den Leer-Zustand auf eine eigene Fläche statt auf den Seitengrund", async () => {
    render(<AggregatedRequestList view="open" filters={NO_FILTERS} />);

    const title = await screen.findByText("Keine offenen Anfragen.");
    expect(title.closest("section")).not.toBeNull();
  });

  it("zeigt einen Ladefehler vor Ort mit Vorgangskennung und Wiederholen", async () => {
    mockListOpen
      .mockRejectedValueOnce(
        new ApiError("kaputt", 500, {
          code: "general.server",
          instance: "req-list",
        }),
      )
      .mockResolvedValueOnce({ items: [] });

    render(<AggregatedRequestList view="open" filters={NO_FILTERS} />);

    expect(
      await screen.findByText(
        "Die Liste der Anfragen konnte nicht bearbeitet werden. Bitte versuchen Sie es später erneut.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Vorgangskennung kopieren" }),
    ).toHaveTextContent("req-list");
    // Ein Ladefehler ist kein Toast und kein Leerzustand.
    expect(
      screen.queryByRole("alert", { name: /^Fehler:/ }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText(/keine offenen Anfragen/i)).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    await waitFor(() => expect(mockListOpen).toHaveBeenCalledTimes(2));
    // Die erneut geladene, leere Liste steht da (nicht nur ein Ladeskelett).
    expect(await screen.findByText("Keine offenen Anfragen.")).toBeVisible();
    expect(screen.queryByText(/konnte nicht bearbeitet werden/)).toBeNull();
  });

  it("zeigt beim erneuten Öffnen sofort die geladene Liste und gleicht sie ohne Skelett ab", async () => {
    const cache: Cache = new Map();
    mockListOpen.mockResolvedValue({ items: [openItem("excused", "4")] });
    const first = renderWithCache(
      <AggregatedRequestList view="open" filters={NO_FILTERS} />,
      cache,
    );
    await screen.findByText("excused-item-4");
    first.unmount();

    // Der Abgleich bringt eine geänderte Antwort.
    let resolveRevalidation!: (value: { items: never[] }) => void;
    mockListOpen.mockReturnValueOnce(
      new Promise((resolve) => {
        resolveRevalidation = resolve;
      }) as never,
    );
    renderWithCache(
      <AggregatedRequestList view="open" filters={NO_FILTERS} />,
      cache,
    );

    // Anker: die Liste steht im selben Render, ohne auf den Abruf zu warten.
    expect(screen.getByText("excused-item-4")).toBeInTheDocument();
    expect(
      screen.queryByRole("status", { name: "Anfragen werden geladen" }),
    ).not.toBeInTheDocument();
    await waitFor(() => expect(mockListOpen).toHaveBeenCalledTimes(2));

    await act(async () => {
      resolveRevalidation({
        items: [openItem("excused", "4"), openItem("master_data", "5")],
      });
    });

    expect(await screen.findByText("master-item-5")).toBeInTheDocument();
    expect(
      screen.queryByRole("status", { name: "Anfragen werden geladen" }),
    ).not.toBeInTheDocument();
  });

  it("meldet einen gescheiterten Abgleich vor Ort und lässt die Liste stehen", async () => {
    const cache: Cache = new Map();
    mockListOpen.mockResolvedValue({ items: [openItem("excused", "4")] });
    const first = renderWithCache(
      <AggregatedRequestList view="open" filters={NO_FILTERS} />,
      cache,
    );
    await screen.findByText("excused-item-4");
    first.unmount();

    mockListOpen.mockRejectedValueOnce(
      new ApiError("page unavailable", 503, { code: "general.unavailable" }),
    );
    renderWithCache(
      <AggregatedRequestList view="open" filters={NO_FILTERS} />,
      cache,
    );

    expect(
      await screen.findByText(
        "Die Liste der Anfragen ist gerade nicht erreichbar. Bitte versuchen Sie es erneut.",
      ),
    ).toBeVisible();
    expect(screen.getByText("excused-item-4")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Wiederholen" })).toBeVisible();
  });

  it("zeigt eine entschiedene Anfrage nach dem Filterwechsel nicht wieder", async () => {
    // Ungefiltert laden, nach „Mia“ filtern, dort entscheiden, Filter wieder
    // löschen: die ungefilterte Liste im Cache darf die Anfrage nicht mehr
    // zeigen, auch nicht für den Moment bis zum Abgleich (#4988).
    // Wie in der App: der Standard-Cache von SWR, kein eigener Provider.
    await clearDefaultCache();
    mockListOpen.mockResolvedValue({
      items: [openItem("master_data", "1"), openItem("excused", "4")],
    });
    const { rerender } = renderComponent(
      <AggregatedRequestList view="open" filters={NO_FILTERS} />,
      { wrapper: DefaultCacheProviders },
    );
    await screen.findByText("excused-item-4");

    mockListOpen.mockResolvedValue({ items: [openItem("master_data", "1")] });
    const filtered = { ...NO_FILTERS, search: "Mia" };
    rerender(<AggregatedRequestList view="open" filters={filtered} />);
    await waitFor(() =>
      expect(screen.queryByText("excused-item-4")).not.toBeInTheDocument(),
    );

    // Der Server kennt die Anfrage danach nicht mehr als offen.
    mockListOpen.mockResolvedValue({ items: [openItem("excused", "4")] });
    fireEvent.click(screen.getByText("master-item-1"));

    rerender(<AggregatedRequestList view="open" filters={NO_FILTERS} />);
    expect(screen.queryByText("master-item-1")).not.toBeInTheDocument();
    expect(await screen.findByText("excused-item-4")).toBeInTheDocument();
    expect(screen.queryByText("master-item-1")).not.toBeInTheDocument();
    await clearDefaultCache();
  });

  it("verwirft beim Zurückkehren keinen laufenden Abgleich durch Nachladen", async () => {
    // Liste mit weiterer Seite laden, Abgleich starten, Seite verlassen und
    // während des Abgleichs zurückkehren und nachladen wollen: die frische
    // Antwort des Abgleichs muss stehen bleiben (#4988).
    const firstPage = Array.from({ length: 25 }, (_, index) =>
      stampedItem(
        "excused",
        `alt-${index + 1}`,
        `2026-08-29T09:${String(index).padStart(2, "0")}:00Z`,
      ),
    );
    mockListOpen.mockResolvedValueOnce({
      items: firstPage,
      next_cursor: "cursor-1",
    });
    const { rerender } = render(
      <AggregatedRequestList view="open" filters={NO_FILTERS} />,
    );
    await screen.findByText("excused-item-alt-1");

    let resolveRefresh!: (value: { items: never[] }) => void;
    mockListOpen.mockReturnValueOnce(
      new Promise((resolve) => {
        resolveRefresh = resolve;
      }) as never,
    );
    mockListOpen.mockResolvedValue({
      items: [
        stampedItem("excused", "alt-26", "2026-08-28T09:00:00Z"),
      ] as never[],
    });
    act(() => {
      window.dispatchEvent(new Event("change-requests-refresh"));
    });
    await waitFor(() => expect(mockListOpen).toHaveBeenCalledTimes(2));
    // Seite verlassen und zurückkehren; der Cache bleibt wie in der App.
    rerender(<div />);
    rerender(<AggregatedRequestList view="open" filters={NO_FILTERS} />);
    // Anker: die Liste steht aus dem Cache.
    expect(screen.getByText("excused-item-alt-1")).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", {
        name: /^(Weitere Einträge laden|Wird geladen…)$/,
      }),
    );
    await act(async () => {});

    await act(async () => {
      resolveRefresh({ items: [openItem("excused", "frisch")] });
    });

    expect(await screen.findByText("excused-item-frisch")).toBeInTheDocument();
    expect(screen.queryByText("excused-item-alt-1")).not.toBeInTheDocument();
  });

  it("meldet das fehlende Recht über den Code, nicht über den Backend-Satz", async () => {
    mockListOpen.mockRejectedValue(
      new ApiError("absence read required", 403, {
        code: "students.absence_read_required",
      }),
    );

    render(<AggregatedRequestList view="open" filters={NO_FILTERS} />);

    expect(
      await screen.findByText(
        /^Für die Liste der Anfragen brauchen Sie zusätzlich das Recht/,
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("absence read required")).toBeNull();
  });

  function useFeeds(
    filters: AggregatedRequestFilters,
    view: "open" | "history" = "history",
  ) {
    const sources = useRequestSources(view, filters);
    const feed = useMergedRequestFeed(sources, view, filters);
    const withdrawals = useWithdrawalFeed(view, filters);
    return { feed, withdrawals };
  }

  it.each(["merged", "withdrawal"] as const)(
    "retains the %s pagination error and retry when the other feed reloads",
    async (kind) => {
      const unavailable = new ApiError("page unavailable", 503, {
        code: "general.unavailable",
      });
      const requestPage = {
        items: Array.from({ length: 25 }, (_, i) =>
          openItem("excused", String(i)),
        ),
        next_cursor: "next",
      };
      mockListHistory.mockResolvedValue(requestPage);
      mockListWithdrawals.mockResolvedValue({
        items: [withdrawalItem("first")],
        total: 26,
        page: 1,
        pageSize: 25,
      });
      const { result } = renderHook(
        () => useFeeds({ ...NO_FILTERS, includeCareWithdrawals: true }),
        { wrapper: providersWith(new Map()) },
      );
      await waitFor(() => {
        expect(result.current.feed.items).toHaveLength(25);
        expect(result.current.withdrawals.items).toEqual([
          withdrawalItem("first"),
        ]);
      });
      if (kind === "merged") mockListHistory.mockRejectedValueOnce(unavailable);
      else mockListWithdrawals.mockRejectedValueOnce(unavailable);
      const failedFeed = () =>
        kind === "merged" ? result.current.feed : result.current.withdrawals;
      const displayedError = () =>
        result.current.feed.error ?? result.current.withdrawals.error;
      await act(async () => {
        await failedFeed().loadMore();
      });
      await waitFor(() => expect(displayedError()?.retry).toBeTruthy());
      const error = displayedError();
      await act(async () => {
        if (kind === "merged") await result.current.withdrawals.load();
        else await result.current.feed.reload();
      });
      expect(displayedError()).toBe(error);
      if (kind === "merged") {
        mockListHistory.mockResolvedValueOnce({
          items: [openItem("excused", "next")],
        });
      } else {
        mockListWithdrawals.mockResolvedValueOnce({
          items: [withdrawalItem("next")],
          total: 26,
          page: 2,
          pageSize: 25,
        });
      }
      act(() => error?.retry?.onClick());
      await waitFor(() => {
        expect(failedFeed().items).toHaveLength(kind === "merged" ? 26 : 2);
      });
      expect(displayedError()).toBeNull();
    },
  );

  it.each(["changed", "identical"] as const)(
    "rejects a withdrawal page after a same-filter reload with %s contents",
    async (contents) => {
      const item = {
        id: "first",
        studentId: "10",
        firstName: "Mia",
        lastName: "Muster",
        schoolClass: "2a",
        firstBookinglessDay: "2026-09-01",
        urgency: "planned" as const,
        state: "pending" as const,
      };
      const first = { items: [item], total: 26, page: 1, pageSize: 25 };
      let finishPage!: (page: typeof first) => void;
      const pending = new Promise<typeof first>((resolve) => {
        finishPage = resolve;
      });
      const replacement =
        contents === "changed" ? { ...item, id: "replacement" } : item;
      mockListWithdrawals
        .mockResolvedValueOnce(first)
        .mockReturnValueOnce(pending)
        .mockResolvedValueOnce({ ...first, items: [replacement] });
      const { result } = renderHook(
        () => useFeeds({ ...NO_FILTERS, includeCareWithdrawals: true }),
        {
          wrapper: providersWith(new Map()),
        },
      );
      await waitFor(() =>
        expect(result.current.withdrawals.items).toEqual([item]),
      );
      let loading!: Promise<void>;
      act(() => {
        loading = result.current.withdrawals.loadMore();
      });
      await act(async () => {
        await result.current.withdrawals.load();
      });
      expect(result.current.withdrawals.items).toEqual([replacement]);
      await act(async () => {
        finishPage({ ...first, items: [{ ...item, id: "obsolete" }], page: 2 });
        await loading;
      });
      expect(result.current.withdrawals.items).toEqual([replacement]);
    },
  );

  it.each(["reload", "filter"] as const)(
    "clears a pagination error after %s replaces the request feed",
    async (replacement) => {
      mockListHistory
        .mockResolvedValueOnce({
          items: Array.from({ length: 25 }, (_, i) =>
            openItem("excused", String(i)),
          ),
          next_cursor: "next",
        })
        .mockRejectedValueOnce(
          new ApiError("page unavailable", 503, {
            code: "general.unavailable",
          }),
        )
        .mockResolvedValueOnce({ items: [openItem("excused", "replacement")] });
      const { result, rerender } = renderHook(useFeeds, {
        initialProps: NO_FILTERS,
        wrapper: providersWith(new Map()),
      });
      await waitFor(() => expect(result.current.feed.items).toHaveLength(25));
      await act(async () => {
        await result.current.feed.loadMore();
      });
      await waitFor(() => expect(result.current.feed.error).toBeTruthy());
      if (replacement === "reload") {
        await act(async () => {
          await result.current.feed.reload();
        });
      } else {
        rerender({ ...NO_FILTERS, search: "Mia" });
      }
      await waitFor(() =>
        expect(result.current.feed.items).toEqual([
          openItem("excused", "replacement"),
        ]),
      );
      expect(result.current.feed.error).toBeNull();
    },
  );

  it.each(["reload", "filter"] as const)(
    "clears a withdrawal pagination error after %s",
    async (replacement) => {
      const item = {
        id: "first",
        studentId: "10",
        firstName: "Mia",
        lastName: "Muster",
        schoolClass: "2a",
        firstBookinglessDay: "2026-09-01",
        urgency: "planned" as const,
        state: "pending" as const,
      };
      mockListWithdrawals
        .mockResolvedValueOnce({
          items: [item],
          total: 26,
          page: 1,
          pageSize: 25,
        })
        .mockRejectedValueOnce(
          new ApiError("page unavailable", 503, {
            code: "general.unavailable",
          }),
        )
        .mockResolvedValueOnce({
          items: [{ ...item, id: "replacement" }],
          total: 1,
          page: 1,
          pageSize: 25,
        });
      const filters = { ...NO_FILTERS, includeCareWithdrawals: true };
      const { result, rerender } = renderHook(useFeeds, {
        initialProps: filters,
        wrapper: providersWith(new Map()),
      });
      await waitFor(() =>
        expect(result.current.withdrawals.items).toEqual([item]),
      );
      await act(async () => {
        await result.current.withdrawals.loadMore();
      });
      await waitFor(() =>
        expect(result.current.withdrawals.error).toBeTruthy(),
      );
      if (replacement === "reload") {
        await act(async () => {
          await result.current.withdrawals.load();
        });
      } else {
        rerender({ ...filters, search: "Mia" });
      }
      await waitFor(() =>
        expect(result.current.withdrawals.items[0]?.id).toBe("replacement"),
      );
      expect(result.current.withdrawals.error).toBeNull();
    },
  );

  function deferred<T>() {
    let resolve!: (value: T | PromiseLike<T>) => void;
    let reject!: (reason?: unknown) => void;
    const promise = new Promise<T>((onResolve, onReject) => {
      resolve = onResolve;
      reject = onReject;
    });
    return { promise, resolve, reject };
  }

  function withdrawalItem(id: string) {
    return {
      id,
      studentId: "10",
      firstName: "Mia",
      lastName: "Muster",
      schoolClass: "2a",
      firstBookinglessDay: "2026-09-01",
      urgency: "planned" as const,
      state: "pending" as const,
    };
  }

  // #5025: das Änderungsprotokoll eines Kindes liest die abgeschlossenen
  // Abmeldungen über die Kind-Route; die Liste der Schule verlangt
  // users:delete und bleibt unberührt.
  it("reads a child's completed withdrawals from the child route", async () => {
    const resolved = {
      ...withdrawalItem("done"),
      state: "resolved" as const,
      outcome: "care_ended" as const,
      resolvedAt: "2026-09-02T10:00:00+02:00",
    };
    mockListStudentWithdrawals.mockResolvedValue({
      items: [resolved],
      total: 1,
      page: 1,
      pageSize: 25,
    });
    const { result } = renderHook(
      () =>
        useWithdrawalFeed("history", {
          ...NO_FILTERS,
          studentId: "10",
          includeCareWithdrawals: true,
        }),
      { wrapper: providersWith(new Map()) },
    );

    await waitFor(() => expect(result.current.items).toEqual([resolved]));
    expect(mockListStudentWithdrawals).toHaveBeenCalledWith("10", {
      page: 1,
      pageSize: 25,
    });
    expect(mockListWithdrawals).not.toHaveBeenCalled();
  });

  it("keeps the school's withdrawal list for the open view of a child", async () => {
    mockListWithdrawals.mockResolvedValue({
      items: [withdrawalItem("open")],
      total: 1,
      page: 1,
      pageSize: 25,
    });
    const { result } = renderHook(
      () =>
        useWithdrawalFeed("open", {
          ...NO_FILTERS,
          studentId: "10",
          includeCareWithdrawals: true,
        }),
      { wrapper: providersWith(new Map()) },
    );

    await waitFor(() =>
      expect(result.current.items).toEqual([withdrawalItem("open")]),
    );
    expect(mockListWithdrawals).toHaveBeenCalledWith({
      search: "",
      studentId: "10",
      page: 1,
      pageSize: 25,
    });
    expect(mockListStudentWithdrawals).not.toHaveBeenCalled();
  });

  it.each([
    ["merged", true],
    ["merged", false],
    ["withdrawal", true],
    ["withdrawal", false],
  ] as const)(
    "recovers %s automatic pagination after a background refresh (more pages: %s)",
    async (kind, hasMore) => {
      await clearDefaultCache();
      mockListOpen.mockReset().mockResolvedValue({ items: [] });
      mockListWithdrawals.mockReset().mockResolvedValue({
        items: [],
        total: 0,
        page: 1,
        pageSize: 25,
      });
      const urgentItems = Array.from({ length: 25 }, (_, i) =>
        stampedItem("excused", String(i), "2026-09-01T12:00:00Z", true),
      );
      const pageError = new ApiError("page unavailable", 503, {
        code: "general.unavailable",
      });
      if (kind === "merged") {
        mockListOpen
          .mockResolvedValueOnce({ items: urgentItems, next_cursor: "next" })
          .mockRejectedValueOnce(pageError)
          .mockResolvedValueOnce({
            items: urgentItems,
            ...(hasMore ? { next_cursor: "next" } : {}),
          })
          .mockResolvedValue({ items: [openItem("excused", "new-page")] });
      } else {
        mockListWithdrawals
          .mockResolvedValueOnce({
            items: [withdrawalItem("first")],
            total: 26,
            page: 1,
            pageSize: 25,
          })
          .mockRejectedValueOnce(pageError)
          .mockResolvedValueOnce({
            items: [withdrawalItem("first")],
            total: hasMore ? 26 : 1,
            page: 1,
            pageSize: 25,
          })
          .mockResolvedValue({
            items: [withdrawalItem("new-page")],
            total: 26,
            page: 2,
            pageSize: 25,
          });
      }
      const { result } = renderHook(
        () => ({
          ...useFeeds(
            { ...NO_FILTERS, includeCareWithdrawals: kind === "withdrawal" },
            "open",
          ),
        }),
        { wrapper: DefaultCacheProviders },
      );
      await waitFor(() =>
        expect(
          (kind === "merged" ? result.current.feed : result.current.withdrawals)
            .error,
        ).toBeTruthy(),
      );
      const fetchPage = kind === "merged" ? mockListOpen : mockListWithdrawals;
      expect(fetchPage).toHaveBeenCalledTimes(2);

      // Derselbe SWR-Abgleich wie bei Fokus/Reconnect, ohne explizites reload.
      await act(async () => {
        await mutate(
          (key) =>
            typeof key === "string" &&
            key.includes(
              kind === "merged" ? "request-feed:" : "care-withdrawal-feed:",
            ),
        );
      });

      await waitFor(() => {
        const items =
          kind === "merged"
            ? result.current.feed.items
            : result.current.withdrawals.items;
        expect(items).toEqual(
          kind === "merged"
            ? [
                ...urgentItems,
                ...(hasMore ? [openItem("excused", "new-page")] : []),
              ]
            : [
                withdrawalItem("first"),
                ...(hasMore ? [withdrawalItem("new-page")] : []),
              ],
        );
        expect(
          (kind === "merged" ? result.current.feed : result.current.withdrawals)
            .error,
        ).toBeNull();
      });
      expect(fetchPage).toHaveBeenCalledTimes(hasMore ? 4 : 3);
    },
  );

  it.each(["merged", "withdrawal"] as const)(
    "loads a new filter while old %s pagination is pending and keeps the new request busy",
    async (kind) => {
      const oldPage = deferred<void>();
      const newPage = deferred<void>();
      mockListHistory.mockImplementation(async (params) => {
        if (params?.cursor) {
          await (params.search === "Mia" ? newPage : oldPage).promise;
          return { items: [openItem("excused", "next")] };
        }
        return {
          items: Array.from({ length: 25 }, (_, i) =>
            openItem("excused", `${params?.search ?? "all"}-${i}`),
          ),
          next_cursor: "next",
        };
      });
      mockListWithdrawals.mockImplementation(async (params) => {
        if (params?.page === 2) {
          await (params.search === "Mia" ? newPage : oldPage).promise;
        }
        return {
          items: [withdrawalItem(`${params?.search ?? "all"}-${params?.page}`)],
          total: 26,
          page: params?.page ?? 1,
          pageSize: 25,
        };
      });
      const filters = { ...NO_FILTERS, includeCareWithdrawals: true };
      const { result, rerender } = renderHook(useFeeds, {
        initialProps: filters,
        wrapper: providersWith(new Map()),
      });
      const current = () =>
        kind === "merged" ? result.current.feed : result.current.withdrawals;
      await waitFor(() => {
        expect(current().hasMore).toBe(true);
        expect(current().loadingMore).toBe(false);
      });
      let oldLoading!: Promise<void>;
      act(() => {
        oldLoading = current().loadMore();
      });
      expect(current().loadingMore).toBe(true);
      rerender({ ...filters, search: "Mia" });
      await waitFor(() => {
        expect(current().hasMore).toBe(true);
        expect(current().loadingMore).toBe(false);
      });
      let newLoading!: Promise<void>;
      act(() => {
        newLoading = current().loadMore();
      });
      expect(current().loadingMore).toBe(true);
      await act(async () => {
        oldPage.resolve();
        await oldLoading;
      });
      expect(current().loadingMore).toBe(true);
      await act(async () => {
        newPage.resolve();
        await newLoading;
      });
      expect(current().loadingMore).toBe(false);
      expect(current().items).toHaveLength(kind === "merged" ? 26 : 2);
    },
  );

  it.each(["merged", "withdrawal"] as const)(
    "preserves the remounted refresh when unmounted %s pagination completes",
    async (kind) => {
      const oldPage = deferred<void>();
      const refresh = deferred<void>();
      let refreshing = false;
      mockListHistory.mockImplementation(async (params) => {
        if (params?.cursor) {
          await oldPage.promise;
          return { items: [openItem("excused", "obsolete")] };
        }
        if (refreshing) {
          await refresh.promise;
          return { items: [openItem("excused", "replacement")] };
        }
        return {
          items: Array.from({ length: 25 }, (_, i) =>
            openItem("excused", String(i)),
          ),
          next_cursor: "next",
        };
      });
      mockListWithdrawals.mockImplementation(async (params) => {
        if (params?.page === 2) {
          await oldPage.promise;
          return {
            items: [withdrawalItem("obsolete")],
            total: 26,
            page: 2,
            pageSize: 25,
          };
        }
        if (refreshing) await refresh.promise;
        return {
          items: [withdrawalItem(refreshing ? "replacement" : "first")],
          total: refreshing ? 1 : 26,
          page: 1,
          pageSize: 25,
        };
      });
      const wrapper = providersWith(new Map());
      const hook = () =>
        useFeeds({ ...NO_FILTERS, includeCareWithdrawals: true });
      const first = renderHook(hook, { wrapper });
      const current = () =>
        kind === "merged"
          ? first.result.current.feed
          : first.result.current.withdrawals;
      await waitFor(() => expect(current().hasMore).toBe(true));
      let loading!: Promise<void>;
      act(() => {
        loading = current().loadMore();
      });
      first.unmount();
      refreshing = true;
      const second = renderHook(hook, { wrapper });
      await waitFor(() =>
        expect(
          kind === "merged" ? mockListHistory : mockListWithdrawals,
        ).toHaveBeenCalledTimes(3),
      );
      await act(async () => {
        oldPage.resolve();
        await loading;
      });
      await act(async () => {
        refresh.resolve();
      });
      await waitFor(() =>
        expect(
          kind === "merged"
            ? second.result.current.feed.items
            : second.result.current.withdrawals.items,
        ).toEqual(
          kind === "merged"
            ? [openItem("excused", "replacement")]
            : [withdrawalItem("replacement")],
        ),
      );
    },
  );

  it("removes decided requests from another filter's cached source buffers", async () => {
    await clearDefaultCache();
    const decided = stampedItem(
      "master_data",
      "decided",
      "2026-08-28T09:00:00Z",
    );
    const remaining = stampedItem(
      "excused",
      "remaining",
      "2026-08-28T08:00:00Z",
    );
    const firstItems = Array.from({ length: 25 }, (_, i) =>
      stampedItem("enrollment", String(i), "2026-08-29T09:00:00Z"),
    );
    mockListOpen.mockResolvedValue({ items: [decided, remaining] });
    mockListEnrollment.mockResolvedValue({ items: firstItems });
    const filters = { ...NO_FILTERS, includeEnrollment: true };
    const { result, rerender } = renderHook(
      (filters) => useFeeds(filters, "open"),
      {
        initialProps: filters,
        wrapper: DefaultCacheProviders,
      },
    );
    await waitFor(() => expect(result.current.feed.items).toEqual(firstItems));
    mockListEnrollment.mockResolvedValue({ items: [] });
    mockListOpen.mockResolvedValue({ items: [decided] });
    rerender({ ...filters, search: "Mia" });
    await waitFor(() => expect(result.current.feed.items).toEqual([decided]));
    await act(async () => {
      result.current.feed.setItems((items) =>
        items.filter((item) => itemKey(item) !== itemKey(decided)),
      );
    });
    mockListOpen.mockRejectedValue(new Error("refresh failed"));
    rerender(filters);
    await waitFor(() => expect(result.current.feed.error).toBeTruthy());
    await act(async () => {
      await result.current.feed.loadMore();
    });
    expect(result.current.feed.items).toEqual([...firstItems, remaining]);
    await clearDefaultCache();
  });

  it("rejects pending pages after a local decision changes the snapshot", async () => {
    const oldPage = deferred<void>();
    const decided = openItem("master_data", "decided");
    const firstItems = [
      decided,
      ...Array.from({ length: 24 }, (_, i) => openItem("excused", String(i))),
    ];
    mockListOpen.mockImplementation(async (params) => {
      if (params?.cursor) {
        await oldPage.promise;
        return { items: [decided] };
      }
      return { items: firstItems, next_cursor: "next" };
    });
    const { result } = renderHook(() => useFeeds(NO_FILTERS, "open"), {
      wrapper: providersWith(new Map()),
    });
    await waitFor(() => expect(result.current.feed.items).toEqual(firstItems));
    let loading!: Promise<void>;
    act(() => {
      loading = result.current.feed.loadMore();
    });
    await act(async () => {
      result.current.feed.setItems((items) =>
        items.filter((item) => itemKey(item) !== itemKey(decided)),
      );
    });
    await act(async () => {
      oldPage.resolve();
      await loading;
    });
    expect(result.current.feed.items).toEqual(firstItems.slice(1));
  });

  it.each(["merged", "withdrawal"] as const)(
    "does not discard an uncached filter's initial response when old %s pagination completes",
    async (kind) => {
      const oldPage = deferred<void>();
      const newFirstPage = deferred<void>();
      mockListHistory.mockImplementation(async (params) => {
        if (params?.cursor) {
          await oldPage.promise;
          return { items: [openItem("excused", "obsolete")] };
        }
        if (params?.search === "Mia") {
          await newFirstPage.promise;
          return { items: [openItem("excused", "replacement")] };
        }
        return {
          items: Array.from({ length: 25 }, (_, i) =>
            openItem("excused", String(i)),
          ),
          next_cursor: "next",
        };
      });
      mockListWithdrawals.mockImplementation(async (params) => {
        if (params?.page === 2) {
          await oldPage.promise;
          return {
            items: [withdrawalItem("obsolete")],
            total: 26,
            page: 2,
            pageSize: 25,
          };
        }
        if (params?.search === "Mia") await newFirstPage.promise;
        return {
          items: [
            withdrawalItem(params?.search === "Mia" ? "replacement" : "first"),
          ],
          total: params?.search === "Mia" ? 1 : 26,
          page: 1,
          pageSize: 25,
        };
      });
      const filters = { ...NO_FILTERS, includeCareWithdrawals: true };
      const { result, rerender } = renderHook(useFeeds, {
        initialProps: filters,
        wrapper: providersWith(new Map()),
      });
      const current = () =>
        kind === "merged" ? result.current.feed : result.current.withdrawals;
      await waitFor(() => expect(current().hasMore).toBe(true));
      let loading!: Promise<void>;
      act(() => {
        loading = current().loadMore();
      });
      rerender({ ...filters, search: "Mia" });
      expect(current().loading).toBe(true);
      await act(async () => {
        oldPage.resolve();
        await loading;
      });
      await act(async () => {
        newFirstPage.resolve();
      });
      await waitFor(() => expect(current().loading).toBe(false));
      expect(current().items).toEqual(
        kind === "merged"
          ? [openItem("excused", "replacement")]
          : [withdrawalItem("replacement")],
      );
    },
  );

  it.each([
    ["merged", false],
    ["merged", true],
    ["withdrawal", false],
    ["withdrawal", true],
  ] as const)(
    "preserves an in-flight refresh against %s pagination (refresh first: %s)",
    async (kind, refreshFirst) => {
      const oldPage = deferred<void>();
      const refresh = deferred<void>();
      const firstItems = Array.from({ length: 25 }, (_, i) =>
        openItem("excused", String(i)),
      );
      let refreshing = false;
      mockListHistory.mockImplementation(async (params) => {
        if (params?.cursor) {
          await oldPage.promise;
          return { items: [openItem("excused", "obsolete")] };
        }
        if (refreshing) {
          await refresh.promise;
          return { items: [openItem("excused", "replacement")] };
        }
        return { items: firstItems, next_cursor: "next" };
      });
      mockListWithdrawals.mockImplementation(async (params) => {
        if (params?.page === 2) {
          await oldPage.promise;
          return {
            items: [withdrawalItem("obsolete")],
            total: 26,
            page: 2,
            pageSize: 25,
          };
        }
        if (refreshing) await refresh.promise;
        return {
          items: [withdrawalItem(refreshing ? "replacement" : "first")],
          total: refreshing ? 1 : 26,
          page: 1,
          pageSize: 25,
        };
      });
      const { result } = renderHook(
        () => useFeeds({ ...NO_FILTERS, includeCareWithdrawals: true }),
        { wrapper: providersWith(new Map()) },
      );
      const current = () =>
        kind === "merged" ? result.current.feed : result.current.withdrawals;
      await waitFor(() => expect(current().hasMore).toBe(true));
      let loading!: Promise<void>;
      if (!refreshFirst) {
        act(() => {
          loading = current().loadMore();
        });
      }
      refreshing = true;
      let reloading!: Promise<void>;
      act(() => {
        reloading =
          kind === "merged"
            ? result.current.feed.reload()
            : result.current.withdrawals.load();
      });
      await waitFor(() =>
        expect(
          kind === "merged" ? mockListHistory : mockListWithdrawals,
        ).toHaveBeenCalledTimes(refreshFirst ? 2 : 3),
      );
      if (refreshFirst) {
        act(() => {
          loading = current().loadMore();
        });
      }
      await act(async () => {
        oldPage.resolve();
        await loading;
      });
      // Bis die Antwort des Abgleichs da ist, gilt der bisherige Stand samt
      // Folgeseite; die Seite berührt den Cache nicht (#4988).
      expect(current().items).toEqual(
        kind === "merged"
          ? [...firstItems, openItem("excused", "obsolete")]
          : [withdrawalItem("first"), withdrawalItem("obsolete")],
      );
      await act(async () => {
        refresh.resolve();
        await reloading;
      });
      await waitFor(() =>
        expect(current().items).toEqual(
          kind === "merged"
            ? [openItem("excused", "replacement")]
            : [withdrawalItem("replacement")],
        ),
      );
    },
  );

  it("rejects a merged page after an identical first-page reload", async () => {
    const first = {
      items: Array.from({ length: 25 }, (_, i) =>
        openItem("excused", String(i)),
      ),
      next_cursor: "next",
    };
    const oldPage = deferred<typeof first>();
    mockListHistory
      .mockResolvedValue(first)
      .mockResolvedValueOnce(first)
      .mockReturnValueOnce(oldPage.promise);
    const { result } = renderHook(() => useFeeds(NO_FILTERS), {
      wrapper: providersWith(new Map()),
    });
    await waitFor(() => expect(result.current.feed.items).toHaveLength(25));
    let loading!: Promise<void>;
    act(() => {
      loading = result.current.feed.loadMore();
    });
    await act(async () => {
      await result.current.feed.reload();
    });
    await act(async () => {
      oldPage.resolve({
        items: [openItem("excused", "obsolete")],
        next_cursor: "last",
      });
      await loading;
    });
    expect(result.current.feed.items).toEqual(first.items);
  });

  it.each(["reload", "filter"] as const)(
    "ignores superseded pagination failures after %s and still loads new pages automatically",
    async (replacement) => {
      const oldPage = deferred<void>();
      const newPage = deferred<void>();
      let replaced = false;
      const urgentItems = Array.from({ length: 25 }, (_, i) =>
        stampedItem("excused", String(i), "2026-09-01T12:00:00Z", true),
      );
      mockListOpen.mockImplementation(async (params) => {
        if (params?.cursor) {
          await (replaced ? newPage : oldPage).promise;
          return { items: [openItem("excused", "new-page")] };
        }
        return { items: urgentItems, next_cursor: "next" };
      });
      mockListWithdrawals.mockImplementation(async (params) => {
        if (params?.page === 2) await (replaced ? newPage : oldPage).promise;
        return {
          items: [withdrawalItem(params?.page === 2 ? "new-page" : "first")],
          total: params?.page === 2 ? 2 : 26,
          page: params?.page ?? 1,
          pageSize: 25,
        };
      });
      const filters = { ...NO_FILTERS, includeCareWithdrawals: true };
      const { result, rerender } = renderHook(
        (props) => useFeeds(props, "open"),
        {
          initialProps: filters,
          wrapper: providersWith(new Map()),
        },
      );
      await waitFor(() => {
        expect(result.current.feed.loadingMore).toBe(true);
        expect(result.current.withdrawals.loadingMore).toBe(true);
      });
      replaced = true;
      if (replacement === "reload") {
        await act(async () => {
          await Promise.all([
            result.current.feed.reload(),
            result.current.withdrawals.load(),
          ]);
        });
      } else {
        rerender({ ...filters, search: "Mia" });
      }
      await waitFor(() => {
        expect(result.current.feed.items).toEqual(urgentItems);
        expect(result.current.withdrawals.items).toEqual([
          withdrawalItem("first"),
        ]);
      });
      await act(async () => {
        oldPage.reject(
          new ApiError("old page unavailable", 503, {
            code: "general.unavailable",
          }),
        );
      });
      await act(async () => {
        newPage.resolve();
      });
      await waitFor(() => {
        expect(result.current.feed.items).toHaveLength(26);
        expect(result.current.withdrawals.items).toEqual([
          withdrawalItem("first"),
          withdrawalItem("new-page"),
        ]);
      });
      expect(result.current.feed.error).toBeNull();
      expect(result.current.withdrawals.error).toBeNull();
    },
  );
  // Eine Entscheidung in Filter B, während die erste Antwort von Filter A
  // noch aussteht: A darf danach nicht im Ladeskelett hängen bleiben (#4988).
  it("beendet die erste Ladung eines Filters nach einer Entscheidung in einem anderen", async () => {
    await clearDefaultCache();
    const pending = deferred<void>();
    const decided = openItem("master_data", "decided");
    const remaining = openItem("excused", "remaining");
    mockListOpen.mockImplementation(async (params) => {
      if (params?.search === "Mia") return { items: [decided] };
      await pending.promise;
      return { items: [remaining] };
    });
    const { result, rerender } = renderHook(
      (filters) => useFeeds(filters, "open"),
      { initialProps: NO_FILTERS, wrapper: DefaultCacheProviders },
    );
    await waitFor(() => expect(mockListOpen).toHaveBeenCalledTimes(1));
    expect(result.current.feed.loading).toBe(true);
    rerender({ ...NO_FILTERS, search: "Mia" });
    await waitFor(() => expect(result.current.feed.items).toEqual([decided]));

    await act(async () => {
      result.current.feed.setItems((items) =>
        items.filter((item) => itemKey(item) !== itemKey(decided)),
      );
    });
    rerender(NO_FILTERS);
    await act(async () => {
      pending.resolve();
    });

    await waitFor(() => expect(result.current.feed.loading).toBe(false));
    expect(result.current.feed.items).toEqual([remaining]);
    await clearDefaultCache();
  });

  it("beendet die erste Ladung eines Filters ohne Entscheidung (Gegenprobe)", async () => {
    await clearDefaultCache();
    const pending = deferred<void>();
    const decided = openItem("master_data", "decided");
    const remaining = openItem("excused", "remaining");
    mockListOpen.mockImplementation(async (params) => {
      if (params?.search === "Mia") return { items: [decided] };
      await pending.promise;
      return { items: [remaining] };
    });
    const { result, rerender } = renderHook(
      (filters) => useFeeds(filters, "open"),
      { initialProps: NO_FILTERS, wrapper: DefaultCacheProviders },
    );
    await waitFor(() => expect(mockListOpen).toHaveBeenCalledTimes(1));
    expect(result.current.feed.loading).toBe(true);
    rerender({ ...NO_FILTERS, search: "Mia" });
    await waitFor(() => expect(result.current.feed.items).toEqual([decided]));

    rerender(NO_FILTERS);
    await act(async () => {
      pending.resolve();
    });

    await waitFor(() => expect(result.current.feed.loading).toBe(false));
    expect(result.current.feed.items).toEqual([remaining]);
    await clearDefaultCache();
  });

  it("zeigt den ersten Filter nach einer Entscheidung im zweiten fertig an", async () => {
    await clearDefaultCache();
    const pending = deferred<void>();
    const decided = openItem("master_data", "decided");
    const remaining = openItem("excused", "remaining");
    mockListOpen.mockImplementation(async (params) => {
      if (params?.search === "Mia") {
        await pending.promise;
        return { items: [remaining] };
      }
      return { items: [decided, remaining] };
    });
    const firstFilter = { ...NO_FILTERS, search: "Mia" };
    const { rerender } = renderComponent(
      <AggregatedRequestList view="open" filters={firstFilter} />,
      { wrapper: DefaultCacheProviders },
    );
    await waitFor(() => expect(mockListOpen).toHaveBeenCalledTimes(1));
    rerender(<AggregatedRequestList view="open" filters={NO_FILTERS} />);
    fireEvent.click(await screen.findByText("master-item-decided"));

    rerender(<AggregatedRequestList view="open" filters={firstFilter} />);
    await act(async () => {
      pending.resolve();
    });

    expect(
      await screen.findByText("excused-item-remaining"),
    ).toBeInTheDocument();
    await clearDefaultCache();
  });

  // Filter A hat schon Daten, und sein Abgleich läuft, während in Filter B
  // entschieden wird: zurück in A bleibt die Anfrage weg, und der frische
  // Stand erscheint statt der alten Antwort des Abgleichs (#4988).
  it("zeigt nach der Rückkehr den frischen Stand, wenn der Abgleich während der Entscheidung lief", async () => {
    await clearDefaultCache();
    const staleRefresh = deferred<void>();
    const decided = openItem("master_data", "decided");
    const remaining = openItem("excused", "remaining");
    const fresh = openItem("excused", "fresh");
    let unfilteredCalls = 0;
    mockListOpen.mockImplementation(async (params) => {
      if (params?.search === "Mia") return { items: [decided] };
      unfilteredCalls += 1;
      if (unfilteredCalls === 1) return { items: [decided, remaining] };
      if (unfilteredCalls === 2) {
        // Der Abgleich, der vor der Entscheidung begann.
        await staleRefresh.promise;
        return { items: [decided, remaining] };
      }
      return { items: [remaining, fresh] };
    });
    const { rerender } = renderComponent(
      <AggregatedRequestList view="open" filters={NO_FILTERS} />,
      { wrapper: DefaultCacheProviders },
    );
    await screen.findByText("excused-item-remaining");
    act(() => {
      window.dispatchEvent(new Event("change-requests-refresh"));
    });
    await waitFor(() => expect(unfilteredCalls).toBe(2));

    const filtered = { ...NO_FILTERS, search: "Mia" };
    rerender(<AggregatedRequestList view="open" filters={filtered} />);
    await waitFor(() =>
      expect(
        screen.queryByText("excused-item-remaining"),
      ).not.toBeInTheDocument(),
    );
    fireEvent.click(screen.getByText("master-item-decided"));

    rerender(<AggregatedRequestList view="open" filters={NO_FILTERS} />);
    // Anker: A steht sofort aus dem Cache, ohne die entschiedene Anfrage.
    expect(screen.getByText("excused-item-remaining")).toBeInTheDocument();
    expect(screen.queryByText("master-item-decided")).not.toBeInTheDocument();

    await act(async () => {
      staleRefresh.resolve();
    });
    expect(await screen.findByText("excused-item-fresh")).toBeInTheDocument();
    expect(screen.queryByText("master-item-decided")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("status", { name: "Anfragen werden geladen" }),
    ).not.toBeInTheDocument();
    await clearDefaultCache();
  });
  // Runde 3 (#4988): die Entscheidung in Filter A wird erst fertig, nachdem
  // schon zu Filter B gewechselt wurde. Zurück in A darf die Anfrage nicht
  // wieder erscheinen, auch ohne neuen Abgleich.
  it("entfernt eine Anfrage, deren Entscheidung erst nach einem Filterwechsel fertig wird", async () => {
    const decided = openItem("master_data", "decided");
    const remaining = openItem("excused", "remaining");
    mockListOpen.mockImplementation(async (params) => ({
      items: params?.search ? [remaining] : [decided, remaining],
    }));
    const { result, rerender } = renderHook(
      (filters) => useFeeds(filters, "open"),
      { initialProps: NO_FILTERS, wrapper: providersWith(new Map()) },
    );
    await waitFor(() =>
      expect(result.current.feed.items).toEqual([decided, remaining]),
    );
    // Die Entscheidung behält die Funktion aus dem Render, in dem sie begann.
    const finishDecision = result.current.feed.setItems;
    rerender({ ...NO_FILTERS, search: "other" });
    await waitFor(() => expect(result.current.feed.items).toEqual([remaining]));

    await act(async () => {
      finishDecision((items) =>
        items.filter((item) => itemKey(item) !== itemKey(decided)),
      );
    });
    mockListOpen.mockImplementation(() => new Promise(() => undefined));
    rerender(NO_FILTERS);

    expect(result.current.feed.items.map(itemKey)).toEqual([
      itemKey(remaining),
    ]);
  });

  // Runde 3 (#4988): ein Abgleich läuft, ein zweiter ersetzt ihn, der erste
  // scheitert danach mit 503, und es wird nachgeladen. Die Antwort des
  // zweiten Abgleichs muss stehen bleiben.
  it.each([false, true])(
    "behält die Antwort des neueren Abgleichs nach einem späten 503 (Nachladen: %s)",
    async (paginate) => {
      const stale = deferred<void>();
      const fresh = deferred<void>();
      const decided = openItem("master_data", "decided");
      const first = Array.from({ length: 24 }, (_, i) =>
        openItem("excused", String(i)),
      );
      let firstPageCalls = 0;
      mockListOpen.mockImplementation(async (params) => {
        if (params?.cursor) {
          return { items: [openItem("excused", "page-two")] };
        }
        firstPageCalls += 1;
        if (firstPageCalls === 2) await stale.promise;
        if (firstPageCalls >= 3) {
          await fresh.promise;
          return { items: [openItem("excused", "fresh")] };
        }
        return { items: [decided, ...first], next_cursor: "next" };
      });
      const { result } = renderHook(() => useFeeds(NO_FILTERS, "open"), {
        wrapper: providersWith(new Map()),
      });
      await waitFor(() => expect(result.current.feed.items).toHaveLength(25));
      let staleReload!: Promise<void>;
      act(() => {
        staleReload = result.current.feed.reload();
      });
      await waitFor(() => expect(firstPageCalls).toBe(2));
      await act(async () => {
        result.current.feed.setItems((items) =>
          items.filter((item) => itemKey(item) !== itemKey(decided)),
        );
      });
      // Ein neuerer Abgleich, etwa durch ein Ereignis einer anderen Fläche.
      act(() => {
        void result.current.feed.reload();
      });
      await waitFor(() => expect(firstPageCalls).toBeGreaterThanOrEqual(3));
      await act(async () => {
        stale.reject(
          new ApiError("old request failed", 503, {
            code: "general.unavailable",
          }),
        );
        await staleReload;
      });
      if (paginate) {
        await act(async () => {
          await result.current.feed.loadMore();
        });
      }
      await act(async () => {
        fresh.resolve();
      });

      await waitFor(() =>
        expect(result.current.feed.items.map(itemKey)).toEqual([
          itemKey(openItem("excused", "fresh")),
        ]),
      );
    },
  );
  // Runde 4 (#4988): A lädt automatisch eine Folgeseite, B ebenso, zurück in
  // A läuft ein Abgleich. A darf dieselbe Seite nicht ein zweites Mal holen
  // und nicht zweimal anhängen, auch nicht, wenn der Abgleich danach mit 503
  // scheitert.
  it.each(["merged", "withdrawal"] as const)(
    "holt und hängt die Folgeseite nach A, B, A nur einmal an (%s)",
    async (kind) => {
      await clearDefaultCache();
      const pageA = deferred<void>();
      const pageB = deferred<void>();
      const refreshA = deferred<void>();
      let firstA = 0;
      let nextA = 0;
      let nextB = 0;
      const firstItems = Array.from({ length: 25 }, (_, i) =>
        stampedItem("excused", String(i), "2026-09-09T12:00:00Z", true),
      );
      const withdrawalItems = Array.from({ length: 25 }, (_, i) =>
        withdrawalItem(String(i)),
      );
      if (kind === "merged") {
        mockListOpen.mockImplementation(async (params) => {
          if (params?.cursor) {
            if (params.search) {
              nextB += 1;
              await pageB.promise;
            } else {
              nextA += 1;
              await pageA.promise;
            }
            return { items: [openItem("excused", "page-two")] };
          }
          if (!params?.search && ++firstA > 1) await refreshA.promise;
          return { items: firstItems, next_cursor: "next" };
        });
      } else {
        mockListWithdrawals.mockImplementation(async (params) => {
          if (params?.page === 2) {
            if (params.search) {
              nextB += 1;
              await pageB.promise;
            } else {
              nextA += 1;
              await pageA.promise;
            }
            return {
              items: [withdrawalItem("page-two")],
              total: 26,
              page: 2,
              pageSize: 25,
            };
          }
          if (!params?.search && ++firstA > 1) await refreshA.promise;
          return { items: withdrawalItems, total: 26, page: 1, pageSize: 25 };
        });
      }
      const filters = {
        ...NO_FILTERS,
        includeCareWithdrawals: kind === "withdrawal",
      };
      const { result, rerender } = renderHook(
        (props) => useFeeds(props, "open"),
        { initialProps: filters, wrapper: DefaultCacheProviders },
      );
      const current = () =>
        kind === "merged" ? result.current.feed : result.current.withdrawals;
      const expected =
        kind === "merged"
          ? [...firstItems, openItem("excused", "page-two")]
          : [...withdrawalItems, withdrawalItem("page-two")];
      await waitFor(() => expect(nextA).toBe(1));
      rerender({ ...filters, search: "Mia" });
      await waitFor(() => expect(nextB).toBe(1));
      rerender(filters);
      await waitFor(() => expect(firstA).toBe(2));
      await act(async () => {
        await Promise.resolve();
      });

      await act(async () => {
        pageA.resolve();
      });
      await waitFor(() => expect(current().items.length).toBeGreaterThan(25));
      expect(current().items).toEqual(expected);
      expect(nextA).toBe(1);

      await act(async () => {
        refreshA.reject(
          new ApiError("refresh unavailable", 503, {
            code: "general.unavailable",
          }),
        );
      });
      await waitFor(() => expect(current().error).toBeTruthy());
      expect(current().items).toEqual(expected);
      expect(nextA).toBe(1);
      await clearDefaultCache();
    },
  );

  // Die ID-Invariante (#4988): überschneiden sich zwei Seiten, steht jeder
  // Eintrag trotzdem nur einmal in der Liste.
  it.each(["merged", "withdrawal"] as const)(
    "zeigt jede ID nur einmal, auch wenn sich Seiten überschneiden (%s)",
    async (kind) => {
      const firstItems = Array.from({ length: 25 }, (_, i) =>
        stampedItem("excused", String(i), "2026-09-09T12:00:00Z", true),
      );
      const withdrawalItems = Array.from({ length: 25 }, (_, i) =>
        withdrawalItem(String(i)),
      );
      mockListOpen.mockImplementation(async (params) =>
        params?.cursor
          ? { items: [firstItems[24], openItem("excused", "new")] as never[] }
          : { items: firstItems, next_cursor: "next" },
      );
      mockListWithdrawals.mockImplementation(async (params) =>
        params?.page === 2
          ? {
              items: [withdrawalItems[24]!, withdrawalItem("new")],
              total: 27,
              page: 2,
              pageSize: 25,
            }
          : { items: withdrawalItems, total: 27, page: 1, pageSize: 25 },
      );
      const { result } = renderHook(
        () =>
          useFeeds(
            { ...NO_FILTERS, includeCareWithdrawals: kind === "withdrawal" },
            "open",
          ),
        { wrapper: providersWith(new Map()) },
      );
      const current = () =>
        kind === "merged" ? result.current.feed : result.current.withdrawals;

      await waitFor(() => expect(current().items).toHaveLength(26));
      const ids = current().items.map((item) =>
        kind === "merged"
          ? itemKey(item as never)
          : (item as { id: string }).id,
      );
      expect(new Set(ids).size).toBe(ids.length);
    },
  );
  // Runde 5 (#4988): ein Fehler im Hintergrundabgleich nach einer
  // Entscheidung ersetzt nie die übrigen geladenen Einträge.
  it.each(["merged", "withdrawal"] as const)(
    "lässt die übrigen Einträge stehen, wenn der Abgleich nach einer Entscheidung scheitert (%s)",
    async (kind) => {
      const decided = openItem("master_data", "decided");
      const remaining = openItem("excused", "remaining");
      const unavailable = new ApiError("unavailable", 503, {
        code: "general.unavailable",
      });
      let failing = false;
      mockListOpen.mockImplementation(async () => {
        if (failing && kind === "merged") throw unavailable;
        return { items: [decided, remaining] };
      });
      mockListWithdrawals.mockImplementation(async () => {
        if (failing && kind === "withdrawal") throw unavailable;
        return {
          items: [withdrawalItem("decided"), withdrawalItem("remaining")],
          total: 2,
          page: 1,
          pageSize: 25,
        };
      });
      const { result } = renderHook(
        () =>
          useFeeds(
            { ...NO_FILTERS, includeCareWithdrawals: kind === "withdrawal" },
            "open",
          ),
        { wrapper: providersWith(new Map()) },
      );
      const current = () =>
        kind === "merged" ? result.current.feed : result.current.withdrawals;
      await waitFor(() => expect(current().items).toHaveLength(2));

      act(() => {
        if (kind === "merged") {
          result.current.feed.setItems((items) =>
            items.filter((item) => itemKey(item) !== itemKey(decided)),
          );
        } else {
          result.current.withdrawals.setItems((items) =>
            items.filter((item) => item.id !== "decided"),
          );
        }
      });
      failing = true;
      await act(async () => {
        await (kind === "merged"
          ? result.current.feed.reload()
          : result.current.withdrawals.load());
      });

      await waitFor(() => expect(current().error).toBeTruthy());
      expect(current().loading).toBe(false);
      expect(current().items).toEqual(
        kind === "merged" ? [remaining] : [withdrawalItem("remaining")],
      );
    },
  );
});
