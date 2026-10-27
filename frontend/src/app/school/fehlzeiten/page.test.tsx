import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ToastProvider } from "~/contexts/ToastContext";
import type { AbsenceList, ClassDayClasses } from "~/lib/school-class-day-api";

const { mockUseSWRAuth, mockDownloadBlob, swrData } = vi.hoisted(() => ({
  mockUseSWRAuth: vi.fn(),
  mockDownloadBlob: vi.fn(),
  swrData: new Map<string, unknown>(),
}));

vi.mock("~/lib/swr", () => ({ useSWRAuth: mockUseSWRAuth }));
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { scope: "school" } },
    status: "authenticated",
  }),
}));
vi.mock("~/components/ui/date-picker", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  ...(await import("~/test/mocks/date-picker")).isoDatePickerMock(),
}));
vi.mock("vaul", () => import("~/test/mocks/vaul"));
vi.mock("~/lib/file-download", () => ({
  downloadBlob: mockDownloadBlob,
  filenameFromDisposition: () => "fehlzeiten-klasse-3a.xlsx",
  openBlobForPrint: vi.fn(),
}));

import SchoolAbsenceListPage from "./page";

const CLASSES_KEY = "class-day-my-classes";

function absenceList(overrides: Partial<AbsenceList> = {}): AbsenceList {
  return {
    schoolClass: "3a",
    from: "2026-08-01",
    to: "2027-01-31",
    periodName: "1. Halbjahr 2026/27",
    countedUntil: "2026-09-09",
    schoolDays: 28,
    children: [
      {
        studentId: "1",
        firstName: "Anna",
        lastName: "Arndt",
        absentDays: 3,
        excusedDays: 2,
        unexcusedDays: 1,
        notRecordedDays: 4,
      },
    ],
    totals: {
      absentDays: 3,
      excusedDays: 2,
      unexcusedDays: 1,
      notRecordedDays: 4,
    },
    ...overrides,
  };
}

function classes(list: string[]): ClassDayClasses {
  return { classes: list, can_write_arrival_exception: false };
}

function renderPage() {
  return render(
    <ToastProvider>
      <SchoolAbsenceListPage />
    </ToastProvider>,
  );
}

/** The absence-list keys the page asked for, in order. */
function listKeys(): string[] {
  return mockUseSWRAuth.mock.calls
    .map(([key]) => key as string | null)
    .filter((key): key is string => key?.startsWith("school-absence") ?? false);
}

beforeEach(() => {
  vi.clearAllMocks();
  swrData.clear();
  // Every absence-list key answers with the list of its period.
  mockUseSWRAuth.mockImplementation((key: string | null) => {
    if (key === null) return { data: undefined, mutate: vi.fn() };
    if (key.startsWith("school-absence-list")) {
      return {
        data: swrData.get(key) ?? swrData.get("list"),
        mutate: vi.fn(),
      };
    }
    return { data: swrData.get(key), mutate: vi.fn() };
  });
});

describe("SchoolAbsenceListPage", () => {
  it("shows the absence days of every child in the preselected half-year", () => {
    swrData.set(CLASSES_KEY, classes(["3a", "3b"]));
    swrData.set("list", absenceList());

    renderPage();

    expect(screen.getByRole("heading", { name: "Fehlzeiten" })).toBeVisible();
    expect(screen.getAllByText("Arndt, Anna")[0]).toBeInTheDocument();
    expect(screen.getByLabelText<HTMLInputElement>("Von")).toHaveValue(
      "2026-08-01",
    );
    expect(screen.getByLabelText<HTMLInputElement>("Bis")).toHaveValue(
      "2027-01-31",
    );
    expect(
      screen.getByText(
        "1. Halbjahr 2026/27 · Gezählt bis 09.09.2026 · 28 Schultage · Klasse gesamt: 3 Fehltage, davon 2 entschuldigt",
      ),
    ).toBeInTheDocument();
    // Without an own choice the page asks for the backend's half-year.
    expect(listKeys()).toContain("school-absence-list-3a--");
  });

  it("loads a chosen period and refuses an end before the start", () => {
    swrData.set(CLASSES_KEY, classes(["3a"]));
    swrData.set("list", absenceList());

    renderPage();

    fireEvent.change(screen.getByLabelText("Bis"), {
      target: { value: "2026-07-01" },
    });
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Das Bis-Datum liegt vor dem Von-Datum.",
    );
    expect(listKeys()).not.toContain(
      "school-absence-list-3a-2026-08-01-2026-07-01",
    );

    fireEvent.change(screen.getByLabelText("Bis"), {
      target: { value: "2026-09-30" },
    });
    expect(
      screen.queryByText("Das Bis-Datum liegt vor dem Von-Datum."),
    ).toBeNull();
    expect(listKeys()).toContain(
      "school-absence-list-3a-2026-08-01-2026-09-30",
    );
  });

  it("asks for the start at the Von field while it is empty", () => {
    swrData.set(CLASSES_KEY, classes(["3a"]));

    renderPage();

    fireEvent.change(screen.getByLabelText("Bis"), {
      target: { value: "2026-09-30" },
    });
    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent("Bitte wählen Sie ein Von-Datum.");
    // The stub renders the error inside the field's own wrapper.
    expect(alert.parentElement).toContainElement(screen.getByLabelText("Von"));
    expect(listKeys()).not.toContain("school-absence-list-3a--2026-09-30");
  });

  it("says that nothing is counted for a period ahead", () => {
    swrData.set(CLASSES_KEY, classes(["3a"]));
    swrData.set(
      "list",
      absenceList({ periodName: "", countedUntil: "", schoolDays: 0 }),
    );

    renderPage();

    expect(
      screen.getByText(
        "Der Zeitraum liegt in der Zukunft. Noch kein Tag ist gezählt.",
      ),
    ).toBeInTheDocument();
  });

  it("explains a missing class assignment instead of an empty list", () => {
    swrData.set(CLASSES_KEY, classes([]));

    renderPage();

    expect(
      screen.getByText("Ihnen ist keine Klasse zugeordnet."),
    ).toBeInTheDocument();
    expect(listKeys()).toEqual([]);
  });

  it("exports the shown class in the preselected half-year as an Excel file", async () => {
    swrData.set(CLASSES_KEY, classes(["3a"]));
    swrData.set("list", absenceList());
    const fetchMock = vi.fn(async () => new Response(new Blob(["xlsx"])));
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Exportieren" }));
    fireEvent.click(screen.getByRole("button", { name: "Excel" }));
    fireEvent.click(screen.getByRole("button", { name: "Herunterladen" }));

    await waitFor(() => expect(mockDownloadBlob).toHaveBeenCalled());
    const [url] = fetchMock.mock.calls[0] as unknown as [string];
    expect(url).toBe(
      "/api/school/class-day/absence-list/export?class=3a&format=xlsx",
    );
    expect(mockDownloadBlob).toHaveBeenCalledWith(
      expect.any(Blob),
      "fehlzeiten-klasse-3a.xlsx",
    );
  });
});
