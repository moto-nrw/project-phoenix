import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { OfferingRequestReviewItem } from "./offering-request-review-item";
import {
  OfferingRequestApiError,
  decideOfferingChangeRequest,
  previewOfferingChangeRequest,
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

describe("OfferingRequestReviewItem", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockPreview.mockResolvedValue({ selections: [] });
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

    fireEvent.click(screen.getByRole("button", { name: /Freigeben/ }));

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
      "Änderung übernommen, gültig ab 01.02.2027",
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

    fireEvent.click(screen.getByRole("button", { name: /Freigeben/ }));

    expect(await screen.findByText(/kein Platz mehr frei/)).toBeInTheDocument();
    // The card survives a failed approval: the switch was not applied.
    expect(screen.getByText(/Lara Beispiel/)).toBeInTheDocument();
    expect(onDecided).not.toHaveBeenCalled();
  });

  it("explains an already-decided request", async () => {
    mockDecide.mockRejectedValue(
      new OfferingRequestApiError("gone", "change_request_not_pending"),
    );
    renderItem();

    fireEvent.click(screen.getByRole("button", { name: /Freigeben/ }));

    expect(
      await screen.findByText(/bereits entschieden oder von den Eltern/),
    ).toBeInTheDocument();
  });

  it("explains a missing enrollment", async () => {
    mockDecide.mockRejectedValue(
      new OfferingRequestApiError("gone", "offering_changes_no_enrollment"),
    );
    renderItem();

    fireEvent.click(screen.getByRole("button", { name: /Freigeben/ }));

    expect(
      await screen.findByText(/keine gültige Anmeldung mehr vor/),
    ).toBeInTheDocument();
  });

  it("falls back to a generic message for unknown decide errors", async () => {
    mockDecide.mockRejectedValue(new Error("boom"));
    renderItem();

    fireEvent.click(screen.getByRole("button", { name: /Freigeben/ }));

    expect(
      await screen.findByText(
        "Die Entscheidung konnte nicht gespeichert werden.",
      ),
    ).toBeInTheDocument();
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
    mockPreview.mockResolvedValue({
      selections: [{ offering_id: "9", new: "Mo, Mi" }],
    });
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
    mockPreview.mockResolvedValue({
      selections: [{ offering_id: "9", new: "Mo, Mi" }],
    });
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
    mockPreview.mockResolvedValue({
      selections: [{ offering_id: "5", new: "Mo, Di, Mi, Do, Fr" }],
    });
    renderItem(request({ diff: ruleDiff }));

    fireEvent.click(
      screen.getByRole("checkbox", {
        name: /Ganztagsbetreuung bis 14.30 Uhr automatisch mitbuchen/,
      }),
    );
    await waitFor(() =>
      expect(mockPreview).toHaveBeenCalledWith("77", ["9"], "2027-02-01"),
    );
    fireEvent.click(screen.getByRole("button", { name: /Freigeben/ }));

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
    mockPreview.mockResolvedValue({
      selections: [{ offering_id: "5", new: "Mo, Di, Mi, Do, Fr" }],
    });
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
    mockPreview.mockResolvedValue({
      selections: [{ offering_id: "9", new: "Mo, Mi" }],
    });
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
    mockPreview.mockResolvedValue({
      selections: [
        { offering_id: "5", new: "Di" },
        { offering_id: "9", new: "Mo, Mi" },
        { offering_id: "11", new: "Mo, Mi" },
      ],
    });
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
    mockPreview.mockResolvedValue({
      selections: [
        { offering_id: "5", new: "Mo, Di, Mi, Do, Fr" },
        { offering_id: "9", new: "abgemeldet", removed: true },
        { offering_id: "11", new: "abgemeldet", removed: true },
      ],
    });
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

    expect(
      await screen.findByText(
        "Die Vorschau konnte nicht aktualisiert werden. Bitte versuchen Sie es noch einmal.",
      ),
    ).toBeInTheDocument();
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

    it("names what to check after the approval", () => {
      renderItem();

      expect(
        screen.getByText("Nach dem Freigeben bitte prüfen"),
      ).toBeInTheDocument();
      expect(screen.getByText("Gehzeiten des Kindes")).toBeInTheDocument();
      expect(screen.getByText("Zuordnung im Stundenplan")).toBeInTheDocument();
      expect(screen.getByText("Listen und Ausdrucke")).toBeInTheDocument();
    });
  });
});

// Gültigkeitsdatum bei der Freigabe (#2484): Die Schule bestätigt, ab wann die
// Umstellung gilt — vorbelegt mit dem Wunsch der Eltern.
describe("OfferingRequestReviewItem — Gültig ab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockPreview.mockResolvedValue({ selections: [] });
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

    fireEvent.click(screen.getByLabelText("Anderes Datum wählen"));

    await waitFor(() =>
      expect(screen.getByText("01.02.2027")).toBeInTheDocument(),
    );
    expect(screen.queryByLabelText("Gültig ab")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Freigeben/ }));
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

    fireEvent.click(screen.getByRole("button", { name: /Freigeben/ }));

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
      "Änderung übernommen, gültig ab 01.03.2027",
    );
  });

  it("keeps the approval usable when the preview fails transiently", async () => {
    mockPreview.mockRejectedValue(new Error("network"));
    renderItem();
    unlockDate();

    fireEvent.change(screen.getByLabelText("Gültig ab"), {
      target: { value: "2027-03-01" },
    });

    expect(
      await screen.findByText(/Vorschau konnte nicht aktualisiert werden/),
    ).toBeInTheDocument();
    // Ein Netzfehler ist gleich wieder weg: die Karte darf nicht verriegeln.
    expect(screen.getByRole("button", { name: /Freigeben/ })).toBeEnabled();
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

    expect(
      await screen.findByText(/Zu diesem Datum kann die Änderung nicht gelten/),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Freigeben/ })).toBeDisabled();
    // Ablehnen bleibt möglich: eine so nicht umsetzbare Anfrage muss aus der
    // Liste kommen können.
    expect(screen.getByRole("button", { name: /Ablehnen/ })).toBeEnabled();
    expect(mockDecide).not.toHaveBeenCalled();
  });
});
