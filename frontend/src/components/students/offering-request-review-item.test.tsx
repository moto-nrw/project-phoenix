import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

// Fehler laufen als Toast; die Karte selbst zeigt keinen Fehlerkasten mehr.
const mockToast = {
  success: vi.fn(),
  error: vi.fn(),
  warning: vi.fn(),
  info: vi.fn(),
};
vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => mockToast,
}));

// Fehlermeldungen laufen als Toast: geprüft wird der Toast-Aufruf, nicht die DOM.
async function expectErrorToast(pattern: RegExp) {
  await waitFor(() =>
    expect(mockToast.error).toHaveBeenCalledWith(
      expect.stringMatching(pattern),
      expect.anything(),
    ),
  );
  await act(async () => undefined);
}

import { OfferingRequestReviewItem } from "./offering-request-review-item";
import {
  OfferingRequestApiError,
  decideOfferingChangeRequest,
  previewOfferingChangeRequest,
  type OfferingRequestPreview,
  type StaffOfferingRequest,
} from "~/lib/offering-request-review-api";

// Mock only the network functions; keep the real error class so the
// component's `err instanceof OfferingRequestApiError` branch resolves.
// Das Gültigkeitsdatum wird als Feld getestet, nicht als Kalender-Overlay: die
// Regel dahinter ist, welches Datum die Freigabe mitnimmt (#2484).
vi.mock("~/components/ui/date-picker", async (importOriginal) => {
  const { isoDatePickerMock } = await import("~/test/mocks/date-picker");
  return { ...(await importOriginal<object>()), ...isoDatePickerMock() };
});

vi.mock("~/lib/offering-request-review-api", async (importActual) => {
  const actual =
    await importActual<typeof import("~/lib/offering-request-review-api")>();
  return {
    ...actual,
    decideOfferingChangeRequest: vi.fn(),
    previewOfferingChangeRequest: vi.fn(),
  };
});

const mockDecide = vi.mocked(decideOfferingChangeRequest);
const mockPreview = vi.mocked(previewOfferingChangeRequest);

function previewResult(
  overrides: Partial<OfferingRequestPreview> = {},
): OfferingRequestPreview {
  return {
    selections: [],
    manual_planning_conflicts: [],
    arrival_expectations_follow_bookings: false,
    ...overrides,
  };
}

function request(
  overrides: Partial<StaffOfferingRequest> = {},
): StaffOfferingRequest {
  return {
    id: "77",
    student_id: "42",
    student_name: "Lara Beispiel",
    status: "pending",
    effective_from: "2027-02-01",
    diff: [
      {
        offering_id: "1",
        label: "Regelbetreuung",
        old: "Mo, Di, Mi",
        new: "Mo, Di",
      },
    ],
    created_at: "2026-07-30T09:00:00Z",
    ...overrides,
  };
}

// The card renders collapsed; expand so the diff and the actions render.
function renderItem(
  row: StaffOfferingRequest = request(),
  onDecided: (notice: string) => void = vi.fn(),
) {
  render(<OfferingRequestReviewItem row={row} onDecided={onDecided} />);
  fireEvent.click(screen.getByRole("button", { name: /Lara Beispiel/ }));
  return onDecided;
}

async function confirmApproval() {
  fireEvent.click(screen.getByRole("button", { name: /^Freigeben$/ }));
  const confirm = await screen.findByRole("button", {
    name: "Änderung freigeben",
  });
  await act(async () => {
    fireEvent.click(confirm);
    await Promise.resolve();
  });
}

