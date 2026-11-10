// #6593: Lebenszyklus, Löschen und Anwesenheit sperren das Terminpanel, bis
// der Aufrufer antwortet; eine späte Antwort lässt im Panel eines anderen
// Termins nichts belegt. Echter SlideOver und echte Rückfragen.
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import type { EnrichedInstance } from "~/lib/timetable-types";
import { catalogText } from "~/test/error-catalog-text";
import {
  deferred,
  expectSlideOverSaveLocked,
  expectSlideOverUnlocked,
} from "~/test/helpers/form-modal-pending";

vi.mock("vaul", () => import("~/test/mocks/vaul"));

import { InstanceDetailModal } from "./instance-detail-modal";

function instance(overrides: Partial<EnrichedInstance> = {}): EnrichedInstance {
  return {
    id: "42",
    date: "2026-05-04",
    startTime: "12:00",
    endTime: "13:00",
    title: "Mensa",
    status: "active",
    isSpontaneous: false,
    isLive: true,
    activityType: "activity",
    roomId: "3",
    roomName: "Raum 3",
    staff: [],
    students: [{ studentId: "21", status: "expected" }],
    studentIds: ["21"],
    staffCount: 0,
    absentStaffCount: 0,
    expectedStudentsCount: 1,
    notScheduledStudentsCount: 0,
    presentStudentsCount: 0,
    requiredStaffCount: 0,
    assignedStaffCount: 0,
    canComplete: true,
    conflictWarnings: [],
    ...overrides,
  };
}

const OTHER = instance({ id: "43", title: "Lernzeit" });

/** The kit modal stacked last: the open confirmation. */
function confirmDialogButton(name: string) {
  const dialogs = screen.getAllByRole("dialog");
  return within(dialogs[dialogs.length - 1]!).getByRole("button", { name });
}

function renderPanel(
  target: EnrichedInstance,
  props: Partial<React.ComponentProps<typeof InstanceDetailModal>> = {},
) {
  const onClose = vi.fn();
  const onLifecycleAction = vi.fn();
  const panel = (shown: EnrichedInstance) => (
    <InstanceDetailModal
      instance={shown}
      onClose={onClose}
      onLifecycleAction={onLifecycleAction}
      {...props}
    />
  );
  const view = render(panel(target));
  return {
    onClose,
    onLifecycleAction,
    switchTo: (next: EnrichedInstance) => view.rerender(panel(next)),
  };
}

