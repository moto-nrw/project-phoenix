import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { ClassDayReport } from "~/lib/class-day-api";
import { setTestClock } from "~/test/clock";
import { ClassDayOverview } from "./class-day-overview";

// Die Übersicht lädt am Wochenende keine Klasse, außer die Schule betreut
// Samstag und Sonntag nach dem Freitagsplan (#3921).

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(),
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
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
