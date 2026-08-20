import { render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { RoomDetailContent, RoomDetailLoader } from "./room-detail-content";

// ----------------------------------------------------------------------------
// Mocks: keep dependencies cheap so we exercise this file's own logic
// (useRoomDetail fetcher branches, mapping helpers, render conditionals,
// loader states) rather than session/SSE plumbing.
// ----------------------------------------------------------------------------

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { token: "test-token" } },
    status: "authenticated",
  }),
}));

vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: () => "test-tenant",
  // RoomDetailSkeleton + StudentsInRoomSection both call
  // useStudentPhotosEnabled, which reads from useTenantSafe. Returning a
  // null tenant keeps the photo feature off in this test (matching the
  // pre-feature shape the skeleton assertions were written against).
  useTenantSafe: () => ({ tenantSlug: "test-tenant", tenant: null }),
  useNFCEnabled: vi.fn(() => true),
}));

// The global test setup mocks `swr` to always return isLoading=true. To
// exercise useRoomDetail's actual fetcher body we mock the wrapper layer
// (`~/lib/swr`) with a tiny re-implementation that runs the fetcher and
// drives state via real React hooks.
vi.mock("~/lib/swr", async () => {
  const React = await import("react");
  return {
    useSWRAuth: <T,>(
      key: string | null,
      fetcher: () => Promise<T>,
    ): { data: T | undefined; error: unknown; isLoading: boolean } => {
      const [data, setData] = React.useState<T | undefined>(undefined);
      const [error, setError] = React.useState<unknown>(undefined);
      const [loading, setLoading] = React.useState(true);

      // Hold the fetcher in a ref so the effect can depend only on `key`
      // without lint complaints. The fetcher closure is recreated on
      // every render of useRoomDetail, so re-running the effect on each
      // identity change would loop. This mirrors the dedup behavior the
      // real SWR provides via its own cache.
      const fetcherRef = React.useRef(fetcher);
      fetcherRef.current = fetcher;

      React.useEffect(() => {
        if (key === null) {
          setLoading(false);
          return;
        }
        let cancelled = false;
        fetcherRef
          .current()
          .then((d) => {
            if (!cancelled) {
              setData(d);
              setLoading(false);
            }
          })
          .catch((e) => {
            if (!cancelled) {
              setError(e);
              setLoading(false);
            }
          });
        return () => {
          cancelled = true;
        };
      }, [key]);

      return { data, error, isLoading: loading };
    },
  };
});

// Loading widget is no longer used by RoomDetailLoader (replaced with a
// content-shaped RoomDetailSkeleton in #1323 review). The test below now
// targets the skeleton's data-testid + role="status" instead of the old
// "loading" testid. Same business assertion (a loading state IS shown
// while SWR fetches), new selector that matches the new shape.

vi.mock("~/components/ui/info-card", () => ({
  InfoCard: ({
    title,
    children,
  }: {
    title: string;
    icon?: React.ReactNode;
    children: React.ReactNode;
  }) => (
    <section data-testid={`info-card-${title}`}>
      <h2>{title}</h2>
      {children}
    </section>
  ),
  InfoItem: ({ label, value }: { label: string; value: React.ReactNode }) => (
    <div data-testid={`info-item-${label}`}>
      <span>{label}</span>: <span>{value}</span>
    </div>
  ),
}));

// The students-in-room section pulls in its own SWR + user-context tree;
// stub it out so this test stays focused on RoomDetailContent itself.
vi.mock("./students-in-room-section", () => ({
  StudentsInRoomSection: ({
    roomId,
    roomName,
  }: {
    roomId: string;
    roomName: string;
  }) => (
    <div data-testid="students-section">
      students for {roomId} ({roomName})
    </div>
  ),
}));

vi.mock("~/lib/date-helpers", () => ({
  formatDate: (ts: string) => `D:${ts}`,
  formatTime: (ts: string) => `T:${ts}`,
  calculateDuration: () => 30,
  formatDuration: (m: number) => `${m}m`,
}));

// Pull ROOM_HISTORY_STATUS_FEATURE_DISABLED (and any other future exports)
// from the real module so a backend/frontend constant rename can't silently
// desync the test. Only the formatting helpers need stubbing — keep them
// deterministic so timeline cards render without hitting transitive imports.
vi.mock("~/lib/room-helpers", async (importOriginal) => {
  const actual = await importOriginal<typeof import("~/lib/room-helpers")>();
  return {
    ...actual,
    formatFloor: (n: number) => `Etage ${n}`,
    getRoomCategoryColor: () => "#6B7280",
  };
});