describe("InstanceDetailModal pending lock (#6593)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("Lebenszyklus aus der Rückfrage", () => {
    function renderAndComplete() {
      const answer = deferred();
      const view = renderPanel(instance());
      view.onLifecycleAction.mockReturnValue(answer.promise);
      fireEvent.click(screen.getByRole("button", { name: "Beenden" }));
      fireEvent.click(confirmDialogButton("Beenden"));
      expect(view.onLifecycleAction).toHaveBeenCalledWith("complete");
      return { ...view, answer };
    }

    it("sperrt das Panel, solange beendet wird, und gibt es nach einem Fehler frei", async () => {
      const { answer, onClose } = renderAndComplete();

      expectSlideOverSaveLocked("Mensa", ["Beende …", "Absagen"]);

      // Der Aufrufer hat den Fehler schon als Toast gemeldet.
      await act(async () => {
        answer.reject(new Error("bereits gemeldet"));
      });

      expectSlideOverUnlocked("Mensa");
      expect(screen.getByRole("button", { name: "Beenden" })).toBeEnabled();
      expect(onClose).not.toHaveBeenCalled();
    });

    it("gibt das Panel nach dem Beenden frei", async () => {
      const { answer, onClose } = renderAndComplete();

      await act(async () => {
        answer.resolve();
      });

      expectSlideOverUnlocked("Mensa");
      expect(screen.queryByText("Termin beenden?")).not.toBeInTheDocument();
      expect(onClose).not.toHaveBeenCalled();
    });

    it("lässt das Panel eines anderen Termins frei, während die alte Antwort aussteht", async () => {
      const { answer, switchTo } = renderAndComplete();

      switchTo(OTHER);
      expectSlideOverUnlocked("Lernzeit");
      expect(screen.getByRole("button", { name: "Beenden" })).toBeEnabled();

      await act(async () => {
        answer.resolve();
      });
      expectSlideOverUnlocked("Lernzeit");
    });
  });

  describe("Löschen aus dem Löschdialog", () => {
    function renderAndDelete() {
      const answer = deferred();
      const onDeleteCancelled = vi.fn(() => answer.promise);
      const view = renderPanel(instance({ status: "cancelled" }), {
        onDeleteCancelled,
      });
      fireEvent.click(screen.getByRole("button", { name: "Löschen" }));
      fireEvent.click(confirmDialogButton("Löschen"));
      fireEvent.click(confirmDialogButton("Endgültig löschen"));
      expect(onDeleteCancelled).toHaveBeenCalledOnce();
      return { ...view, answer };
    }

    it("hält den Dialog bei einem Fehler offen und gibt das Panel danach frei", async () => {
      const { answer } = renderAndDelete();
      const dialog = screen.getByRole("dialog", {
        name: "Abgesagten Termin löschen",
      });
      expect(
        within(dialog).getByRole("button", { name: "Wird gelöscht…" }),
      ).toBeDisabled();

      await act(async () => {
        answer.reject(
          new ApiError("conflict", 409, {
            code: "timetable.invalid_transition",
          }),
        );
      });

      expect(
        await within(dialog).findByText(
          catalogText(
            "timetable.invalid_transition",
            "das Löschen des Termins",
          ),
        ),
      ).toBeInTheDocument();
      fireEvent.click(
        within(dialog).getByRole("button", { name: "Abbrechen" }),
      );
      await waitFor(() =>
        expect(
          screen.queryByRole("dialog", { name: "Abgesagten Termin löschen" }),
        ).not.toBeInTheDocument(),
      );
      expectSlideOverUnlocked("Mensa");
    });

    it("schließt den Dialog nach dem Löschen", async () => {
      const { answer } = renderAndDelete();

      await act(async () => {
        answer.resolve();
      });

      await waitFor(() =>
        expect(
          screen.queryByRole("dialog", { name: "Abgesagten Termin löschen" }),
        ).not.toBeInTheDocument(),
      );
      expectSlideOverUnlocked("Mensa");
    });

    it("zeigt einen späten Fehler nicht im Löschdialog eines anderen Termins", async () => {
      const { answer, switchTo } = renderAndDelete();

      switchTo({ ...OTHER, status: "cancelled" });
      fireEvent.click(screen.getByRole("button", { name: "Löschen" }));
      const dialog = screen.getByRole("dialog", {
        name: "Abgesagten Termin löschen",
      });
      expect(
        within(dialog).getByRole("button", { name: "Löschen" }),
      ).toBeEnabled();

      await act(async () => {
        answer.reject(new ApiError("db down", 500, { code: "general.server" }));
      });

      expect(
        within(dialog).getByRole("button", { name: "Löschen" }),
      ).toBeEnabled();
      expect(within(dialog).queryByRole("alert")).not.toBeInTheDocument();
    });
  });

  describe("Anwesenheit im Panel", () => {
    function renderAndPatch() {
      const answer = deferred();
      const onAttendancePatch = vi.fn(() => answer.promise);
      const view = renderPanel(instance(), { onAttendancePatch });
      fireEvent.click(
        screen.getByRole("button", { name: "Kind #21 als anwesend markieren" }),
      );
      expect(onAttendancePatch).toHaveBeenCalledWith("42", "21", {
        status: "present",
        substatus: null,
      });
      return { ...view, answer };
    }

    it("sperrt das Panel, solange die Anwesenheit gespeichert wird", async () => {
      const { answer } = renderAndPatch();

      expectSlideOverSaveLocked("Mensa", ["Beenden", "Absagen"]);

      await act(async () => {
        answer.resolve();
      });

      expectSlideOverUnlocked("Mensa");
      expect(
        screen.getByRole("button", { name: "Kind #21 als anwesend markieren" }),
      ).toBeEnabled();
    });

    it("lässt die Anwesenheit eines anderen Termins frei, während die alte Antwort aussteht", () => {
      const { switchTo } = renderAndPatch();

      switchTo(OTHER);

      expectSlideOverUnlocked("Lernzeit");
      expect(
        screen.getByRole("button", { name: "Kind #21 als anwesend markieren" }),
      ).toBeEnabled();
    });
  });
});
