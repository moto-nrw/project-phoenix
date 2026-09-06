import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import type { ClassDayReport } from "~/lib/class-day-api";
import { todayISO } from "~/lib/date-helpers";

// Die Klassenansicht zeigt die Klassen-Tagesausnahme als Zeile und bietet
// den Eintrag nur an, wenn die OGS moto schule dafür freigegeben hat (#2970).

const { mockSearchParams } = vi.hoisted(() => ({
  mockSearchParams: new URLSearchParams(),
}));

vi.mock("next/navigation", () => ({
  useSearchParams: () => mockSearchParams,
}));

vi.mock("~/lib/swr", async () => {
  const React = await import("react");
  return {
    useSWRAuth: <T,>(key: string | null, fetcher: () => Promise<T>) => {
      const [data, setData] = React.useState<T | undefined>(undefined);
      const [error, setError] = React.useState<unknown>(undefined);
      const [loading, setLoading] = React.useState(true);
      const fetcherRef = React.useRef(fetcher);
      fetcherRef.current = fetcher;
      const load = React.useCallback(() => {
        if (key === null) {
          setLoading(false);
          return Promise.resolve(undefined);
        }
        return fetcherRef
          .current()
          .then((d) => {
            setData(d);
            setLoading(false);
            return d;
          })
          .catch((e: unknown) => {
            setError(e);
            setLoading(false);
            return undefined;
          });
      }, [key]);
      React.useEffect(() => {
        void load();
      }, [load]);
      return { data, error, isLoading: loading, mutate: load };
    },
  };
});

vi.mock("~/lib/school-url", () => ({
  schoolPath: (path: string) => path,
}));

vi.mock("./class-arrival-exception-dialog", () => ({
  ClassArrivalExceptionDialog: ({
    isOpen,
    schoolClass,
  }: {
    isOpen: boolean;
    schoolClass: string;
  }) =>
    isOpen ? (
      <div role="dialog" aria-label={`Dialog ${schoolClass}`}>
        Dialog offen
      </div>
    ) : null,
}));

import { ClassDayClass, classArrivalExceptionLine } from "./class-day-class";

function report(overrides: Partial<ClassDayReport> = {}): ClassDayReport {
  return {
    school_class: "4a",
    date: todayISO(),
    weekday: "mon",
    school_day: true,
    enrollment_known: true,
    totals: { students: 1, staying: 1, leaving: 0, absent: 0, list_entries: 0 },
    rows: [
      {
        student_id: 1,
        first_name: "Klara",
        last_name: "Klassentag",
        registered: true,
        stays_today: true,
        offerings: ["Ganztag"],
      },
    ],
    ...overrides,
  };
}

describe("classArrivalExceptionLine", () => {
  it("names today or the date and appends the reason", () => {
    const today = "2026-09-07";
    expect(
      classArrivalExceptionLine(
        {
          class_arrival_exception: {
            arrival_time: "12:45",
            reason: "Unterricht fällt aus",
            origin: "school",
          },
        },
        today,
        today,
      ),
    ).toBe("Heute kommt die Klasse um 12:45 Uhr (Unterricht fällt aus)");
    expect(
      classArrivalExceptionLine(
        { class_arrival_exception: { arrival_time: "11:30", origin: "ogs" } },
        "2026-09-08",
        today,
      ),
    ).toBe("Am 08.09.2026 kommt die Klasse um 11:30 Uhr");
    expect(classArrivalExceptionLine({}, today, today)).toBeNull();
  });
});

