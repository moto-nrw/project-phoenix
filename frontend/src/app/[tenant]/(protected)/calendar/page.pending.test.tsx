// #6593: Während ein Termin gespeichert wird, lässt sich das Panel nicht
// schließen; eine späte Antwort schließt kein neu geöffnetes Panel.
import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { setTestClock } from "~/test/clock";
import {
  deferred,
  expectSlideOverSaveLocked,
  expectSlideOverUnlocked,
} from "~/test/helpers/form-modal-pending";

vi.mock("vaul", () => import("~/test/mocks/vaul"));
vi.mock("~/components/ui/date-picker", async () =>
  (await import("~/test/mocks/date-picker")).datePickerModuleMock(),
);

const {
  mockUseSWRAuth,
  mockCreateStaffAppointment,
  mockToastSuccess,
  mockMutate,
  mockUseSession,
} = vi.hoisted(() => ({
  mockUseSWRAuth: vi.fn(),
  mockCreateStaffAppointment: vi.fn(),
  mockToastSuccess: vi.fn(),
  mockMutate: vi.fn(),
  mockUseSession: vi.fn(),
}));

vi.mock("~/lib/swr", () => ({ useSWRAuth: mockUseSWRAuth }));
vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({
    success: mockToastSuccess,
    error: vi.fn(),
    warning: vi.fn(),
  }),
}));
vi.mock("next-auth/react", () => ({ useSession: mockUseSession }));
vi.mock("next/navigation", async (importOriginal) => {
  const actual = await importOriginal<typeof import("next/navigation")>();
  return { ...actual, useSearchParams: () => null };
});
vi.mock("~/components/timetable/betreuungsplan-view", () => ({
  BetreuungsplanView: () => <div data-testid="school-plan-view" />,
}));
vi.mock("~/lib/personal-calendar-api", async () => {
  const actual = await vi.importActual<
    typeof import("~/lib/personal-calendar-api")
  >("~/lib/personal-calendar-api");
  return {
    ...actual,
    getStaffCalendar: vi.fn(),
    getCalendarRecipientOptions: vi.fn(),
    createStaffAppointment: mockCreateStaffAppointment,
  };
});

import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type {
  CalendarRecipientOptions,
  CalendarResponse,
} from "~/lib/personal-calendar-api";
import StaffCalendarPage from "./page";

const TITLE = "Termin erstellen";
const FOOTER = ["Abbrechen", /Speichert/];

const calendarResponse: CalendarResponse = {
  from: "2026-01-05",
  to: "2026-01-11",
  events: [],
};
const recipientOptions: CalendarRecipientOptions = {
  staff: [{ id: "7", name: "Anna Mitarbeiterin" }],
  parents: [],
  groups: [],
  classes: [],
  students: [],
};

function allowManage(allowed: boolean) {
  mockUseSession.mockReturnValue({
    data: {
      user: {
        permissions: allowed
          ? ["calendar:own", "calendar:manage"]
          : ["calendar:own"],
      },
    },
    status: "authenticated",
  });
}

async function startCreating() {
  const answer = deferred<unknown>();
  mockCreateStaffAppointment.mockReturnValue(answer.promise);
  const view = render(<StaffCalendarPage />, { wrapper: ToastProvider });
  fireEvent.click(screen.getByRole("button", { name: "Neuer Termin" }));
  fireEvent.change(screen.getByLabelText("Titel"), {
    target: { value: "Teamsitzung" },
  });
  fireEvent.click(screen.getByLabelText("Anna Mitarbeiterin"));
  fireEvent.click(screen.getByRole("button", { name: "Termin speichern" }));
  await screen.findByRole("button", { name: /Speichert/ });
  return { ...view, answer };
}

describe("Kalender: Speichern sperrt das Panel (#6593)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setTestClock(new Date(2026, 0, 7));
    allowManage(true);
    mockUseSWRAuth.mockImplementation((key: unknown) => {
      const cacheKey = typeof key === "string" ? key : "";
      if (cacheKey.startsWith("calendar-recipient-options")) {
        return { data: recipientOptions, isLoading: false };
      }
      if (cacheKey.startsWith("staff-calendar")) {
        return {
          data: calendarResponse,
          error: null,
          isLoading: false,
          mutate: mockMutate,
        };
      }
      return {
        data: undefined,
        error: null,
        isLoading: false,
        mutate: vi.fn(),
      };
    });
    mockMutate.mockResolvedValue(undefined);
  });

  it("locks the panel while saving and unlocks it with the error after a failure", async () => {
    const { answer } = await startCreating();

    expectSlideOverSaveLocked(TITLE, FOOTER);

    await act(async () => {
      answer.reject(new ApiError("down", 503, { code: "general.unavailable" }));
    });

    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expectSlideOverUnlocked(TITLE);
    expect(screen.getByLabelText("Titel")).toHaveValue("Teamsitzung");
    expect(mockMutate).not.toHaveBeenCalled();
  });

  it("closes the panel and refreshes the calendar after saving", async () => {
    const { answer } = await startCreating();

    await act(async () => {
      answer.resolve({ appointment: { id: "1" } });
    });

    expect(mockToastSuccess).toHaveBeenCalledWith(
      "Der Termin „Teamsitzung“ ist gespeichert.",
    );
    expect(mockMutate).toHaveBeenCalledOnce();
    // Anker: die Seite ist wieder bedienbar.
    expect(screen.getByRole("button", { name: "Neuer Termin" })).toBeEnabled();
    expect(screen.queryByRole("heading", { name: TITLE })).toBeNull();
  });

  it("keeps a reopened panel open when the old answer arrives late", async () => {
    const { answer, rerender } = await startCreating();

    // Fällt das Recht weg, schließt das Panel; kommt es zurück, öffnet es neu.
    allowManage(false);
    rerender(<StaffCalendarPage />);
    // Anker: die Kalenderseite ist ohne Verwaltungsrecht weiter gerendert.
    expect(
      screen.getByRole("heading", { name: "Mein Kalender" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: TITLE })).toBeNull();
    allowManage(true);
    rerender(<StaffCalendarPage />);
    expectSlideOverUnlocked(TITLE);

    await act(async () => {
      answer.resolve({ appointment: { id: "1" } });
    });

    expect(mockToastSuccess).toHaveBeenCalledOnce();
    expect(mockMutate).toHaveBeenCalledOnce();
    expectSlideOverUnlocked(TITLE);
    expect(screen.getByLabelText("Titel")).toHaveValue("Teamsitzung");
  });
});
