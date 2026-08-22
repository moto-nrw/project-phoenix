import { fireEvent, render, screen, waitFor } from "@testing-library/react";
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
}

import { CareRequestReviewItem } from "./care-request-review-item";
import {
  CareRequestApiError,
  decideCareScheduleChangeRequest,
  type StaffCareRequest,
} from "~/lib/care-request-review-api";

// Mock only the network function; keep the real CareRequestApiError so the
// component's `err instanceof CareRequestApiError` code-branch resolves against
// the actual class instead of an undefined stub.
vi.mock("~/lib/care-request-review-api", async (importActual) => {
  const actual =
    await importActual<typeof import("~/lib/care-request-review-api")>();
  return {
    ...actual,
    decideCareScheduleChangeRequest: vi.fn(),
  };
});

const mockDecide = vi.mocked(decideCareScheduleChangeRequest);

// Cards render collapsed (compact queue); expand via the header button so the
// diff and the Freigeben/Ablehnen actions render. The header button's
// accessible name contains the child name.
function expand() {
  fireEvent.click(screen.getByRole("button", { name: /Lara Beispiel/ }));
}

function row(overrides: Partial<StaffCareRequest> = {}): StaffCareRequest {
  return {
    id: "200",
    student_id: "42",
    first_name: "Lara",
    last_name: "Beispiel",
    status: "pending",
    request_kind: "weekly_schedule",
    affected_blocks: [],
    impact_available: true,
    impact_token: "impact-v1",
    diff: [
      {
        label: "Montag · Abholzeit",
        old: "—",
        new: "15:00",
        weekday: 1,
        care_kind: "pickup",
      },
      {
        label: "Montag · Abholart",
        old: "Fährt Bus / Wird abgeholt",
        new: "Geht alleine",
        weekday: 1,
        care_kind: "departure_mode",
        old_modes: ["bus", "pickup"],
        new_mode: "alone",
      },
    ],
    created_at: "2026-07-01T12:00:00Z",
    ...overrides,
  };
}

function pickupRow(
  overrides: Partial<StaffCareRequest> = {},
): StaffCareRequest {
  return row({
    request_kind: "pickup_change",
    request_reason: "Arzttermin",
    affected_blocks: [
      {
        id: "81",
        title: "Nachmittags-AG",
        start_time: "15:00",
        end_time: "16:00",
      },
    ],
    diff: [
      {
        label: "17.08.2026 · Abholzeit",
        old: "15:30",
        new: "14:30",
        care_kind: "pickup",
      },
    ],
    ...overrides,
  });
}

