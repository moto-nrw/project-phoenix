import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ConflictDecisionGroup } from "./conflict-decision-group";
import type { ConflictGroup, ReviewItem } from "./case-model";
import { resolveRequestConflict } from "~/lib/change-request-list-api";

vi.mock("~/lib/change-request-list-api", async () => {
  const actual = await vi.importActual<
    typeof import("~/lib/change-request-list-api")
  >("~/lib/change-request-list-api");
  return { ...actual, resolveRequestConflict: vi.fn() };
});

// Wie in offering-change-request-modal.test.tsx: der Kalender selbst ist
// eigenständig geprüft, hier zählt nur der Wert, den er liefert.
vi.mock("~/components/ui/date-picker", () => ({
  ISODatePicker: ({
    label,
    onChange,
  }: {
    label?: string;
    onChange: (date: string) => void;
  }) => (
    <button type="button" onClick={() => onChange("2026-09-01")}>
      {label}
    </button>
  ),
}));

const mockResolve = vi.mocked(resolveRequestConflict);

function excusedItem(id: string, dates: string[]): ReviewItem {
  return {
    request_type: "excused",
    occurred_at: "2026-08-29T09:00:00Z",
    student_id: "10",
    student_name: "Mia Muster",
    expected_version: `v${id}`,
    urgent_today: false,
    bulk_eligible: false,
    family_protected: false,
    data: { id, dates, absence_status: "sick" },
  } as never;
}

function group(overrides: Partial<ConflictGroup> = {}): ConflictGroup {
  return {
    key: "absence:2026-08-29",
    label: "Abwesenheit am 29.08.2026",
    items: [excusedItem("1", ["2026-08-29"]), excusedItem("2", ["2026-08-29"])],
    expectedCount: 2,
    complete: true,
    staffValueInput: "status" as const,
    ...overrides,
  };
}

