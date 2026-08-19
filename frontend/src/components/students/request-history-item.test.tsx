import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { RequestHistoryItem } from "./request-history-item";
import type { AggregatedHistoryRequest } from "~/lib/change-request-list-api";
import type { StaffMasterDataHistoryEntry } from "~/lib/master-data-review-api";

function masterDataItem(
  overrides: Partial<StaffMasterDataHistoryEntry> = {},
): AggregatedHistoryRequest {
  return {
    request_type: "master_data",
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

describe("RequestHistoryItem", () => {
  it("zeigt bei Stammdaten Status, Person, Begründung und alt → neu", () => {
    render(<RequestHistoryItem item={masterDataItem()} />);

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

    expect(screen.getByText("Automatisch übernommen")).toBeInTheDocument();
    expect(
      screen.getByText(/Entschieden am 18\.08\.2026$/),
    ).toBeInTheDocument();
  });

  it("zeigt die Abholzeit einer entschiedenen Abholzeit-Anfrage", () => {
    const item: AggregatedHistoryRequest = {
      request_type: "care_schedule",
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

    expect(screen.getByText(/· Abholzeit$/)).toBeInTheDocument();
    expect(screen.getByText("Beantragt")).toBeInTheDocument();
    expect(
      screen.getByText("20.08.2026 · Abholzeit: 15:30"),
    ).toBeInTheDocument();
  });

  it("zeigt bei einer zurückgezogenen Angebots-Anfrage die gespeicherten Angebote", () => {
    const item: AggregatedHistoryRequest = {
      request_type: "offering",
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

    expect(screen.getByText("Zurückgezogen")).toBeInTheDocument();
    expect(screen.getByText("Beantragt")).toBeInTheDocument();
    expect(screen.getByText("Kreativ-AG: Di")).toBeInTheDocument();
  });

  it("zeigt bei einer entschuldigten Abmeldung Daten und Notiz", () => {
    const item: AggregatedHistoryRequest = {
      request_type: "excused",
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

    expect(screen.getByText("Freigegeben")).toBeInTheDocument();
    expect(
      screen.getByText("20.08.2026, 21.08.2026 · Arzttermin"),
    ).toBeInTheDocument();
  });
});
