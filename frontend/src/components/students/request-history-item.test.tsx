import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { RequestHistoryItem } from "./request-history-item";
import type { AggregatedHistoryRequest } from "~/lib/change-request-list-api";
import type { StaffMasterDataHistoryEntry } from "~/lib/master-data-review-api";

function masterDataItem(
  overrides: Partial<StaffMasterDataHistoryEntry> = {},
): AggregatedHistoryRequest {
  return {
    request_type: "master_data",
    occurred_at: "2026-08-20T10:00:00Z",
    data: {
      id: "1",
      student_id: "42",
      first_name: "Lara",
      last_name: "Lehmann",
      target: "person",
      field_key: "first_name",
      old_value: "Lara",
      new_value: "Clara",
      status: "rejected",
      created_at: "2026-08-17T09:00:00Z",
      decided_at: "2026-08-18T10:00:00Z",
      decided_by_name: "Rieke Reviewer",
      review_reason: "zu kurz",
      ...overrides,
    },
  };
}

/**
 * Eine Historien-Zeile ist zugeklappt; Zeitpunkt, Begruendung und das
 * Aenderungspanel stehen erst danach. Die Zeile ist die einzige Schaltflaeche
 * einer gerenderten Karte, deshalb reicht der Rollen-Zugriff ohne Namen.
 */
function aufklappen() {
  fireEvent.click(screen.getByRole("button"));
}

