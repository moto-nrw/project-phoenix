// #6593: Solange die Sammel-Vertretung speichert, ist das Panel gesperrt; eine
// späte Antwort schließt kein neu geöffnetes Panel. Echter SlideOver.
import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type {
  BulkSubstitutionResponse,
  EnrichedInstance,
  WeeklyInstancesResponse,
} from "~/lib/timetable-types";
import {
  deferred,
  expectSlideOverSaveLocked,
  expectSlideOverUnlocked,
} from "~/test/helpers/form-modal-pending";

vi.mock("vaul", () => import("~/test/mocks/vaul"));

vi.mock("~/components/ui/date-picker", async () =>
  (await import("~/test/mocks/date-picker")).datePickerModuleMock(),
);

const { mockGetWeek, mockApplyBulk, mockToastSuccess, mockClearPreviewCache } =
  vi.hoisted(() => ({
    mockGetWeek: vi.fn(),
    mockApplyBulk: vi.fn(),
    mockToastSuccess: vi.fn(),
    mockClearPreviewCache: vi.fn(() => Promise.resolve()),
  }));

vi.mock("~/lib/timetable-api", () => ({
  timetableService: { getWeek: mockGetWeek },
}));

vi.mock("~/lib/substitution-api", () => ({
  substitutionService: { applyBulkSubstitution: mockApplyBulk },
}));

// Nur die Toasts ersetzen; der Fehlerweg des Formulars bleibt echt.
vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({
    success: mockToastSuccess,
    error: vi.fn(),
    warning: vi.fn(),
  }),
}));

// Vorschau synchron aus mockGetWeek, wie in bulk-substitution-modal.test.tsx.
vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string | null) =>
    key === null
      ? { data: undefined, isLoading: false, error: undefined }
      : {
          data: mockGetWeek() as WeeklyInstancesResponse,
          isLoading: false,
          error: undefined,
          mutate: vi.fn(),
        },
  useTenantMutateMatching: () => mockClearPreviewCache,
}));

import { BulkSubstitutionModal } from "./bulk-substitution-modal";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

const TITLE = "Sammel-Vertretung";
const CLOSE = "Sammel-Vertretung schließen";
// Der Tag nach TEST_CLOCK_TODAY: vergangene Tage zeigt das Panel nicht.
const DAY = "2026-09-10";
const STAFF_OPTIONS = [
  { id: "11", name: "Anna Alt" },
  { id: "12", name: "Bernd Neu" },
];

function instance(): EnrichedInstance {
  return {
    id: "1",
    date: DAY,
    startTime: "14:00",
    endTime: "15:00",
    title: "Malen-AG",
    status: "planned",
    staff: [
      { staffId: "11", isPrimary: true, isAbsent: false, isSubstitute: false },
    ],
  } as unknown as EnrichedInstance;
}

function response(): BulkSubstitutionResponse {
  return {
    days: [{ date: DAY, affectedInstances: [], warningCount: 0 }],
    totalAffected: 1,
    warningCount: 0,
  };
}

function renderAndSave() {
  const answer = deferred<BulkSubstitutionResponse>();
  mockApplyBulk.mockReturnValue(answer.promise);
  const onClose = vi.fn();
  const onSaved = vi.fn();
  const panel = (isOpen: boolean) => (
    <BulkSubstitutionModal
      isOpen={isOpen}
      onClose={onClose}
      staffOptions={STAFF_OPTIONS}
      staffLoadError={false}
      onSaved={onSaved}
    />
  );
  const view = render(panel(true));
  fireEvent.click(screen.getByRole("combobox", { name: "Abwesende Person" }));
  fireEvent.click(screen.getByRole("option", { name: "Anna Alt" }));
  fireEvent.change(screen.getByLabelText("Von"), { target: { value: DAY } });
  fireEvent.change(screen.getByLabelText("Bis"), { target: { value: DAY } });
  fireEvent.change(screen.getByLabelText("Grund (optional)"), {
    target: { value: "Krankheit" },
  });
  const saveButton = screen.getByRole("button", {
    name: "Für 1 Tag(e) speichern",
  });
  expect(saveButton).toBeEnabled();
  fireEvent.click(saveButton);
  expect(mockApplyBulk).toHaveBeenCalledOnce();
  return {
    answer,
    onClose,
    onSaved,
    reopen: () => {
      view.rerender(panel(false));
      view.rerender(panel(true));
    },
  };
}

describe("BulkSubstitutionModal pending lock (#6593)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetWeek.mockReturnValue({
      instances: [instance()],
    } as unknown as WeeklyInstancesResponse);
  });

  it("sperrt das Panel beim Speichern und gibt es nach einem Fehler mit allen Eingaben frei", async () => {
    const { answer, onClose } = renderAndSave();

    expectSlideOverSaveLocked(TITLE, ["Abbrechen", "Speichert…"], CLOSE);

    await act(async () => {
      answer.reject(
        new ApiError("abwesend", 409, {
          code: "timetable.staff_absent_on_date",
        }),
      );
    });

    expect(await screen.findByRole("alert")).toHaveTextContent(
      catalogText("timetable.staff_absent_on_date", "die Sammel-Vertretung"),
    );
    expectSlideOverUnlocked(TITLE, CLOSE);
    expect(screen.getByLabelText("Grund (optional)")).toHaveValue("Krankheit");
    expect(onClose).not.toHaveBeenCalled();
  });

  it("schließt das Panel nach dem Speichern", async () => {
    const { answer, onClose, onSaved } = renderAndSave();

    await act(async () => {
      answer.resolve(response());
    });

    expect(onSaved).toHaveBeenCalledOnce();
    expect(onClose).toHaveBeenCalledOnce();
  });

  it("frischt nach einer späten Antwort die Daten auf, schließt das neu geöffnete Panel aber nicht", async () => {
    const { answer, onClose, onSaved, reopen } = renderAndSave();

    reopen();
    expectSlideOverUnlocked(TITLE, CLOSE);
    await act(async () => {
      answer.resolve(response());
    });

    expect(mockToastSuccess).toHaveBeenCalledOnce();
    expect(onSaved).toHaveBeenCalledOnce();
    expect(onClose).not.toHaveBeenCalled();
    expect(screen.getByRole("heading", { name: TITLE })).toBeInTheDocument();
  });

  it("zeigt einen späten Fehler nicht im neu geöffneten Panel", async () => {
    const { answer, reopen } = renderAndSave();

    reopen();
    await act(async () => {
      answer.reject(new ApiError("kaputt", 500, { code: "general.server" }));
    });

    // Anker: das neu geöffnete Panel ist da.
    expect(screen.getByRole("heading", { name: TITLE })).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });
});
