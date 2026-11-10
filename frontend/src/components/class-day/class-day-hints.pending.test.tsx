import { describe, it, expect, vi, beforeEach } from "vitest";
import { setTestClock } from "~/test/clock";
import {
  act,
  render as rtlRender,
  screen,
  fireEvent,
} from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import { ApiError } from "~/lib/api-error";
import type { ClassDayReport } from "~/lib/class-day-api";
import {
  deferred,
  expectDialogUnlocked,
  expectSaveLocksDialog,
} from "~/test/helpers/form-modal-pending";

// #6533: Schließen während des Speicherns eines Tageshinweises ist gesperrt;
// ein Fehler entsperrt den Dialog wieder. Echter FormModal.

const { mockSearchParams, saveDayHint, deleteDayHint } = vi.hoisted(() => ({
  mockSearchParams: new URLSearchParams(),
  saveDayHint: vi.fn(() => Promise.resolve()),
  deleteDayHint: vi.fn(() => Promise.resolve()),
}));

vi.mock("next/navigation", () => ({
  useSearchParams: () => mockSearchParams,
}));

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: {
      user: { id: "teacher", scope: "school", tenantId: 1 },
      expires: "2099-01-01",
    },
    status: "authenticated",
  }),
}));

vi.unmock("swr");
vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: () => null,
  useTenantRoutingModeSafe: () => "path",
}));
import { SWRConfig } from "~/lib/swr/testing";
import { ToastProvider } from "~/contexts/ToastContext";
import type { ReactElement } from "react";

function render(ui: ReactElement) {
  const cache = new Map();
  return rtlRender(ui, {
    wrapper: ({ children }) => (
      <SWRConfig value={{ provider: () => cache, shouldRetryOnError: false }}>
        <ToastProvider>{children}</ToastProvider>
      </SWRConfig>
    ),
  });
}

vi.mock("~/lib/school-url", () => ({ schoolPath: (path: string) => path }));

vi.mock("./class-arrival-exception-dialog", () => ({
  ClassArrivalExceptionDialog: () => null,
}));

vi.mock("~/lib/school-class-day-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/school-class-day-api")>()),
  saveDayHintSchool: saveDayHint,
  deleteDayHintSchool: deleteDayHint,
}));

import { ClassDayClass } from "./class-day-class";

const TODAY = "2026-09-07";

function report(overrides: Partial<ClassDayReport> = {}): ClassDayReport {
  return {
    school_class: "4a",
    date: TODAY,
    weekday: "mon",
    school_day: true,
    enrollment_known: true,
    totals: { students: 2, staying: 2, leaving: 0, absent: 0, list_entries: 0 },
    rows: [
      {
        student_id: 1,
        first_name: "Klara",
        last_name: "Klassentag",
        registered: true,
        stays_today: true,
        offerings: [],
      },
      {
        student_id: 2,
        first_name: "Ida",
        last_name: "Hinweis",
        registered: true,
        stays_today: true,
        offerings: [],
        day_hints: [
          {
            body: "Hat Bauchweh",
            author_name: "Lena Lehrerin",
            own: false,
            updated_at: "2026-09-07T07:00:00Z",
          },
        ],
      },
    ],
    ...overrides,
  };
}

const classes = () =>
  Promise.resolve({ classes: ["4a"], can_write_arrival_exception: false });

describe("ClassDayClass day hint pending lock (#6533)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockSearchParams.delete("tag");
    setTestClock(new Date(`${TODAY}T10:00:00+02:00`));
  });

  it("keeps the hint dialog shut while saving and unlocks it after a failure", async () => {
    const answer = deferred();
    saveDayHint.mockReturnValue(answer.promise);
    render(
      <ClassDayClass
        schoolClass="4a"
        fetchClassDay={() =>
          Promise.resolve(report({ can_write_day_hints: true }))
        }
        fetchClasses={classes}
      />,
    );

    fireEvent.click(
      await screen.findByRole("button", { name: "Hinweis für die OGS" }),
    );
    fireEvent.change(screen.getByLabelText("Hinweis"), {
      target: { value: "Heute Sportfest" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expectSaveLocksDialog("Hinweis für die OGS");

    await act(async () => {
      answer.reject(new ApiError("kaputt", 500, { code: "general.server" }));
    });
    expect((await screen.findAllByRole("alert")).length).toBeGreaterThan(0);
    expectDialogUnlocked("Hinweis für die OGS");
    expect(screen.getByLabelText("Hinweis")).toHaveValue("Heute Sportfest");
  });
});