describe("CareRequestReviewItem", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockDecide.mockReset();
  });

  it("renders the weekly diff plus summary and approves without requiring a reason", async () => {
    mockDecide.mockResolvedValue(row({ status: "approved" }));
    const onDecided = vi.fn();

    render(<CareRequestReviewItem row={row()} onDecided={onDecided} />);

    // Collapsed summary: Geltungstag vorn, dahinter die Arten aus den
    // Diff-Labels.
    expect(
      screen.getByText("Montag · Abholzeit + Abholart"),
    ).toBeInTheDocument();
    expand();
    expect(screen.getByText("Montag · Abholzeit:")).toBeInTheDocument();
    expect(screen.getByText("15:00")).toBeInTheDocument();
    expect(screen.getByText("Geht alleine")).toBeInTheDocument();
    expect(screen.queryByText("Nach dem Freigeben")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Freigeben" }));

    await waitFor(() =>
      expect(mockDecide).toHaveBeenCalledWith(
        "200",
        true,
        undefined,
        "impact-v1",
      ),
    );
    expect(onDecided).toHaveBeenCalledWith("Betreuungszeiten übernommen");
  });

  it("preserves the request kind in an empty collapsed summary", () => {
    const { rerender } = render(
      <CareRequestReviewItem row={row({ diff: [] })} onDecided={vi.fn()} />,
    );
    expect(
      screen.getByRole("button", {
        name: /Betreuungszeiten\. Betreuungszeiten\./,
      }),
    ).toBeInTheDocument();

    rerender(
      <CareRequestReviewItem
        row={pickupRow({ diff: [] })}
        onDecided={vi.fn()}
      />,
    );
    expect(
      screen.getByRole("button", {
        name: /Einzelner Tag\. Abholzeit\./,
      }),
    ).toBeInTheDocument();
  });

  it("nennt bei einer Tages-Anfrage den Geltungstag und kennzeichnet sie als einzelnen Tag", () => {
    render(<CareRequestReviewItem row={pickupRow()} onDecided={vi.fn()} />);

    // Ohne Klick muss die Zeile sagen, dass die Änderung nur für diesen einen
    // Tag gilt — sonst liest sie sich wie eine dauerhafte Änderung (#2480).
    expect(screen.getByText("17.08.2026 · Abholzeit")).toBeInTheDocument();
    expect(screen.getByText("Einzelner Tag")).toBeInTheDocument();
    expect(screen.queryByText("Betreuungszeiten")).not.toBeInTheDocument();
  });

  it("fasst mehrere Wochentage einer dauerhaften Anfrage zu einer Zeile zusammen", () => {
    render(
      <CareRequestReviewItem
        row={row({
          diff: [
            { label: "Montag · Abholzeit", old: "—", new: "15:00" },
            { label: "Montag · Abholart", old: "—", new: "Geht alleine" },
            { label: "Dienstag · Abholzeit", old: "—", new: "15:00" },
          ],
        })}
        onDecided={vi.fn()}
      />,
    );

    expect(
      screen.getByText("Montag, Dienstag · Abholzeit + Abholart"),
    ).toBeInTheDocument();
    expect(screen.getByText("Betreuungszeiten")).toBeInTheDocument();
  });

  it("kürzt ab drei Wochentagen auf die Anzahl, damit die Änderungsart nicht abgeschnitten wird", () => {
    render(
      <CareRequestReviewItem
        row={row({
          diff: [
            { label: "Montag · Abholzeit", old: "—", new: "15:00" },
            { label: "Dienstag · Abholzeit", old: "—", new: "15:00" },
            { label: "Mittwoch · Abholzeit", old: "—", new: "15:00" },
            { label: "Donnerstag · Abholzeit", old: "—", new: "15:00" },
            { label: "Freitag · Abholart", old: "—", new: "Geht alleine" },
          ],
        })}
        onDecided={vi.fn()}
      />,
    );

    expect(
      screen.getByText("5 Wochentage · Abholzeit + Abholart"),
    ).toBeInTheDocument();
  });

  it("requires a reason before rejecting", async () => {
    mockDecide.mockResolvedValue(row({ status: "rejected" }));
    const onDecided = vi.fn();

    render(<CareRequestReviewItem row={row()} onDecided={onDecided} />);

    expand();
    fireEvent.click(screen.getByRole("button", { name: "Ablehnen" }));

    expect(
      await screen.findByText(
        "Für eine Ablehnung ist eine Begründung erforderlich.",
      ),
    ).toBeInTheDocument();
    expect(mockDecide).not.toHaveBeenCalled();
    expect(onDecided).not.toHaveBeenCalled();

    fireEvent.change(
      screen.getByPlaceholderText("Begründung (Pflicht bei Ablehnung)"),
      { target: { value: " zu kurzfristig " } },
    );
    fireEvent.click(screen.getByRole("button", { name: "Ablehnen" }));

    await waitFor(() =>
      expect(mockDecide).toHaveBeenCalledWith(
        "200",
        false,
        "zu kurzfristig",
        "impact-v1",
      ),
    );
    expect(onDecided).toHaveBeenCalledWith("Betreuungszeit-Anfrage abgelehnt");
  });

  it("shows the parent's mandatory reason for a pickup change and reports the pickup notice", async () => {
    mockDecide.mockResolvedValue(row({ status: "approved" }));
    const onDecided = vi.fn();

    render(<CareRequestReviewItem row={pickupRow()} onDecided={onDecided} />);

    expand();
    expect(screen.getByText("Grund der Eltern:")).toBeInTheDocument();
    expect(screen.getByText("Arzttermin")).toBeInTheDocument();
    expect(screen.getByText("Nach dem Freigeben")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Im Betreuungsplan wird das Kind von diesen Terminen abgemeldet:",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Nachmittags-AG, 15:00–16:00 Uhr"),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Freigeben" }));

    await waitFor(() =>
      expect(onDecided).toHaveBeenCalledWith("Abholzeit übernommen"),
    );
  });

  it("states when a pickup change removes no timetable blocks", () => {
    render(
      <CareRequestReviewItem
        row={pickupRow({ affected_blocks: [] })}
        onDecided={vi.fn()}
      />,
    );

    expand();
    expect(
      screen.getByText(
        "Das Kind bleibt im Betreuungsplan für alle Termine eingeplant.",
      ),
    ).toBeInTheDocument();
  });

  it("blocks approval when the affected blocks could not be loaded", async () => {
    render(
      <CareRequestReviewItem
        row={pickupRow({ impact_available: false, affected_blocks: [] })}
        onDecided={vi.fn()}
      />,
    );
    expand();
    fireEvent.click(screen.getByRole("button", { name: "Freigeben" }));
    expect(mockDecide).not.toHaveBeenCalled();
    await expectErrorToast(
      /Die Anfrage kann nicht freigegeben werden\. Bitte laden Sie die Seite neu\./,
    );
  });

  it("shows a generic decision error without calling onDecided", async () => {
    mockDecide.mockRejectedValueOnce(new Error("boom"));
    const onDecided = vi.fn();

    render(<CareRequestReviewItem row={row()} onDecided={onDecided} />);

    expand();
    fireEvent.click(screen.getByRole("button", { name: "Freigeben" }));

    await expectErrorToast(
      /Die Entscheidung konnte nicht gespeichert werden\./,
    );
    expect(onDecided).not.toHaveBeenCalled();
    expect(screen.getByText("Lara Beispiel")).toBeInTheDocument();
  });

  it("asks for a reload when the pickup impact changed", async () => {
    mockDecide.mockRejectedValueOnce(
      new CareRequestApiError("changed", "pickup_change_impact_changed"),
    );
    render(<CareRequestReviewItem row={pickupRow()} onDecided={vi.fn()} />);

    expand();
    fireEvent.click(screen.getByRole("button", { name: "Freigeben" }));

    await expectErrorToast(/Der Betreuungsplan hat sich geändert\./);
  });

  it("surfaces the recovery action when approval is blocked by messaging_disabled", async () => {
    mockDecide.mockRejectedValueOnce(
      new CareRequestApiError(
        "schedule: care request messaging disabled",
        "messaging_disabled",
      ),
    );
    const onDecided = vi.fn();

    render(<CareRequestReviewItem row={row()} onDecided={onDecided} />);

    expand();
    fireEvent.click(screen.getByRole("button", { name: "Freigeben" }));

    // The blocking reason must tell the reviewer to reject instead — not a
    // generic failure that leaves the request silently pending.
    await expectErrorToast(/Bitte die Anfrage stattdessen ablehnen\./);
    expect(onDecided).not.toHaveBeenCalled();
  });

  it("surfaces the recovery action when approval is blocked by pickup_change_conflict", async () => {
    mockDecide.mockRejectedValueOnce(
      new CareRequestApiError(
        "schedule: pickup change conflict",
        "pickup_change_conflict",
      ),
    );
    const onDecided = vi.fn();

    render(
      <CareRequestReviewItem
        row={row({ request_kind: "pickup_change" })}
        onDecided={onDecided}
      />,
    );

    expand();
    fireEvent.click(screen.getByRole("button", { name: "Freigeben" }));

    await expectErrorToast(
      /Für diesen Tag wurde inzwischen bereits eine Änderung durch die OGS eingetragen\./,
    );
    expect(onDecided).not.toHaveBeenCalled();
  });

  it("directs booking-managed care days to the offering change flow", async () => {
    mockDecide.mockRejectedValueOnce(
      new CareRequestApiError(
        "schedule: care day is managed by an offering booking",
        "care_day_managed_by_booking",
      ),
    );

    render(<CareRequestReviewItem row={row()} onDecided={vi.fn()} />);
    expand();
    fireEvent.click(screen.getByRole("button", { name: "Freigeben" }));

    expect(
      await screen.findByText(
        "Dieser Betreuungstag gehört zu einem gebuchten Angebot. Ändern Sie zuerst die Buchung des Kindes. Lehnen Sie diese Anfrage danach ab.",
      ),
    ).toBeInTheDocument();
  });
});