// ----------------------------------------------------------------------------
// Helpers
// ----------------------------------------------------------------------------

// The mocked useSWRAuth has no shared cache, so each render starts fresh.
// A no-op wrapper keeps the JSX structure consistent with the previous
// SWRConfig-based version.
const Wrapper = ({ children }: { children: React.ReactNode }) => (
  <>{children}</>
);

type FetchResponse = { ok: boolean; json: () => Promise<unknown> };
const mockFetch = vi.fn();

const originalFetch = globalThis.fetch;
beforeEach(() => {
  mockFetch.mockReset();
  globalThis.fetch = mockFetch as unknown as typeof fetch;
});

afterEach(() => {
  globalThis.fetch = originalFetch;
});

const okJson = (body: unknown): FetchResponse => ({
  ok: true,
  json: async () => body,
});

const notOk = (): FetchResponse => ({
  ok: false,
  json: async () => ({}),
});

// ----------------------------------------------------------------------------
// useRoomDetail, exercised through the RoomDetailLoader render path.
// ----------------------------------------------------------------------------

describe("useRoomDetail (via RoomDetailLoader)", () => {
  it("maps a room with the bare-array history shape (aggregated sessions)", async () => {
    mockFetch
      .mockResolvedValueOnce(
        okJson({
          id: 1001,
          name: "Schmetterlingsraum",
          building: "Hauptgebäude",
          floor: 2,
          category: "Gruppenraum",
          is_occupied: true,
          group_name: "Schildkröten",
          supervisor_names: "Birgit Braun",
          student_count: 4,
        }),
      )
      .mockResolvedValueOnce(
        okJson([
          {
            session_id: 7001,
            started_at: "2026-04-30T08:00:00Z",
            ended_at: "2026-04-30T09:00:00Z",
            duration_minutes: 60,
            activity_name: "Freispiel",
            supervisor_name: "Birgit Braun",
            student_count: 4,
          },
        ]),
      );

    render(
      <Wrapper>
        <RoomDetailLoader roomId="bare-array" />
      </Wrapper>,
    );

    await waitFor(() =>
      expect(screen.getAllByText("Schmetterlingsraum")[0]).toBeInTheDocument(),
    );
    expect(mockFetch).toHaveBeenCalledTimes(2);
    expect(screen.getAllByText(/Hauptgebäude/).length).toBeGreaterThan(0);
    // The activity name renders inside the aggregated history card.
    expect(screen.getByText("Freispiel")).toBeInTheDocument();
    // Both the room header (supervisor_names) and the session row
    // (supervisor_name) carry "Birgit Braun".
    expect(screen.getAllByText(/Birgit Braun/).length).toBeGreaterThan(0);
  });

  it("maps a room with the wrapped history shape ({ data: [...] })", async () => {
    mockFetch
      .mockResolvedValueOnce(
        okJson({ data: { id: 2002, name: "Saal", is_occupied: false } }),
      )
      .mockResolvedValueOnce(
        okJson({
          status: "success",
          data: [
            {
              session_id: 9,
              started_at: "2026-04-29T10:00:00Z",
              ended_at: null,
              duration_minutes: null,
              activity_name: "Lesen",
              supervisor_name: "",
              student_count: 0,
            },
          ],
        }),
      );

    render(
      <Wrapper>
        <RoomDetailLoader roomId="wrapped" />
      </Wrapper>,
    );

    await waitFor(() =>
      expect(screen.getAllByText("Saal")[0]).toBeInTheDocument(),
    );
    // A session without ended_at renders the "Laufend" marker.
    expect(screen.getByText("Laufend")).toBeInTheDocument();
    expect(screen.getByText("Lesen")).toBeInTheDocument();
  });

  it("hides history when the wrapper carries data:null (#1374 regression guard)", async () => {
    mockFetch
      .mockResolvedValueOnce(
        okJson({ id: 3003, name: "Kleiner Raum", is_occupied: false }),
      )
      .mockResolvedValueOnce(
        okJson({
          status: "success",
          data: null,
          message: "no history",
        }),
      );

    render(
      <Wrapper>
        <RoomDetailLoader roomId="null-data" />
      </Wrapper>,
    );

    await waitFor(() =>
      expect(screen.getAllByText("Kleiner Raum")[0]).toBeInTheDocument(),
    );
    expect(screen.queryByText("Belegungshistorie")).not.toBeInTheDocument();
  });

  it("hides history deliberately when the tenant has gdpr.attendance_log_enabled = false", async () => {
    // Issue #1425 follow-up: the proxy route translates the backend's 403
    // into { status: "feature_disabled", data: [] }. The hook must surface
    // historyDisabled so the section is hidden EXPLICITLY (not just
    // because `data` happens to be empty). A future placeholder for the
    // empty-history case must not accidentally render here.
    mockFetch
      .mockResolvedValueOnce(
        okJson({ id: 5005, name: "GDPR-Off Raum", is_occupied: false }),
      )
      .mockResolvedValueOnce(
        okJson({
          status: "feature_disabled",
          data: [],
        }),
      );

    render(
      <Wrapper>
        <RoomDetailLoader roomId="feature-disabled" />
      </Wrapper>,
    );

    await waitFor(() =>
      expect(screen.getAllByText("GDPR-Off Raum")[0]).toBeInTheDocument(),
    );
    expect(screen.queryByText("Belegungshistorie")).not.toBeInTheDocument();
  });

  it("treats a non-OK history response as empty (does not throw)", async () => {
    mockFetch
      .mockResolvedValueOnce(
        okJson({ id: 4004, name: "Sporthalle", is_occupied: false }),
      )
      .mockResolvedValueOnce(notOk());

    render(
      <Wrapper>
        <RoomDetailLoader roomId="history-not-ok" />
      </Wrapper>,
    );

    await waitFor(() =>
      expect(screen.getAllByText("Sporthalle")[0]).toBeInTheDocument(),
    );
    expect(screen.queryByText("Belegungshistorie")).not.toBeInTheDocument();
  });

  it("renders the error placeholder when the room fetch fails", async () => {
    mockFetch.mockResolvedValueOnce(notOk());

    render(
      <Wrapper>
        <RoomDetailLoader roomId="room-fetch-fails" />
      </Wrapper>,
    );

    await waitFor(() =>
      expect(
        screen.getByText("Fehler beim Laden der Raumdaten."),
      ).toBeInTheDocument(),
    );
  });

  it("falls back through name → room_name → '' when name is missing", async () => {
    mockFetch
      .mockResolvedValueOnce(
        okJson({ id: 5005, room_name: "Garten", is_occupied: false }),
      )
      .mockResolvedValueOnce(okJson([]));

    render(
      <Wrapper>
        <RoomDetailLoader roomId="name-fallback" />
      </Wrapper>,
    );

    await waitFor(() =>
      expect(screen.getAllByText("Garten")[0]).toBeInTheDocument(),
    );
  });
});

