vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: {
      user: { id: "account-1", tenantId: 1, scope: "operator" },
      expires: "2099-01-01",
    },
    status: "authenticated",
  }),
}));
import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SWRConfig } from "~/lib/swr/testing";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  mapBelegRetentionOverview,
  type BelegRetentionOverview,
} from "~/lib/operator/beleg-retention-api";

const { mockGetOverview, mockRecordReview } = vi.hoisted(() => ({
  mockGetOverview: vi.fn(),
  mockRecordReview: vi.fn(),
}));

vi.unmock("swr");

vi.mock("~/lib/operator/beleg-retention-api", async (importOriginal) => ({
  ...(await importOriginal<
    typeof import("~/lib/operator/beleg-retention-api")
  >()),
  belegRetentionService: {
    getOverview: mockGetOverview,
    recordInstruction: vi.fn(),
    recordSuspension: vi.fn(),
    liftSuspension: vi.fn(),
    recordClarificationReview: mockRecordReview,
  },
}));

vi.mock("~/components/ui/date-picker", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  ...(await import("~/test/mocks/date-picker")).isoDatePickerMock(),
}));

import { ToastProvider } from "~/contexts/ToastContext";
import { TraegerBelegRetentionSection } from "./traeger-beleg-retention-section";

// The test clock stands on 2026-09-09.
const overview: BelegRetentionOverview = mapBelegRetentionOverview({
  instructions: [
    {
      id: 3,
      beleg_type: "holiday_document",
      trigger_event: "issued",
      period: 8,
      basis: "booking_voucher",
      legal_basis: "§ 147 AO",
      instructed_by: "Kämmerei",
      instructed_on: "2026-09-01",
      instruction_reference: "AVV-Nachtrag",
      recorded_at: "2026-09-01T10:00:00Z",
    },
  ],
  suspensions: [],
  summary: [
    {
      beleg_type: "holiday_document",
      scheduled: 4,
      running: 0,
      clarification: 0,
      suspended: 0,
      next_end: "2035-01-01",
    },
  ],
  clarifications: [
    {
      beleg_type: "care_contract",
      reason: "no_instruction",
      count: 12,
      overdue: 12,
      listed_on: "2026-08-01",
      review_on: "2026-08-31",
      responsible: "Florian (Abstimmung mit dem Träger)",
    },
    {
      beleg_type: "sepa_mandate",
      reason: "no_collection",
      count: 1,
      overdue: 0,
      listed_on: "2026-09-08",
      review_on: "2026-10-08",
      responsible: "Florian (Abstimmung mit dem Träger)",
      last_review: {
        id: 5,
        note: "Bank prüft den letzten Einzug",
        escalated: true,
        escalation_reason: "Träger antwortet nicht",
        reviewed_on: "2026-09-08",
        next_review_on: "2026-10-08",
      },
    },
  ],
  deletions: [],
  evaluated_on: "2026-09-09",
});

function renderSection() {
  return render(
    <ToastProvider>
      <SWRConfig value={{ provider: () => new Map(), dedupingInterval: 0 }}>
        <TraegerBelegRetentionSection organizationId="1" />
      </SWRConfig>
    </ToastProvider>,
  );
}

describe("TraegerBelegRetentionSection", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows the clarification list with an overdue review as escalation", async () => {
    mockGetOverview.mockResolvedValue(overview);

    renderSection();

    const overdue = await screen.findByText(
      "31.08.2026 · überfällig, Eskalation",
    );
    const contractRow = overdue.closest("tr");
    expect(contractRow).not.toBeNull();
    expect(
      within(contractRow as HTMLElement).getByText("Betreuungsverträge"),
    ).toBeInTheDocument();
    expect(screen.getByText("08.10.2026")).toBeInTheDocument();
    expect(screen.getByText("Mandat ohne Einzug")).toBeInTheDocument();
    expect(screen.getByText("8 Jahre ab Ausstellung")).toBeInTheDocument();
    expect(screen.getByText("01.01.2035")).toBeInTheDocument();
    expect(mockGetOverview).toHaveBeenCalledWith("1");
    expect(
      screen.getByText("Buchungsbeleg (AO, HGB, UStG): § 147 AO"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("08.09.2026: Bank prüft den letzten Einzug"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("An Florian eskaliert: Träger antwortet nicht"),
    ).toBeInTheDocument();
  });

  it("records the review of a clarification entry with escalation and next review", async () => {
    const user = userEvent.setup();
    mockGetOverview.mockResolvedValue(overview);
    mockRecordReview.mockResolvedValue(undefined);

    renderSection();

    await user.click(
      await screen.findByRole("button", {
        name: "Aktionen für Betreuungsverträge",
      }),
    );
    await user.click(
      await screen.findByRole("menuitem", { name: /Prüfung erfassen/ }),
    );
    const dialog = await screen.findByRole("dialog");
    await user.type(
      within(dialog).getByLabelText(/Ergebnis der Prüfung/),
      "Träger prüft die Einordnung",
    );
    await user.click(within(dialog).getByText("An Florian eskalieren"));
    await user.type(
      within(dialog).getByLabelText(/Grund der Eskalation/),
      "Keine Antwort seit 30 Tagen",
    );
    const submit = within(dialog).getByRole("button", {
      name: "Prüfung erfassen",
    });
    expect(submit).toBeDisabled();
    fireEvent.change(within(dialog).getByLabelText(/Nächste Prüfung/), {
      target: { value: "2026-10-09" },
    });
    await user.click(submit);

    expect(mockRecordReview).toHaveBeenCalledWith("1", {
      beleg_type: "care_contract",
      reason: "no_instruction",
      note: "Träger prüft die Einordnung",
      escalated: true,
      escalation_reason: "Keine Antwort seit 30 Tagen",
      next_review_on: "2026-10-09",
    });
  });
});

describe("mapBelegRetentionOverview", () => {
  it("maps IDs to strings and keeps a missing day as null", () => {
    const mapped = mapBelegRetentionOverview({
      instructions: [{ id: 42, beleg_type: "care_notice" }],
      suspensions: [
        { id: 4, beleg_type: "sepa_debit", review_on: "2027-01-01" },
      ],
      summary: [{ beleg_type: "care_notice", scheduled: 0 }],
    });
    expect(mapped.instructions[0]?.id).toBe(String(42));
    expect(mapped.suspensions[0]?.belegId).toBeNull();
    expect(mapped.summary[0]?.nextEnd).toBeNull();
    expect(mapped.evaluatedOn).toBeNull();
    expect(mapped.clarifications).toEqual([]);
    expect(mapped.instructions[0]?.basis).toBe("");
  });
});
