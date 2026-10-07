import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { RequestDecisionRow } from "./request-decision-row";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type { AggregatedOpenRequest } from "~/lib/change-request-list-api";
import {
  ChangeRequestStaleError,
  markRequestDone,
} from "~/lib/change-request-list-api";

vi.mock("~/lib/change-request-list-api", async () => {
  const actual = await vi.importActual<
    typeof import("~/lib/change-request-list-api")
  >("~/lib/change-request-list-api");
  return { ...actual, markRequestDone: vi.fn() };
});

vi.mock("~/components/students/excused-request-review-item", () => ({
  ExcusedRequestReviewItem: ({
    row,
    decisionDisabledReason,
  }: {
    row: { id: string };
    decisionDisabledReason?: string;
  }) => (
    <div>
      excused-item-{row.id}
      {decisionDisabledReason ? <p>{decisionDisabledReason}</p> : null}
    </div>
  ),
}));

const mockMarkDone = vi.mocked(markRequestDone);

function item(overrides: Record<string, unknown> = {}): AggregatedOpenRequest {
  return {
    request_type: "excused",
    occurred_at: "2026-08-29T09:00:00Z",
    student_id: "10",
    student_name: "Mia Muster",
    expected_version: "v1",
    urgent_today: false,
    bulk_eligible: true,
    family_protected: false,
    data: { id: "1", dates: ["2026-08-01"], absence_status: "sick" },
    ...overrides,
  } as never;
}

function renderRow(
  request: AggregatedOpenRequest,
  handlers: Partial<{
    onDecided: (key: string, notice: string) => void;
    onStale: () => void;
    inConflict: boolean;
  }> = {},
) {
  return render(
    <ToastProvider>
      <RequestDecisionRow
        request={request}
        selected={false}
        position={1}
        total={1}
        inConflict={handlers.inConflict ?? false}
        onSelectionChange={vi.fn()}
        onDecided={handlers.onDecided ?? vi.fn()}
        onStale={handlers.onStale ?? vi.fn()}
      />
    </ToastProvider>,
  );
}

describe("RequestDecisionRow", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockMarkDone.mockResolvedValue(undefined);
  });

  it("nennt den Grund sichtbar, warum nur einzeln entschieden werden kann", () => {
    renderRow(
      item({
        bulk_eligible: false,
        bulk_ineligible_reason: "conflict",
      }),
    );

    expect(
      screen.getByText(
        "Nur einzeln entscheiden: Diese Anfrage widerspricht einer anderen Anfrage.",
      ),
    ).toBeVisible();
    expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
  });

  it("nennt den Namen im Namen der Auswahl", () => {
    renderRow(item());
    expect(
      screen.getByRole("checkbox", { name: "Gemeinsam freigeben: Mia Muster" }),
    ).toBeVisible();
  });

  it("warnt, wenn die OGS den Wert nach der Anfrage geändert hat", () => {
    renderRow(item({ current_value_changed: true }));
    expect(
      screen.getByText(
        "Die OGS hat diesen Wert nach der Anfrage geändert. Prüfen Sie, welcher Wert jetzt gelten soll.",
      ),
    ).toBeVisible();
  });

  it("bietet bei einer abgelaufenen Anfrage den Abschluss statt der Freigabe an", async () => {
    const onDecided = vi.fn();
    renderRow(item({ past: true, bulk_eligible: false }), { onDecided });

    expect(
      screen.getByText(
        "Diese Anfrage betrifft nur vergangene Tage. Sie ändert nichts mehr.",
      ),
    ).toBeVisible();
    fireEvent.click(
      screen.getByRole("button", { name: "Als erledigt markieren" }),
    );

    await waitFor(() =>
      expect(mockMarkDone).toHaveBeenCalledWith("excused", "1", "v1"),
    );
    await waitFor(() =>
      expect(onDecided).toHaveBeenCalledWith(
        "excused:1",
        "Die Anfrage wurde abgeschlossen.",
      ),
    );
  });

  it("meldet ein gescheitertes Abschließen als Toast mit Wiederholen", async () => {
    mockMarkDone.mockRejectedValueOnce(
      new ApiError("boom", 500, { code: "general.server", instance: "req-1" }),
    );
    renderRow(item({ past: true, bulk_eligible: false }));

    fireEvent.click(
      screen.getByRole("button", { name: "Als erledigt markieren" }),
    );

    const toast = await screen.findByRole("alert", { name: /^Fehler:/ });
    expect(toast).toHaveTextContent(
      "Die Anfrage konnte nicht bearbeitet werden. Bitte versuchen Sie es später erneut.",
    );
    mockMarkDone.mockResolvedValueOnce(undefined);
    fireEvent.click(screen.getByRole("button", { name: /Wiederholen/ }));
    await waitFor(() => expect(mockMarkDone).toHaveBeenCalledTimes(2));
  });

  it("lädt bei einer veralteten Anfrage neu und meldet den Code-Text", async () => {
    const onStale = vi.fn();
    mockMarkDone.mockRejectedValueOnce(new ChangeRequestStaleError());
    renderRow(item({ past: true, bulk_eligible: false }), { onStale });

    fireEvent.click(
      screen.getByRole("button", { name: "Als erledigt markieren" }),
    );

    await waitFor(() => expect(onStale).toHaveBeenCalledOnce());
    expect(
      await screen.findByRole("alert", { name: /^Fehler:/ }),
    ).toHaveTextContent(/wurde inzwischen geändert/);
    expect(
      screen.queryByRole("button", { name: /Wiederholen/ }),
    ).not.toBeInTheDocument();
  });

  it("sperrt die Einzel-Entscheidung, solange ein Widerspruch offen ist", () => {
    renderRow(item(), { inConflict: true });
    expect(
      screen.getByText(
        "Diese Anfragen widersprechen sich. Legen Sie oben ein Ergebnis fest.",
      ),
    ).toBeVisible();
  });
});
