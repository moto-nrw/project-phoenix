import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { ClassDayReport } from "~/lib/class-day-api";
import type { SchoolDayReport } from "~/lib/school-class-day-api";
import { setTestClock } from "~/test/clock";
import { ClassDayOverview } from "./class-day-overview";

// Die Übersicht lädt am Wochenende keine Klasse, außer die Schule betreut
// Samstag und Sonntag nach dem Freitagsplan (#3921).

const navigation = vi.hoisted(() => ({ search: "", replace: vi.fn() }));

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(navigation.search),
  useRouter: () => ({ push: vi.fn(), replace: navigation.replace }),
}));

vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: null, status: "authenticated" }),
}));

vi.mock("~/lib/school-url", () => ({
  schoolPath: (path: string) => path,
}));

vi.mock("~/lib/swr", async () => {
  const React = await import("react");
  return {
    useSWRAuth: <T,>(key: string | null, fetcher: () => Promise<T>) => {
      const [data, setData] = React.useState<T | undefined>(undefined);
      const [loading, setLoading] = React.useState(true);
      const fetcherRef = React.useRef(fetcher);
      fetcherRef.current = fetcher;
      const load = React.useCallback(() => {
        if (key === null) {
          setLoading(false);
          return Promise.resolve(undefined);
        }
        return fetcherRef.current().then((d) => {
          setData(d);
          setLoading(false);
          return d;
        });
      }, [key]);
      React.useEffect(() => {
        void load();
      }, [load]);
      return { data, error: undefined, isLoading: loading, mutate: load };
    },
  };
});

function report(date: string): ClassDayReport {
  return {
    school_class: "4a",
    date,
    weekday: "fri",
    school_day: true,
    enrollment_known: true,
    totals: { students: 1, staying: 1, leaving: 0, absent: 0, list_entries: 0 },
    rows: [],
  };
}

function classList(weekendFollowsFriday: boolean) {
  return {
    classes: ["4a"],
    can_write_arrival_exception: false,
    weekend_follows_friday: weekendFollowsFriday,
  };
}

describe("ClassDayOverview on the weekend", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    navigation.search = "";
    setTestClock(new Date("2026-09-05T12:00:00+02:00")); // Samstag
  });

  it("loads no class without the weekend plan", async () => {
    const fetchClassDay = vi.fn((_: string, date: string) =>
      Promise.resolve(report(date)),
    );
    render(
      <ClassDayOverview
        fetchClasses={() => Promise.resolve(classList(false))}
        fetchClassDay={fetchClassDay}
      />,
    );

    expect(await screen.findByText(/Kein Schultag|Wochenende/)).toBeVisible();
    expect(fetchClassDay).not.toHaveBeenCalled();
  });

  it("loads the classes when the weekend follows Friday's plan", async () => {
    const fetchClassDay = vi.fn((_: string, date: string) =>
      Promise.resolve(report(date)),
    );
    const fetchClasses = vi.fn(() => Promise.resolve(classList(true)));
    render(
      <ClassDayOverview
        fetchClasses={fetchClasses}
        fetchClassDay={fetchClassDay}
      />,
    );

    await waitFor(() =>
      expect(fetchClassDay).toHaveBeenCalledWith("4a", "2026-09-05"),
    );
    expect(fetchClasses).toHaveBeenCalledTimes(1);
  });
});

const schoolTotals = {
  students: 0,
  missing: 0,
  absent: 0,
  sick: 0,
  excused: 0,
  class_trip: 0,
  staying: 0,
  leaving: 0,
  list_entries: 0,
  changes: 0,
};

function schoolDay(date: string): SchoolDayReport {
  return {
    date,
    weekday: "wed",
    school_day: true,
    enrollment_known: true,
    totals: {
      ...schoolTotals,
      students: 3,
      missing: 1,
      absent: 1,
      sick: 1,
      staying: 1,
      leaving: 1,
    },
    classes: [
      {
        school_class: "1a",
        totals: {
          ...schoolTotals,
          students: 2,
          missing: 1,
          absent: 1,
          sick: 1,
          staying: 1,
          changes: 1,
        },
        children: [
          {
            student_id: "1",
            first_name: "Anna",
            last_name: "Arndt",
            after_school: "stays",
          },
          {
            student_id: "2",
            first_name: "Ben",
            last_name: "Busch",
            status: "sick",
            missing: true,
            changed: true,
          },
        ],
      },
      {
        school_class: "2b",
        totals: { ...schoolTotals, students: 1, leaving: 1 },
        children: [
          {
            student_id: "3",
            first_name: "Carl",
            last_name: "Claus",
            after_school: "goes_home",
          },
        ],
      },
    ],
  };
}

