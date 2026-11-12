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
import { ApiError } from "~/lib/api-error";
import { STALE_DATA_SUFFIX } from "~/lib/hooks/use-refresh-after-write";
import {
  belegRetentionService,
  mapBelegRetentionOverview,
  type BelegRetentionOverview,
} from "~/lib/operator/beleg-retention-api";
import { catalogText } from "~/test/error-catalog-text";

const { mockGetOverview, mockRecordReview, mockRecordInstruction } = vi.hoisted(
  () => ({
    mockGetOverview: vi.fn(),
    mockRecordReview: vi.fn(),
    mockRecordInstruction: vi.fn(),
  }),
);

vi.unmock("swr");

vi.mock("~/lib/operator/beleg-retention-api", async (importOriginal) => ({
  ...(await importOriginal<
    typeof import("~/lib/operator/beleg-retention-api")
  >()),
  belegRetentionService: {
    getOverview: mockGetOverview,
    recordInstruction: mockRecordInstruction,
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

  it("requires the escalation for an overdue entry", async () => {
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
    const escalation = within(dialog).getByRole("checkbox");
    expect(escalation).toBeChecked();
    expect(escalation).toBeDisabled();
    expect(
      within(dialog).getByText(
        "Das Prüfdatum ist erreicht. Die offene Zuordnung geht an Florian.",
      ),
    ).toBeInTheDocument();
    await user.type(
      within(dialog).getByLabelText(/Ergebnis der Prüfung/),
      "Träger hat noch nicht geantwortet",
    );
    fireEvent.change(within(dialog).getByLabelText(/Nächste Prüfung/), {
      target: { value: "2026-10-09" },
    });
    const submit = within(dialog).getByRole("button", {
      name: "Prüfung erfassen",
    });
    expect(submit).toBeDisabled();
    await user.type(
      within(dialog).getByLabelText(/Grund der Eskalation/),
      "Keine Antwort seit 40 Tagen",
    );
    await user.click(submit);

    expect(mockRecordReview).toHaveBeenCalledWith(
      "1",
      expect.objectContaining({
        escalated: true,
        escalation_reason: "Keine Antwort seit 40 Tagen",
      }),
    );
  });

  it("requires the escalation once a review moves the review date later", async () => {
    const user = userEvent.setup();
    mockGetOverview.mockResolvedValue(overview);
    mockRecordReview.mockResolvedValue(undefined);

    renderSection();

    await user.click(
      await screen.findByRole("button", {
        name: "Aktionen für SEPA-Mandate",
      }),
    );
    await user.click(
      await screen.findByRole("menuitem", { name: /Prüfung erfassen/ }),
    );
    const dialog = await screen.findByRole("dialog");
    const escalation = within(dialog).getByRole("checkbox");
    expect(escalation).not.toBeChecked();
    expect(escalation).toBeEnabled();
    await user.type(
      within(dialog).getByLabelText(/Ergebnis der Prüfung/),
      "Bank meldet sich",
    );
    const next = within(dialog).getByLabelText(/Nächste Prüfung/);
    fireEvent.change(next, { target: { value: "2026-10-20" } });
    expect(escalation).toBeChecked();
    expect(escalation).toBeDisabled();
    expect(
      within(dialog).getByText(
        "Ein Prüfdatum nach dem 08.10.2026 geht nur mit Eskalation.",
      ),
    ).toBeInTheDocument();
    const submit = within(dialog).getByRole("button", {
      name: "Prüfung erfassen",
    });
    expect(submit).toBeDisabled();

    fireEvent.change(next, { target: { value: "2026-10-01" } });
    expect(escalation).not.toBeChecked();
    expect(submit).toBeEnabled();
    await user.click(submit);

    expect(mockRecordReview).toHaveBeenCalledWith("1", {
      beleg_type: "sepa_mandate",
      reason: "no_collection",
      note: "Bank meldet sich",
      escalated: false,
      escalation_reason: "",
      next_review_on: "2026-10-01",
    });
  });
  it("records a shorter period only with the Träger's written confirmation", async () => {
    const user = userEvent.setup();
    mockGetOverview.mockResolvedValue(overview);
    mockRecordInstruction.mockResolvedValue(undefined);

    renderSection();

    await user.click(
      await screen.findByRole("button", { name: "Weisung erfassen" }),
    );
    const dialog = await screen.findByRole("dialog");
    await user.click(
      within(dialog).getByRole("combobox", { name: /Belegart/ }),
    );
    await user.click(
      screen.getByRole("option", { name: "SEPA-Lastschriften" }),
    );
    await user.click(
      within(dialog).getByRole("combobox", { name: /Einordnung/ }),
    );
    await user.click(
      screen.getByRole("option", {
        name: "Vorschriften nicht anwendbar",
      }),
    );
    const period = within(dialog).getByLabelText(/Frist in Jahren/);
    await user.clear(period);
    await user.type(period, "1");
    await user.type(within(dialog).getByLabelText(/Fundstelle/), "АО");
    await user.type(
      within(dialog).getByLabelText(/Warum die Vorschriften nicht gelten/),
      "Träger ist nicht buchführungspflichtig",
    );
    await user.type(
      within(dialog).getByLabelText(/Weisungsberechtigte Person/),
      "Frau Beispiel, Kämmerei",
    );
    fireEvent.change(within(dialog).getByLabelText(/Weisung vom/), {
      target: { value: "2026-09-01" },
    });
    await user.type(
      within(dialog).getByLabelText(/Nachweis der schriftlichen Weisung/),
      "Mail der Kämmerei vom 01.09.2026",
    );
    const submit = within(dialog).getByRole("button", {
      name: "Weisung erfassen",
    });
    expect(submit).toBeDisabled();

    await user.click(
      within(dialog).getByLabelText(/Der Träger hat schriftlich bestätigt/),
    );
    expect(submit).toBeEnabled();
    await user.click(submit);

    expect(mockRecordInstruction).toHaveBeenCalledWith("1", {
      beleg_type: "sepa_debit",
      trigger_event: "issued",
      period: 1,
      basis: "not_applicable",
      legal_basis: "АО",
      basis_justification: "Träger ist nicht buchführungspflichtig",
      instructed_by: "Frau Beispiel, Kämmerei",
      instructed_on: "2026-09-01",
      instruction_reference: "Mail der Kämmerei vom 01.09.2026",
      statutes_not_applicable_confirmed: true,
    });
  });

  it("starts every new Weisung without the previous confirmation", async () => {
    const user = userEvent.setup();
    mockGetOverview.mockResolvedValue(overview);
    mockRecordInstruction.mockResolvedValue(undefined);

    renderSection();

    const open = await screen.findByRole("button", {
      name: "Weisung erfassen",
    });
    await user.click(open);
    let dialog = await screen.findByRole("dialog");
    await user.click(
      within(dialog).getByRole("combobox", { name: /Einordnung/ }),
    );
    await user.click(
      screen.getByRole("option", { name: "Vorschriften nicht anwendbar" }),
    );
    const period = within(dialog).getByLabelText(/Frist in Jahren/);
    await user.clear(period);
    await user.type(period, "1");
    await user.type(within(dialog).getByLabelText(/Fundstelle/), "Satzung");
    await user.type(
      within(dialog).getByLabelText(/Warum die Vorschriften nicht gelten/),
      "Träger ist nicht buchführungspflichtig",
    );
    await user.type(
      within(dialog).getByLabelText(/Weisungsberechtigte Person/),
      "Frau Beispiel, Kämmerei",
    );
    fireEvent.change(within(dialog).getByLabelText(/Weisung vom/), {
      target: { value: "2026-09-01" },
    });
    await user.type(
      within(dialog).getByLabelText(/Nachweis der schriftlichen Weisung/),
      "Mail der Kämmerei vom 01.09.2026",
    );
    await user.click(
      within(dialog).getByLabelText(/Der Träger hat schriftlich bestätigt/),
    );
    await user.click(
      within(dialog).getByRole("button", { name: "Weisung erfassen" }),
    );
    expect(mockRecordInstruction).toHaveBeenCalledTimes(1);

    await user.click(open);
    dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByLabelText(/Frist in Jahren/)).toHaveValue(8);
    expect(
      within(dialog).queryByLabelText(/Der Träger hat schriftlich bestätigt/),
    ).not.toBeInTheDocument();
    await user.click(
      within(dialog).getByRole("combobox", { name: /Einordnung/ }),
    );
    await user.click(
      screen.getByRole("option", { name: "Vorschriften nicht anwendbar" }),
    );
    expect(
      within(dialog).getByLabelText(/Der Träger hat schriftlich bestätigt/),
    ).not.toBeChecked();
  });

  it("drops the confirmation when the basis changes back", async () => {
    const user = userEvent.setup();
    mockGetOverview.mockResolvedValue(overview);

    renderSection();

    await user.click(
      await screen.findByRole("button", { name: "Weisung erfassen" }),
    );
    const dialog = await screen.findByRole("dialog");
    const kind = within(dialog).getByRole("combobox", { name: /Einordnung/ });
    await user.click(kind);
    await user.click(
      screen.getByRole("option", {
        name: "Vorschriften nicht anwendbar",
      }),
    );
    await user.click(
      within(dialog).getByLabelText(/Der Träger hat schriftlich bestätigt/),
    );
    await user.click(kind);
    await user.click(
      screen.getByRole("option", {
        name: "Buchungsbeleg (AO, HGB, UStG)",
      }),
    );
    expect(
      within(dialog).queryByLabelText(/Der Träger hat schriftlich bestätigt/),
    ).not.toBeInTheDocument();
    await user.click(kind);
    await user.click(
      screen.getByRole("option", {
        name: "Vorschriften nicht anwendbar",
      }),
    );
    expect(
      within(dialog).getByLabelText(/Der Träger hat schriftlich bestätigt/),
    ).not.toBeChecked();
  });

  it("shows the Träger's confirmation of a shorter period", async () => {
    mockGetOverview.mockResolvedValue(
      mapBelegRetentionOverview({
        instructions: [
          {
            id: 7,
            beleg_type: "sepa_debit",
            trigger_event: "issued",
            period: 1,
            basis: "not_applicable",
            legal_basis: "Satzung des Trägers",
            basis_justification: "Träger ist nicht buchführungspflichtig",
            instructed_by: "Kämmerei",
            instructed_on: "2026-09-01",
            instruction_reference: "Mail vom 01.09.2026",
            statutes_not_applicable_confirmed: true,
            confirmed_at: "2026-09-02T10:00:00Z",
            recorded_at: "2026-09-02T10:00:00Z",
          },
        ],
      }),
    );

    renderSection();

    expect(
      await screen.findByText(
        "Schriftlich bestätigt: AO, HGB und UStG gelten nicht (erfasst am 02.09.2026)",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("1 Jahr ab Ausstellung")).toBeInTheDocument();
  });
});

// #6636: a failed reload keeps the confirmed tables; the error replaces the
// section only while no data has loaded.
describe("TraegerBelegRetentionSection reload errors", () => {
  const unavailable = () =>
    new ApiError("down", 503, { code: "general.unavailable" });
  const withSuspension: BelegRetentionOverview = {
    ...overview,
    suspensions: [
      {
        id: "9",
        belegType: "holiday_document",
        belegId: null,
        reason: "Betriebsprüfung",
        scope: "Alle Ferienbelege",
        responsible: "Kämmerei",
        reviewOn: "2026-12-01",
      },
    ],
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("keeps the tables when the reload after lifting a suspension fails", async () => {
    mockGetOverview
      .mockResolvedValueOnce(withSuspension)
      .mockRejectedValue(unavailable());
    vi.mocked(belegRetentionService.liftSuspension).mockResolvedValue(
      undefined,
    );
    const user = userEvent.setup();

    renderSection();
    await user.click(
      await screen.findByRole("button", {
        name: /Aktionen für die Aussetzung/,
      }),
    );
    await user.click(
      screen.getByRole("menuitem", { name: /Aussetzung aufheben/ }),
    );
    await user.click(
      within(await screen.findByRole("dialog")).getByRole("button", {
        name: "Aufheben",
      }),
    );

    const loadText = catalogText(
      "general.unavailable",
      "die Belegaufbewahrung",
    );
    expect(
      await screen.findByText((text) => text.startsWith(loadText)),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Die Aussetzung ist aufgehoben."),
    ).toBeInTheDocument();
    // Anker: die bestätigten Tabellen bleiben stehen.
    expect(screen.getByText("Betriebsprüfung")).toBeInTheDocument();
    expect(
      screen.getByText("Buchungsbeleg (AO, HGB, UStG): § 147 AO"),
    ).toBeInTheDocument();
    const message = `${loadText} ${STALE_DATA_SUFFIX}`;
    expect(screen.getByText(message)).toBeInTheDocument();
    expect(
      screen.queryByText(
        catalogText("general.unavailable", "die Aufhebung der Aussetzung"),
      ),
    ).not.toBeInTheDocument();

    mockGetOverview.mockResolvedValue(overview);
    const alert = screen.getByText(message).closest('[role="alert"]');
    expect(alert).not.toBeNull();
    await user.click(
      within(alert as HTMLElement).getByRole("button", {
        name: "Wiederholen",
      }),
    );
    expect(await screen.findByText("Keine Aussetzung")).toBeInTheDocument();
    expect(screen.queryByText(message)).not.toBeInTheDocument();
    // Wiederholen lädt nur nach; aufgehoben wird nicht noch einmal.
    expect(belegRetentionService.liftSuspension).toHaveBeenCalledTimes(1);
    expect(belegRetentionService.liftSuspension).toHaveBeenCalledWith("1", "9");
  });

  it("shows a lift failure as the error of the lift and keeps the suspension", async () => {
    mockGetOverview.mockResolvedValue(withSuspension);
    vi.mocked(belegRetentionService.liftSuspension).mockRejectedValue(
      unavailable(),
    );
    const user = userEvent.setup();

    renderSection();
    await user.click(
      await screen.findByRole("button", {
        name: /Aktionen für die Aussetzung/,
      }),
    );
    await user.click(
      screen.getByRole("menuitem", { name: /Aussetzung aufheben/ }),
    );
    await user.click(
      within(await screen.findByRole("dialog")).getByRole("button", {
        name: "Aufheben",
      }),
    );

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Aufhebung der Aussetzung"),
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Betriebsprüfung")).toBeInTheDocument();
    expect(
      screen.queryByText("Die Aussetzung ist aufgehoben."),
    ).not.toBeInTheDocument();
  });

  it("replaces the section with the error while no data has loaded", async () => {
    mockGetOverview.mockRejectedValue(unavailable());

    renderSection();

    const message = catalogText("general.unavailable", "die Belegaufbewahrung");
    expect(await screen.findByText(message)).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "Klärungsliste" }),
    ).not.toBeInTheDocument();
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
    expect(mapped.instructions[0]?.statutesNotApplicableConfirmed).toBe(false);
    expect(mapped.instructions[0]?.confirmedAt).toBeNull();
  });
});
