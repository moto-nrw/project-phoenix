// #6593: Während „Betreuung beenden“ läuft, bleibt das Panel zu; eine späte
// Antwort schließt keinen neueren Vorgang. Echter SlideOver, vaul-Mock.
import { act, fireEvent, render, screen } from "@testing-library/react";
import type { ReactElement } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ModalProvider } from "~/components/ui/modal-context";
import { ApiError } from "~/lib/api-error";
import type { CareExitPreview } from "~/lib/care-exit-api";
import {
  deferred,
  expectSlideOverSaveLocked,
  expectSlideOverUnlocked,
} from "~/test/helpers/form-modal-pending";

vi.mock("vaul", () => import("~/test/mocks/vaul"));

const { mockPreview, mockConfirm } = vi.hoisted(() => ({
  mockPreview: vi.fn(),
  mockConfirm: vi.fn(),
}));

vi.mock("~/lib/care-exit-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/care-exit-api")>()),
  previewCareExit: mockPreview,
  confirmCareExit: mockConfirm,
}));

import { CareExitModal } from "./care-exit-modal";

const TITLE = "Betreuung beenden";
const LOCKED_FOOTER = ["Zurück", "Wird beendet…"] as const;

const PREVIEW: CareExitPreview = {
  token: "token-1",
  lastCareDay: "2026-09-30",
  reason: "moved_away",
  reasonNote: "",
  blocked: false,
  students: [
    {
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
    },
  ],
};

async function renderAndConfirm() {
  const answer = deferred<unknown>();
  mockConfirm.mockReturnValue(answer.promise);
  const onClose = vi.fn();
  const onFinished = vi.fn();
  const modal = (isOpen: boolean): ReactElement => (
    <ModalProvider>
      <CareExitModal
        isOpen={isOpen}
        studentIds={["1"]}
        onClose={onClose}
        onFinished={onFinished}
      />
    </ModalProvider>
  );
  const view = render(modal(true));
  fireEvent.click(screen.getByRole("combobox", { name: "Grund" }));
  fireEvent.click(screen.getByRole("option", { name: "Umzug" }));
  fireEvent.click(screen.getByRole("button", { name: "Weiter" }));
  const confirm = await screen.findByRole("button", {
    name: /Betreuung beenden/,
  });
  expect(confirm).toBeEnabled();
  fireEvent.click(confirm);
  return { ...view, answer, onClose, onFinished, modal };
}

describe("CareExitModal pending lock (#6593)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockPreview.mockResolvedValue(PREVIEW);
  });

  it("keeps the panel shut while ending care and unlocks it after a refusal", async () => {
    const { answer, onClose, onFinished } = await renderAndConfirm();

    expectSlideOverSaveLocked(TITLE, LOCKED_FOOTER);
    expect(onClose).not.toHaveBeenCalled();

    await act(async () => {
      answer.reject(
        new ApiError("care exit preview changed", 409, {
          code: "general.business_rejection",
        }),
      );
    });

    expect(
      await screen.findByText(
        "Das Betreuungsende konnte nicht geändert werden. Bitte prüfen Sie den aktuellen Stand.",
      ),
    ).toBeVisible();
    expectSlideOverUnlocked(TITLE);
    expect(screen.getByText("Mia Muster")).toBeVisible();
    expect(onClose).not.toHaveBeenCalled();
    expect(onFinished).not.toHaveBeenCalled();

    // Die Angaben aus Schritt 1 sind noch da.
    fireEvent.click(screen.getByRole("button", { name: "Zurück" }));
    expect(screen.getByRole("combobox", { name: "Grund" })).toHaveTextContent(
      "Umzug",
    );
  });

  it("hands the finished exit to the caller on success", async () => {
    const { answer, onFinished } = await renderAndConfirm();

    await act(async () => {
      answer.resolve({ studentsEnded: 1, rosterRowsRemoved: 0 });
    });

    expect(onFinished).toHaveBeenCalledOnce();
  });

  it("does not finish a reopened panel when the old answer arrives late", async () => {
    const { answer, onClose, onFinished, rerender, modal } =
      await renderAndConfirm();

    rerender(modal(false));
    rerender(modal(true));
    expectSlideOverUnlocked(TITLE);

    await act(async () => {
      answer.resolve({ studentsEnded: 1, rosterRowsRemoved: 0 });
    });

    expect(onFinished).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Weiter" })).toBeVisible();
  });

  it("neither reloads nor reports a refusal that arrives after unmount", async () => {
    const { answer, unmount } = await renderAndConfirm();
    expect(mockPreview).toHaveBeenCalledOnce();

    unmount();
    await act(async () => {
      answer.reject(new ApiError("gone", 409, { code: "general.conflict" }));
    });

    expect(mockPreview).toHaveBeenCalledOnce();
  });

  // #6594: „Weiter“ lädt die Vorschau als Vorgang des Panels. Eine späte
  // Antwort nach Schließen und Wiederöffnen springt nicht auf Schritt 2.
  describe("Vorschau (Weiter)", () => {
    async function renderAndContinue() {
      const answer = deferred<CareExitPreview>();
      mockPreview.mockReturnValue(answer.promise);
      const modal = (isOpen: boolean): ReactElement => (
        <ModalProvider>
          <CareExitModal
            isOpen={isOpen}
            studentIds={["1"]}
            onClose={vi.fn()}
            onFinished={vi.fn()}
          />
        </ModalProvider>
      );
      const view = render(modal(true));
      fireEvent.click(screen.getByRole("combobox", { name: "Grund" }));
      fireEvent.click(screen.getByRole("option", { name: "Umzug" }));
      fireEvent.click(screen.getByRole("button", { name: "Weiter" }));
      return { ...view, answer, modal };
    }

    it("sperrt das Panel, solange die Vorschau geprüft wird", async () => {
      const { answer } = await renderAndContinue();

      expectSlideOverSaveLocked(TITLE, ["Abbrechen", "Wird geprüft…"]);

      await act(async () => {
        answer.resolve(PREVIEW);
      });
      expect(await screen.findByText("Mia Muster")).toBeVisible();
      expectSlideOverUnlocked(TITLE);
    });

    it("springt nach Schließen und Wiederöffnen nicht durch eine späte Vorschau auf Schritt 2", async () => {
      const { answer, rerender, modal } = await renderAndContinue();

      rerender(modal(false));
      rerender(modal(true));
      expectSlideOverUnlocked(TITLE);

      await act(async () => {
        answer.resolve(PREVIEW);
      });

      // Anker: das wiedergeöffnete Panel steht frisch auf Schritt 1.
      expect(
        screen.getByText("Schritt 1 von 2: Tag und Grund festlegen."),
      ).toBeVisible();
      expect(screen.getByRole("button", { name: "Weiter" })).toBeVisible();
      expect(screen.queryByText("Mia Muster")).not.toBeInTheDocument();
    });

    it("zeigt eine späte Ablehnung der Vorschau nicht im wiedergeöffneten Panel", async () => {
      const { answer, rerender, modal } = await renderAndContinue();

      rerender(modal(false));
      rerender(modal(true));

      await act(async () => {
        answer.reject(
          new ApiError("down", 503, { code: "general.unavailable" }),
        );
      });

      // Anker: das wiedergeöffnete Panel steht frisch auf Schritt 1.
      expect(
        screen.getByText("Schritt 1 von 2: Tag und Grund festlegen."),
      ).toBeVisible();
      expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    });
  });
});