// Die Schulverwaltung sieht alle Klassen (#4802): die Startseite lädt die
// Tagesübersicht der Schule einmal statt einen Report je Klasse, und jede
// Zahl öffnet die Kinder dahinter, nach Klasse gruppiert.
describe("ClassDayOverview for the Schulverwaltung", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    navigation.search = "";
    setTestClock(new Date("2026-09-09T12:00:00+02:00")); // Mittwoch
  });

  const classes = () =>
    Promise.resolve({
      classes: ["1a", "2b"],
      can_write_arrival_exception: false,
      can_view_school_day: true,
    });

  it("loads the school once instead of one report per class", async () => {
    const fetchClassDay = vi.fn((_: string, date: string) =>
      Promise.resolve(report(date)),
    );
    const fetchSchoolDay = vi.fn((date: string) =>
      Promise.resolve(schoolDay(date)),
    );
    render(
      <ClassDayOverview
        fetchClasses={classes}
        fetchClassDay={fetchClassDay}
        fetchSchoolDay={fetchSchoolDay}
      />,
    );

    expect(await screen.findByText("Ganze Schule")).toBeVisible();
    expect(fetchSchoolDay).toHaveBeenCalledWith("2026-09-09");
    expect(fetchClassDay).not.toHaveBeenCalled();
    expect(
      screen.getByRole("link", { name: "Krank: 1, Liste öffnen" }),
    ).toHaveAttribute("href", "/school?tag=2026-09-09&liste=krank");
    // Die Klassenzeilen kommen aus derselben Antwort.
    expect(screen.getByText("Alle Klassen")).toBeVisible();
    expect(
      screen.getByText(/2 Kinder · 1 bleiben · 0 gehen heim/),
    ).toBeVisible();
    expect(screen.getByText("1 Kind anders als sonst")).toBeVisible();
  });

  it("lists the children of an opened number by class", async () => {
    navigation.search = "tag=2026-09-09&liste=gehen-heim";
    render(
      <ClassDayOverview
        fetchClasses={classes}
        fetchClassDay={(_, date) => Promise.resolve(report(date))}
        fetchSchoolDay={(date) => Promise.resolve(schoolDay(date))}
      />,
    );

    expect(await screen.findByText("Gehen heim: 1")).toBeVisible();
    expect(screen.getByText("Claus, Carl")).toBeVisible();
    expect(screen.getByRole("link", { name: /2b · 1/ })).toHaveAttribute(
      "href",
      "/school/klasse?klasse=2b&tag=2026-09-09&liste=gehen-heim",
    );
    expect(screen.getByRole("link", { name: /2b 1 Kinder/ })).toHaveAttribute(
      "href",
      "/school/klasse?klasse=2b&tag=2026-09-09&liste=gehen-heim",
    );
    expect(screen.queryByText("Busch, Ben")).not.toBeInTheDocument();
  });

  it("keeps the opened list when the day changes", async () => {
    navigation.search = "tag=2026-09-09&liste=krank";
    render(
      <ClassDayOverview
        fetchClasses={classes}
        fetchClassDay={(_, date) => Promise.resolve(report(date))}
        fetchSchoolDay={(date) => Promise.resolve(schoolDay(date))}
      />,
    );

    expect(await screen.findByText("Krank: 1")).toBeVisible();
    fireEvent.click(screen.getByRole("button", { name: "Vorheriger Tag" }));
    expect(navigation.replace).toHaveBeenCalledWith(
      "/school?tag=2026-09-08&liste=krank",
      { scroll: false },
    );
  });

  it("keeps the per-class reports for a Lehrkraft", async () => {
    const fetchSchoolDay = vi.fn((date: string) =>
      Promise.resolve(schoolDay(date)),
    );
    const fetchClassDay = vi.fn((_: string, date: string) =>
      Promise.resolve(report(date)),
    );
    render(
      <ClassDayOverview
        fetchClasses={() =>
          Promise.resolve({
            classes: ["4a"],
            can_write_arrival_exception: false,
          })
        }
        fetchClassDay={fetchClassDay}
        fetchSchoolDay={fetchSchoolDay}
      />,
    );

    await waitFor(() =>
      expect(fetchClassDay).toHaveBeenCalledWith("4a", "2026-09-09"),
    );
    expect(screen.getByText("Ihre Klassen")).toBeVisible();
    expect(fetchSchoolDay).not.toHaveBeenCalled();
  });
});
