import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { EnrollmentRequestItem } from "./enrollment-request-item";
import type { EnrollmentChangeRequest } from "~/lib/change-request-list-api";

vi.mock("~/lib/tenant-path", () => ({
  useTenantAwarePath: () => (href: string) => `/demo${href}`,
}));

function request(
  overrides: Partial<EnrollmentChangeRequest> = {},
): EnrollmentChangeRequest {
  return {
    id: "12",
    request_id: "57",
    origin: "parent",
    status: "pending_review",
    child_names: ["Lina Beispiel", "Timo Beispiel"],
    guardian_name: "Anna Beispiel",
    parent_note: "Bitte Telefonnummer ändern.",
    base_snapshot: {
      guardian_phone: "0221 12345",
      children: [
        { first_name: "Lina" },
        { first_name: "Timo", target_grade_level: 1 },
      ],
    },
    proposed_snapshot: {
      guardian_phone: "0231 98765",
      children: [
        { first_name: "Lina" },
        { first_name: "Timo", target_grade_level: 2 },
      ],
    },
    diff: { changed: ["guardian_phone", "children"] },
    created_at: "2026-08-19T09:00:00Z",
    ...overrides,
  };
}

describe("EnrollmentRequestItem", () => {
  it("zeigt Kinder, Einreichung und die betroffenen Teile der Anmeldung", () => {
    render(<EnrollmentRequestItem row={request()} view="open" />);
    // Die Zeile ist zugeklappt; das Detail steht erst danach.
    fireEvent.click(screen.getByRole("button"));

    expect(screen.getByText("Lina Beispiel, Timo Beispiel")).toBeVisible();
    expect(screen.getByText("Wartet auf Prüfung")).toBeVisible();
    // Das echte vorher → nachher, nicht nur die Namen der Bereiche.
    expect(screen.getByText("Änderungen")).toBeVisible();
    expect(screen.getByText("Telefon: 0221 12345 → 0231 98765")).toBeVisible();
    expect(screen.getByText("Kind 2 · Zielklasse: 1 → 2")).toBeVisible();
    expect(
      screen.getByText(/Eingereicht am 19\.08\.2026 von Anna Beispiel/),
    ).toBeVisible();
    // Entschieden wird in der Detailansicht mit Rückfrage-Dialog.
    expect(screen.getByRole("link", { name: /Prüfen/ })).toHaveAttribute(
      "href",
      "/demo/admin/enrollments/change-requests/12",
    );
  });

  it("zeigt in der Historie beide Begründungen und die Entscheidung", () => {
    render(
      <EnrollmentRequestItem
        row={request({
          status: "approved",
          decision_note: "Passt, freigegeben.",
          decided_at: "2026-08-20T09:00:00Z",
          decided_by_name: "Anna Müller",
        })}
        view="history"
      />,
    );
    // Die Zeile ist zugeklappt; das Detail steht erst danach.
    fireEvent.click(screen.getByRole("button"));

    expect(screen.getByText("Freigegeben")).toBeVisible();
    expect(
      screen.getByText(/Entschieden am 20\.08\.2026 von Anna Müller/),
    ).toBeVisible();
    // Die Begründung der Familie darf die der Entscheidung nicht verdrängen.
    expect(screen.getByText(/Bitte Telefonnummer ändern\./)).toBeVisible();
    expect(screen.getByText(/Passt, freigegeben\./)).toBeVisible();
  });

  it("kürzt sehr viele Feldänderungen und sagt, wie viele fehlen", () => {
    const many = Object.fromEntries(
      Array.from({ length: 9 }, (_, index) => [
        `feld_${index}`,
        `alt-${index}`,
      ]),
    );
    render(
      <EnrollmentRequestItem
        row={request({
          base_snapshot: many,
          proposed_snapshot: Object.fromEntries(
            Object.keys(many).map((key) => [key, "neu"]),
          ),
          diff: { changed: Object.keys(many) },
        })}
        view="open"
      />,
    );
    // Die Zeile ist zugeklappt; das Detail steht erst danach.
    fireEvent.click(screen.getByRole("button"));

    expect(screen.getByText("und 3 weitere Änderungen")).toBeVisible();
  });

  it("kennzeichnet eine Korrektur der OGS als solche", () => {
    render(
      <EnrollmentRequestItem
        row={request({ origin: "admin", status: "approved" })}
        view="history"
      />,
    );
    // Die Zeile ist zugeklappt; das Detail steht erst danach.
    fireEvent.click(screen.getByRole("button"));

    expect(screen.getByText(/Korrektur der OGS/)).toBeVisible();
    expect(screen.queryByText(/von Anna Beispiel/)).toBeNull();
  });
});
