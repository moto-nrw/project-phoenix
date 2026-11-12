// #6533: Schließen während des Erfassens ist gesperrt (bisher nur der
// Hintergrund), eine späte Antwort schließt keinen anderen Dialog.
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: {
      user: { id: "account-1", tenantId: 1, scope: "operator" },
      expires: "2099-01-01",
    },
    status: "authenticated",
  }),
}));

import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import {
  mapBelegRetentionOverview,
  type BelegRetentionOverview,
} from "~/lib/operator/beleg-retention-api";
import { SWRConfig } from "~/lib/swr/testing";
import {
  deferred,
  expectDialogUnlocked,
  expectSaveLocksDialog,
} from "~/test/helpers/form-modal-pending";

const mocks = vi.hoisted(() => ({
  getOverview: vi.fn(),
  recordReview: vi.fn(),
  recordInstruction: vi.fn(),
  recordSuspension: vi.fn(),
}));

vi.unmock("swr");
vi.mock("~/lib/operator/beleg-retention-api", async (importOriginal) => ({
  ...(await importOriginal<
    typeof import("~/lib/operator/beleg-retention-api")
  >()),
  belegRetentionService: {
    getOverview: mocks.getOverview,
    recordInstruction: mocks.recordInstruction,
    recordSuspension: mocks.recordSuspension,
    liftSuspension: vi.fn(),
    recordClarificationReview: mocks.recordReview,
  },
}));
vi.mock("~/components/ui/date-picker", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  ...(await import("~/test/mocks/date-picker")).isoDatePickerMock(),
}));

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

const failure = () => new ApiError("kaputt", 500, { code: "general.server" });

describe("TraegerBelegRetentionSection pending lock (#6533)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.getOverview.mockResolvedValue(overview);
  });

  it("keeps the review dialog shut while recording and unlocks it after a failure", async () => {
    const user = userEvent.setup();
    const answer = deferred();
    mocks.recordReview.mockReturnValue(answer.promise);
    renderSection();

    await user.click(
      await screen.findByRole("button", { name: "Aktionen für SEPA-Mandate" }),
    );
    await user.click(
      await screen.findByRole("menuitem", { name: /Prüfung erfassen/ }),
    );
    const dialog = await screen.findByRole("dialog");
    await user.type(
      within(dialog).getByLabelText(/Ergebnis der Prüfung/),
      "Bank prüft",
    );
    fireEvent.change(within(dialog).getByLabelText(/Nächste Prüfung/), {
      target: { value: "2026-09-20" },
    });
    await user.click(
      within(dialog).getByRole("button", { name: "Prüfung erfassen" }),
    );

    expectSaveLocksDialog("Prüfung erfassen", []);

    await act(async () => {
      answer.reject(failure());
    });
    expect(await within(dialog).findByRole("alert")).toBeInTheDocument();
    expectDialogUnlocked("Prüfung erfassen");
    expect(within(dialog).getByLabelText(/Ergebnis der Prüfung/)).toHaveValue(
      "Bank prüft",
    );
  });

  it("keeps the instruction dialog shut while recording and unlocks it after a failure", async () => {
    const user = userEvent.setup();
    const answer = deferred();
    mocks.recordInstruction.mockReturnValue(answer.promise);
    renderSection();

    await user.click(
      await screen.findByRole("button", { name: "Weisung erfassen" }),
    );
    const dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByLabelText(/Fundstelle/), "§ 147 AO");
    await user.type(
      within(dialog).getByLabelText(/Weisungsberechtigte Person/),
      "Frau Beispiel",
    );
    fireEvent.change(within(dialog).getByLabelText(/Weisung vom/), {
      target: { value: "2026-09-01" },
    });
    await user.type(
      within(dialog).getByLabelText(/Nachweis der schriftlichen Weisung/),
      "Mail vom 01.09.2026",
    );
    await user.click(
      within(dialog).getByRole("button", { name: "Weisung erfassen" }),
    );

    expectSaveLocksDialog("Weisung erfassen", []);

    await act(async () => {
      answer.reject(failure());
    });
    expect(await within(dialog).findByRole("alert")).toBeInTheDocument();
    expectDialogUnlocked("Weisung erfassen");
  });

  it("keeps the suspension dialog shut while recording and unlocks it after a failure", async () => {
    const user = userEvent.setup();
    const answer = deferred();
    mocks.recordSuspension.mockReturnValue(answer.promise);
    renderSection();

    await user.click(
      await screen.findByRole("button", { name: "Löschung aussetzen" }),
    );
    const dialog = await screen.findByRole("dialog");
    await user.type(
      within(dialog).getByLabelText(/Umfang/),
      "Alle Belege 2025",
    );
    await user.type(within(dialog).getByLabelText(/Grund/), "Betriebsprüfung");
    await user.type(within(dialog).getByLabelText(/Zuständig/), "Florian");
    fireEvent.change(within(dialog).getByLabelText(/Nächste Prüfung/), {
      target: { value: "2026-10-01" },
    });
    await user.click(within(dialog).getByRole("button", { name: "Aussetzen" }));

    expectSaveLocksDialog("Löschung aussetzen", []);

    await act(async () => {
      answer.reject(failure());
    });
    expect(await within(dialog).findByRole("alert")).toBeInTheDocument();
    expectDialogUnlocked("Löschung aussetzen");
  });
});