describe("OfferingRequestReviewItem", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockPreview.mockResolvedValue(previewResult());
  });

  it("renders the request with its effective date and diff", () => {
    renderItem();

    expect(screen.getByText(/Lara Beispiel/)).toBeInTheDocument();
    expect(screen.getByText(/ab 01\.02\.2027/)).toBeInTheDocument();
    expect(screen.getByText("Mo, Di, Mi")).toBeInTheDocument();
    expect(screen.getByText("Mo, Di")).toBeInTheDocument();
  });

  it("approves the request and reports the notice", async () => {
    mockDecide.mockResolvedValue(undefined);
    const onDecided = vi.fn();
    renderItem(request(), onDecided);

    fireEvent.click(screen.getByRole("button", { name: /^Freigeben$/ }));

    await waitFor(() =>
      expect(mockPreview).toHaveBeenCalledWith("77", [], "2027-02-01"),
    );
    expect(mockDecide).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Änderung freigeben" }));

    await waitFor(() =>
      expect(mockDecide).toHaveBeenCalledWith(
        "77",
        true,
        undefined,
        [],
        "2027-02-01",
      ),
    );
    expect(onDecided).toHaveBeenCalledWith(
      "Änderung übernommen, gültig ab 01.02.2027. Die angezeigten Folgeänderungen wurden übernommen.",
    );
  });

  it("requires a reason before rejecting", async () => {
    renderItem();

    fireEvent.click(screen.getByRole("button", { name: /Ablehnen/ }));

    expect(
      await screen.findByText(
        "Für eine Ablehnung ist eine Begründung erforderlich.",
      ),
    ).toBeInTheDocument();
    expect(mockDecide).not.toHaveBeenCalled();
  });

  it("rejects with a reason and reports the notice", async () => {
    mockDecide.mockResolvedValue(undefined);
    const onDecided = vi.fn();
    renderItem(request(), onDecided);

    fireEvent.change(
      screen.getByPlaceholderText("Begründung (Pflicht bei Ablehnung)"),
      { target: { value: "Kein Bedarf" } },
    );
    fireEvent.click(screen.getByRole("button", { name: /Ablehnen/ }));

    await waitFor(() =>
      expect(mockDecide).toHaveBeenCalledWith(
        "77",
        false,
        "Kein Bedarf",
        [],
        undefined,
      ),
    );
    expect(onDecided).toHaveBeenCalledWith("Angebots-Anfrage abgelehnt");
  });

  it("names the capacity conflict and keeps the card pending", async () => {
    mockDecide.mockRejectedValue(
      new OfferingRequestApiError("full", "offering_change_capacity_full"),
    );
    const onDecided = vi.fn();
    renderItem(request(), onDecided);

    await confirmApproval();

    await expectErrorToast(/kein Platz mehr frei/);
    // The card survives a failed approval: the switch was not applied.
    expect(screen.getByText(/Lara Beispiel/)).toBeInTheDocument();
    expect(onDecided).not.toHaveBeenCalled();
  });

  it("explains an already-decided request", async () => {
    mockDecide.mockRejectedValue(
      new OfferingRequestApiError("gone", "change_request_not_pending"),
    );
    renderItem();

    await confirmApproval();

    await expectErrorToast(/bereits entschieden oder von den Eltern/);
  });

  it("explains a missing enrollment", async () => {
    mockDecide.mockRejectedValue(
      new OfferingRequestApiError("gone", "offering_changes_no_enrollment"),
    );
    renderItem();

    await confirmApproval();

    await expectErrorToast(/keine gültige Anmeldung mehr vor/);
  });

  it("falls back to a generic message for unknown decide errors", async () => {
    mockDecide.mockRejectedValue(new Error("boom"));
    renderItem();

    await confirmApproval();

    await expectErrorToast(
      /Die Entscheidung konnte nicht gespeichert werden\./,
    );
  });

  it("shows the parent note when one was added", () => {
    renderItem(request({ note: "Neuer Arbeitsbeginn" }));

    expect(
      screen.getByText(/Nachricht der Eltern: Neuer Arbeitsbeginn/),
    ).toBeInTheDocument();
  });

  // Mitbuchungs-Regeln (#2365/#2370): rule-added lines are marked, name their
  // trigger, and can be unticked for this one approval.
  const ruleDiff = [
    {
      offering_id: "5",
      label: "Randstunde",
      old: "nicht gebucht",
      new: "Mo, Di, Mi, Do, Fr",
    },
    {
      offering_id: "9",
      label: "Ganztagsbetreuung bis 14.30 Uhr",
      old: "nicht gebucht",
      new: "Mo, Di, Mi, Do, Fr",
      automatic: true,
      automatic_days: "Mo, Di, Mi, Do, Fr",
      trigger_ids: ["5"],
      trigger_names: ["Randstunde"],
      optoutable: true,
    },
    {
      offering_id: "11",
      label: "Ganztagsbetreuung bis 16 Uhr",
      old: "nicht gebucht",
      new: "Mo, Di, Mi, Do, Fr",
      automatic: true,
      automatic_days: "Mo, Di, Mi, Do, Fr",
      trigger_ids: ["9"],
      trigger_names: ["Ganztagsbetreuung bis 14.30 Uhr"],
      optoutable: true,
    },
  ];

  const mixedRuleDiff = [
    {
      offering_id: "9",
      label: "Ganztagsbetreuung bis 14.30 Uhr",
      old: "nicht gebucht",
      new: "Mo, Di, Mi",
      automatic: true,
      automatic_days: "Di, Mi",
      rule_days: "Di",
      new_when_excluded: "Mo, Mi",
      trigger_ids: ["5"],
      trigger_names: ["Randstunde"],
      optoutable: true,
    },
  ];

  it("marks a rule-added line and names its trigger", () => {
    renderItem(request({ diff: ruleDiff }));

    expect(screen.getAllByText("Automatisch mitgebucht")).toHaveLength(2);
    expect(
      screen.getByText(/weil „Randstunde“ gewählt ist/),
    ).toBeInTheDocument();
  });

  it("attributes only rule-derived days to the selected trigger", () => {
    renderItem(request({ diff: mixedRuleDiff }));

    expect(
      screen.getByText(/Die Tage Di kommen automatisch dazu, weil/),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/Die Tage Di, Mi kommen automatisch dazu, weil/),
    ).not.toBeInTheDocument();
  });

  it("hides the automatic-addition hint after opting out", async () => {
    mockPreview.mockResolvedValue(
      previewResult({
        selections: [{ offering_id: "9", new: "Mo, Mi" }],
      }),
    );
    renderItem(request({ diff: mixedRuleDiff }));

    fireEvent.click(
      screen.getByRole("checkbox", {
        name: /Ganztagsbetreuung bis 14.30 Uhr automatisch mitbuchen/,
      }),
    );

    await waitFor(() =>
      expect(
        screen.queryByText(/kommen automatisch dazu/),
      ).not.toBeInTheDocument(),
    );
  });

  it("describes the disabled co-booking rule after opting out", async () => {
    mockPreview.mockResolvedValue(
      previewResult({
        selections: [{ offering_id: "9", new: "Mo, Mi" }],
      }),
    );
    renderItem(request({ diff: mixedRuleDiff }));

    fireEvent.click(
      screen.getByRole("checkbox", {
        name: /Ganztagsbetreuung bis 14.30 Uhr automatisch mitbuchen/,
      }),
    );

    expect(
      await screen.findByText(
        "Die Mitbuchungs-Regel gilt für diese Anfrage nicht.",
      ),
    ).toBeInTheDocument();
  });

  it("sends unticked rule-added lines as excluded on approve", async () => {
    mockDecide.mockResolvedValue(undefined);
    mockPreview.mockResolvedValue(
      previewResult({
        selections: [{ offering_id: "5", new: "Mo, Di, Mi, Do, Fr" }],
      }),
    );
    renderItem(request({ diff: ruleDiff }));

    fireEvent.click(
      screen.getByRole("checkbox", {
        name: /Ganztagsbetreuung bis 14.30 Uhr automatisch mitbuchen/,
      }),
    );
    await waitFor(() =>
      expect(mockPreview).toHaveBeenCalledWith("77", ["9"], "2027-02-01"),
    );
    await confirmApproval();

    await waitFor(() =>
      expect(mockDecide).toHaveBeenCalledWith(
        "77",
        true,
        undefined,
        ["9"],
        "2027-02-01",
      ),
    );
  });

  it("does not send excluded lines on reject", async () => {
    mockDecide.mockResolvedValue(undefined);
    mockPreview.mockResolvedValue(
      previewResult({
        selections: [{ offering_id: "5", new: "Mo, Di, Mi, Do, Fr" }],
      }),
    );
    renderItem(request({ diff: ruleDiff }));

    fireEvent.click(
      screen.getByRole("checkbox", {
        name: /Ganztagsbetreuung bis 14.30 Uhr automatisch mitbuchen/,
      }),
    );
    await waitFor(() =>
      expect(mockPreview).toHaveBeenCalledWith("77", ["9"], "2027-02-01"),
    );
    fireEvent.change(
      screen.getByPlaceholderText("Begründung (Pflicht bei Ablehnung)"),
      { target: { value: "Kein Bedarf" } },
    );
    fireEvent.click(screen.getByRole("button", { name: /Ablehnen/ }));

    await waitFor(() =>
      expect(mockDecide).toHaveBeenCalledWith(
        "77",
        false,
        "Kein Bedarf",
        [],
        undefined,
      ),
    );
  });

  it("keeps manual and required days visible when rule days are unticked", async () => {
    mockPreview.mockResolvedValue(
      previewResult({
        selections: [{ offering_id: "9", new: "Mo, Mi" }],
      }),
    );
    renderItem(
      request({
        diff: [
          {
            offering_id: "9",
            label: "Ganztagsbetreuung bis 14.30 Uhr",
            old: "nicht gebucht",
            new: "Mo, Di, Mi",
            automatic: true,
            automatic_days: "Di, Mi",
            new_when_excluded: "Mo, Mi",
            trigger_ids: ["5"],
            trigger_names: ["Randstunde"],
            optoutable: true,
          },
        ],
      }),
    );

    fireEvent.click(
      screen.getByRole("checkbox", {
        name: /Ganztagsbetreuung bis 14.30 Uhr automatisch mitbuchen/,
      }),
    );

    expect(await screen.findByText("Mo, Mi")).toBeInTheDocument();
    expect(screen.queryByText("Mo, Di, Mi")).not.toBeInTheDocument();
  });

  it("recomputes downstream rule days after a partial override", async () => {
    mockPreview.mockResolvedValue(
      previewResult({
        selections: [
          { offering_id: "5", new: "Di" },
          { offering_id: "9", new: "Mo, Mi" },
          { offering_id: "11", new: "Mo, Mi" },
        ],
      }),
    );
    renderItem(
      request({
        diff: [
          {
            offering_id: "5",
            label: "Randstunde",
            old: "nicht gebucht",
            new: "Di",
          },
          {
            offering_id: "9",
            label: "Ganztagsbetreuung bis 14.30 Uhr",
            old: "nicht gebucht",
            new: "Mo, Di, Mi",
            automatic: true,
            automatic_days: "Di",
            new_when_excluded: "Mo, Mi",
            trigger_ids: ["5"],
            trigger_names: ["Randstunde"],
            optoutable: true,
          },
          {
            offering_id: "11",
            label: "Ganztagsbetreuung bis 16 Uhr",
            old: "nicht gebucht",
            new: "Mo, Di, Mi",
            automatic: true,
            automatic_days: "Mo, Di, Mi",
            trigger_ids: ["9"],
            trigger_names: ["Ganztagsbetreuung bis 14.30 Uhr"],
            optoutable: true,
          },
        ],
      }),
    );

    fireEvent.click(
      screen.getByRole("checkbox", {
        name: /Ganztagsbetreuung bis 14.30 Uhr automatisch mitbuchen/,
      }),
    );

    await waitFor(() => {
      const downstream = screen.getByText(
        /Ganztagsbetreuung bis 16 Uhr/,
      ).parentElement;
      expect(downstream?.textContent).toContain("Mo, Mi");
      expect(downstream?.textContent).not.toContain("Mo, Di, Mi");
      expect(downstream?.textContent).not.toContain("Kommt automatisch dazu");
    });
  });

  it("greys a line whose trigger was unticked", async () => {
    mockPreview.mockResolvedValue(
      previewResult({
        selections: [
          { offering_id: "5", new: "Mo, Di, Mi, Do, Fr" },
          { offering_id: "9", new: "abgemeldet", removed: true },
          { offering_id: "11", new: "abgemeldet", removed: true },
        ],
      }),
    );
    renderItem(request({ diff: ruleDiff }));

    fireEvent.click(
      screen.getByRole("checkbox", {
        name: /Ganztagsbetreuung bis 14.30 Uhr automatisch mitbuchen/,
      }),
    );

    expect(
      await screen.findByText(
        /Entfällt, weil „Ganztagsbetreuung bis 14.30 Uhr“ nicht mitgebucht wird/,
      ),
    ).toBeInTheDocument();
  });

  it("reports a preview failure and keeps the card usable", async () => {
    mockPreview.mockRejectedValue(new Error("network"));
    renderItem(request({ diff: mixedRuleDiff }));

    fireEvent.click(
      screen.getByRole("checkbox", {
        name: /Ganztagsbetreuung bis 14.30 Uhr automatisch mitbuchen/,
      }),
    );

    await expectErrorToast(
      /Die Vorschau konnte nicht aktualisiert werden\. Bitte versuchen Sie es noch einmal\./,
    );
    // The opt-out did not take effect: the hint is still shown.
    expect(
      screen.getByText(/Die Tage Di kommen automatisch dazu/),
    ).toBeInTheDocument();
  });
  // Folgen-Anzeige (#2434): eine Komplett-Abmeldung darf nicht wie ein
  // gewöhnlicher Antrag aussehen.
  describe("Folgen der Entscheidung", () => {
    const withdrawal = () =>
      request({
        full_withdrawal: true,
        diff: [
          {
            offering_id: "1",
            label: "Regelbetreuung",
            old: "Mo, Di, Mi",
            new: "abgemeldet",
          },
        ],
      });

    it("warns before a full withdrawal, naming the child", () => {
      renderItem(withdrawal());

      expect(
        screen.getByText(
          "Damit wird Lara Beispiel von allen Angeboten abgemeldet. Danach ist kein Angebot mehr gebucht.",
        ),
      ).toBeInTheDocument();
    });

    it("flags the full withdrawal before the card is expanded", () => {
      render(
        <OfferingRequestReviewItem row={withdrawal()} onDecided={vi.fn()} />,
      );

      expect(screen.getByText("Komplett-Abmeldung")).toBeInTheDocument();
    });

    it("shows no warning for an ordinary request", () => {
      renderItem();

      expect(screen.queryByText(/von allen Angeboten abgemeldet/)).toBeNull();
      expect(screen.queryByText("Komplett-Abmeldung")).toBeNull();
    });

    it("lists the bookings the request leaves untouched", () => {
      renderItem(
        request({
          unchanged: [
            { offering_id: "9", label: "Mittagessen", days: "Mo, Di" },
          ],
        }),
      );

      expect(screen.getByText("Bleibt gebucht")).toBeInTheDocument();
      expect(screen.getByText("Mittagessen:")).toBeInTheDocument();
    });

    it("keeps approval consequences out of the open request", () => {
      renderItem();

      expect(
        screen.queryByText("Das ändert moto automatisch:"),
      ).not.toBeInTheDocument();
    });

    it("shows the automatic consequences before approval", async () => {
      mockPreview.mockResolvedValue(
        previewResult({ arrival_expectations_follow_bookings: true }),
      );
      renderItem();

      fireEvent.click(screen.getByRole("button", { name: /^Freigeben$/ }));

      expect(
        await screen.findByText("Das ändert moto automatisch:"),
      ).toBeInTheDocument();
      expect(
        screen.getByText(
          "Angebotsgebundene Gruppen im Betreuungsplan und Gehzeiten werden angepasst.",
        ),
      ).toBeInTheDocument();
      expect(
        screen.getByText(
          "Die neuen Buchungen bestimmen die erwarteten Betreuungstage. Die Ankunftszeit kommt weiterhin aus der Klassenzeit oder einer eigenen Zeit.",
        ),
      ).toBeInTheDocument();
      expect(mockDecide).not.toHaveBeenCalled();
    });

    it("names manual planning conflicts and the next action", async () => {
      mockPreview.mockResolvedValue(
        previewResult({
          manual_planning_conflicts: [
            {
              activity_group_id: "17",
              activity_group_name: "Freie Hausaufgaben-Gruppe",
              days: ["tue"],
              first_date: "2027-02-02",
              occurrence_count: 8,
            },
          ],
        }),
      );
      renderItem();

      fireEvent.click(screen.getByRole("button", { name: /^Freigeben$/ }));

      expect(
        await screen.findByText(/Freie Hausaufgaben-Gruppe/),
      ).toHaveTextContent("Freie Hausaufgaben-Gruppe");
      expect(
        screen.getByText(/Freie Hausaufgaben-Gruppe/).parentElement,
      ).toHaveTextContent("Di, ab 02.02.2027 · 8 Termine");
      expect(
        screen.getByText(
          "Nach der Freigabe: Öffnen Sie den Betreuungsplan. Entfernen Sie Lara Beispiel an den genannten Tagen aus diesen Gruppen oder ändern Sie die Gruppentage.",
        ),
      ).toBeInTheDocument();
    });

    it("shows no manual warning when the preview has no conflict", async () => {
      renderItem();

      fireEvent.click(screen.getByRole("button", { name: /^Freigeben$/ }));

      await screen.findByText("Das ändert moto automatisch:");
      expect(
        screen.getByText(
          "Ankunftszeit und erwartete Betreuungstage bleiben wie bisher.",
        ),
      ).toBeInTheDocument();
      expect(
        screen.queryByText(/moto ändert sie nicht automatisch/),
      ).not.toBeInTheDocument();
    });

    it("does not approve when the consequence preview fails", async () => {
      mockPreview.mockRejectedValue(new Error("network"));
      renderItem();

      fireEvent.click(screen.getByRole("button", { name: /^Freigeben$/ }));

      await expectErrorToast(/Folgen der Freigabe konnten nicht geprüft/);
      expect(mockDecide).not.toHaveBeenCalled();
      expect(
        screen.queryByRole("button", { name: "Änderung freigeben" }),
      ).not.toBeInTheDocument();
    });
  });
});