// ----------------------------------------------------------------------------
// RoomDetailContent, render-state branches the loader-path tests don't reach.
// ----------------------------------------------------------------------------

const baseRoom = {
  id: "1",
  name: "Testraum",
  isOccupied: false,
};

describe("RoomDetailContent", () => {
  it("renders 'Frei' and hides the activity/supervisor rows when the room is unoccupied", () => {
    render(
      <Wrapper>
        <RoomDetailContent room={{ ...baseRoom }} history={[]} />
      </Wrapper>,
    );
    // "Frei" renders in the StatusBadge AND in the Rauminformationen
    // Status row, both must be present to give staff the same signal
    // wherever they look.
    expect(screen.getAllByText(/Frei/).length).toBeGreaterThanOrEqual(2);
    expect(screen.queryByText("Aktuelle Aktivität")).not.toBeInTheDocument();
    expect(screen.queryByText("Aktuelle Aufsicht")).not.toBeInTheDocument();
    expect(screen.queryByText("Aktuell anwesend")).not.toBeInTheDocument();
  });

  it("renders 'Belegt', the activity, supervisor, and student count when occupied", () => {
    render(
      <Wrapper>
        <RoomDetailContent
          room={{
            ...baseRoom,
            isOccupied: true,
            groupName: "Schildkröten",
            supervisorName: "Birgit Braun",
            studentCount: 5,
          }}
          history={[]}
        />
      </Wrapper>,
    );
    // "Belegt" appears in the badge and the Status DetailRow.
    expect(screen.getAllByText(/Belegt/).length).toBeGreaterThanOrEqual(2);
    // DetailRow pairs label + value as siblings; assert both are present.
    expect(screen.getByText("Aktuelle Aktivität")).toBeInTheDocument();
    expect(screen.getByText("Schildkröten")).toBeInTheDocument();
    expect(screen.getByText("Aktuelle Aufsicht")).toBeInTheDocument();
    expect(screen.getByText("Birgit Braun")).toBeInTheDocument();
    expect(screen.getByText("Aktuell anwesend")).toBeInTheDocument();
    // student_count !== 1 → the "Kinder" plural branch runs.
    expect(screen.getByText("5 Kinder")).toBeInTheDocument();
  });

  it("uses the singular 'Kind' when student count is exactly 1", () => {
    render(
      <Wrapper>
        <RoomDetailContent
          room={{
            ...baseRoom,
            isOccupied: true,
            groupName: "Schildkröten",
            studentCount: 1,
          }}
          history={[]}
        />
      </Wrapper>,
    );
    expect(screen.getByText("1 Kind")).toBeInTheDocument();
  });

  it("hides the supervisor and student-count rows when those fields are missing", () => {
    // Defensive: if the backend doesn't report a supervisor/count yet
    // (e.g. group is in transition), the detail block must collapse.
    // Empty rows would look like "Aktuelle Aufsicht: -" placeholders
    // which is worse UX than just not showing them.
    render(
      <Wrapper>
        <RoomDetailContent
          room={{ ...baseRoom, isOccupied: true, groupName: "Schildkröten" }}
          history={[]}
        />
      </Wrapper>,
    );
    expect(screen.getByText("Aktuelle Aktivität")).toBeInTheDocument();
    expect(screen.queryByText("Aktuelle Aufsicht")).not.toBeInTheDocument();
    expect(screen.queryByText("Aktuell anwesend")).not.toBeInTheDocument();
  });

  it("renders Gebäude / Etage / Kategorie rows but NOT a separate Raumname row", () => {
    // Raumname intentionally omitted from the detail block. Review
    // feedback (#1323): the room name is already the h1 above the
    // detail card, so a "Raumname: …" row was pure redundancy. The
    // remaining static fields must still all render.
    render(
      <Wrapper>
        <RoomDetailContent
          room={{
            ...baseRoom,
            building: "Hauptgebäude",
            floor: 2,
            category: "Gruppenraum",
          }}
          history={[]}
        />
      </Wrapper>,
    );
    expect(screen.queryByText("Raumname")).not.toBeInTheDocument();
    expect(screen.getByText("Gebäude")).toBeInTheDocument();
    expect(screen.getByText("Hauptgebäude")).toBeInTheDocument();
    expect(screen.getByText("Etage")).toBeInTheDocument();
    expect(screen.getByText("Kategorie")).toBeInTheDocument();
    expect(screen.getByText("Gruppenraum")).toBeInTheDocument();
  });

  it("does not render the legacy building · floor · category subline under the header", () => {
    // The "Hauptgebäude · Etage 2" subline that used to sit under the
    // h1 was removed (#1323). Those values now live exclusively in
    // the IconDetailRow list, so showing them twice was clutter.
    render(
      <Wrapper>
        <RoomDetailContent
          room={{
            ...baseRoom,
            building: "Hauptgebäude",
            floor: 2,
            category: "Gruppenraum",
          }}
          history={[]}
        />
      </Wrapper>,
    );
    expect(
      screen.queryByText(/Hauptgebäude · Etage 2/),
    ).not.toBeInTheDocument();
  });

  it("renders the building or the floor in the detail block when only one is present", () => {
    const { rerender } = render(
      <Wrapper>
        <RoomDetailContent
          room={{ ...baseRoom, building: "NurGebäude" }}
          history={[]}
        />
      </Wrapper>,
    );
    expect(screen.getAllByText("NurGebäude")[0]).toBeInTheDocument();

    rerender(
      <Wrapper>
        <RoomDetailContent room={{ ...baseRoom, floor: 3 }} history={[]} />
      </Wrapper>,
    );
    expect(screen.getAllByText("Etage 3")[0]).toBeInTheDocument();
  });

  it("hides the history section when there are no entries", () => {
    render(
      <Wrapper>
        <RoomDetailContent room={{ ...baseRoom }} history={[]} />
      </Wrapper>,
    );
    expect(screen.queryByText("Belegungshistorie")).not.toBeInTheDocument();
  });

  it("renders an aggregated session as a single card with the formatted duration", () => {
    render(
      <Wrapper>
        <RoomDetailContent
          room={{ ...baseRoom }}
          history={[
            {
              sessionId: "a",
              startedAt: "2026-04-30T08:00:00Z",
              endedAt: "2026-04-30T08:45:00Z",
              durationMinutes: 45,
              activityName: "Mathe",
              supervisorName: "Frau A",
              studentCount: 12,
            },
          ]}
        />
      </Wrapper>,
    );
    expect(screen.getByText("Mathe")).toBeInTheDocument();
    // duration_minutes from the session takes precedence over calculateDuration().
    expect(screen.getByText("45m")).toBeInTheDocument();
    expect(screen.getByText(/Frau A/)).toBeInTheDocument();
    expect(screen.getByText(/Kinder: 12/)).toBeInTheDocument();
    expect(screen.queryByText("Laufend")).not.toBeInTheDocument();
  });

  it("marks a session without ended_at as 'Laufend'", () => {
    render(
      <Wrapper>
        <RoomDetailContent
          room={{ ...baseRoom }}
          history={[
            {
              sessionId: "a",
              startedAt: "2026-04-30T08:00:00Z",
              endedAt: null,
              durationMinutes: null,
              activityName: "Sport",
              supervisorName: "",
              studentCount: 0,
            },
          ]}
        />
      </Wrapper>,
    );
    expect(screen.getByText("Laufend")).toBeInTheDocument();
  });

  it("focuses the room title when rendered as drawer content", async () => {
    render(
      <Wrapper>
        <RoomDetailContent
          room={{ ...baseRoom }}
          history={[]}
          headerAction={<button type="button">Schließen</button>}
        />
      </Wrapper>,
    );

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "Testraum" })).toHaveFocus();
    });
  });

  it("sorts history days newest first and sessions within a day by start time DESC", () => {
    render(
      <Wrapper>
        <RoomDetailContent
          room={{ ...baseRoom }}
          history={[
            {
              sessionId: "a",
              startedAt: "2026-04-29T09:30:00Z",
              endedAt: null,
              durationMinutes: null,
              activityName: "Mittag",
              supervisorName: "",
              studentCount: 0,
            },
            {
              sessionId: "b",
              startedAt: "2026-04-30T08:00:00Z",
              endedAt: null,
              durationMinutes: null,
              activityName: "Neuer Tag",
              supervisorName: "",
              studentCount: 0,
            },
            {
              sessionId: "c",
              startedAt: "2026-04-29T08:00:00Z",
              endedAt: null,
              durationMinutes: null,
              activityName: "Morgens",
              supervisorName: "",
              studentCount: 0,
            },
          ]}
        />
      </Wrapper>,
    );

    const activityNames = screen
      .getAllByRole("heading", { level: 4 })
      .map((heading) => heading.textContent);
    // Newest day first, within a day newest session first.
    expect(activityNames).toEqual(["Neuer Tag", "Mittag", "Morgens"]);
  });
});

