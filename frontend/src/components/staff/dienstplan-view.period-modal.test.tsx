// #6593: Der Dienstplan lädt nach dem Anlegen eines Kalenderzeitraums nur neu.
// Der Dialog schließt sich selbst, und nur ohne Überschneidungswarnung; der
// Dienstplan schließt ihn nicht schon beim Speichern. Echte View mit echtem
// CalendarPeriodModal und vaul-Mock.
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type {
  CalendarPeriod,
  CalendarPeriodWarning,
} from "~/lib/calendar-period-helpers";
import {
  deferred,
  expectSlideOverSaveLocked,
} from "~/test/helpers/form-modal-pending";

const mocks = vi.hoisted(() => ({
  useSWRAuth: vi.fn(),
  mutatePeriods: vi.fn(),
  create: vi.fn(),
  toastSuccess: vi.fn(),
}));

vi.mock("vaul", () => import("~/test/mocks/vaul"));

vi.mock("~/components/ui/date-picker", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  ...(await import("~/test/mocks/date-picker")).isoDatePickerMock(),
}));

vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
  useSearchParams: () => new URLSearchParams(""),
}));

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { id: "1", isAdmin: true, token: "token" } },
    status: "authenticated",
  }),
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ replace: vi.fn() }),
}));

vi.mock("~/lib/auth-utils", () => ({
  isAdmin: () => true,
  hasPermission: () => true,
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: mocks.useSWRAuth,
  useTenantMutateMatching: () => () => Promise.resolve(),
}));

vi.mock("~/lib/hooks/use-berlin-today", () => ({
  useBerlinToday: () => "2026-07-06",
}));

vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({ success: mocks.toastSuccess, error: vi.fn() }),
}));

vi.mock("~/lib/calendar-period-api", () => ({
  calendarPeriodService: {
    create: mocks.create,
    update: vi.fn(),
    delete: vi.fn(),
  },
}));

// Der Zeitraum-Switcher öffnet hier nur den Dialog.
vi.mock("~/components/timetable/period-switcher-dropdown", () => ({
  PeriodSwitcherDropdown: ({ onCreate }: { onCreate: () => void }) => (
    <button type="button" onClick={onCreate}>
      Neuer Zeitraum
    </button>
  ),
}));

// Eigene Tests in absence-plan-gaps.test.tsx; hier zählt nur das Raster.
vi.mock("~/components/staff/absence-plan-gaps", () => ({
  AbsencePlanGaps: () => null,
}));

vi.mock("~/components/staff/dienstplan-resource-grid", () => ({
  DienstplanResourceGrid: () => <div data-testid="dienstplan-grid" />,
}));

import { DienstplanView } from "./dienstplan-view";

const TITLE = "Kalenderzeitraum anlegen";
const SAVED_TITLE = "Kalenderzeitraum bearbeiten";
const CLOSE = "Zeitraum schließen";

const period = {
  id: "5",
  tenantId: "1",
  name: "Sommerferien 2026",
  periodType: "holiday",
  startDate: "2026-07-20",
  endDate: "2026-08-28",
  weekCycleLength: 1,
  weekCycleAnchor: null,
  isActive: true,
  createdAt: "2026-07-01T00:00:00Z",
  updatedAt: "2026-07-01T00:00:00Z",
} as CalendarPeriod;

const OVERLAP: CalendarPeriodWarning = {
  code: "overlapping_active_periods",
  message:
    'Der Zeitraum überschneidet sich mit dem aktiven Zeitraum "Schuljahr 2025/2026".',
  overlappingPeriodIds: ["3"],
  overlappingPeriodNames: ["Schuljahr 2025/2026"],
};

/** Opens the dialog from the Dienstplan, fills it and saves; the answer waits. */
async function createPeriod() {
  const answer = deferred<{
    period: CalendarPeriod;
    warnings: CalendarPeriodWarning[];
  }>();
  mocks.create.mockReturnValue(answer.promise);
  render(<DienstplanView />);

  fireEvent.click(
    await screen.findByRole("button", { name: "Neuer Zeitraum" }),
  );
  fireEvent.change(screen.getByLabelText("Bezeichnung"), {
    target: { value: "Sommerferien 2026" },
  });
  fireEvent.change(screen.getByLabelText("Startdatum"), {
    target: { value: "2026-07-20" },
  });
  fireEvent.change(screen.getByLabelText("Enddatum"), {
    target: { value: "2026-08-28" },
  });
  const create = screen.getByRole("button", { name: "Anlegen" });
  expect(create).toBeEnabled();
  fireEvent.click(create);
  expect(mocks.create).toHaveBeenCalledOnce();
  expectSlideOverSaveLocked(TITLE, ["Abbrechen", "Speichere …"], CLOSE);
  return answer;
}

describe("DienstplanView: Kalenderzeitraum anlegen (#6593)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    window.history.replaceState(null, "", "/acme/dienstplan");
    mocks.useSWRAuth.mockImplementation((key: string | null) => {
      if (key === "database-calendar-periods-list") {
        return {
          data: [],
          error: undefined,
          isLoading: false,
          mutate: mocks.mutatePeriods,
        };
      }
      if (key?.startsWith("dienstplan-overview-")) {
        return {
          data: {
            from: "",
            to: "",
            dienstplanInUse: true,
            staff: [{ id: "7", firstName: "Ada", lastName: "Lovelace" }],
            shifts: [],
            assignments: [],
          },
          error: undefined,
          isLoading: false,
          mutate: vi.fn(),
        };
      }
      return { data: [], error: undefined, isLoading: false, mutate: vi.fn() };
    });
  });

  it("lässt den Dialog mit der Überschneidungswarnung offen, bis sie geschlossen wird", async () => {
    const answer = await createPeriod();

    await act(async () => answer.resolve({ period, warnings: [OVERLAP] }));

    expect(mocks.mutatePeriods).toHaveBeenCalled();
    expect(screen.getByText(OVERLAP.message)).toBeVisible();
    // Gespeichert ist er schon: der offene Dialog bearbeitet ihn jetzt.
    // Anker: der Dialog bearbeitet den gespeicherten Zeitraum.
    expect(screen.getByRole("heading", { name: SAVED_TITLE })).toBeVisible();

    fireEvent.click(screen.getByRole("button", { name: "Schließen" }));
    await waitFor(() =>
      expect(
        screen.queryByRole("heading", { name: SAVED_TITLE }),
      ).not.toBeInTheDocument(),
    );
  });

  it("schließt den Dialog nach dem Anlegen ohne Warnung", async () => {
    const answer = await createPeriod();

    await act(async () => answer.resolve({ period, warnings: [] }));

    expect(mocks.mutatePeriods).toHaveBeenCalled();
    expect(mocks.toastSuccess).toHaveBeenCalledWith(
      "Der Kalenderzeitraum „Sommerferien 2026“ ist angelegt.",
    );
    // Anker: der Dienstplan ist weiter gerendert.
    expect(screen.getByTestId("dienstplan-grid")).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: TITLE }),
    ).not.toBeInTheDocument();
  });
});