// Gültigkeitsdatum bei der Freigabe (#2484): Die Schule bestätigt, ab wann die
// Umstellung gilt — vorbelegt mit dem Wunsch der Eltern.
describe("OfferingRequestReviewItem — Gültig ab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockPreview.mockResolvedValue(previewResult());
  });

  // Das Häkchen „Anderes Datum wählen" schaltet das Feld frei. Ohne es steht
  // dort nur, was die Eltern eingetragen haben.
  function unlockDate() {
    fireEvent.click(screen.getByLabelText("Anderes Datum wählen"));
  }

  it("shows the parents' date read-only with its calendar week", () => {
    renderItem();

    expect(screen.getByText("01.02.2027")).toBeInTheDocument();
    expect(screen.getByText(/KW 5/)).toBeInTheDocument();
    expect(
      screen.getByText("So haben es die Eltern eingetragen."),
    ).toBeInTheDocument();
    expect(screen.queryByLabelText("Gültig ab")).not.toBeInTheDocument();
  });

  it("names the date the parents wanted when it can no longer be used", () => {
    renderItem(
      request({
        effective_from: "2026-08-22",
        requested_effective_from: "2026-08-15",
      }),
    );

    expect(
      screen.getByText(
        "Die Eltern wünschten den 15.08.2026. Das geht nicht, deshalb steht hier der früheste mögliche Tag.",
      ),
    ).toBeInTheDocument();
  });

  it("bounds the field to the range the backend reports", () => {
    renderItem(
      request({
        earliest_effective_from: "2026-08-21",
        latest_effective_from: "2027-07-31",
      }),
    );
    unlockDate();

    const field = screen.getByLabelText("Gültig ab");
    expect(field).toHaveAttribute("min", "2026-08-21");
    expect(field).toHaveAttribute("max", "2027-07-31");
  });

  it("returns to the parents' date when the tick is taken back", async () => {
    renderItem();
    unlockDate();

    fireEvent.change(screen.getByLabelText("Gültig ab"), {
      target: { value: "2027-03-01" },
    });
    await waitFor(() =>
      expect(mockPreview).toHaveBeenCalledWith("77", [], "2027-03-01"),
    );

    const editDateCheckbox = screen.getByLabelText("Anderes Datum wählen");
    await waitFor(() => expect(editDateCheckbox).toBeEnabled());
    fireEvent.click(editDateCheckbox);

    await waitFor(() =>
      expect(screen.getByText("01.02.2027")).toBeInTheDocument(),
    );
    expect(screen.queryByLabelText("Gültig ab")).not.toBeInTheDocument();

    await confirmApproval();
    await waitFor(() =>
      expect(mockDecide).toHaveBeenCalledWith(
        "77",
        true,
        undefined,
        [],
        "2027-02-01",
      ),
    );
  });

  it("approves with the date the office confirmed, not the parents' wish", async () => {
    mockDecide.mockResolvedValue(undefined);
    const onDecided = vi.fn();
    renderItem(request(), onDecided);

    unlockDate();

    fireEvent.change(screen.getByLabelText("Gültig ab"), {
      target: { value: "2027-03-01" },
    });

    await waitFor(() =>
      expect(mockPreview).toHaveBeenCalledWith("77", [], "2027-03-01"),
    );
    expect(screen.getByText("KW 9")).toBeInTheDocument();

    await confirmApproval();

    await waitFor(() =>
      expect(mockDecide).toHaveBeenCalledWith(
        "77",
        true,
        undefined,
        [],
        "2027-03-01",
      ),
    );
    expect(onDecided).toHaveBeenCalledWith(
      "Änderung übernommen, gültig ab 01.03.2027. Die angezeigten Folgeänderungen wurden übernommen.",
    );
  });

  it("keeps the approval usable when the preview fails transiently", async () => {
    mockPreview.mockRejectedValue(new Error("network"));
    renderItem();
    unlockDate();

    fireEvent.change(screen.getByLabelText("Gültig ab"), {
      target: { value: "2027-03-01" },
    });

    await expectErrorToast(/Vorschau konnte nicht aktualisiert werden/);
    // Ein Netzfehler ist gleich wieder weg: die Karte darf nicht verriegeln.
    expect(screen.getByRole("button", { name: /Freigeben/ })).toBeEnabled();
  });

  it("clears an earlier preview when a later preview fails", async () => {
    mockPreview
      .mockResolvedValueOnce(
        previewResult({
          selections: [{ offering_id: "1", new: "Mo" }],
        }),
      )
      .mockRejectedValueOnce(new Error("network"));
    renderItem(
      request({
        diff: [
          {
            offering_id: "1",
            label: "Regelbetreuung",
            old: "Mo, Di, Mi",
            new: "Mo, Di",
            automatic: true,
            optoutable: true,
          },
        ],
      }),
    );

    fireEvent.click(
      screen.getByLabelText("Regelbetreuung automatisch mitbuchen"),
    );
    await waitFor(() => expect(screen.getByText("Mo")).toBeInTheDocument());

    unlockDate();
    fireEvent.change(screen.getByLabelText("Gültig ab"), {
      target: { value: "2027-03-01" },
    });

    await expectErrorToast(/Vorschau konnte nicht aktualisiert werden/);
    expect(screen.queryByText("Mo")).not.toBeInTheDocument();
    expect(screen.getByText("Mo, Di")).toBeInTheDocument();
  });

  it("explains the parents' date after staff selects a later date", async () => {
    renderItem(
      request({
        effective_from: "2026-08-22",
        requested_effective_from: "2026-08-15",
      }),
    );
    unlockDate();

    fireEvent.change(screen.getByLabelText("Gültig ab"), {
      target: { value: "2026-09-01" },
    });

    await waitFor(() =>
      expect(
        screen.getByText("Die Eltern hatten den 22.08.2026 eingetragen."),
      ).toBeInTheDocument(),
    );
    expect(screen.queryByText(/früheste mögliche Tag/)).not.toBeInTheDocument();
  });

  it("names the selectable range under the field", () => {
    renderItem(
      request({
        earliest_effective_from: "2026-10-20",
        latest_effective_from: "2027-07-31",
      }),
    );
    unlockDate();

    expect(
      screen.getByText("Wählbar von 20.10.2026 bis 31.07.2027."),
    ).toBeInTheDocument();
  });

  it("blocks the approval while the chosen date does not work out", async () => {
    mockPreview.mockRejectedValue(
      new OfferingRequestApiError("range", "offering_change_date_out_of_range"),
    );
    renderItem();
    unlockDate();

    fireEvent.change(screen.getByLabelText("Gültig ab"), {
      target: { value: "2020-01-06" },
    });

    await expectErrorToast(/Zu diesem Datum kann die Änderung nicht gelten/);
    expect(screen.getByRole("button", { name: /Freigeben/ })).toBeDisabled();
    // Ein ausgegrauter Knopf ohne Grund ist eine Sackgasse: der Grund bleibt an
    // der Karte stehen, auch wenn der Toast längst weg ist.
    expect(
      screen.getByText(
        "Zu diesem Datum kann die Änderung nicht gelten. Freigeben ist gesperrt.",
      ),
    ).toBeInTheDocument();
    // Ablehnen bleibt möglich: eine so nicht umsetzbare Anfrage muss aus der
    // Liste kommen können.
    expect(screen.getByRole("button", { name: /Ablehnen/ })).toBeEnabled();
    expect(mockDecide).not.toHaveBeenCalled();
  });
});
