import { describe, it, expect, vi, beforeEach } from "vitest";
import { setTestClock } from "~/test/clock";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import { ApiError } from "~/lib/api-error";
import type { ClassDayReport } from "~/lib/class-day-api";

// Der Hinweis der Lehrkraft an die OGS (#4795): nur heute, nur mit Freigabe
// der OGS, für die ganze Klasse oder ein Kind.

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

vi.mock("~/lib/swr", async () => {
  const React = await import("react");
  return {
    useSWRAuth: <T,>(key: string | null, fetcher: () => Promise<T>) => {
      const [data, setData] = React.useState<T | undefined>(undefined);
      const fetcherRef = React.useRef(fetcher);
      fetcherRef.current = fetcher;
      const load = React.useCallback(() => {
        if (key === null) return Promise.resolve(undefined);
        return fetcherRef.current().then((d) => {
          setData(d);
          return d;
        });
      }, [key]);
      React.useEffect(() => {
        void load();
      }, [load]);
      return {
        data,
        error: undefined,
        isLoading: data === undefined && key !== null,
        mutate: load,
      };
    },
  };
});

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

describe("ClassDayClass day hints", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockSearchParams.delete("tag");
    setTestClock(new Date(`${TODAY}T10:00:00+02:00`));
  });

  it("shows the hints at the class and at the child, with a colleague's name", async () => {
    render(
      <ClassDayClass
        schoolClass="4a"
        fetchClassDay={() =>
          Promise.resolve(
            report({
              class_day_hints: [
                {
                  body: "Heute Sportfest",
                  own: true,
                  updated_at: "2026-09-07T07:00:00Z",
                },
              ],
            }),
          )
        }
        fetchClasses={classes}
      />,
    );

    expect(await screen.findByText(/Heute Sportfest/)).toBeInTheDocument();
    expect(
      screen.getByText(/Hat Bauchweh \(von Lena Lehrerin\)/),
    ).toBeInTheDocument();
    // Lesen ist ohne Freigabe möglich, schreiben nicht.
    expect(
      screen.queryByRole("button", { name: "Hinweis für die OGS" }),
    ).not.toBeInTheDocument();
  });

  it("writes a hint for the whole class when the school opened it", async () => {
    let current = report({ can_write_day_hints: true });
    const fetchClassDay = vi.fn(() => Promise.resolve(current));
    render(
      <ClassDayClass
        schoolClass="4a"
        fetchClassDay={fetchClassDay}
        fetchClasses={classes}
      />,
    );

    fireEvent.click(
      await screen.findByRole("button", { name: "Hinweis für die OGS" }),
    );
    const field = screen.getByLabelText("Hinweis");
    // Der Browser zählt UTF-16-Einheiten; die Grenze lässt 500 Zeichen mit
    // Emojis zu, die eigentliche Prüfung zählt wie das Backend.
    expect(field).toHaveAttribute("maxLength", "1000");

    // Leer speichern sagt, was fehlt, und schickt nichts.
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    expect(
      (await screen.findAllByText("Bitte schreiben Sie einen Hinweis.")).length,
    ).toBeGreaterThan(0);
    expect(saveDayHint).not.toHaveBeenCalled();

    fireEvent.change(field, { target: { value: "  Heute Sportfest  " } });
    expect(screen.getByText("19 von 500 Zeichen")).toBeInTheDocument();
    current = report({
      can_write_day_hints: true,
      class_day_hints: [
        {
          body: "Heute Sportfest",
          own: true,
          updated_at: "2026-09-07T08:00:00Z",
        },
      ],
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(saveDayHint).toHaveBeenCalledWith(
        "4a",
        TODAY,
        "Heute Sportfest",
        undefined,
      ),
    );
    expect(await screen.findByText(/Heute Sportfest/)).toBeInTheDocument();
    expect(fetchClassDay).toHaveBeenCalledTimes(2);
  });

  it("deletes the teacher's own class hint after the confirmation", async () => {
    render(
      <ClassDayClass
        schoolClass="4a"
        fetchClassDay={() =>
          Promise.resolve(
            report({
              can_write_day_hints: true,
              class_day_hints: [
                {
                  body: "Heute Sportfest",
                  own: true,
                  updated_at: "2026-09-07T07:00:00Z",
                },
              ],
            }),
          )
        }
        fetchClasses={classes}
      />,
    );

    fireEvent.click(
      await screen.findByRole("button", { name: "Hinweis für die OGS" }),
    );
    // Der eigene Hinweis ist zum Ändern vorbelegt.
    expect(screen.getByLabelText("Hinweis")).toHaveValue("Heute Sportfest");
    fireEvent.click(screen.getByRole("button", { name: "Hinweis löschen" }));
    fireEvent.click(await screen.findByRole("button", { name: "Ja, löschen" }));
    fireEvent.click(
      await screen.findByRole("button", { name: "Endgültig löschen" }),
    );

    await waitFor(() =>
      expect(deleteDayHint).toHaveBeenCalledWith("4a", TODAY, undefined),
    );
  });

  it("counts characters like the backend, an emoji as one", async () => {
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
    const field = screen.getByLabelText("Hinweis");

    // 501 Zeichen sind zu lang: die Meldung steht am Feld, nichts geht raus.
    fireEvent.change(field, { target: { value: "😀".repeat(501) } });
    expect(screen.getByText("501 von 500 Zeichen")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    expect(
      (
        await screen.findAllByText(
          "Der Hinweis ist zu lang. Bitte kürzen Sie ihn auf 500 Zeichen.",
        )
      ).length,
    ).toBeGreaterThan(0);
    expect(saveDayHint).not.toHaveBeenCalled();

    // 500 Emojis sind 1000 UTF-16-Einheiten und trotzdem erlaubt.
    fireEvent.change(field, { target: { value: "😀".repeat(500) } });
    expect(screen.getByText("500 von 500 Zeichen")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await waitFor(() =>
      expect(saveDayHint).toHaveBeenCalledWith(
        "4a",
        TODAY,
        "😀".repeat(500),
        undefined,
      ),
    );
  });

  it("keeps a failed deletion in the dialog with its request ID and a retry", async () => {
    deleteDayHint.mockRejectedValueOnce(
      new ApiError("boom", 500, {
        code: "general.server",
        instance: "req-hint",
      }),
    );
    render(
      <ClassDayClass
        schoolClass="4a"
        fetchClassDay={() =>
          Promise.resolve(
            report({
              can_write_day_hints: true,
              class_day_hints: [
                {
                  body: "Heute Sportfest",
                  own: true,
                  updated_at: "2026-09-07T07:00:00Z",
                },
              ],
            }),
          )
        }
        fetchClasses={classes}
      />,
    );

    fireEvent.click(
      await screen.findByRole("button", { name: "Hinweis für die OGS" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Hinweis löschen" }));
    fireEvent.click(await screen.findByRole("button", { name: "Ja, löschen" }));
    fireEvent.click(
      await screen.findByRole("button", { name: "Endgültig löschen" }),
    );

    expect(
      await screen.findByRole("button", { name: "Vorgangskennung kopieren" }),
    ).toHaveTextContent("req-hint");
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    await waitFor(() => expect(deleteDayHint).toHaveBeenCalledTimes(2));
  });

  it("offers no hint for another day", async () => {
    mockSearchParams.set("tag", "2026-09-08");
    render(
      <ClassDayClass
        schoolClass="4a"
        fetchClassDay={() =>
          Promise.resolve(
            report({ date: "2026-09-08", can_write_day_hints: false }),
          )
        }
        fetchClasses={classes}
      />,
    );

    expect(await screen.findByText("Klassentag, Klara")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Hinweis für die OGS" }),
    ).not.toBeInTheDocument();
  });
});
