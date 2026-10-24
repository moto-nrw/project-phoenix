import { describe, it, expect, vi, beforeEach } from "vitest";
import { setTestClock } from "~/test/clock";
import {
  render,
  screen,
  fireEvent,
  waitFor,
  within,
} from "@testing-library/react";
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

vi.mock("~/components/students/class-trip-bulk-status-modal", () => ({
  ClassTripBulkStatusModal: ({
    isOpen,
    students,
    onSuccess,
    onClose,
    schoolReport,
  }: {
    isOpen: boolean;
    students: { id: string; name: string }[];
    onSuccess?: () => void;
    onClose: () => void;
    schoolReport?: { save: (input: unknown) => Promise<void> };
  }) =>
    isOpen ? (
      <div role="dialog" aria-label="Ausflug melden">
        {students.map((student) => student.name).join(", ")}
        <button
          type="button"
          onClick={() =>
            void schoolReport
              ?.save({
                studentIds: students.map((student) => student.id),
                from: "2026-09-07",
                to: "2026-09-07",
                returnTime: "13:30",
              })
              .then(() => {
                onSuccess?.();
                onClose();
              })
          }
        >
          Melden
        </button>
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
    setTestClock(new Date("2026-09-07T12:00:00+02:00"));
  });

  it.each(["2026-09-05T12:00:00+02:00", "2026-09-06T12:00:00+02:00"])(
    "does not load a class day or offer arrival changes on the weekend (%s)",
    (date) => {
      setTestClock(new Date(date));
      const fetchClassDay = vi.fn(() => Promise.resolve(report()));

      render(<ClassDayClass schoolClass="4a" fetchClassDay={fetchClassDay} />);

      expect(screen.getByText("Kein Schultag")).toBeInTheDocument();
      expect(fetchClassDay).not.toHaveBeenCalled();
      expect(
        screen.queryByRole("button", { name: /Ankunft/ }),
      ).not.toBeInTheDocument();
    },
  );

  it("loads the weekend like a school day when it follows Friday's plan (#3921)", async () => {
    setTestClock(new Date("2026-09-05T12:00:00+02:00"));
    const fetchClassDay = vi.fn(() => Promise.resolve(report()));

    render(
      <ClassDayClass
        schoolClass="4a"
        fetchClassDay={fetchClassDay}
        fetchClasses={() =>
          Promise.resolve({
            classes: ["4a"],
            can_write_arrival_exception: false,
            weekend_follows_friday: true,
          })
        }
      />,
    );

    // Anker: die Klassenliste des Samstags ist geladen.
    expect(await screen.findByText("Klassentag, Klara")).toBeInTheDocument();
    expect(fetchClassDay).toHaveBeenCalledWith("4a", "2026-09-05");
    expect(screen.queryByText("Kein Schultag")).not.toBeInTheDocument();
  });

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

  describe("morning attendance (#4790)", () => {
    function morningReport(): ClassDayReport {
      return report({
        can_record_morning_attendance: true,
        totals: {
          students: 3,
          staying: 1,
          leaving: 0,
          absent: 1,
          list_entries: 1,
        },
        rows: [
          {
            student_id: 1,
            first_name: "Klara",
            last_name: "Klassentag",
            registered: true,
            stays_today: true,
            offerings: ["Ganztag"],
          },
          {
            student_id: 2,
            first_name: "Nico",
            last_name: "Krank",
            registered: true,
            stays_today: false,
            offerings: [],
            status: "sick",
            morning_attendance: { status: "absent", source: "report" },
          },
          {
            student_id: 0,
            list_entry: true,
            list_entry_id: "9007199254740993",
            first_name: "Lisa",
            last_name: "Liste",
            registered: false,
            stays_today: false,
            offerings: [],
          },
        ],
      });
    }

    function group(name: string) {
      return screen.getByRole("group", { name: `Anwesenheit von ${name}` });
    }

    it("prefills reported children, highlights an unreported absence and saves the draft", async () => {
      const record = vi.fn(() => Promise.resolve());
      render(
        <ClassDayClass
          schoolClass="4a"
          fetchClassDay={() => Promise.resolve(morningReport())}
          recordMorningAttendance={record}
        />,
      );

      expect(
        await screen.findByText(
          "Anwesenheit am Vormittag: 0 da · 1 fehlt · 2 noch offen",
        ),
      ).toBeInTheDocument();
      // Der gemeldete Nico steht schon auf „Fehlt“, Klara ist offen.
      expect(
        within(group("Nico Krank")).getByRole("button", { name: "Fehlt" }),
      ).toHaveAttribute("aria-pressed", "true");
      expect(
        within(group("Klara Klassentag")).getByRole("button", { name: "Da" }),
      ).toHaveAttribute("aria-pressed", "false");
      expect(
        screen.queryByRole("button", { name: "Anwesenheit speichern" }),
      ).not.toBeInTheDocument();

      fireEvent.click(
        within(group("Klara Klassentag")).getByRole("button", {
          name: "Fehlt",
        }),
      );
      expect(screen.getByText("Fehlt ohne Meldung")).toBeInTheDocument();
      fireEvent.click(screen.getByRole("button", { name: "Alle übrigen da" }));
      expect(
        screen.getByText(
          "Anwesenheit am Vormittag: 1 da · 2 fehlen · 1 ohne Meldung",
        ),
      ).toBeInTheDocument();
      expect(record).not.toHaveBeenCalled();

      fireEvent.click(
        screen.getByRole("button", { name: "Anwesenheit speichern" }),
      );

      await waitFor(() =>
        expect(record).toHaveBeenCalledWith("4a", todayISO(), [
          { student_id: 1, status: "absent" },
          { list_entry_id: "9007199254740993", status: "present" },
        ]),
      );
      await waitFor(() =>
        expect(
          screen.queryByRole("button", { name: "Anwesenheit speichern" }),
        ).not.toBeInTheDocument(),
      );
    });

    it("keeps the draft and shows the error when saving fails", async () => {
      const record = vi
        .fn()
        .mockRejectedValueOnce(new Error("save failed"))
        .mockResolvedValueOnce(undefined);
      render(
        <ClassDayClass
          schoolClass="4a"
          fetchClassDay={() => Promise.resolve(morningReport())}
          recordMorningAttendance={record}
        />,
      );

      fireEvent.click(
        within(
          await screen.findByRole("group", {
            name: "Anwesenheit von Klara Klassentag",
          }),
        ).getByRole("button", { name: "Da" }),
      );
      fireEvent.click(
        screen.getByRole("button", { name: "Anwesenheit speichern" }),
      );

      expect(await screen.findByRole("alert")).toBeInTheDocument();
      expect(
        within(group("Klara Klassentag")).getByRole("button", { name: "Da" }),
      ).toHaveAttribute("aria-pressed", "true");

      fireEvent.click(
        screen.getByRole("button", { name: "Anwesenheit speichern" }),
      );
      await waitFor(() => expect(record).toHaveBeenCalledTimes(2));
      await waitFor(() =>
        expect(
          screen.queryByRole("button", { name: "Anwesenheit speichern" }),
        ).not.toBeInTheDocument(),
      );
      expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    });

    it("discards the draft", async () => {
      render(
        <ClassDayClass
          schoolClass="4a"
          fetchClassDay={() => Promise.resolve(morningReport())}
          recordMorningAttendance={() => Promise.resolve()}
        />,
      );

      fireEvent.click(
        within(
          await screen.findByRole("group", {
            name: "Anwesenheit von Klara Klassentag",
          }),
        ).getByRole("button", { name: "Da" }),
      );
      fireEvent.click(screen.getByRole("button", { name: "Verwerfen" }));

      expect(
        within(group("Klara Klassentag")).getByRole("button", { name: "Da" }),
      ).toHaveAttribute("aria-pressed", "false");
      expect(
        screen.queryByRole("button", { name: "Anwesenheit speichern" }),
      ).not.toBeInTheDocument();
    });

    it("shows attendance and the highlight when the day cannot be recorded", async () => {
      const shown = morningReport();
      shown.can_record_morning_attendance = false;
      shown.rows[0]!.morning_attendance = {
        status: "absent",
        source: "teacher",
      };
      render(
        <ClassDayClass
          schoolClass="4a"
          fetchClassDay={() => Promise.resolve(shown)}
          recordMorningAttendance={() => Promise.resolve()}
        />,
      );

      expect(await screen.findByText("Fehlt ohne Meldung")).toBeInTheDocument();
      expect(screen.getAllByText("Vormittag: Fehlt")).toHaveLength(2);
      expect(
        screen.queryByRole("group", { name: /Anwesenheit von/ }),
      ).not.toBeInTheDocument();
      expect(
        screen.queryByText(/Anwesenheit am Vormittag/),
      ).not.toBeInTheDocument();
    });

    it("shows saved present marks alongside reports without marking open children", async () => {
      const shown = morningReport();
      shown.can_record_morning_attendance = false;
      shown.rows[1]!.morning_attendance = {
        status: "present",
        source: "teacher",
      };
      render(
        <ClassDayClass
          schoolClass="4a"
          fetchClassDay={() => Promise.resolve(shown)}
          recordMorningAttendance={() => Promise.resolve()}
        />,
      );

      const child = (await screen.findByText("Krank, Nico")).closest("li")!;
      expect(within(child).getByText("Vormittag: Da")).toBeInTheDocument();
      expect(within(child).getByText("Krank")).toBeInTheDocument();
      // Anker: die geladene Zeile zeigt den gespeicherten Stand und die Meldung.
      expect(within(child).queryByRole("button")).not.toBeInTheDocument();
      const openChild = screen.getByText("Klassentag, Klara").closest("li")!;
      // Anker: die offene Zeile zeigt weiterhin die OGS-Anmeldung.
      expect(within(openChild).getByText("Ganztag")).toBeInTheDocument();
      expect(
        within(openChild).queryByText(/Vormittag:/),
      ).not.toBeInTheDocument();
    });
  });
});

