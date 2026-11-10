// #6593: Solange die Vertretung gespeichert wird, ist das Panel gesperrt; eine
// späte Antwort schließt kein Panel, das inzwischen einen anderen Block zeigt.
// Echter SlideOver.
import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import { useSWRAuth } from "~/lib/swr";
import type {
  ApplyDeviationsInput,
  EnrichedInstance,
} from "~/lib/timetable-types";
import { catalogText } from "~/test/error-catalog-text";
import {
  deferred,
  expectSlideOverSaveLocked,
  expectSlideOverUnlocked,
} from "~/test/helpers/form-modal-pending";

vi.mock("vaul", () => import("~/test/mocks/vaul"));

vi.mock("~/lib/swr", () => ({ useSWRAuth: vi.fn() }));

vi.mock("~/lib/timetable-api", () => ({
  timetableService: { getDeviationHistory: vi.fn() },
}));

import { SubstitutionSlideOver } from "./substitution-slide-over";

// Weit in der Zukunft: der Block bleibt bearbeitbar.
const FUTURE_DATE = "2099-09-21";

function block(id: string, title: string): EnrichedInstance {
  return {
    id,
    date: FUTURE_DATE,
    startTime: "14:00",
    endTime: "15:00",
    title,
    status: "planned",
    activityGroupId: "7",
    activityType: "activity",
    roomId: "1",
    roomName: "Atelier",
    staff: [
      { staffId: "11", isPrimary: true, isAbsent: false, isSubstitute: false },
    ],
    studentIds: [],
    students: [],
  } as unknown as EnrichedInstance;
}

const STAFF_NAMES = new Map([
  ["11", "Anna Alt"],
  ["12", "Bernd Neu"],
]);

function renderAndSave() {
  const answer = deferred();
  const onApply = vi.fn((_input: ApplyDeviationsInput) => answer.promise);
  const onClose = vi.fn();
  const panel = (instance: EnrichedInstance) => (
    <SubstitutionSlideOver
      instance={instance}
      dayInstances={[instance]}
      staffOptions={[{ id: "12", name: "Bernd Neu" }]}
      staffNames={STAFF_NAMES}
      canManage
      onClose={onClose}
      onApply={onApply}
    />
  );
  const view = render(panel(block("42", "Malen-AG")));
  fireEvent.click(screen.getByRole("button", { name: /als abwesend/ }));
  fireEvent.click(
    screen.getByRole("radio", { name: /^Alle noch offenen Termine/ }),
  );
  const saveButton = screen.getByRole("button", { name: "Speichern" });
  expect(saveButton).toBeEnabled();
  fireEvent.click(saveButton);
  expect(onApply).toHaveBeenCalledOnce();
  return {
    answer,
    onClose,
    switchBlock: () => view.rerender(panel(block("43", "Lernzeit"))),
  };
}

describe("SubstitutionSlideOver pending lock (#6593)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useSWRAuth).mockReturnValue({
      data: { events: [] },
      isLoading: false,
      error: undefined,
      mutate: vi.fn(),
      isValidating: false,
    } as unknown as ReturnType<typeof useSWRAuth>);
  });

  it("sperrt das Panel beim Speichern und gibt es nach einem Fehler mit allen Eingaben frei", async () => {
    const { answer, onClose } = renderAndSave();

    expectSlideOverSaveLocked("Malen-AG", ["Abbrechen", "Speichere …"]);

    await act(async () => {
      answer.reject(
        new ApiError("conflict", 409, {
          code: "timetable.staff_absent_on_date",
        }),
      );
    });

    expect(await screen.findByRole("alert")).toHaveTextContent(
      catalogText("timetable.staff_absent_on_date", "die Vertretung"),
    );
    expectSlideOverUnlocked("Malen-AG");
    expect(
      screen.getByRole("radio", { name: /^Alle noch offenen Termine/ }),
    ).toBeChecked();
    expect(onClose).not.toHaveBeenCalled();
  });

  it("schließt das Panel nach dem Speichern", async () => {
    const { answer, onClose } = renderAndSave();

    await act(async () => {
      answer.resolve();
    });

    expect(onClose).toHaveBeenCalledOnce();
  });

  it("schließt nach einer späten Antwort nicht das Panel eines anderen Blocks", async () => {
    const { answer, onClose, switchBlock } = renderAndSave();

    switchBlock();
    expectSlideOverUnlocked("Lernzeit");
    await act(async () => {
      answer.resolve();
    });

    expect(onClose).not.toHaveBeenCalled();
    expect(
      screen.getByRole("heading", { name: "Lernzeit" }),
    ).toBeInTheDocument();
  });

  it("zeigt einen späten Fehler nicht im Panel eines anderen Blocks", async () => {
    const { answer, switchBlock } = renderAndSave();

    switchBlock();
    await act(async () => {
      answer.reject(new ApiError("down", 503, { code: "general.unavailable" }));
    });

    expect(
      screen.getByRole("heading", { name: "Lernzeit" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });
});