describe("ConflictDecisionGroup", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockResolve.mockResolvedValue(2);
  });

  it("bietet genau eine Antwort über eine gemeinsame Radio-Gruppe an", () => {
    render(
      <ConflictDecisionGroup
        group={group()}
        onResolved={vi.fn()}
        onStale={vi.fn()}
      />,
    );

    expect(
      screen.getByText("2 Wünsche für Abwesenheit am 29.08.2026"),
    ).toBeVisible();
    const radios = screen.getAllByRole("radio");
    // Zwei Wünsche, ein eigener Wert und „Keine Änderung".
    expect(radios).toHaveLength(4);
    const names = new Set(
      radios.map((radio) => (radio as HTMLInputElement).name),
    );
    expect(names.size).toBe(1);
    expect(radios.filter((r) => (r as HTMLInputElement).checked)).toHaveLength(
      0,
    );
  });

  it("trägt bei einem Angebot Datum und Wochentage ein", async () => {
    // Der Schlüssel `offer:<id>` benennt genau EIN Angebot, also reichen ein
    // Gültigkeitsdatum und die Tage — kein Katalog, kein zweiter Dialog.
    render(
      <ConflictDecisionGroup
        group={group({ key: "offer:7", staffValueInput: "offering" })}
        onResolved={vi.fn()}
        onStale={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByLabelText("Eigenen Wert eintragen"));
    expect(
      screen.getByText("Kein Tag = Abmeldung von diesem Angebot"),
    ).toBeVisible();

    fireEvent.click(screen.getByRole("button", { name: "Gilt ab" }));
    fireEvent.click(screen.getByLabelText("Montag"));
    fireEvent.change(screen.getByLabelText("Begründung"), {
      target: { value: "Mit den Eltern geklärt" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Ergebnis festlegen" }));

    expect(
      screen.getByText("So wird es gespeichert: ab 01.09.2026: Montag"),
    ).toBeVisible();
    fireEvent.click(screen.getByRole("button", { name: "Ergebnis speichern" }));

    await waitFor(() =>
      expect(mockResolve).toHaveBeenCalledWith(
        expect.objectContaining({
          kind: "excused",
          conflictKey: "offer:7",
          staffValue: {
            effective_from: "2026-09-01",
            selections: [{ offering_id: "7", selected_days: ["mon"] }],
          },
        }),
      ),
    );
  });

  it("liest kein gewählter Tag als Abmeldung", () => {
    render(
      <ConflictDecisionGroup
        group={group({ key: "offer:7", staffValueInput: "offering" })}
        onResolved={vi.fn()}
        onStale={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByLabelText("Eigenen Wert eintragen"));
    fireEvent.click(screen.getByRole("button", { name: "Gilt ab" }));
    fireEvent.change(screen.getByLabelText("Begründung"), {
      target: { value: "Platz gebraucht" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Ergebnis festlegen" }));

    expect(
      screen.getByText(
        "So wird es gespeichert: ab 01.09.2026: Abmeldung von diesem Angebot",
      ),
    ).toBeVisible();
  });

  it("zeigt das passende Feld für den eigenen Wert, erst nach der Auswahl", () => {
    const { unmount } = render(
      <ConflictDecisionGroup
        group={group({ key: "care:1:pickup", staffValueInput: "time" })}
        onResolved={vi.fn()}
        onStale={vi.fn()}
      />,
    );

    expect(screen.queryByLabelText(/Eigener Wert/)).not.toBeInTheDocument();
    fireEvent.click(screen.getByLabelText("Eigenen Wert eintragen"));
    // Uhrzeit: ein Textfeld mit sichtbarem Format statt eines Systemrads.
    expect(screen.getByLabelText(/Eigener Wert/)).toBeVisible();
    expect(screen.getByText("Uhrzeit im Format 15:30")).toBeVisible();
    unmount();

    render(
      <ConflictDecisionGroup
        group={group()}
        onResolved={vi.fn()}
        onStale={vi.fn()}
      />,
    );
    fireEvent.click(screen.getByLabelText("Eigenen Wert eintragen"));
    // Abwesenheit: eine Auswahl der möglichen Stände, kein freies Textfeld.
    expect(
      screen.getByRole("combobox", { name: /Eigener Wert/ }),
    ).toBeVisible();
  });

  it("löst eine Abholzeit als pickup_change auf und schickt den Schlüssel mit", async () => {
    render(
      <ConflictDecisionGroup
        group={group({ key: "pickup:2026-08-29", staffValueInput: "time" })}
        onResolved={vi.fn()}
        onStale={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByLabelText("Eigenen Wert eintragen"));
    fireEvent.change(screen.getByLabelText(/Eigener Wert/), {
      target: { value: "1530" },
    });
    fireEvent.change(screen.getByLabelText("Begründung"), {
      target: { value: "Mit den Eltern geklärt" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Ergebnis festlegen" }));
    fireEvent.click(screen.getByRole("button", { name: "Ergebnis speichern" }));

    await waitFor(() =>
      expect(mockResolve).toHaveBeenCalledWith({
        // Eine Abholzeit reist als care_schedule durch die Liste, wird aber
        // als pickup_change aufgelöst.
        kind: "pickup_change",
        conflictKey: "pickup:2026-08-29",
        requestIDs: ["1", "2"],
        expectedVersions: ["v1", "v2"],
        staffValue: "15:30",
        reason: "Mit den Eltern geklärt",
      }),
    );
  });

  it("entscheidet nicht, solange ein Beteiligter noch nicht geladen ist", () => {
    render(
      <ConflictDecisionGroup
        group={group({ expectedCount: 3, complete: false })}
        onResolved={vi.fn()}
        onStale={vi.fn()}
      />,
    );

    expect(
      screen.getByText("3 Wünsche für Abwesenheit am 29.08.2026"),
    ).toBeVisible();
    expect(
      screen.getByText(
        "1 weiterer Wunsch ist noch nicht geladen. Laden Sie zuerst weitere Einträge.",
      ),
    ).toBeVisible();
    fireEvent.click(screen.getAllByRole("radio")[0]!);
    expect(
      screen.getByRole("button", { name: "Ergebnis festlegen" }),
    ).toBeDisabled();
  });

  it("wiederholt das Ergebnis vor dem Speichern und schickt alle Versionen mit", async () => {
    const onResolved = vi.fn();
    render(
      <ConflictDecisionGroup
        group={group()}
        onResolved={onResolved}
        onStale={vi.fn()}
      />,
    );

    const save = screen.getByRole("button", { name: "Ergebnis festlegen" });
    expect(save).toBeDisabled();

    fireEvent.click(screen.getAllByRole("radio")[0]!);
    fireEvent.change(screen.getByLabelText("Begründung"), {
      target: { value: "Mit den Eltern geklärt" },
    });
    fireEvent.click(save);

    expect(
      screen.getByText("So wird es gespeichert: Krank: 2026-08-29"),
    ).toBeVisible();
    expect(
      screen.getByText("Die anderen Wünsche werden abgelehnt."),
    ).toBeVisible();
    fireEvent.click(screen.getByRole("button", { name: "Ergebnis speichern" }));

    await waitFor(() =>
      expect(mockResolve).toHaveBeenCalledWith({
        kind: "excused",
        conflictKey: "absence:2026-08-29",
        requestIDs: ["1", "2"],
        expectedVersions: ["v1", "v2"],
        chosenRequestID: "1",
        reason: "Mit den Eltern geklärt",
      }),
    );
    await waitFor(() => expect(onResolved).toHaveBeenCalled());
  });
});
