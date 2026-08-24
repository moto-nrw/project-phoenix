import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ModalProvider } from "~/components/dashboard/modal-context";
import type { CareExitImpact, CareExitPreview } from "~/lib/care-exit-api";
import { CareExitModal } from "./care-exit-modal";

const {
  mockPreview,
  mockConfirm,
  mockWithdrawalPreview,
  mockWithdrawalConfirm,
} = vi.hoisted(() => ({
  mockPreview: vi.fn(),
  mockConfirm: vi.fn(),
  mockWithdrawalPreview: vi.fn(),
  mockWithdrawalConfirm: vi.fn(),
}));

vi.mock("~/lib/care-exit-api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("~/lib/care-exit-api")>();
  return {
    ...actual,
    previewCareExit: mockPreview,
    confirmCareExit: mockConfirm,
    previewWithdrawalCareEnd: mockWithdrawalPreview,
    confirmWithdrawalCareEnd: mockWithdrawalConfirm,
  };
});

function impact(overrides: Partial<CareExitImpact> = {}): CareExitImpact {
  return {
    studentId: "1",
    firstName: "Mia",
    lastName: "Muster",
    schoolClass: "2a",
    plannedRosterRows: 0,
    activityBookings: 0,
    openParentRequests: 0,
    hasRfidTag: false,
    currentlyPresent: false,
    sourceOfferings: [],
    weeklyPlans: [],
    plannedEndsOn: null,
    blocker: "",
    ...overrides,
  };
}

function preview(overrides: Partial<CareExitPreview> = {}): CareExitPreview {
  return {
    token: "token-1",
    lastCareDay: "2026-09-30",
    reason: "moved_away",
    reasonNote: "",
    blocked: false,
    students: [impact()],
    ...overrides,
  };
}

function renderModal(
  studentIds = ["1"],
  onFinished = vi.fn(),
  plannedLastCareDay?: string,
) {
  const onClose = vi.fn();
  render(
    <ModalProvider>
      <CareExitModal
        isOpen
        studentIds={studentIds}
        plannedLastCareDay={plannedLastCareDay}
        onClose={onClose}
        onFinished={onFinished}
      />
    </ModalProvider>,
  );
  return { onClose, onFinished };
}

function renderWithdrawalModal(onFinished = vi.fn()) {
  const onClose = vi.fn();
  render(
    <ModalProvider>
      <CareExitModal
        isOpen
        studentIds={["1"]}
        completionId="completion-1"
        firstBookinglessDay="2026-09-01"
        onClose={onClose}
        onFinished={onFinished}
      />
    </ModalProvider>,
  );
  return { onClose, onFinished };
}

function pickReason(label: string) {
  fireEvent.click(screen.getByRole("combobox", { name: "Grund" }));
  fireEvent.click(screen.getByRole("option", { name: label }));
}