describe("ClassDayClass", () => {
  // Die Ansicht zeigt am Wochenende „Kein Schultag“ statt der Klassenliste.
  // Ohne feste Uhr sind diese Fälle samstags und sonntags rot, deshalb ein
  // fester Montag.
  beforeEach(() => {
    vi.clearAllMocks();
    mockSearchParams.delete("tag");
    // Weekday fixtures must not depend on the day CI runs. Keep async timers real.
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2026-09-07T12:00:00+02:00"));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it.each(["2026-09-05T12:00:00+02:00", "2026-09-06T12:00:00+02:00"])(
    "does not load a class day or offer arrival changes on the weekend (%s)",
    (date) => {
      vi.setSystemTime(new Date(date));
      const fetchClassDay = vi.fn(() => Promise.resolve(report()));

      render(<ClassDayClass schoolClass="4a" fetchClassDay={fetchClassDay} />);

      expect(screen.getByText("Kein Schultag")).toBeInTheDocument();
      expect(fetchClassDay).not.toHaveBeenCalled();
      expect(
        screen.queryByRole("button", { name: /Ankunft/ }),
      ).not.toBeInTheDocument();
    },
  );

  it("shows the class-wide exception line even without write access", async () => {
    render(
      <ClassDayClass
        schoolClass="4a"
        fetchClassDay={() =>
          Promise.resolve(
            report({
              class_arrival_exception: {
                arrival_time: "12:45",
                reason: "Unterricht fällt aus",
                origin: "ogs",
              },
            }),
          )
        }
        fetchClasses={() =>
          Promise.resolve({
            classes: ["4a"],
            can_write_arrival_exception: false,
          })
        }
      />,
    );

    expect(
      await screen.findByText(
        "Heute kommt die Klasse um 12:45 Uhr (Unterricht fällt aus)",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Ankunft/ }),
    ).not.toBeInTheDocument();
  });

  it("offers the entry only when the school opened it and opens the dialog", async () => {
    render(
      <ClassDayClass
        schoolClass="4a"
        fetchClassDay={() => Promise.resolve(report())}
        fetchClasses={() =>
          Promise.resolve({
            classes: ["4a"],
            can_write_arrival_exception: true,
          })
        }
      />,
    );

    const button = await screen.findByRole("button", {
      name: "Ankunft heute ändern",
    });
    expect(screen.queryByText(/kommt die Klasse um/)).not.toBeInTheDocument();

    fireEvent.click(button);
    await waitFor(() => {
      expect(
        screen.getByRole("dialog", { name: "Dialog 4a" }),
      ).toBeInTheDocument();
    });
  });

  it("names the shown day on the button when it is not today", async () => {
    mockSearchParams.set("tag", "2099-03-02");
    render(
      <ClassDayClass
        schoolClass="4a"
        fetchClassDay={() => Promise.resolve(report({ date: "2099-03-02" }))}
        fetchClasses={() =>
          Promise.resolve({
            classes: ["4a"],
            can_write_arrival_exception: true,
          })
        }
      />,
    );

    expect(
      await screen.findByRole("button", {
        name: "Ankunft an diesem Tag ändern",
      }),
    ).toBeInTheDocument();
  });

  it("stays read-only for a class the teacher is not assigned to", async () => {
    render(
      <ClassDayClass
        schoolClass="4a"
        fetchClassDay={() => Promise.resolve(report())}
        fetchClasses={() =>
          Promise.resolve({
            classes: ["3b"],
            can_write_arrival_exception: true,
          })
        }
      />,
    );

    expect(await screen.findByText("Klassentag, Klara")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Ankunft/ }),
    ).not.toBeInTheDocument();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("matches the assigned class like the backend, ignoring case and spaces", async () => {
    render(
      <ClassDayClass
        schoolClass="4A"
        fetchClassDay={() => Promise.resolve(report())}
        fetchClasses={() =>
          Promise.resolve({
            classes: [" 4a"],
            can_write_arrival_exception: true,
          })
        }
      />,
    );

    expect(
      await screen.findByRole("button", { name: "Ankunft heute ändern" }),
    ).toBeInTheDocument();
  });

  it("stays read-only without a classes fetcher", async () => {
    render(
      <ClassDayClass
        schoolClass="4a"
        fetchClassDay={() => Promise.resolve(report())}
      />,
    );

    expect(await screen.findByText("Klassentag, Klara")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Ankunft/ }),
    ).not.toBeInTheDocument();
  });
});