describe("ClassDayClass class trips (#4799)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockSearchParams.delete("tag");
    setTestClock(new Date("2026-09-07T12:00:00+02:00"));
  });

  const withListEntry = () =>
    report({
      rows: [
        ...report().rows,
        {
          student_id: 0,
          list_entry: true,
          list_entry_id: "77",
          first_name: "Lena",
          last_name: "Liste",
          registered: false,
          stays_today: false,
          offerings: [],
        },
      ],
    });

  it("reports a trip for the OGS children of the class and confirms it on the page", async () => {
    const reportClassTrip = vi.fn().mockResolvedValue({
      kind: "return_time",
      student_count: 1,
      date_count: 1,
    });
    render(
      <ClassDayClass
        schoolClass="4a"
        fetchClassDay={() => Promise.resolve(withListEntry())}
        fetchClasses={() =>
          Promise.resolve({
            classes: ["4a"],
            can_write_arrival_exception: false,
            can_report_class_trip: true,
          })
        }
        reportClassTrip={reportClassTrip}
      />,
    );

    fireEvent.click(
      await screen.findByRole("button", {
        name: "Ausflug oder Klassenfahrt melden",
      }),
    );
    const dialog = screen.getByRole("dialog", { name: "Ausflug melden" });
    // Only children with an OGS record can be reported.
    expect(dialog).toHaveTextContent("Klara Klassentag");
    expect(dialog).not.toHaveTextContent("Lena Liste");

    fireEvent.click(screen.getByRole("button", { name: "Melden" }));
    await waitFor(() => {
      expect(reportClassTrip).toHaveBeenCalledWith("4a", {
        studentIds: ["1"],
        from: "2026-09-07",
        to: "2026-09-07",
        returnTime: "13:30",
      });
    });
    expect(
      await screen.findByText(
        "Gespeichert. 1 Kind kommt um 13:30 Uhr in die OGS.",
      ),
    ).toBeInTheDocument();
  });

  it("offers no trip report without the release of the OGS", async () => {
    render(
      <ClassDayClass
        schoolClass="4a"
        fetchClassDay={() => Promise.resolve(report())}
        fetchClasses={() =>
          Promise.resolve({
            classes: ["4a"],
            can_write_arrival_exception: true,
            can_report_class_trip: false,
          })
        }
        reportClassTrip={vi.fn()}
      />,
    );

    expect(
      await screen.findByRole("button", { name: "Ankunft heute ändern" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", {
        name: "Ausflug oder Klassenfahrt melden",
      }),
    ).not.toBeInTheDocument();
  });
});