describe("CareExitModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockPreview.mockResolvedValue(preview());
    mockConfirm.mockResolvedValue({
      studentsEnded: 1,
      rosterRowsRemoved: 0,
      bookingsEnded: 0,
    });
    mockWithdrawalPreview.mockResolvedValue(
      preview({ lastCareDay: "2026-08-31", reason: "no_care_needed" }),
    );
    mockWithdrawalConfirm.mockResolvedValue({
      studentsEnded: 1,
      rosterRowsRemoved: 0,
      bookingsEnded: 0,
    });
  });

  it("says that the last care day still counts", () => {
    renderModal();
    expect(
      screen.getByText(/Das Kind nimmt am letzten Betreuungstag noch teil/),
    ).toBeVisible();
  });

  it("requires a reason before the preview can be loaded", () => {
    renderModal();
    expect(screen.getByRole("button", { name: "Weiter" })).toBeDisabled();
    pickReason("Umzug");
    expect(screen.getByRole("button", { name: "Weiter" })).toBeEnabled();
  });

  it("asks for a short explanation only for 'Anderer Grund'", () => {
    renderModal();
    pickReason("Umzug");
    expect(screen.queryByLabelText("Kurze Erklärung")).toBeNull();

    pickReason("Anderer Grund");
    const note = screen.getByLabelText("Kurze Erklärung");
    expect(note).toBeVisible();
    expect(screen.getByRole("button", { name: "Weiter" })).toBeDisabled();

    fireEvent.change(note, { target: { value: "Wechsel in den Hort" } });
    expect(screen.getByRole("button", { name: "Weiter" })).toBeEnabled();
  });

  it("names every child and its consequences before confirming", async () => {
    mockPreview.mockResolvedValue(
      preview({
        students: [
          impact({
            plannedRosterRows: 3,
            activityBookings: 1,
            openParentRequests: 2,
            hasRfidTag: true,
          }),
        ],
      }),
    );
    renderModal();
    pickReason("Umzug");
    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));

    expect(await screen.findByText("Mia Muster")).toBeVisible();
    expect(
      screen.getByText("3 geplante Termine danach entfallen"),
    ).toBeVisible();
    expect(
      screen.getByText("1 Angebot endet am letzten Betreuungstag"),
    ).toBeVisible();
    expect(
      screen.getByText("2 offene Eltern-Anfragen werden geschlossen"),
    ).toBeVisible();
    expect(
      screen.getByText("Das Armband wird frei und kann neu vergeben werden"),
    ).toBeVisible();
  });

  it("names each blocked child and refuses to confirm", async () => {
    mockPreview.mockResolvedValue(
      preview({
        blocked: true,
        students: [
          impact({ studentId: "1", firstName: "Mia" }),
          impact({
            studentId: "2",
            firstName: "Ben",
            lastName: "Wirth",
            blocker: "Die Betreuung dieses Kindes ist bereits beendet.",
          }),
        ],
      }),
    );
    renderModal(["1", "2"]);
    pickReason("Umzug");
    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));

    expect(
      await screen.findByText("Die Betreuung wurde nicht beendet"),
    ).toBeVisible();
    expect(screen.getByText("Ben Wirth")).toBeVisible();
    expect(
      screen.getByText("Die Betreuung dieses Kindes ist bereits beendet."),
    ).toBeVisible();
    expect(
      screen.getByRole("button", { name: /Betreuung beenden/ }),
    ).toBeDisabled();
    expect(mockConfirm).not.toHaveBeenCalled();
  });

  it("confirms with exactly the token the preview handed out", async () => {
    const { onFinished } = renderModal();
    pickReason("Umzug");
    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));

    const confirmButton = await screen.findByRole("button", {
      name: /Betreuung beenden/,
    });
    fireEvent.click(confirmButton);

    await waitFor(() => {
      expect(mockConfirm).toHaveBeenCalledWith("token-1", {
        studentIds: ["1"],
        lastCareDay: expect.any(String),
        reason: "moved_away",
        reasonNote: "",
      });
    });
    await waitFor(() => expect(onFinished).toHaveBeenCalled());
  });

  it("shows the server's reason and reloads the preview after a refusal", async () => {
    mockConfirm.mockRejectedValue(
      new Error(
        "Die Betreuung wurde nicht beendet. Die Daten haben sich seit der Vorschau geändert.",
      ),
    );
    renderModal();
    pickReason("Umzug");
    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));

    fireEvent.click(
      await screen.findByRole("button", { name: /Betreuung beenden/ }),
    );

    expect(
      await screen.findByText(
        /Die Daten haben sich seit der Vorschau geändert/,
      ),
    ).toBeVisible();
    await waitFor(() => expect(mockPreview).toHaveBeenCalledTimes(2));
  });

  it("says that a selection shares one day and one reason", () => {
    renderModal(["1", "2", "3"]);
    expect(
      screen.getByRole("heading", { name: "Betreuung von 3 Kindern beenden" }),
    ).toBeVisible();
    expect(
      screen.getByText(
        "3 Kinder sind ausgewählt. Alle bekommen denselben Tag und denselben Grund.",
      ),
    ).toBeVisible();
  });

  // Ein geplantes Ende zu ändern darf den eingetragenen Tag nicht stillschweigend
  // auf heute zurücksetzen — wer nur den Grund korrigiert, würde die Betreuung
  // sonst um Wochen vorziehen (#2487).
  it("keeps the recorded last care day when a planned exit is corrected", async () => {
    renderModal(["1"], vi.fn(), "2026-12-18");

    expect(
      screen.getByRole("heading", { name: "Ende der Betreuung ändern" }),
    ).toBeVisible();
    expect(screen.getByText("18.12.2026")).toBeVisible();

    pickReason("Umzug");
    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));

    await waitFor(() =>
      expect(mockPreview).toHaveBeenCalledWith(
        expect.objectContaining({ lastCareDay: "2026-12-18" }),
      ),
    );
  });

  it("starts a fresh exit on today, not on some earlier plan", () => {
    renderModal();
    expect(
      screen.getByRole("heading", { name: "Betreuung beenden" }),
    ).toBeVisible();
  });

  it("uses the existing exit flow for an authoritative complete withdrawal", async () => {
    const { onFinished } = renderWithdrawalModal();

    expect(screen.getByText("31.08.2026")).toBeVisible();
    expect(
      screen.getByText(/Der letzte Betreuungstag ist am 31\.08\.2026/),
    ).toBeVisible();
    expect(screen.getByRole("button", { name: "Weiter" })).toBeEnabled();
    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));

    await waitFor(() =>
      expect(mockWithdrawalPreview).toHaveBeenCalledWith("completion-1", {
        studentIds: ["1"],
        lastCareDay: "2026-08-31",
        reason: "no_care_needed",
        reasonNote: "",
      }),
    );
    fireEvent.click(
      await screen.findByRole("button", { name: /Betreuung beenden/ }),
    );

    await waitFor(() =>
      expect(mockWithdrawalConfirm).toHaveBeenCalledWith(
        "completion-1",
        "token-1",
        expect.objectContaining({
          lastCareDay: "2026-08-31",
          reason: "no_care_needed",
        }),
      ),
    );
    expect(mockConfirm).not.toHaveBeenCalled();
    await waitFor(() => expect(onFinished).toHaveBeenCalled());
  });

  it("shows source offerings and the full exit consequences", async () => {
    mockWithdrawalPreview.mockResolvedValue(
      preview({
        lastCareDay: "2026-08-31",
        reason: "no_care_needed",
        students: [
          impact({
            sourceOfferings: [{ name: "OGS", days: ["mon", "wed", "fri"] }],
            weeklyPlans: [
              "Ankunft am Montag: 08:00",
              "Abholung am Montag: 15:00",
            ],
          }),
        ],
      }),
    );
    renderWithdrawalModal();
    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));

    expect(
      await screen.findByText("Gebuchte Angebote: OGS (Mo, Mi, Fr)"),
    ).toBeVisible();
    expect(
      screen.getByText(
        "Wöchentliche Zeiten: Ankunft am Montag: 08:00; Abholung am Montag: 15:00",
      ),
    ).toBeVisible();
    expect(screen.getByText("Folgen des Austritts")).toBeVisible();
    expect(
      screen.getByText(/Vergangene Anwesenheit.*bleiben erhalten/),
    ).toBeVisible();
  });
});