describe("RequestHistoryItem", () => {
  it("zeigt bei Stammdaten Status, Person, Begründung und alt → neu", () => {
    render(<RequestHistoryItem item={masterDataItem()} />);
    aufklappen();

    expect(screen.getByText("Lara Lehmann")).toBeInTheDocument();
    expect(screen.getByText("Abgelehnt")).toBeInTheDocument();
    expect(
      screen.getByText(/Entschieden am 18\.08\.2026 von Rieke Reviewer/),
    ).toBeInTheDocument();
    expect(screen.getByText("„zu kurz“")).toBeInTheDocument();
    expect(screen.getByText(/Lara → Clara/)).toBeInTheDocument();
  });

  it("lässt bei automatisch übernommenen Änderungen die Person weg", () => {
    render(
      <RequestHistoryItem
        item={masterDataItem({
          status: "auto_applied",
          decided_by_name: undefined,
          review_reason: undefined,
        })}
      />,
    );
    aufklappen();

    expect(screen.getByText("Automatisch übernommen")).toBeInTheDocument();
    expect(
      screen.getByText(/Entschieden am 18\.08\.2026$/),
    ).toBeInTheDocument();
  });

  it("zeigt die Abholzeit einer entschiedenen Abholzeit-Anfrage", () => {
    const item: AggregatedHistoryRequest = {
      request_type: "care_schedule",
      occurred_at: "2026-08-20T10:00:00Z",
      data: {
        id: "pickup-1",
        student_id: "42",
        first_name: "Lara",
        last_name: "Lehmann",
        status: "rejected",
        request_kind: "pickup_change",
        requested: [
          {
            label: "20.08.2026 · Abholzeit",
            old: "",
            new: "15:30",
            care_kind: "pickup",
          },
        ],
        decision_reason: "Nicht möglich",
        created_at: "2026-08-17T09:00:00Z",
        decided_at: "2026-08-18T10:00:00Z",
      },
    };

    render(<RequestHistoryItem item={item} />);

    aufklappen();

    // Die Zusammenfassung steht in einer eigenen Spalte; der frühere Trenner
    // vor ihr war der zum Kindernamen auf derselben Zeile.
    expect(screen.getByText("Abholzeit")).toBeInTheDocument();
    expect(screen.getByText("Beantragt")).toBeInTheDocument();
    expect(
      screen.getByText("20.08.2026 · Abholzeit: 15:30"),
    ).toBeInTheDocument();
  });

  it("zeigt bei Betreuungszeiten den eingefrorenen alt → neu Diff statt der Beantragt-Liste", () => {
    const item: AggregatedHistoryRequest = {
      request_type: "care_schedule",
      occurred_at: "2026-08-20T10:00:00Z",
      data: {
        id: "care-1",
        student_id: "42",
        first_name: "Lara",
        last_name: "Lehmann",
        status: "approved",
        request_kind: "weekly_schedule",
        requested: [
          {
            label: "Montag · Abholzeit",
            old: "",
            new: "16:00",
            care_kind: "pickup",
          },
        ],
        diff: [
          {
            label: "Montag · Abholzeit",
            old: "15:00",
            new: "16:00",
            care_kind: "pickup",
          },
        ],
        created_at: "2026-08-17T09:00:00Z",
        decided_at: "2026-08-18T10:00:00Z",
        decided_by_name: "Rieke Reviewer",
      },
    };

    render(<RequestHistoryItem item={item} />);

    aufklappen();

    expect(screen.getByText("Änderungen")).toBeInTheDocument();
    expect(
      screen.getByText("Montag · Abholzeit: 15:00 → 16:00"),
    ).toBeInTheDocument();
    expect(screen.queryByText("Beantragt")).not.toBeInTheDocument();
  });

  it("zeigt bei einer zurückgezogenen Angebots-Anfrage die gespeicherten Angebote", () => {
    const item: AggregatedHistoryRequest = {
      request_type: "offering",
      occurred_at: "2026-08-20T10:00:00Z",
      data: {
        id: "offering-1",
        student_id: "42",
        student_name: "Lara Lehmann",
        status: "withdrawn",
        effective_from: "2026-08-20",
        diff: [],
        requested: [{ offering_id: "17", label: "Kreativ-AG", new: "Di" }],
        created_at: "2026-08-17T09:00:00Z",
        decided_at: "2026-08-18T10:00:00Z",
      },
    };

    render(<RequestHistoryItem item={item} />);

    aufklappen();

    expect(screen.getByText("Zurückgezogen")).toBeInTheDocument();
    expect(screen.getByText("Beantragt")).toBeInTheDocument();
    expect(screen.getByText("Kreativ-AG: Di")).toBeInTheDocument();
  });

  it("zeigt ohne Diff und beantragte Angebote keine leere Beantragt-Fläche", () => {
    const item: AggregatedHistoryRequest = {
      request_type: "offering",
      occurred_at: "2026-08-20T10:00:00Z",
      data: {
        id: "offering-empty",
        student_id: "42",
        student_name: "Lara Lehmann",
        status: "withdrawn",
        effective_from: "2026-08-20",
        diff: [],
        requested: [],
        created_at: "2026-08-17T09:00:00Z",
        decided_at: "2026-08-18T10:00:00Z",
      },
    };

    render(<RequestHistoryItem item={item} />);

    aufklappen();

    expect(screen.queryByText("Beantragt")).not.toBeInTheDocument();
  });

  it("zeigt bei einer entschuldigten Abmeldung Daten und Notiz", () => {
    const item: AggregatedHistoryRequest = {
      request_type: "excused",
      occurred_at: "2026-08-20T10:00:00Z",
      data: {
        id: "excused-1",
        student_id: "42",
        first_name: "Lara",
        last_name: "Lehmann",
        status: "approved",
        dates: ["2026-08-20", "2026-08-21"],
        note: "Arzttermin",
        created_at: "2026-08-17T09:00:00Z",
        decided_at: "2026-08-18T10:00:00Z",
        decided_by_name: "Rieke Reviewer",
      },
    };

    render(<RequestHistoryItem item={item} />);

    aufklappen();

    expect(screen.getByText("Freigegeben")).toBeInTheDocument();
    expect(
      screen.getByText("20.08.2026, 21.08.2026 · Arzttermin"),
    ).toBeInTheDocument();
  });

  it("zeigt eine Direkt-Korrektur als eigene Zeilen-Art mit vorher → nachher", () => {
    const item: AggregatedHistoryRequest = {
      request_type: "direct_correction",
      occurred_at: "2026-08-20T10:00:00Z",
      data: {
        id: "9",
        student_id: "42",
        student_name: "Lara Lehmann",
        changed_at: "2026-08-18T10:00:00Z",
        changed_by_name: "Olga Office",
        reason: "Telefonisch gemeldet",
        diff: [
          {
            offering_id: "3",
            label: "Mittagessen",
            old: "Mo",
            new: "abgemeldet",
          },
        ],
      },
    };

    render(<RequestHistoryItem item={item} />);

    aufklappen();

    expect(screen.getByText("Direkt-Korrektur")).toBeInTheDocument();
    // Keine Anfrage: nichts wurde eingereicht und nichts entschieden.
    expect(
      screen.getByText(/^Geändert am 18\.08\.2026 von Olga Office$/),
    ).toBeInTheDocument();
    expect(screen.queryByText(/Eingereicht am/)).not.toBeInTheDocument();
    expect(screen.getByText("„Telefonisch gemeldet“")).toBeInTheDocument();
    expect(
      screen.getByText("Mittagessen: Mo → abgemeldet"),
    ).toBeInTheDocument();
  });

  it("hält eine Korrektur ohne Tagesänderung trotzdem fest", () => {
    // Kommt vor, wenn sich nur intern etwas verschiebt (selbst gesetzte gegen
    // automatisch übernommene Tage) und die gebuchten Tage gleich bleiben.
    const item: AggregatedHistoryRequest = {
      request_type: "direct_correction",
      occurred_at: "2026-08-20T10:00:00Z",
      data: {
        id: "10",
        student_id: "42",
        student_name: "Lara Lehmann",
        changed_at: "2026-08-18T10:00:00Z",
        changed_by_name: "Olga Office",
        reason: "Telefonisch gemeldet",
        diff: [],
      },
    };

    render(<RequestHistoryItem item={item} />);

    aufklappen();

    // Jede Änderung wird dokumentiert, auch die ohne sichtbaren Unterschied
    // an den Tagen — dann sagt die Karte das ausdrücklich.
    expect(screen.getByText("Direkt-Korrektur")).toBeInTheDocument();
    expect(screen.getByText("„Telefonisch gemeldet“")).toBeInTheDocument();
    expect(
      screen.getByText("Keine Änderung an den gebuchten Tagen"),
    ).toBeInTheDocument();
  });
});
