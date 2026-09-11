import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { HistoryRequestList } from "./history-request-list";
import { correctRequestDecision } from "~/lib/change-request-list-api";
import type { AnyItem } from "./case-model";

vi.mock("~/lib/change-request-list-api", async () => {
  const actual = await vi.importActual<
    typeof import("~/lib/change-request-list-api")
  >("~/lib/change-request-list-api");
  return { ...actual, correctRequestDecision: vi.fn() };
});

const mockCorrect = vi.mocked(correctRequestDecision);

function decidedItem(overrides: Record<string, unknown> = {}): AnyItem {
  return {
    request_type: "excused",
    occurred_at: "2026-08-20T09:00:00Z",
    expected_version: "v9",
    data: {
      id: "1",
      first_name: "Mia",
      last_name: "Muster",
      absence_status: "sick",
      status: "approved",
      dates: ["2026-08-19"],
      note: "",
      created_at: "2026-08-18T09:00:00Z",
      decided_at: "2026-08-20T09:00:00Z",
      decided_by_name: "Frau Berg",
      reason: "Attest lag vor",
    },
    ...overrides,
  } as never;
}

function renderList(item: AnyItem) {
  return render(
    <HistoryRequestList
      items={[item]}
      withdrawals={[]}
      reasonRequired
      onCorrected={vi.fn()}
    />,
  );
}

describe("Entscheidung korrigieren", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockCorrect.mockResolvedValue({ approved: false });
  });

  it("versteckt die Korrektur, wenn das Backend sie ausdrücklich verbietet", () => {
    renderList(decidedItem({ can_correct: false }));
    fireEvent.click(screen.getAllByRole("button")[0]!);
    expect(
      screen.queryByRole("button", { name: "Entscheidung korrigieren" }),
    ).not.toBeInTheDocument();
  });

  it("bietet die Korrektur an einer entschiedenen Zeile auch ohne can_correct an", () => {
    // Solange das Feld noch nicht mitkommt, wird geschätzt. Zu viel anbieten
    // ist besser als verstecken: das Backend antwortet verständlich.
    renderList(decidedItem());
    fireEvent.click(screen.getAllByRole("button")[0]!);
    expect(
      screen.getByRole("button", { name: "Entscheidung korrigieren" }),
    ).toBeVisible();
  });

  it("korrigiert eine Abholzeit als care_schedule, wenn das Backend sie freigibt", async () => {
    // Eine Abholzeit liegt in der Betreuungszeiten-Warteschlange. Ohne
    // `can_correct` würde die Schätzung sie verstecken; mit dem Feld gewinnt
    // das Backend, und korrigiert wird unter der Art der Warteschlange.
    renderList(
      decidedItem({
        request_type: "care_schedule",
        can_correct: true,
        expected_version: "v4",
        data: {
          id: "8",
          first_name: "Mia",
          last_name: "Muster",
          status: "approved",
          request_kind: "pickup_change",
          requested: [],
          diff: [],
          created_at: "2026-08-18T09:00:00Z",
          decided_at: "2026-08-20T09:00:00Z",
        },
      }),
    );

    fireEvent.click(screen.getAllByRole("button")[0]!);
    fireEvent.click(
      screen.getByRole("button", { name: "Entscheidung korrigieren" }),
    );
    fireEvent.change(screen.getByLabelText(/Warum korrigieren Sie\?/), {
      target: { value: "Falsche Uhrzeit" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Korrektur speichern" }),
    );

    await waitFor(() =>
      expect(mockCorrect).toHaveBeenCalledWith("care_schedule", "8", {
        approve: false,
        reason: "Falsche Uhrzeit",
        expectedVersion: "v4",
      }),
    );
  });

  it("bietet ohne can_correct bei Betreuungszeiten keine Korrektur an", () => {
    // Dort gibt es keinen Ausgangszustand; das Backend weist es mit
    // correction_unsupported ab.
    renderList(
      decidedItem({
        request_type: "care_schedule",
        data: {
          id: "2",
          first_name: "Mia",
          last_name: "Muster",
          status: "approved",
          request_kind: "weekly",
          requested: [],
          diff: [],
          created_at: "2026-08-18T09:00:00Z",
          decided_at: "2026-08-20T09:00:00Z",
        },
      }),
    );
    fireEvent.click(screen.getAllByRole("button")[0]!);
    expect(
      screen.queryByRole("button", { name: "Entscheidung korrigieren" }),
    ).not.toBeInTheDocument();
  });

  it("zeigt die Begründung des Backends wörtlich, wenn eine Korrektur nicht geht", async () => {
    mockCorrect.mockRejectedValue(
      new Error(
        "Diese Entscheidung lässt sich nicht zurücknehmen, weil der frühere Wochenplan nicht gespeichert ist.",
      ),
    );
    renderList(decidedItem({ can_correct: true }));

    fireEvent.click(screen.getAllByRole("button")[0]!);
    fireEvent.click(
      screen.getByRole("button", { name: "Entscheidung korrigieren" }),
    );
    fireEvent.change(screen.getByLabelText(/Warum korrigieren Sie\?/), {
      target: { value: "Falsch entschieden" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Korrektur speichern" }),
    );

    expect(
      await screen.findByText(
        "Diese Entscheidung lässt sich nicht zurücknehmen, weil der frühere Wochenplan nicht gespeichert ist.",
      ),
    ).toBeVisible();
  });

  it("verlangt einen Grund und schickt die Fassung mit", async () => {
    renderList(decidedItem({ can_correct: true }));

    // Die Zeile aufklappen; die Korrektur steht bewusst nicht im Umschalter.
    fireEvent.click(screen.getAllByRole("button")[0]!);
    fireEvent.click(
      screen.getByRole("button", { name: "Entscheidung korrigieren" }),
    );

    expect(
      screen.getByText(/Bisher: Freigegeben am 20\.08\.2026 von Frau Berg/),
    ).toBeVisible();
    // Zweimal: in der aufgeklappten Zeile und im Korrektur-Dialog.
    expect(screen.getAllByText("„Attest lag vor“").length).toBe(2);

    fireEvent.click(
      screen.getByRole("button", { name: "Korrektur speichern" }),
    );
    expect(
      screen.getByText("Bitte tragen Sie ein, warum Sie korrigieren."),
    ).toBeVisible();
    expect(mockCorrect).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText(/Warum korrigieren Sie\?/), {
      target: { value: "Falsches Kind" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Korrektur speichern" }),
    );

    await waitFor(() =>
      expect(mockCorrect).toHaveBeenCalledWith("excused", "1", {
        approve: false,
        reason: "Falsches Kind",
        expectedVersion: "v9",
      }),
    );
  });
});