// ----------------------------------------------------------------------------
// RoomDetailLoader states
// ----------------------------------------------------------------------------

describe("RoomDetailLoader states", () => {
  it("renders the content-shaped skeleton while the SWR fetch is in flight", () => {
    // fetch returns a never-resolving promise → SWR stays in loading state.
    // The skeleton mirrors the three-card content layout so the slide-over
    // body doesn't visibly resize when real data arrives (#1323 review).
    mockFetch.mockReturnValue(new Promise(() => {}));

    render(
      <Wrapper>
        <RoomDetailLoader roomId="loading-state" />
      </Wrapper>,
    );

    expect(screen.getByTestId("room-detail-skeleton")).toBeInTheDocument();
    // role="status" + aria-label preserves the AT-announcement contract
    // the previous Loading widget provided.
    expect(
      screen.getByRole("status", { name: "Raumdetails werden geladen" }),
    ).toBeInTheDocument();
  });

  it("renders the supplied emptyAction below the error message on fetch failure", async () => {
    mockFetch.mockResolvedValueOnce(notOk());

    render(
      <Wrapper>
        <RoomDetailLoader
          roomId="empty-action"
          emptyAction={<button type="button">Zurück</button>}
        />
      </Wrapper>,
    );

    await waitFor(() =>
      expect(
        screen.getByText("Fehler beim Laden der Raumdaten."),
      ).toBeInTheDocument(),
    );
    expect(screen.getByRole("button", { name: "Zurück" })).toBeInTheDocument();
  });
});
