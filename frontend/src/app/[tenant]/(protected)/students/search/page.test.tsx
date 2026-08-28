import {
  render,
  screen,
  fireEvent,
  waitFor,
  cleanup,
  act,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { TrackingIndicatorsResponse } from "~/lib/active-helpers";
import { roomService } from "~/lib/api";
import StudentSearchPage from "./page";

const STUDENT_SEARCH_FILTER_STORAGE_KEY =
  "student-search:last-filters:tenant-7:user-1";

// Mock next-auth/react
vi.mock("next-auth/react", () => ({
  useSession: vi.fn(() => ({
    data: { user: { id: "user-1", tenantId: 7, token: "test-token" } },
    status: "authenticated",
  })),
}));

// Mock next/navigation
const mockSearchParams = new URLSearchParams();
vi.mock("next/navigation", () => ({
  useRouter: vi.fn(() => ({
    push: vi.fn(),
    replace: vi.fn(),
  })),
  useSearchParams: vi.fn(() => mockSearchParams),
}));

// Mock breadcrumb context
vi.mock("~/lib/breadcrumb-context", () => ({
  useSetBreadcrumb: vi.fn(),
  useBreadcrumb: vi.fn(() => ({ breadcrumb: {}, setBreadcrumb: vi.fn() })),
  BreadcrumbProvider: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
}));

// Mock Alert component
vi.mock("~/components/ui/alert", () => ({
  Alert: ({ message, type }: { message: string; type: string }) => (
    <div data-testid={`alert-${type}`}>{message}</div>
  ),
}));

// Mock OverflowMenu: das Aktionsmenü der Seite sitzt seit der Kopfkarte in
// PageIntro und nicht mehr in PageHeaderWithSearch; die Testids bleiben.
vi.mock("~/components/ui/page-header/OverflowMenu", () => ({
  OverflowMenu: ({
    items,
  }: {
    items: Array<{ label?: string; onClick?: () => void }>;
  }) => (
    <div data-testid="overflow-menu">
      {items
        .filter((item) => item.label)
        .map((item) => (
          <button
            type="button"
            key={item.label}
            data-testid={`overflow-${item.label}`}
            onClick={item.onClick}
          >
            {item.label}
          </button>
        ))}
    </div>
  ),
}));

// Mock PageHeaderWithSearch
vi.mock("~/components/ui/page-header/PageHeaderWithSearch", () => ({
  PageHeaderWithSearch: ({
    filters,
    activeFilters,
    onClearAllFilters,
    search,
    overflowMenu,
  }: {
    filters: Array<{
      id: string;
      value: string | string[];
      onChange: (v: string | string[]) => void;
      options?: Array<{ value: string; label: string }>;
      multiSelect?: boolean;
    }>;
    activeFilters: Array<{ id: string; label: string }>;
    onClearAllFilters: () => void;
    search: { value: string; onChange: (v: string) => void };
    overflowMenu?: Array<{ label: string; onClick: () => void }>;
  }) => (
    <div data-testid="page-header">
      <input
        data-testid="search-input"
        value={search.value}
        onChange={(e) => search.onChange(e.target.value)}
      />
      {filters.map((f) => (
        <select
          key={f.id}
          data-testid={`filter-${f.id}`}
          multiple={f.multiSelect}
          value={f.value}
          onChange={(e) =>
            f.onChange(
              f.multiSelect
                ? Array.from(e.target.selectedOptions, (option) => option.value)
                : e.target.value,
            )
          }
        >
          {f.options ? (
            f.options.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))
          ) : (
            <>
              <option value="all">All</option>
              <option value="anwesend">Anwesend</option>
              <option value="abwesend">Abwesend</option>
              <option value="unterwegs">Unterwegs</option>
              <option value="schulhof">Schulhof</option>
            </>
          )}
        </select>
      ))}
      <div data-testid="active-filters">
        {activeFilters.map(
          (f: { id: string; label: string; onRemove?: () => void }) => (
            <button
              type="button"
              key={f.id}
              data-testid={`active-filter-${f.id}`}
              onClick={f.onRemove}
            >
              {f.label}
            </button>
          ),
        )}
      </div>
      <button
        type="button"
        data-testid="clear-filters"
        onClick={onClearAllFilters}
      >
        Clear
      </button>
      {(overflowMenu ?? []).map((item) => (
        <button
          type="button"
          key={item.label}
          data-testid={`overflow-${item.label}`}
          onClick={item.onClick}
        >
          {item.label}
        </button>
      ))}
    </div>
  ),
}));

// Mock the export modal so a test can capture the filters prop it receives
// and confirm no active filter is dropped on the way to the export payload.
vi.mock("~/components/students/student-export-modal", () => ({
  StudentExportModal: ({
    isOpen,
    filters,
  }: {
    isOpen: boolean;
    filters: Record<string, string>;
  }) =>
    isOpen ? (
      <div data-testid="export-modal" data-filters={JSON.stringify(filters)} />
    ) : null,
}));

// Mock StudentPresenceBadge: wrapper the page now renders instead of
// the bare LocationBadge, so binary-mode tenants can hide detailed labels.
vi.mock("@/components/ui/student-presence-badge", () => ({
  StudentPresenceBadge: ({
    student,
  }: {
    student: { current_location: string };
  }) => <span data-testid="location-badge">{student.current_location}</span>,
}));

// Mock LocationBadge (still referenced transitively by any non-mocked paths)
vi.mock("@/components/ui/location-badge", () => ({
  LocationBadge: ({ student }: { student: { current_location: string } }) => (
    <span data-testid="location-badge">{student.current_location}</span>
  ),
}));

// Mock location helpers: LOCATION_COLORS is consumed by student-card.tsx
// (check-in mode tint), so the mock must expose the brand palette even if
// individual tests don't assert on colors. Partial mock via importOriginal
// so MOTO_COLOR_PALETTE (consumed transitively by MotoDuotoneIcon, now
// reached through DetailIcons in students/search/page.tsx) stays available.
vi.mock("~/lib/location-helper", async (importOriginal) => {
  const actual = await importOriginal<typeof import("~/lib/location-helper")>();
  return {
    ...actual,
    isHomeLocation: (loc: string) => loc === "Zuhause" || loc === "",
    isPresentLocation: (loc: string) =>
      loc !== "Zuhause" &&
      loc !== "" &&
      loc !== "Unterwegs" &&
      loc !== "Schulhof",
    isTransitLocation: (loc: string) => loc === "Unterwegs",
    isSchoolyardLocation: (loc: string) => loc === "Schulhof",
    LOCATION_COLORS: {
      GROUP_ROOM: "#83CD2D",
      OTHER_ROOM: "#5080D8",
      HOME: "#DC2626",
      SCHOOLYARD: "#F78C10",
      TRANSIT: "#D946EF",
      UNKNOWN: "#6B7280",
      SICK: "#EAB308",
      EXCUSED: "#7C3AED",
    },
    LOCATION_STATUSES: {
      PRESENT: "Anwesend",
      HOME: "Zuhause",
      SCHOOLYARD: "Schulhof",
      TRANSIT: "Unterwegs",
      UNKNOWN: "Unbekannt",
      SICK: "Krank",
      EXCUSED: "Entschuldigt",
    },
  };
});

// Mock school-checkin FAB + hook so existing search tests aren't
// responsible for the new floating mode trigger;
// page.school-checkin.test.tsx covers it dedicatedly.
vi.mock("~/components/students/school-checkin-fab", () => ({
  SchoolCheckinFab: () => <div data-testid="school-checkin-fab" />,
}));

vi.mock("~/lib/hooks/use-school-checkin-mode", () => ({
  useSchoolCheckinMode: () => ({
    isActive: false,
    toggleActive: vi.fn(),
    deactivate: vi.fn(),
    pendingIds: new Set<string>(),
    successCount: 0,
    toggle: vi.fn(),
    selectionActive: false,
    setSelectionActive: vi.fn(),
    selectedIds: new Set<string>(),
    toggleSelected: vi.fn(),
    clearSelection: vi.fn(),
    isBulkRunning: false,
    runBulk: vi.fn(),
  }),
  deriveCheckinState: () => "unknown",
  checkoutConfirmationRoom: () => null,
}));

// Mock student-helpers
vi.mock("~/lib/student-helpers", async (importOriginal) => {
  const actual = await importOriginal<typeof import("~/lib/student-helpers")>();
  return {
    ...actual,
    SCHOOL_YEAR_FILTER_OPTIONS: [
      { value: "all", label: "Alle" },
      { value: "1", label: "1" },
      { value: "2", label: "2" },
      { value: "3", label: "3" },
      { value: "4", label: "4" },
    ],
  };
});

// Mock SSE hook
vi.mock("~/lib/hooks/use-sse", () => ({
  useSSE: vi.fn(() => ({
    status: "connected",
    isConnected: true,
    error: null,
  })),
}));

// Mock SWR hooks - configured per test in beforeEach
vi.mock("~/lib/swr", () => ({
  useImmutableSWR: vi.fn(),
  useSWRAuth: vi.fn(),
  mutate: vi.fn(),
  useTenantMutate: vi.fn(() => vi.fn()),
}));

// Mock API services
const mockStudents = [
  {
    id: "1",
    first_name: "Max",
    second_name: "Mustermann",
    school_class: "1a",
    group_name: "Gruppe A",
    current_location: "Raum 101",
    arrival_time: "08:00",
    pickup_time: "15:30",
    pickup_status: "Geht alleine nach Hause",
    departure_modes: ["alone"],
    bus: true,
    photo_consent_given: true,
    photo_consent_given_at: "2026-01-01T10:00:00Z",
    has_full_access: true,
  },
  {
    id: "2",
    first_name: "Anna",
    second_name: "Schmidt",
    school_class: "2b",
    group_name: "Gruppe B",
    current_location: "Zuhause",
    arrival_time: "08:30",
    pickup_status: "Wird abgeholt",
    departure_modes: ["pickup"],
    bus: false,
    photo_consent_given: false,
    has_full_access: true,
  },
  {
    id: "3",
    first_name: "Tom",
    second_name: "Weber",
    school_class: "1a",
    group_name: "Gruppe A",
    current_location: "Unterwegs",
    arrival_time: "08:15",
    pickup_time: "16:00",
    pickup_status: "Geht alleine nach Hause",
    departure_modes: ["alone"],
    bus: true,
    photo_consent_given: false,
    has_full_access: true,
  },
  {
    id: "4",
    first_name: "Lisa",
    second_name: "Müller",
    school_class: "3c",
    group_name: "Gruppe C",
    current_location: "Schulhof",
    arrival_time: "09:00",
    pickup_status: "Wird abgeholt",
    departure_modes: ["pickup"],
    bus: false,
    photo_consent_given: true,
    photo_consent_given_at: "2026-01-02T10:00:00Z",
    has_full_access: true,
  },
];

vi.mock("~/lib/api", () => ({
  studentService: {
    getStudents: vi.fn(() =>
      Promise.resolve({
        students: mockStudents,
      }),
    ),
  },
  groupService: {
    getGroups: vi.fn(() =>
      Promise.resolve([
        { id: "1", name: "Gruppe A" },
        { id: "2", name: "Gruppe B" },
        { id: "3", name: "Gruppe C" },
      ]),
    ),
  },
  roomService: {
    getRooms: vi.fn(() =>
      Promise.resolve([
        { id: "101", name: "Raum 101", isOccupied: true },
        { id: "102", name: "Raum 102", isOccupied: false },
      ]),
    ),
  },
}));

vi.mock("~/lib/usercontext-api", () => ({
  userContextService: {
    getMyEducationalGroups: vi.fn(() => Promise.resolve([])),
    getMySupervisedGroups: vi.fn(() => Promise.resolve([])),
  },
}));

// Selects several options of a multi-select filter (#2218) and fires the one
// change event the real control would. fireEvent.change with a plain `value`
// can only ever select a single option.
function selectFilterOptions(select: HTMLElement, values: string[]) {
  const element = select as HTMLSelectElement;
  for (const option of Array.from(element.options)) {
    option.selected = values.includes(option.value);
  }
  fireEvent.change(element);
}

function mockUseSWRAuthWithStudents(
  swrModule: typeof import("~/lib/swr"),
  response: ReturnType<typeof swrModule.useSWRAuth>,
) {
  vi.mocked(swrModule.useSWRAuth).mockImplementation((key) => {
    if (key === "search-rooms-list") {
      return {
        data: [
          { id: "101", name: "Raum 101", isOccupied: true },
          { id: "102", name: "Raum 102", isOccupied: false },
        ],
        isLoading: false,
        error: null,
      } as ReturnType<typeof swrModule.useSWRAuth>;
    }

    if (typeof key === "string" && key.startsWith("tracking-indicators-")) {
      return {
        data: undefined,
        isLoading: false,
        error: null,
      } as ReturnType<typeof swrModule.useSWRAuth>;
    }

    return response;
  });
}

// Mocks useSWRAuth so the students SWR fetcher is captured (not auto-run) and
// returns it. Lets a test execute the fetcher itself and assert which filters
// the page forwards to studentService.getStudents, the real backend contract
// for the #1492 administrative filters.
function captureStudentsFetcher(swrModule: typeof import("~/lib/swr")) {
  const captured: { current: (() => Promise<unknown>) | null } = {
    current: null,
  };
  vi.mocked(swrModule.useSWRAuth).mockImplementation((key, fetcher) => {
    if (
      typeof key === "string" &&
      key.startsWith("search-students-") &&
      fetcher
    ) {
      captured.current = fetcher as () => Promise<unknown>;
    }
    if (typeof key === "string" && key.startsWith("tracking-indicators-")) {
      return {
        data: undefined,
        isLoading: false,
        error: null,
      } as ReturnType<typeof swrModule.useSWRAuth>;
    }
    if (key === "search-rooms-list") {
      return {
        data: [],
        isLoading: false,
        error: null,
      } as ReturnType<typeof swrModule.useSWRAuth>;
    }
    return {
      data: { students: mockStudents },
      isLoading: false,
      error: null,
    } as ReturnType<typeof swrModule.useSWRAuth>;
  });
  return captured;
}

describe("StudentSearchPage", () => {
  beforeEach(async () => {
    vi.clearAllMocks();
    mockSearchParams.delete("status");
    mockSearchParams.delete("year");
    mockSearchParams.delete("group_id");
    mockSearchParams.delete("school_class");
    mockSearchParams.delete("room_id");
    mockSearchParams.delete("room_name");
    mockSearchParams.delete("bus");
    mockSearchParams.delete("photo_consent");
    mockSearchParams.delete("pickup_status");
    mockSearchParams.delete("pickup_time");
    mockSearchParams.delete("arrival_time");
    mockSearchParams.delete("day_status");
    mockSearchParams.delete("tracking");
    mockSearchParams.delete("sort");
    mockSearchParams.delete("view");
    localStorage.clear();
    window.history.replaceState(null, "", "/students/search");

    // Reset useSession mock to authenticated state
    const sessionModule = await import("next-auth/react");
    vi.mocked(sessionModule.useSession).mockReturnValue({
      data: {
        user: { id: "user-1", tenantId: 7, token: "test-token" },
        expires: "2099-01-01",
      },
      status: "authenticated",
      update: vi.fn(),
    } as unknown as ReturnType<typeof sessionModule.useSession>);

    const tenantProvider = await import("~/lib/tenant-context");
    vi.mocked(tenantProvider.useTenantSafe).mockReturnValue({
      tenantSlug: "test-tenant",
      tenant: { studentPhotosEnabled: true },
    } as unknown as ReturnType<typeof tenantProvider.useTenantSafe>);

    // Reset SWR mock data for each test
    const swrModule = await import("~/lib/swr");
    vi.mocked(swrModule.useImmutableSWR).mockImplementation((key) => {
      if (key === "search-groups-list") {
        return {
          data: [
            { id: "1", name: "Gruppe A" },
            { id: "2", name: "Gruppe B" },
            { id: "3", name: "Gruppe C" },
          ],
          isLoading: false,
          error: null,
        } as ReturnType<typeof swrModule.useImmutableSWR>;
      }

      return {
        data: [],
        isLoading: false,
        error: null,
      } as ReturnType<typeof swrModule.useImmutableSWR>;
    });
    mockUseSWRAuthWithStudents(swrModule, {
      data: { students: mockStudents },
      isLoading: false,
      error: null,
    } as ReturnType<typeof swrModule.useSWRAuth>);
  });

  afterEach(() => {
    cleanup();
    localStorage.clear();
  });

  describe("URL Parameter Handling", () => {
    it("defaults to 'all' when no status param is present", async () => {
      render(<StudentSearchPage />);

      await waitFor(() => {
        const attendanceFilter = screen.getByTestId("filter-attendance");
        expect(attendanceFilter).toHaveValue("all");
      });
    });

    it("reads day_status from URL params", async () => {
      mockSearchParams.set("day_status", "comes_today");

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByTestId("filter-dayStatus")).toHaveValue(
          "comes_today",
        );
      });
    });

    it("reads 'anwesend' status from URL params", async () => {
      mockSearchParams.set("status", "anwesend");

      render(<StudentSearchPage />);

      await waitFor(() => {
        const attendanceFilter = screen.getByTestId("filter-attendance");
        expect(attendanceFilter).toHaveValue("anwesend");
      });
    });

    it("reads 'unterwegs' status from URL params", async () => {
      mockSearchParams.set("status", "unterwegs");

      render(<StudentSearchPage />);

      await waitFor(() => {
        const attendanceFilter = screen.getByTestId("filter-attendance");
        expect(attendanceFilter).toHaveValue("unterwegs");
      });
    });

    it("reads 'schulhof' status from URL params", async () => {
      mockSearchParams.set("status", "schulhof");

      render(<StudentSearchPage />);

      await waitFor(() => {
        const attendanceFilter = screen.getByTestId("filter-attendance");
        expect(attendanceFilter).toHaveValue("schulhof");
      });
    });

    it("reads 'abwesend' status from URL params", async () => {
      mockSearchParams.set("status", "abwesend");

      render(<StudentSearchPage />);

      await waitFor(() => {
        const attendanceFilter = screen.getByTestId("filter-attendance");
        expect(attendanceFilter).toHaveValue("abwesend");
      });
    });

    it("reads 'entschuldigt' status from URL params", async () => {
      mockSearchParams.set("status", "entschuldigt");

      render(<StudentSearchPage />);

      await waitFor(() => {
        const attendanceFilter = screen.getByTestId("filter-attendance");
        expect(attendanceFilter).toHaveValue("entschuldigt");
      });
    });

    it("falls back to 'all' for invalid status param", async () => {
      mockSearchParams.set("status", "invalid_status");

      render(<StudentSearchPage />);

      await waitFor(() => {
        const attendanceFilter = screen.getByTestId("filter-attendance");
        expect(attendanceFilter).toHaveValue("all");
      });
    });

    it("restores non-text filters from URL params after a reload", async () => {
      mockSearchParams.set("year", "1");
      mockSearchParams.set("group_id", "1");
      mockSearchParams.set("room_id", "101");
      mockSearchParams.set("room_name", "Raum 101");
      mockSearchParams.set("bus", "yes");
      mockSearchParams.set("photo_consent", "no");
      mockSearchParams.set("pickup_status", "pickedUp");
      mockSearchParams.set("pickup_time", "15:30");
      mockSearchParams.set("arrival_time", "08:00");
      mockSearchParams.set("status", "anwesend");
      mockSearchParams.set("sort", "pickup");
      mockSearchParams.set("view", "pickup-status");

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByTestId("filter-year")).toHaveValue(["1"]);
        expect(screen.getByTestId("filter-group")).toHaveValue(["1"]);
        expect(screen.getByTestId("filter-room")).toHaveValue("101");
        expect(screen.getByTestId("filter-bus")).toHaveValue("yes");
        expect(screen.getByTestId("filter-photoConsent")).toHaveValue("no");
        expect(screen.getByTestId("filter-pickupStatus")).toHaveValue(
          "pickedUp",
        );
        expect(screen.getByTestId("filter-pickupTime")).toHaveValue("15:30");
        expect(screen.getByTestId("filter-arrivalTime")).toHaveValue("08:00");
        expect(screen.getByTestId("filter-attendance")).toHaveValue("anwesend");
        expect(screen.getByTestId("filter-sort")).toHaveValue("pickup");
        expect(screen.getByTestId("filter-groupMode")).toHaveValue(
          "pickup-status",
        );
      });
    });

    it("syncs non-text filter changes into the URL and clear-all removes them", async () => {
      window.history.replaceState({ preserved: true }, "", "/students/search");

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByTestId("filter-year")).toBeInTheDocument();
      });

      fireEvent.change(screen.getByTestId("filter-year"), {
        target: { value: "2" },
      });
      fireEvent.change(screen.getByTestId("filter-group"), {
        target: { value: "2" },
      });
      fireEvent.change(screen.getByTestId("filter-attendance"), {
        target: { value: "unterwegs" },
      });
      fireEvent.change(screen.getByTestId("filter-bus"), {
        target: { value: "yes" },
      });
      fireEvent.change(screen.getByTestId("filter-photoConsent"), {
        target: { value: "no" },
      });
      fireEvent.change(screen.getByTestId("filter-pickupStatus"), {
        target: { value: "self" },
      });
      fireEvent.change(screen.getByTestId("filter-sort"), {
        target: { value: "pickup" },
      });
      fireEvent.change(screen.getByTestId("filter-groupMode"), {
        target: { value: "pickup-status" },
      });

      let url = new URL(window.location.href);
      expect(url.searchParams.get("year")).toBe("2");
      expect(url.searchParams.get("group_id")).toBe("2");
      expect(url.searchParams.get("status")).toBe("unterwegs");
      expect(url.searchParams.get("bus")).toBe("yes");
      expect(url.searchParams.get("photo_consent")).toBe("no");
      expect(url.searchParams.get("pickup_status")).toBe("self");
      expect(url.searchParams.get("sort")).toBe("pickup");
      expect(url.searchParams.get("view")).toBe("pickup-status");
      expect(window.history.state).toEqual({ preserved: true });
      expect(
        JSON.parse(
          localStorage.getItem(STUDENT_SEARCH_FILTER_STORAGE_KEY) ?? "{}",
        ),
      ).toEqual({
        bus: "yes",
        group_id: "2",
        photo_consent: "no",
        pickup_status: "self",
        sort: "pickup",
        status: "unterwegs",
        view: "pickup-status",
        year: "2",
      });

      fireEvent.click(screen.getByTestId("clear-filters"));

      url = new URL(window.location.href);
      expect(url.searchParams.toString()).toBe("");
      expect(window.history.state).toEqual({ preserved: true });
      expect(
        localStorage.getItem(STUDENT_SEARCH_FILTER_STORAGE_KEY),
      ).toBeNull();
    });

    it("keeps day_status and administrative filters together across URL, localStorage and export", async () => {
      window.history.replaceState({ preserved: true }, "", "/students/search");

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByTestId("filter-dayStatus")).toBeInTheDocument();
      });

      fireEvent.change(screen.getByTestId("filter-dayStatus"), {
        target: { value: "comes_today" },
      });
      fireEvent.change(screen.getByTestId("filter-bus"), {
        target: { value: "yes" },
      });
      fireEvent.change(screen.getByTestId("filter-photoConsent"), {
        target: { value: "no" },
      });
      fireEvent.change(screen.getByTestId("filter-pickupStatus"), {
        target: { value: "self" },
      });

      // Day-planning filter and the administrative filters must all survive in
      // the URL, not just one group or the other.
      const url = new URL(window.location.href);
      expect(url.searchParams.get("day_status")).toBe("comes_today");
      expect(url.searchParams.get("bus")).toBe("yes");
      expect(url.searchParams.get("photo_consent")).toBe("no");
      expect(url.searchParams.get("pickup_status")).toBe("self");

      // ...and in localStorage.
      expect(
        JSON.parse(
          localStorage.getItem(STUDENT_SEARCH_FILTER_STORAGE_KEY) ?? "{}",
        ),
      ).toMatchObject({
        day_status: "comes_today",
        bus: "yes",
        photo_consent: "no",
        pickup_status: "self",
      });

      // ...and in the payload handed to the export modal.
      fireEvent.click(screen.getByTestId("overflow-Exportieren"));

      await waitFor(() => {
        expect(screen.getByTestId("export-modal")).toBeInTheDocument();
      });

      const exportFilters = JSON.parse(
        screen.getByTestId("export-modal").getAttribute("data-filters") ?? "{}",
      ) as Record<string, string>;
      expect(exportFilters).toMatchObject({
        day_status: "comes_today",
        bus: "yes",
        photo_consent: "no",
        pickup_status: "self",
      });
    });

    it("restores a multi-value class and group selection from the URL (#2218)", async () => {
      // Two groups working together are filtered as one list, so the link the
      // group leader bookmarks has to carry both values back.
      mockSearchParams.set("school_class", "1a,2b");
      mockSearchParams.set("group_id", "1,2");
      mockSearchParams.set("year", "3,4");

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByTestId("filter-schoolClass")).toBeInTheDocument();
      });

      fireEvent.click(screen.getByTestId("overflow-Exportieren"));

      await waitFor(() => {
        expect(screen.getByTestId("export-modal")).toBeInTheDocument();
      });

      const exportFilters = JSON.parse(
        screen.getByTestId("export-modal").getAttribute("data-filters") ?? "{}",
      ) as Record<string, string>;
      expect(exportFilters).toMatchObject({
        school_class: "1a,2b",
        group_id: "1,2",
        year: "3,4",
      });
    });

    it("keeps a class whose name contains the separator in one piece (#2218)", async () => {
      // school_class is free text, so a class may be called "A,B". Escaped it
      // stays one selection all the way into the export payload; unescaped it
      // would arrive as the two classes A and B and filter for neither.
      mockSearchParams.set("school_class", "A\\,B,3a");

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByTestId("filter-schoolClass")).toBeInTheDocument();
      });

      fireEvent.click(screen.getByTestId("overflow-Exportieren"));

      await waitFor(() => {
        expect(screen.getByTestId("export-modal")).toBeInTheDocument();
      });

      const exportFilters = JSON.parse(
        screen.getByTestId("export-modal").getAttribute("data-filters") ?? "{}",
      ) as Record<string, string>;
      expect(exportFilters).toMatchObject({ school_class: "A\\,B,3a" });
    });

    it("restores every occurrence of a repeated filter parameter (#2218)", async () => {
      // We write the selection comma-separated, but a hand-written link (and a
      // plain form submit) repeats the parameter instead — a shape the backend
      // accepts too. Reading only the first occurrence would drop the second
      // class, group and year without a trace.
      mockSearchParams.append("school_class", "1a");
      mockSearchParams.append("school_class", "2b");
      mockSearchParams.append("group_id", "1");
      mockSearchParams.append("group_id", "2");
      mockSearchParams.append("year", "3");
      mockSearchParams.append("year", "4");

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByTestId("filter-schoolClass")).toBeInTheDocument();
      });

      fireEvent.click(screen.getByTestId("overflow-Exportieren"));

      await waitFor(() => {
        expect(screen.getByTestId("export-modal")).toBeInTheDocument();
      });

      const exportFilters = JSON.parse(
        screen.getByTestId("export-modal").getAttribute("data-filters") ?? "{}",
      ) as Record<string, string>;
      expect(exportFilters).toMatchObject({
        school_class: "1a,2b",
        group_id: "1,2",
        year: "3,4",
      });
    });

    it("writes a multi-group selection back into the URL and the export payload (#2218)", async () => {
      window.history.replaceState({ preserved: true }, "", "/students/search");

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByTestId("filter-group")).toBeInTheDocument();
      });

      selectFilterOptions(screen.getByTestId("filter-group"), ["1", "2"]);

      expect(new URL(window.location.href).searchParams.get("group_id")).toBe(
        "1,2",
      );

      fireEvent.click(screen.getByTestId("overflow-Exportieren"));

      await waitFor(() => {
        expect(screen.getByTestId("export-modal")).toBeInTheDocument();
      });

      const exportFilters = JSON.parse(
        screen.getByTestId("export-modal").getAttribute("data-filters") ?? "{}",
      ) as Record<string, string>;
      expect(exportFilters).toMatchObject({ group_id: "1,2" });
    });

    it("restores filters from localStorage when opening without URL params", async () => {
      localStorage.setItem(
        STUDENT_SEARCH_FILTER_STORAGE_KEY,
        JSON.stringify({
          year: "2",
          group_id: "2",
          room_id: "101",
          room_name: "Raum 101",
          bus: "no",
          photo_consent: "yes",
          pickup_status: "none",
          status: "schulhof",
          sort: "arrival",
          view: "pickup-status",
        }),
      );

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByTestId("filter-year")).toHaveValue(["2"]);
        expect(screen.getByTestId("filter-group")).toHaveValue(["2"]);
        expect(screen.getByTestId("filter-room")).toHaveValue("101");
        expect(screen.getByTestId("filter-bus")).toHaveValue("no");
        expect(screen.getByTestId("filter-photoConsent")).toHaveValue("yes");
        expect(screen.getByTestId("filter-pickupStatus")).toHaveValue("none");
        expect(screen.getByTestId("filter-attendance")).toHaveValue("schulhof");
        expect(screen.getByTestId("filter-sort")).toHaveValue("arrival");
        expect(screen.getByTestId("filter-groupMode")).toHaveValue(
          "pickup-status",
        );
      });

      const url = new URL(window.location.href);
      expect(url.searchParams.get("year")).toBe("2");
      expect(url.searchParams.get("group_id")).toBe("2");
      expect(url.searchParams.get("room_id")).toBe("101");
      expect(url.searchParams.get("room_name")).toBe("Raum 101");
      expect(url.searchParams.get("bus")).toBe("no");
      expect(url.searchParams.get("photo_consent")).toBe("yes");
      expect(url.searchParams.get("pickup_status")).toBe("none");
      expect(url.searchParams.get("status")).toBe("schulhof");
    });

    it("uses URL params instead of localStorage when both are present", async () => {
      localStorage.setItem(
        STUDENT_SEARCH_FILTER_STORAGE_KEY,
        JSON.stringify({
          year: "2",
          group_id: "2",
          status: "schulhof",
        }),
      );
      mockSearchParams.set("year", "1");
      mockSearchParams.set("group_id", "1");
      mockSearchParams.set("status", "anwesend");

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByTestId("filter-year")).toHaveValue(["1"]);
        expect(screen.getByTestId("filter-group")).toHaveValue(["1"]);
        expect(screen.getByTestId("filter-attendance")).toHaveValue("anwesend");
      });

      expect(
        JSON.parse(
          localStorage.getItem(STUDENT_SEARCH_FILTER_STORAGE_KEY) ?? "{}",
        ),
      ).toEqual({
        group_id: "1",
        status: "anwesend",
        year: "1",
      });
    });

    it("ignores invalid localStorage filter data", async () => {
      localStorage.setItem(STUDENT_SEARCH_FILTER_STORAGE_KEY, "not-json");

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByTestId("filter-year")).toHaveValue([]);
        expect(screen.getByTestId("filter-group")).toHaveValue([]);
        expect(screen.getByTestId("filter-attendance")).toHaveValue("all");
      });

      expect(
        localStorage.getItem(STUDENT_SEARCH_FILTER_STORAGE_KEY),
      ).toBeNull();
    });

    it("continues with default filters when localStorage access is blocked", async () => {
      const originalLocalStorage = window.localStorage;
      const getItemSpy = vi.fn(() => {
        throw new DOMException("Blocked", "SecurityError");
      });
      const removeItemSpy = vi.fn(() => {
        throw new DOMException("Blocked", "SecurityError");
      });
      Object.defineProperty(window, "localStorage", {
        configurable: true,
        value: {
          clear: vi.fn(),
          getItem: getItemSpy,
          key: vi.fn(),
          removeItem: removeItemSpy,
          setItem: vi.fn(() => {
            throw new DOMException("Blocked", "SecurityError");
          }),
          length: 0,
        },
      });

      try {
        render(<StudentSearchPage />);

        await waitFor(() => {
          expect(screen.getByTestId("filter-year")).toHaveValue([]);
          expect(screen.getByTestId("filter-group")).toHaveValue([]);
          expect(screen.getByTestId("filter-attendance")).toHaveValue("all");
        });

        expect(getItemSpy).toHaveBeenCalled();
        expect(removeItemSpy).toHaveBeenCalled();
      } finally {
        Object.defineProperty(window, "localStorage", {
          configurable: true,
          value: originalLocalStorage,
        });
      }
    });
  });

  describe("Client-Side Filtering", () => {
    it("shows all students when filter is 'all'", async () => {
      render(<StudentSearchPage />);

      await waitFor(() => {
        // All 4 students should be visible (check by first names)
        expect(screen.getByText(new RegExp(`^Max\\b`))).toBeInTheDocument();
        expect(screen.getByText(new RegExp(`^Anna\\b`))).toBeInTheDocument();
        expect(screen.getByText(new RegExp(`^Tom\\b`))).toBeInTheDocument();
        expect(screen.getByText(new RegExp(`^Lisa\\b`))).toBeInTheDocument();
      });
    });

    it("filters to show only present students when 'anwesend' is selected", async () => {
      mockSearchParams.set("status", "anwesend");

      render(<StudentSearchPage />);

      await waitFor(() => {
        // Max (Raum 101), Tom (Unterwegs), Lisa (Schulhof) are on-site
        expect(screen.getByText(new RegExp(`^Max\\b`))).toBeInTheDocument();
        expect(screen.getByText(new RegExp(`^Tom\\b`))).toBeInTheDocument();
        expect(screen.getByText(new RegExp(`^Lisa\\b`))).toBeInTheDocument();
        // Anna (Zuhause) should be filtered out
        expect(
          screen.queryByText(new RegExp(`^Anna\\b`)),
        ).not.toBeInTheDocument();
      });
    });

    it("filters to show only home students when 'abwesend' is selected", async () => {
      mockSearchParams.set("status", "abwesend");

      render(<StudentSearchPage />);

      await waitFor(() => {
        // Only Anna (Zuhause) should be visible
        expect(screen.getByText(new RegExp(`^Anna\\b`))).toBeInTheDocument();
        expect(
          screen.queryByText(new RegExp(`^Max\\b`)),
        ).not.toBeInTheDocument();
        expect(
          screen.queryByText(new RegExp(`^Tom\\b`)),
        ).not.toBeInTheDocument();
        expect(
          screen.queryByText(new RegExp(`^Lisa\\b`)),
        ).not.toBeInTheDocument();
      });
    });

    it("does not show excused-at-home students when 'abwesend' is selected", async () => {
      const swrModule = await import("~/lib/swr");
      mockUseSWRAuthWithStudents(swrModule, {
        data: {
          students: [
            { ...mockStudents[0]!, current_location: "Zuhause", excused: true },
            { ...mockStudents[1]!, current_location: "Zuhause" },
          ],
        },
        isLoading: false,
        error: null,
      } as ReturnType<typeof swrModule.useSWRAuth>);

      mockSearchParams.set("status", "abwesend");

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByText(new RegExp(`^Anna\\b`))).toBeInTheDocument();
        expect(
          screen.queryByText(new RegExp(`^Max\\b`)),
        ).not.toBeInTheDocument();
      });
    });

    it("collapses a sick student's time rows into one neutral 'not coming' line (Variant B)", async () => {
      const swrModule = await import("~/lib/swr");
      mockUseSWRAuthWithStudents(swrModule, {
        data: {
          students: [
            {
              ...mockStudents[0]!,
              first_name: "Kerstin",
              current_location: "Zuhause",
              arrival_time: "08:00",
              pickup_time: "15:30",
              actual_arrival_time: undefined,
              sick: true,
              has_full_access: true,
            },
          ],
        },
        isLoading: false,
        error: null,
      } as ReturnType<typeof swrModule.useSWRAuth>);

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(
          screen.getByText("Kommt heute nicht (krank gemeldet)"),
        ).toBeInTheDocument();
      });
      // The absence line fills the arrival slot; the pickup row stays as a
      // neutral dash (four-row card skeleton), never as a red "overdue"
      // time for a child who isn't coming today.
      expect(screen.queryByText(/Ankunftszeit:/)).not.toBeInTheDocument();
      expect(screen.getByText("Gehzeit: –")).toBeInTheDocument();
      expect(screen.queryByText(/Gehzeit: \d/)).not.toBeInTheDocument();
    });

    it("keeps a sick checked-in student out of the overdue pickup row", async () => {
      const swrModule = await import("~/lib/swr");
      mockUseSWRAuthWithStudents(swrModule, {
        data: {
          students: [
            {
              ...mockStudents[0]!,
              first_name: "Kerstin",
              current_location: "Raum 101",
              arrival_time: "08:00",
              pickup_time: "15:30",
              actual_arrival_time: "08:05",
              actual_pickup_time: undefined,
              sick: true,
              has_full_access: true,
            },
          ],
        },
        isLoading: false,
        error: null,
      } as ReturnType<typeof swrModule.useSWRAuth>);

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(
          screen.getByText("Kommt heute nicht (krank gemeldet)"),
        ).toBeInTheDocument();
      });
      expect(screen.getByText("Gehzeit: –")).toBeInTheDocument();
      expect(screen.queryByText(/Gehzeit: \d/)).not.toBeInTheDocument();
    });

    it("filters to show only transit students when 'unterwegs' is selected", async () => {
      mockSearchParams.set("status", "unterwegs");

      render(<StudentSearchPage />);

      await waitFor(() => {
        // Only Tom (Unterwegs) should be visible
        expect(screen.getByText(new RegExp(`^Tom\\b`))).toBeInTheDocument();
        expect(
          screen.queryByText(new RegExp(`^Max\\b`)),
        ).not.toBeInTheDocument();
        expect(
          screen.queryByText(new RegExp(`^Anna\\b`)),
        ).not.toBeInTheDocument();
        expect(
          screen.queryByText(new RegExp(`^Lisa\\b`)),
        ).not.toBeInTheDocument();
      });
    });

    it("filters to show only schoolyard students when 'schulhof' is selected", async () => {
      mockSearchParams.set("status", "schulhof");

      render(<StudentSearchPage />);

      await waitFor(() => {
        // Only Lisa (Schulhof) should be visible
        expect(screen.getByText(new RegExp(`^Lisa\\b`))).toBeInTheDocument();
        expect(
          screen.queryByText(new RegExp(`^Max\\b`)),
        ).not.toBeInTheDocument();
        expect(
          screen.queryByText(new RegExp(`^Anna\\b`)),
        ).not.toBeInTheDocument();
        expect(
          screen.queryByText(new RegExp(`^Tom\\b`)),
        ).not.toBeInTheDocument();
      });
    });

    it("filters to show only sick students when 'krank' is selected", async () => {
      const swrModule = await import("~/lib/swr");
      mockUseSWRAuthWithStudents(swrModule, {
        data: {
          students: [
            { ...mockStudents[0]!, sick: true },
            { ...mockStudents[1]!, sick: false },
          ],
        },
        isLoading: false,
        error: null,
      } as ReturnType<typeof swrModule.useSWRAuth>);

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByText(new RegExp(`^Max\\b`))).toBeInTheDocument();
      });

      fireEvent.change(screen.getByTestId("filter-attendance"), {
        target: { value: "krank" },
      });

      await waitFor(() => {
        expect(screen.getByText(new RegExp(`^Max\\b`))).toBeInTheDocument();
        expect(
          screen.queryByText(new RegExp(`^Anna\\b`)),
        ).not.toBeInTheDocument();
        expect(
          screen.getByTestId("active-filter-attendance"),
        ).toHaveTextContent("Krank");
      });
    });

    it("filters to show only excused students when 'entschuldigt' is selected", async () => {
      const swrModule = await import("~/lib/swr");
      mockUseSWRAuthWithStudents(swrModule, {
        data: {
          students: [
            { ...mockStudents[0]!, excused: true },
            { ...mockStudents[1]!, excused: false },
          ],
        },
        isLoading: false,
        error: null,
      } as ReturnType<typeof swrModule.useSWRAuth>);

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByText(new RegExp(`^Max\\b`))).toBeInTheDocument();
      });

      fireEvent.change(screen.getByTestId("filter-attendance"), {
        target: { value: "entschuldigt" },
      });

      await waitFor(() => {
        expect(screen.getByText(new RegExp(`^Max\\b`))).toBeInTheDocument();
        expect(
          screen.queryByText(new RegExp(`^Anna\\b`)),
        ).not.toBeInTheDocument();
        expect(
          screen.getByTestId("active-filter-attendance"),
        ).toHaveTextContent("Entschuldigt");
      });
    });
  });

  describe("Year Filtering", () => {
    it("renders the school year filter as a stage multi-select", async () => {
      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByTestId("filter-year")).toBeInTheDocument();
      });

      expect(
        screen.getByRole("option", { name: "Stufe 1" }),
      ).toBeInTheDocument();
      // No neutral "Alle Stufen" entry: an empty selection already means that,
      // and a checkable pseudo-option could be combined with a real stage
      // (#2218).
      expect(
        screen.queryByRole("option", { name: "Alle Stufen" }),
      ).not.toBeInTheDocument();
    });

    it("asks the backend for the selected school year", async () => {
      // The stage filter is applied server-side (#2218) so the reported count
      // covers the whole selection instead of only the fetched page.
      const { studentService } = await import("~/lib/api");
      const swrModule = await import("~/lib/swr");
      const fetcher = captureStudentsFetcher(swrModule);

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByTestId("filter-year")).toBeInTheDocument();
      });

      fireEvent.change(screen.getByTestId("filter-year"), {
        target: { value: "1" },
      });

      await fetcher.current!();
      expect(studentService.getStudents).toHaveBeenCalledWith(
        expect.objectContaining({ gradeLevel: ["1"] }),
      );
    });

    it("keeps two school years in one list and in the URL (#2218)", async () => {
      const { studentService } = await import("~/lib/api");
      const swrModule = await import("~/lib/swr");
      const fetcher = captureStudentsFetcher(swrModule);

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByTestId("filter-year")).toBeInTheDocument();
      });

      selectFilterOptions(screen.getByTestId("filter-year"), ["3", "4"]);

      await fetcher.current!();
      expect(studentService.getStudents).toHaveBeenCalledWith(
        expect.objectContaining({ gradeLevel: ["3", "4"] }),
      );
      expect(new URL(window.location.href).searchParams.get("year")).toBe(
        "3,4",
      );
      expect(screen.getByTestId("active-filter-year")).toHaveTextContent(
        "Jahr 3, 4",
      );
    });
  });

  describe("Loading States", () => {
    it("shows loading state when session is loading", async () => {
      const useSession = await import("next-auth/react");
      vi.mocked(useSession.useSession).mockReturnValue({
        data: null,
        status: "loading",
        update: vi.fn(),
      });

      render(<StudentSearchPage />);

      // Real chrome (page header) renders immediately; only the results
      // region skeletonizes while the session resolves (colocation pattern).
      expect(screen.getByTestId("page-header")).toBeInTheDocument();
      expect(
        screen.getByTestId("student-card-grid-skeleton"),
      ).toBeInTheDocument();
    });

    it("shows loading state while fetching students", async () => {
      const swrModule = await import("~/lib/swr");
      mockUseSWRAuthWithStudents(swrModule, {
        data: undefined,
        isLoading: true,
        // IMPORTANT: Use undefined, not null. The page checks `error !== undefined`
        // to determine if a fetch has completed. null !== undefined is true,
        // which would incorrectly mark the fetch as complete.
        error: undefined,
      } as ReturnType<typeof swrModule.useSWRAuth>);

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(
          screen.getByTestId("student-card-grid-skeleton"),
        ).toBeInTheDocument();
      });
    });
  });

  describe("Error Handling", () => {
    it("renders 403 permission denied error message", async () => {
      const swrModule = await import("~/lib/swr");
      mockUseSWRAuthWithStudents(swrModule, {
        data: undefined,
        isLoading: false,
        error: new Error("403 Forbidden"),
      } as ReturnType<typeof swrModule.useSWRAuth>);

      render(<StudentSearchPage />);

      await waitFor(() => {
        // Check that at least one element contains the error message
        const errorElements = screen.getAllByText(/keine Berechtigung/i);
        expect(errorElements.length).toBeGreaterThan(0);
      });
    });

    it("renders 401 session expired error message", async () => {
      const swrModule = await import("~/lib/swr");
      mockUseSWRAuthWithStudents(swrModule, {
        data: undefined,
        isLoading: false,
        error: new Error("401 Unauthorized"),
      } as ReturnType<typeof swrModule.useSWRAuth>);

      render(<StudentSearchPage />);

      await waitFor(() => {
        const errorElements = screen.getAllByText(/Sitzung ist abgelaufen/i);
        expect(errorElements.length).toBeGreaterThan(0);
      });
    });

    it("renders generic error for other API errors", async () => {
      const swrModule = await import("~/lib/swr");
      mockUseSWRAuthWithStudents(swrModule, {
        data: undefined,
        isLoading: false,
        error: new Error("Network Error"),
      } as ReturnType<typeof swrModule.useSWRAuth>);

      render(<StudentSearchPage />);

      await waitFor(() => {
        const errorElements = screen.getAllByText(
          /Fehler beim Laden der Kinderdaten/i,
        );
        expect(errorElements.length).toBeGreaterThan(0);
      });
    });
  });

  describe("Empty State", () => {
    it("shows empty state when no students match filters", async () => {
      // Mock SWR to return empty students
      const swrModule = await import("~/lib/swr");
      mockUseSWRAuthWithStudents(swrModule, {
        data: { students: [] },
        isLoading: false,
        error: null,
      } as ReturnType<typeof swrModule.useSWRAuth>);

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByText("Keine Kinder gefunden")).toBeInTheDocument();
      });
    });
  });

  describe("Clear Filters", () => {
    it("clears all filters when clear button is clicked", async () => {
      mockSearchParams.set("status", "unterwegs");

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByTestId("filter-attendance")).toHaveValue(
          "unterwegs",
        );
      });

      const clearButton = screen.getByTestId("clear-filters");
      fireEvent.click(clearButton);

      await waitFor(() => {
        expect(screen.getByTestId("filter-attendance")).toHaveValue("all");
      });
    });
  });

  describe("Student Card Navigation", () => {
    it("navigates to student detail when card is clicked", async () => {
      const mockPush = vi.fn();
      const useRouter = await import("next/navigation");
      vi.mocked(useRouter.useRouter).mockReturnValue({
        push: mockPush,
        replace: vi.fn(),
        back: vi.fn(),
        forward: vi.fn(),
        refresh: vi.fn(),
        prefetch: vi.fn(),
        bfcacheId: "test",
      });

      render(<StudentSearchPage />);

      // Wait for students to load - StudentCard displays first_name in h3
      await waitFor(() => {
        expect(screen.getByText(new RegExp(`^Max\\b`))).toBeInTheDocument();
      });

      // Find the student card (button with role) for "Max"
      const studentCard = screen
        .getByText(new RegExp(`^Max\\b`))
        .closest("button");
      if (studentCard) {
        fireEvent.click(studentCard);
      }

      await waitFor(() => {
        expect(mockPush).toHaveBeenCalledWith(
          "/test-tenant/students/1?from=/students/search",
        );
      });
    });

    it("keeps the multi-value class, group and year filters in the back-link", async () => {
      mockSearchParams.set("school_class", "3a,4b");
      mockSearchParams.set("group_id", "1,2");
      mockSearchParams.set("year", "3,4");

      const mockPush = vi.fn();
      const useRouter = await import("next/navigation");
      vi.mocked(useRouter.useRouter).mockReturnValue({
        push: mockPush,
        replace: vi.fn(),
        back: vi.fn(),
        forward: vi.fn(),
        refresh: vi.fn(),
        prefetch: vi.fn(),
        bfcacheId: "test",
      });

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByText(new RegExp(`^Max\\b`))).toBeInTheDocument();
      });

      fireEvent.click(
        screen.getByText(new RegExp(`^Max\\b`)).closest("button")!,
      );

      await waitFor(() => {
        expect(mockPush).toHaveBeenCalledWith(
          `/test-tenant/students/1?from=${encodeURIComponent(
            "/students/search?group_id=1%2C2&school_class=3a%2C4b&year=3%2C4",
          )}`,
        );
      });
    });
  });

  describe("SWR Fetcher Execution", () => {
    it("executes the groups SWR fetcher successfully", async () => {
      const groupService = await import("~/lib/api");
      const mockGetGroups = vi.fn().mockResolvedValue([
        { id: "1", name: "Test Group A" },
        { id: "2", name: "Test Group B" },
      ]);
      vi.mocked(groupService.groupService.getGroups).mockImplementation(
        mockGetGroups,
      );

      let capturedGroupsFetcher: (() => Promise<unknown>) | null = null;

      const swrModule = await import("~/lib/swr");
      vi.mocked(swrModule.useImmutableSWR).mockImplementation(
        (key, fetcher) => {
          if (key === "search-groups-list" && fetcher) {
            capturedGroupsFetcher = fetcher as () => Promise<unknown>;
          }
          return {
            data: [
              { id: "1", name: "Test Group A" },
              { id: "2", name: "Test Group B" },
            ],
            isLoading: false,
            error: null,
          } as ReturnType<typeof swrModule.useImmutableSWR>;
        },
      );

      mockUseSWRAuthWithStudents(swrModule, {
        data: { students: mockStudents },
        isLoading: false,
        error: null,
      } as ReturnType<typeof swrModule.useSWRAuth>);

      await act(async () => {
        render(<StudentSearchPage />);
      });

      // Execute the captured fetcher to cover lines 93-103
      expect(capturedGroupsFetcher).not.toBeNull();
      const result: unknown = await act(async () => {
        return await (
          capturedGroupsFetcher as unknown as () => Promise<unknown>
        )();
      });
      expect(result).toEqual([
        { id: "1", name: "Test Group A" },
        { id: "2", name: "Test Group B" },
      ]);
      expect(mockGetGroups).toHaveBeenCalled();
    });

    it("handles groups fetcher error gracefully", async () => {
      const groupService = await import("~/lib/api");
      const mockGetGroups = vi
        .fn()
        .mockRejectedValue(new Error("Permission denied"));
      vi.mocked(groupService.groupService.getGroups).mockImplementation(
        mockGetGroups,
      );

      let capturedGroupsFetcher: (() => Promise<unknown>) | null = null;

      const consoleWarnSpy = vi
        .spyOn(console, "warn")
        .mockImplementation(() => undefined);

      const swrModule = await import("~/lib/swr");
      vi.mocked(swrModule.useImmutableSWR).mockImplementation(
        (key, fetcher) => {
          if (key === "search-groups-list" && fetcher) {
            capturedGroupsFetcher = fetcher as () => Promise<unknown>;
          }
          return {
            data: [],
            isLoading: false,
            error: null,
          } as ReturnType<typeof swrModule.useImmutableSWR>;
        },
      );

      mockUseSWRAuthWithStudents(swrModule, {
        data: { students: mockStudents },
        isLoading: false,
        error: null,
      } as ReturnType<typeof swrModule.useSWRAuth>);

      await act(async () => {
        render(<StudentSearchPage />);
      });

      // Execute the captured fetcher to cover catch block (lines 98-101)
      expect(capturedGroupsFetcher).not.toBeNull();
      const result: unknown = await act(async () => {
        return await (
          capturedGroupsFetcher as unknown as () => Promise<unknown>
        )();
      });
      // Should return empty array on error
      expect(result).toEqual([]);
      expect(consoleWarnSpy).toHaveBeenCalledWith(
        "could not load groups for filter",
        undefined,
      );

      consoleWarnSpy.mockRestore();
    });

    it("executes the students SWR fetcher", async () => {
      mockSearchParams.set("day_status", "not_coming_today");
      const studentService = await import("~/lib/api");
      const mockGetStudents = vi.fn().mockResolvedValue({
        students: [{ id: "1", first_name: "Test", second_name: "Student" }],
      });
      vi.mocked(studentService.studentService.getStudents).mockImplementation(
        mockGetStudents,
      );

      let capturedStudentsFetcher: (() => Promise<unknown>) | null = null;

      const swrModule = await import("~/lib/swr");
      vi.mocked(swrModule.useImmutableSWR).mockReturnValue({
        data: [],
        isLoading: false,
        error: null,
      } as ReturnType<typeof swrModule.useImmutableSWR>);

      vi.mocked(swrModule.useSWRAuth).mockImplementation((key, fetcher) => {
        if (key === "search-rooms-list") {
          return {
            data: [],
            isLoading: false,
            error: null,
          } as ReturnType<typeof swrModule.useSWRAuth>;
        }

        // Capture the students fetcher when the key contains "search-students"
        // Note: key is null until groupsLoaded state becomes true after useEffect runs
        if (
          typeof key === "string" &&
          key.includes("search-students") &&
          fetcher
        ) {
          capturedStudentsFetcher = fetcher as () => Promise<unknown>;
        }
        return {
          data: { students: mockStudents },
          isLoading: false,
          error: null,
        } as ReturnType<typeof swrModule.useSWRAuth>;
      });

      render(<StudentSearchPage />);

      // Wait for the component to re-render after groupsLoaded becomes true
      // This happens in a useEffect, so we need to wait for it
      await waitFor(() => {
        expect(capturedStudentsFetcher).not.toBeNull();
      });

      // Execute the captured fetcher to cover lines 117-128
      const result: unknown = await (
        capturedStudentsFetcher as unknown as () => Promise<unknown>
      )();
      expect(result).toEqual({
        students: [{ id: "1", first_name: "Test", second_name: "Student" }],
        // The fetcher now stamps the response with the date AND the
        // today/planning mode it was fetched for, so the page can tell a fresh
        // result apart from keepPreviousData holding the previous request's rows
        // during a date OR mode switch — the mode tag catches the midnight
        // rollover where the date is unchanged but semantics flip (#1939).
        requestDate: expect.any(String),
        requestIsToday: expect.any(Boolean),
        // Null whenever the page walk loaded the whole selection, which is the
        // case here: a response without pagination counts as one page (#2218).
        truncatedTotal: null,
      });
      expect(mockGetStudents).toHaveBeenCalledWith(
        expect.objectContaining({ dayStatus: "not_coming_today" }),
      );
    });
  });

  describe("Error Display Rendering", () => {
    // Fix P3 regression test: Error heading now uses errorType instead of substring matching
    it("renders 'Keine Berechtigung' heading for 403 errors (P3 fix)", async () => {
      const swrModule = await import("~/lib/swr");
      mockUseSWRAuthWithStudents(swrModule, {
        data: undefined,
        isLoading: false,
        error: new Error("403 Forbidden"),
      } as ReturnType<typeof swrModule.useSWRAuth>);

      render(<StudentSearchPage />);

      await waitFor(() => {
        // The transformed error message for 403
        expect(
          screen.getAllByText(
            /Sie haben keine Berechtigung, Kinderdaten anzuzeigen/,
          ).length,
        ).toBeGreaterThan(0);
        // P3 FIX: The error heading should now be "Keine Berechtigung" (not "Fehler")
        // because we use errorType === "permission" instead of error.includes("403")
        expect(screen.getByText("Keine Berechtigung")).toBeInTheDocument();
      });
    });

    it("renders 'Fehler' heading for 401 session errors", async () => {
      const swrModule = await import("~/lib/swr");
      mockUseSWRAuthWithStudents(swrModule, {
        data: undefined,
        isLoading: false,
        error: new Error("401 Unauthorized"),
      } as ReturnType<typeof swrModule.useSWRAuth>);

      render(<StudentSearchPage />);

      await waitFor(() => {
        // The transformed error message for 401
        expect(
          screen.getAllByText(/Sitzung ist abgelaufen/).length,
        ).toBeGreaterThan(0);
        // Session errors still show generic "Fehler" heading
        expect(screen.getByText("Fehler")).toBeInTheDocument();
      });
    });

    it("renders generic error heading for non-403/401 errors", async () => {
      const swrModule = await import("~/lib/swr");
      mockUseSWRAuthWithStudents(swrModule, {
        data: undefined,
        isLoading: false,
        error: new Error("500 Internal Server Error"),
      } as ReturnType<typeof swrModule.useSWRAuth>);

      render(<StudentSearchPage />);

      await waitFor(() => {
        // The generic error message
        expect(
          screen.getAllByText(/Fehler beim Laden der Kinderdaten/).length,
        ).toBeGreaterThan(0);
        // The error heading
        expect(screen.getByText("Fehler")).toBeInTheDocument();
      });
    });
  });

  // P1 Regression Tests: Unauthenticated users should be redirected (not see empty state)
  describe("Authentication Redirect Handling (P1 fix)", () => {
    it("redirects to home when user is unauthenticated", async () => {
      const mockPush = vi.fn();
      const useRouter = await import("next/navigation");
      vi.mocked(useRouter.useRouter).mockReturnValue({
        push: mockPush,
        replace: vi.fn(),
        back: vi.fn(),
        forward: vi.fn(),
        refresh: vi.fn(),
        prefetch: vi.fn(),
        bfcacheId: "test",
      });

      const useSession = await import("next-auth/react");
      // Simulate NextAuth's useSession with required: true - it calls onUnauthenticated callback
      vi.mocked(useSession.useSession).mockImplementation((options) => {
        // When required: true and user is unauthenticated, NextAuth calls the callback
        if (
          options &&
          typeof options === "object" &&
          "required" in options &&
          options.required
        ) {
          const opts = options as { onUnauthenticated?: () => void };
          if (opts.onUnauthenticated) {
            opts.onUnauthenticated();
          }
        }
        return {
          data: null,
          status: "unauthenticated",
          update: vi.fn(),
        };
      });

      // SWR won't fetch when unauthenticated
      const swrModule = await import("~/lib/swr");
      mockUseSWRAuthWithStudents(swrModule, {
        data: undefined,
        isLoading: false,
        error: undefined,
      } as ReturnType<typeof swrModule.useSWRAuth>);

      render(<StudentSearchPage />);

      // P1 FIX: Should redirect to home page, NOT show empty state or error message
      expect(mockPush).toHaveBeenCalledWith("/test-tenant/");
    });

    it("shows loading state during auth check (no empty state flash)", async () => {
      const useSession = await import("next-auth/react");
      vi.mocked(useSession.useSession).mockReturnValue({
        data: null,
        status: "loading", // Session check in progress
        update: vi.fn(),
      });

      // SWR won't fetch during auth loading
      const swrModule = await import("~/lib/swr");
      mockUseSWRAuthWithStudents(swrModule, {
        data: undefined,
        isLoading: false,
        error: undefined,
      } as ReturnType<typeof swrModule.useSWRAuth>);

      render(<StudentSearchPage />);

      // Should show loading, NOT empty state. Real chrome (page header)
      // renders immediately; only the results region skeletonizes
      // (colocation pattern).
      expect(screen.getByTestId("page-header")).toBeInTheDocument();
      expect(
        screen.getByTestId("student-card-grid-skeleton"),
      ).toBeInTheDocument();
      expect(
        screen.queryByText("Keine Kinder gefunden"),
      ).not.toBeInTheDocument();
    });

    it("does NOT redirect when user is authenticated with token", async () => {
      const mockPush = vi.fn();
      const useRouter = await import("next/navigation");
      vi.mocked(useRouter.useRouter).mockReturnValue({
        push: mockPush,
        replace: vi.fn(),
        back: vi.fn(),
        forward: vi.fn(),
        refresh: vi.fn(),
        prefetch: vi.fn(),
        bfcacheId: "test",
      });

      // Default authenticated state with token
      const useSession = await import("next-auth/react");
      vi.mocked(useSession.useSession).mockReturnValue({
        data: { user: { token: "valid-token" }, expires: "2099-01-01" },
        status: "authenticated",
        update: vi.fn(),
      } as unknown as ReturnType<typeof useSession.useSession>);

      render(<StudentSearchPage />);

      await waitFor(() => {
        // Should render page header (meaning component rendered normally)
        expect(screen.getByTestId("page-header")).toBeInTheDocument();
      });

      // Should NOT have redirected to home
      expect(mockPush).not.toHaveBeenCalledWith("/test-tenant/");
    });
  });

  // P2 Regression Tests: Empty state should not flash before first fetch
  describe("Empty State Flash Prevention (P2 fix)", () => {
    it("shows loading state before first fetch completes (not empty state)", async () => {
      // Simulate the initial state before groupsLoaded becomes true
      const swrModule = await import("~/lib/swr");

      // Groups haven't loaded yet, so studentsCacheKey is null
      // SWR returns undefined data (not yet fetched)
      mockUseSWRAuthWithStudents(swrModule, {
        data: undefined, // No data yet - first fetch hasn't completed
        isLoading: true, // SWR is loading students
        error: undefined,
      } as ReturnType<typeof swrModule.useSWRAuth>);

      vi.mocked(swrModule.useImmutableSWR).mockReturnValue({
        data: undefined, // Groups also loading
        isLoading: true,
        error: null,
      } as ReturnType<typeof swrModule.useImmutableSWR>);

      await act(async () => {
        render(<StudentSearchPage />);
      });

      // P2 FIX: Should show the grid skeleton, NOT "Keine Kinder gefunden"
      // because we're in initialization phase (groupsLoaded = false)
      expect(
        screen.getByTestId("student-card-grid-skeleton"),
      ).toBeInTheDocument();
      expect(
        screen.queryByText("Keine Kinder gefunden"),
      ).not.toBeInTheDocument();
    });

    it("shows empty state only AFTER first fetch returns empty results", async () => {
      const swrModule = await import("~/lib/swr");

      // Groups have loaded
      vi.mocked(swrModule.useImmutableSWR).mockReturnValue({
        data: [{ id: "1", name: "Gruppe A" }],
        isLoading: false,
        error: null,
      } as ReturnType<typeof swrModule.useImmutableSWR>);

      // Students fetch completed with empty results (hasFetchedOnce = true)
      mockUseSWRAuthWithStudents(swrModule, {
        data: { students: [] }, // Empty results from completed fetch
        isLoading: false,
        error: undefined,
      } as ReturnType<typeof swrModule.useSWRAuth>);

      render(<StudentSearchPage />);

      await waitFor(() => {
        // NOW it's appropriate to show empty state
        expect(screen.getByText("Keine Kinder gefunden")).toBeInTheDocument();
      });
    });
  });

  describe("Active Filter Removal", () => {
    it("removes attendance filter when active filter chip is clicked", async () => {
      mockSearchParams.set("status", "anwesend");

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(
          screen.getByTestId("active-filter-attendance"),
        ).toBeInTheDocument();
      });

      // Click the active filter to remove it
      const activeFilter = screen.getByTestId("active-filter-attendance");
      fireEvent.click(activeFilter);

      await waitFor(() => {
        expect(screen.getByTestId("filter-attendance")).toHaveValue("all");
        expect(
          screen.queryByTestId("active-filter-attendance"),
        ).not.toBeInTheDocument();
      });
    });

    it("removes group filter when active filter chip is clicked", async () => {
      render(<StudentSearchPage />);

      // First set a group filter
      await waitFor(() => {
        expect(screen.getByTestId("filter-group")).toBeInTheDocument();
      });

      const groupFilter = screen.getByTestId("filter-group");
      fireEvent.change(groupFilter, { target: { value: "1" } });

      await waitFor(() => {
        expect(screen.getByTestId("active-filter-group")).toBeInTheDocument();
      });

      // Click the active filter to remove it
      const activeFilter = screen.getByTestId("active-filter-group");
      fireEvent.click(activeFilter);

      await waitFor(() => {
        expect(screen.getByTestId("filter-group")).toHaveValue([]);
        expect(
          screen.queryByTestId("active-filter-group"),
        ).not.toBeInTheDocument();
      });
    });
  });

  describe("Administrative Student Filters", () => {
    it("separates student caches when the photo feature flag changes", async () => {
      const swrModule = await import("~/lib/swr");
      const tenantProvider = await import("~/lib/tenant-context");
      const studentKeys: string[] = [];

      vi.mocked(swrModule.useSWRAuth).mockImplementation((key) => {
        if (key === "search-rooms-list") {
          return {
            data: [],
            isLoading: false,
            error: null,
          } as ReturnType<typeof swrModule.useSWRAuth>;
        }

        if (typeof key === "string" && key.startsWith("search-students-")) {
          studentKeys.push(key);
        }

        return {
          data: { students: mockStudents },
          isLoading: false,
          error: null,
        } as ReturnType<typeof swrModule.useSWRAuth>;
      });

      vi.mocked(tenantProvider.useTenantSafe).mockReturnValue({
        tenantSlug: "test-tenant",
        tenant: { studentPhotosEnabled: false },
      } as unknown as ReturnType<typeof tenantProvider.useTenantSafe>);

      const { rerender } = render(<StudentSearchPage />);

      await waitFor(() => {
        expect(studentKeys.some((key) => key.includes("photos-disabled"))).toBe(
          true,
        );
      });

      vi.mocked(tenantProvider.useTenantSafe).mockReturnValue({
        tenantSlug: "test-tenant",
        tenant: { studentPhotosEnabled: true },
      } as unknown as ReturnType<typeof tenantProvider.useTenantSafe>);

      rerender(<StudentSearchPage />);

      await waitFor(() => {
        expect(studentKeys.some((key) => key.includes("photos-enabled"))).toBe(
          true,
        );
      });
    });

    it("keys tracking indicators by the fetched student ids", async () => {
      mockSearchParams.set("bus", "yes");

      const swrModule = await import("~/lib/swr");
      const trackingKeys: string[] = [];
      const completeStudents = [
        { ...mockStudents[0]!, id: "first-page-student" },
        { ...mockStudents[2]!, id: "later-page-student" },
      ];

      vi.mocked(swrModule.useSWRAuth).mockImplementation((key) => {
        if (key === "search-rooms-list") {
          return {
            data: [],
            isLoading: false,
            error: null,
          } as ReturnType<typeof swrModule.useSWRAuth>;
        }

        if (typeof key === "string" && key.startsWith("tracking-indicators-")) {
          trackingKeys.push(key);
          return {
            data: undefined,
            isLoading: false,
            error: null,
          } as ReturnType<typeof swrModule.useSWRAuth>;
        }

        return {
          data: { students: completeStudents },
          isLoading: false,
          error: null,
        } as ReturnType<typeof swrModule.useSWRAuth>;
      });

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(trackingKeys).toContain(
          "tracking-indicators-first-page-student,later-page-student",
        );
      });
    });

    it("includes pickup status filters in the students SWR cache key", async () => {
      const swrModule = await import("~/lib/swr");
      const studentKeys: string[] = [];

      vi.mocked(swrModule.useSWRAuth).mockImplementation((key) => {
        if (key === "search-rooms-list") {
          return {
            data: [],
            isLoading: false,
            error: null,
          } as ReturnType<typeof swrModule.useSWRAuth>;
        }

        if (typeof key === "string" && key.startsWith("search-students-")) {
          studentKeys.push(key);
        }

        return {
          data: { students: mockStudents },
          isLoading: false,
          error: null,
        } as ReturnType<typeof swrModule.useSWRAuth>;
      });

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByTestId("filter-pickupStatus")).toBeInTheDocument();
      });

      fireEvent.change(screen.getByTestId("filter-pickupStatus"), {
        target: { value: "self" },
      });

      await waitFor(() => {
        // pickup_status is the last segment of the students cache key, so a
        // change to it must produce a key ending in the new value (forces SWR
        // to refetch the now server-filtered page).
        expect(studentKeys.some((key) => key.endsWith("-self"))).toBe(true);
      });
    });

    it("leads the students SWR cache key with the group scope (#2097)", async () => {
      // The SSE flush reads the page's group filter back out of this key to
      // decide whether a check-in in some other group concerns this view, so
      // the scope segment must sit before the free-text term (which may itself
      // contain dashes). See lib/swr/search-students-key.
      mockSearchParams.set("group_id", "7");
      const swrModule = await import("~/lib/swr");
      const studentKeys: string[] = [];

      vi.mocked(swrModule.useSWRAuth).mockImplementation((key) => {
        if (key === "search-rooms-list") {
          return {
            data: [],
            isLoading: false,
            error: null,
          } as ReturnType<typeof swrModule.useSWRAuth>;
        }

        if (typeof key === "string" && key.startsWith("search-students-")) {
          studentKeys.push(key);
        }

        return {
          data: { students: mockStudents },
          isLoading: false,
          error: null,
        } as ReturnType<typeof swrModule.useSWRAuth>;
      });

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(
          studentKeys.some((key) => key.startsWith("search-students-g7-")),
        ).toBe(true);
      });
    });

    it.each([
      { rawGroupId: "007", expectedGroupIds: ["7"], expectedScope: "g7" },
      { rawGroupId: "0", expectedGroupIds: [], expectedScope: "gall" },
      {
        rawGroupId: "not-a-group",
        expectedGroupIds: [],
        expectedScope: "gall",
      },
    ])(
      "normalizes URL group_id '$rawGroupId' to the backend-effective SSE scope",
      async ({ rawGroupId, expectedGroupIds, expectedScope }) => {
        mockSearchParams.set("group_id", rawGroupId);
        const { studentService } = await import("~/lib/api");
        const swrModule = await import("~/lib/swr");
        const studentKeys: string[] = [];
        const fetcher = captureStudentsFetcher(swrModule);

        vi.mocked(swrModule.useSWRAuth).mockImplementation(
          (key, swrFetcher) => {
            if (typeof key === "string" && key.startsWith("search-students-")) {
              studentKeys.push(key);
              if (swrFetcher) {
                fetcher.current = swrFetcher as () => Promise<unknown>;
              }
            }
            if (key === "search-rooms-list") {
              return {
                data: [],
                isLoading: false,
                error: null,
              } as ReturnType<typeof swrModule.useSWRAuth>;
            }
            return {
              data: { students: mockStudents },
              isLoading: false,
              error: null,
            } as ReturnType<typeof swrModule.useSWRAuth>;
          },
        );

        render(<StudentSearchPage />);

        await waitFor(() => {
          expect(
            studentKeys.some((key) =>
              key.startsWith(`search-students-${expectedScope}-`),
            ),
          ).toBe(true);
        });
        await fetcher.current!();
        expect(studentService.getStudents).toHaveBeenCalledWith(
          expect.objectContaining({ groupId: expectedGroupIds }),
        );
      },
    );

    it("requests the slim list projection (#2097)", async () => {
      // The page renders no address, health info, supervisor note, guardian
      // contact or departure map — shipping them costs ~78% of the payload on
      // a list that fetches up to 1000 rows in one trip.
      const { studentService } = await import("~/lib/api");
      const swrModule = await import("~/lib/swr");
      const fetcher = captureStudentsFetcher(swrModule);

      render(<StudentSearchPage />);

      await waitFor(() => expect(fetcher.current).not.toBeNull());
      await fetcher.current!();

      expect(studentService.getStudents).toHaveBeenCalledWith(
        expect.objectContaining({ view: "slim" }),
      );
    });

    it("delegates Buskind filtering to a single backend request (#1492)", async () => {
      // Previously the page fetched every page client-side and filtered locally.
      // The backend now filters + paginates server-side, so the fetcher makes a
      // single getStudents call carrying bus and must NOT loop over pages.
      mockSearchParams.set("bus", "yes");

      const studentService = await import("~/lib/api");
      const mockGetStudents = vi.fn().mockResolvedValue({
        students: [{ ...mockStudents[0]!, id: "101", first_name: "OnlyPage" }],
        pagination: {
          current_page: 1,
          page_size: 50,
          total_pages: 1,
          total_records: 1,
        },
      });
      vi.mocked(studentService.studentService.getStudents).mockImplementation(
        mockGetStudents,
      );

      let capturedStudentsFetcher: (() => Promise<unknown>) | null = null;
      const swrModule = await import("~/lib/swr");
      vi.mocked(swrModule.useSWRAuth).mockImplementation((key, fetcher) => {
        if (key === "search-rooms-list") {
          return {
            data: [],
            isLoading: false,
            error: null,
          } as ReturnType<typeof swrModule.useSWRAuth>;
        }

        if (
          typeof key === "string" &&
          key.includes("search-students") &&
          fetcher
        ) {
          capturedStudentsFetcher = fetcher as () => Promise<unknown>;
        }

        return {
          data: { students: [] },
          isLoading: false,
          error: null,
        } as ReturnType<typeof swrModule.useSWRAuth>;
      });

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(capturedStudentsFetcher).not.toBeNull();
      });

      await (capturedStudentsFetcher as unknown as () => Promise<unknown>)();

      // Exactly one backend call, carrying bus, and no page-2 follow-up.
      expect(mockGetStudents).toHaveBeenCalledTimes(1);
      expect(mockGetStudents).toHaveBeenCalledWith(
        expect.objectContaining({ bus: "yes" }),
      );
      expect(mockGetStudents).not.toHaveBeenCalledWith(
        expect.objectContaining({ page: 2 }),
      );
    });

    it("walks every page when the selection is larger than one response (#2218)", async () => {
      // This view has no pagination control, so a selection the proxy route
      // cannot fit into its 1000-row page has to be walked here — otherwise
      // every child past the first page is unreachable.
      const studentService = await import("~/lib/api");
      const mockGetStudents = vi.fn();
      mockGetStudents.mockImplementation((filters: { page?: number }) => {
        const page = filters.page ?? 1;
        return Promise.resolve({
          students: [
            {
              ...mockStudents[0]!,
              id: `seite-${page}`,
              first_name: `Seite${page}`,
            },
          ],
          pagination: {
            current_page: page,
            page_size: 1000,
            total_pages: 3,
            total_records: 3,
          },
        });
      });
      vi.mocked(studentService.studentService.getStudents).mockImplementation(
        mockGetStudents,
      );

      const swrModule = await import("~/lib/swr");
      const fetcher = captureStudentsFetcher(swrModule);

      render(<StudentSearchPage />);

      await waitFor(() => expect(fetcher.current).not.toBeNull());
      const result = (await fetcher.current!()) as {
        students: { id: string }[];
      };

      expect(mockGetStudents).toHaveBeenCalledTimes(3);
      // The first request stays free of a page parameter, so the common
      // single-page case is unchanged on the wire.
      expect(mockGetStudents.mock.calls[0]?.[0]).not.toHaveProperty("page");
      expect(mockGetStudents).toHaveBeenCalledWith(
        expect.objectContaining({ page: 2 }),
      );
      expect(mockGetStudents).toHaveBeenCalledWith(
        expect.objectContaining({ page: 3 }),
      );
      expect(result.students.map((student) => student.id)).toEqual([
        "seite-1",
        "seite-2",
        "seite-3",
      ]);
    });

    it("reports an incomplete result when the page walk hits its bound (#2218)", async () => {
      // Past the bound the loaded rows are only a part of the selection. The
      // count badge, the grouping and the export all read those rows, so the
      // shortfall has to leave the fetcher instead of ending in the log.
      const studentService = await import("~/lib/api");
      const mockGetStudents = vi.fn();
      mockGetStudents.mockImplementation((filters: { page?: number }) => {
        const page = filters.page ?? 1;
        return Promise.resolve({
          students: [
            {
              ...mockStudents[0]!,
              id: `seite-${page}`,
              first_name: `Seite${page}`,
            },
          ],
          pagination: {
            current_page: page,
            page_size: 1000,
            total_pages: 25,
            total_records: 25000,
          },
        });
      });
      vi.mocked(studentService.studentService.getStudents).mockImplementation(
        mockGetStudents,
      );

      const swrModule = await import("~/lib/swr");
      const fetcher = captureStudentsFetcher(swrModule);

      render(<StudentSearchPage />);

      await waitFor(() => expect(fetcher.current).not.toBeNull());
      const result = (await fetcher.current!()) as {
        students: { id: string }[];
        truncatedTotal: number | null;
      };

      // 20 pages is the bound, not the 25 the backend counted.
      expect(mockGetStudents).toHaveBeenCalledTimes(20);
      expect(result.students).toHaveLength(20);
      expect(result.truncatedTotal).toBe(25000);
    });

    it("keeps truncatedTotal null when every page was loaded (#2218)", async () => {
      const studentService = await import("~/lib/api");
      vi.mocked(studentService.studentService.getStudents).mockResolvedValue({
        students: [mockStudents[0]!],
        pagination: {
          current_page: 1,
          page_size: 1000,
          total_pages: 1,
          total_records: 1,
        },
      } as unknown as Awaited<
        ReturnType<typeof studentService.studentService.getStudents>
      >);

      const swrModule = await import("~/lib/swr");
      const fetcher = captureStudentsFetcher(swrModule);

      render(<StudentSearchPage />);

      await waitFor(() => expect(fetcher.current).not.toBeNull());
      const result = (await fetcher.current!()) as {
        truncatedTotal: number | null;
      };

      expect(result.truncatedTotal).toBeNull();
    });

    it("names the incomplete result on screen (#2218)", async () => {
      const swrModule = await import("~/lib/swr");
      mockUseSWRAuthWithStudents(swrModule, {
        data: {
          students: [mockStudents[0]!, mockStudents[1]!],
          truncatedTotal: 25000,
        },
        isLoading: false,
        error: null,
      } as ReturnType<typeof swrModule.useSWRAuth>);

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(
          screen.getByText(
            /Es werden nur die ersten 2 von 25000 Kindern geladen/,
          ),
        ).toBeInTheDocument();
      });
    });

    it("stays quiet when the result is complete (#2218)", async () => {
      const swrModule = await import("~/lib/swr");
      mockUseSWRAuthWithStudents(swrModule, {
        data: { students: [mockStudents[0]!], truncatedTotal: null },
        isLoading: false,
        error: null,
      } as ReturnType<typeof swrModule.useSWRAuth>);

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByText(new RegExp(`^Max\\b`))).toBeInTheDocument();
      });
      expect(screen.queryByText(/Kindern geladen/)).not.toBeInTheDocument();
    });

    it("uses consistent default labels for administrative filters", async () => {
      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByTestId("filter-bus")).toBeInTheDocument();
      });

      expect(
        (screen.getByTestId("filter-bus") as HTMLSelectElement)
          .selectedOptions[0]?.textContent,
      ).toBe("Alle Kinder");
      expect(
        (screen.getByTestId("filter-photoConsent") as HTMLSelectElement)
          .selectedOptions[0]?.textContent,
      ).toBe("Alle Kinder");
      expect(
        (screen.getByTestId("filter-pickupStatus") as HTMLSelectElement)
          .selectedOptions[0]?.textContent,
      ).toBe("Alle Kinder");
    });

    it("forwards the Buskind filter to the backend (#1492)", async () => {
      // Buskind is now a server-side filter, so the page must hand it to
      // getStudents rather than re-filter the fetched page client-side. The
      // actual exclusion (incl. GDPR redaction) is covered by the backend
      // applyAdministrativeFilters tests.
      mockSearchParams.set("bus", "yes");
      const { studentService } = await import("~/lib/api");
      const swrModule = await import("~/lib/swr");
      const fetcher = captureStudentsFetcher(swrModule);

      render(<StudentSearchPage />);

      await waitFor(() => expect(fetcher.current).not.toBeNull());
      await fetcher.current!();

      expect(studentService.getStudents).toHaveBeenCalledWith(
        expect.objectContaining({ bus: "yes" }),
      );
      expect(screen.getByTestId("active-filter-bus")).toHaveTextContent(
        "Buskind",
      );
    });

    it("forwards the photo consent filter to the backend (#1492)", async () => {
      mockSearchParams.set("photo_consent", "no");
      const { studentService } = await import("~/lib/api");
      const swrModule = await import("~/lib/swr");
      const fetcher = captureStudentsFetcher(swrModule);

      render(<StudentSearchPage />);

      await waitFor(() => expect(fetcher.current).not.toBeNull());
      await fetcher.current!();

      expect(studentService.getStudents).toHaveBeenCalledWith(
        expect.objectContaining({ photoConsent: "no" }),
      );
    });

    it("hides the photo consent filter when the tenant photo feature is disabled", async () => {
      const tenantProvider = await import("~/lib/tenant-context");
      vi.mocked(tenantProvider.useTenantSafe).mockReturnValue({
        tenantSlug: "test-tenant",
        tenant: { studentPhotosEnabled: false },
      } as unknown as ReturnType<typeof tenantProvider.useTenantSafe>);
      mockSearchParams.set("photo_consent", "no");

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(
          screen.queryByTestId("filter-photoConsent"),
        ).not.toBeInTheDocument();
        expect(
          screen.queryByTestId("active-filter-photoConsent"),
        ).not.toBeInTheDocument();
        expect(screen.getByText(new RegExp(`^Max\\b`))).toBeInTheDocument();
        expect(screen.getByText(new RegExp(`^Lisa\\b`))).toBeInTheDocument();
      });
    });

    it("hides the photo consent filter when the API omits consent state", async () => {
      const redactedConsentStudents = mockStudents.map((student) => {
        const copy: Partial<(typeof mockStudents)[number]> = { ...student };
        delete copy.photo_consent_given;
        delete copy.photo_consent_given_at;
        return copy;
      });

      const swrModule = await import("~/lib/swr");
      mockUseSWRAuthWithStudents(swrModule, {
        data: { students: redactedConsentStudents },
        isLoading: false,
        error: null,
      } as ReturnType<typeof swrModule.useSWRAuth>);

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(
          screen.queryByTestId("filter-photoConsent"),
        ).not.toBeInTheDocument();
      });
    });

    // GDPR exclusion of redacted students from administrative filters is now
    // enforced server-side (api/students applyAdministrativeFilters) and
    // covered by its backend tests, so it no longer has a client-side
    // equivalent here.
  });

  describe("Pickup Time Filtering", () => {
    it("filters students by specific pickup time", async () => {
      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByText(new RegExp(`^Max\\b`))).toBeInTheDocument();
      });

      // Filter by 15:30, only Max has this pickup time
      const pickupFilter = screen.getByTestId("filter-pickupTime");
      fireEvent.change(pickupFilter, { target: { value: "15:30" } });

      await waitFor(() => {
        expect(screen.getByText(new RegExp(`^Max\\b`))).toBeInTheDocument();
        expect(
          screen.queryByText(new RegExp(`^Anna\\b`)),
        ).not.toBeInTheDocument();
        expect(
          screen.queryByText(new RegExp(`^Tom\\b`)),
        ).not.toBeInTheDocument();
        expect(
          screen.queryByText(new RegExp(`^Lisa\\b`)),
        ).not.toBeInTheDocument();
      });
    });

    it("filters students with 'none' to show only students without pickup time", async () => {
      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByText(new RegExp(`^Max\\b`))).toBeInTheDocument();
      });

      // Filter by "none": Anna and Lisa have no pickup_time
      const pickupFilter = screen.getByTestId("filter-pickupTime");
      fireEvent.change(pickupFilter, { target: { value: "none" } });

      await waitFor(() => {
        // Anna has no pickup_time and has_full_access=true
        expect(screen.getByText(new RegExp(`^Anna\\b`))).toBeInTheDocument();
        // Lisa has no pickup_time and has_full_access=true
        expect(screen.getByText(new RegExp(`^Lisa\\b`))).toBeInTheDocument();
        // Max and Tom have pickup_time set, so they should be filtered out
        expect(
          screen.queryByText(new RegExp(`^Max\\b`)),
        ).not.toBeInTheDocument();
        expect(
          screen.queryByText(new RegExp(`^Tom\\b`)),
        ).not.toBeInTheDocument();
      });
    });

    it("hides redacted students (has_full_access=false) when pickup time filter is active", async () => {
      // Create mock students where one has has_full_access=false
      const studentsWithRedacted = [
        {
          id: "10",
          first_name: "Visible",
          second_name: "Student",
          school_class: "1a",
          group_name: "Gruppe A",
          current_location: "Raum 101",
          has_full_access: true,
        },
        {
          id: "11",
          first_name: "Redacted",
          second_name: "Student",
          school_class: "1a",
          group_name: "Gruppe A",
          current_location: "Raum 102",
          has_full_access: false,
        },
      ];

      const swrModule = await import("~/lib/swr");
      mockUseSWRAuthWithStudents(swrModule, {
        data: { students: studentsWithRedacted },
        isLoading: false,
        error: null,
      } as ReturnType<typeof swrModule.useSWRAuth>);

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByText(new RegExp(`^Visible\\b`))).toBeInTheDocument();
        expect(
          screen.getByText(new RegExp(`^Redacted\\b`)),
        ).toBeInTheDocument();
      });

      // Apply "none" pickup time filter: redacted student should be excluded (GDPR)
      const pickupFilter = screen.getByTestId("filter-pickupTime");
      fireEvent.change(pickupFilter, { target: { value: "none" } });

      await waitFor(() => {
        expect(screen.getByText(new RegExp(`^Visible\\b`))).toBeInTheDocument();
        // Redacted student must NOT appear: has_full_access=false means we
        // can't know their pickup status, so they're excluded from filtering
        expect(
          screen.queryByText(new RegExp(`^Redacted\\b`)),
        ).not.toBeInTheDocument();
      });
    });

    it("shows active filter chips with correct labels and clears via chip click or clear-all", async () => {
      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByTestId("filter-pickupTime")).toBeInTheDocument();
      });

      const pickupFilter = screen.getByTestId("filter-pickupTime");

      // Verify "Keine Gehzeit" chip label for "none"
      fireEvent.change(pickupFilter, { target: { value: "none" } });
      await waitFor(() => {
        expect(
          screen.getByTestId("active-filter-pickupTime"),
        ).toHaveTextContent("Keine Gehzeit");
      });

      // Switch to specific time: verify chip label and clear-all
      fireEvent.change(pickupFilter, { target: { value: "15:30" } });
      await waitFor(() => {
        expect(
          screen.getByTestId("active-filter-pickupTime"),
        ).toHaveTextContent("Gehzeit 15:30 Uhr");
      });

      // Clear all filters
      fireEvent.click(screen.getByTestId("clear-filters"));
      await waitFor(() => {
        expect(screen.getByTestId("filter-pickupTime")).toHaveValue("all");
        expect(
          screen.queryByTestId("active-filter-pickupTime"),
        ).not.toBeInTheDocument();
      });
    });

    it("displays pickup time on student cards when present", async () => {
      render(<StudentSearchPage />);

      await waitFor(() => {
        // Max has pickup_time="15:30"
        expect(screen.getByText(new RegExp(`^Max\\b`))).toBeInTheDocument();
      });

      // Verify pickup time text appears for students with pickup_time
      await waitFor(() => {
        expect(screen.getByText("Gehzeit: 15:30 Uhr")).toBeInTheDocument();
        expect(screen.getByText("Gehzeit: 16:00 Uhr")).toBeInTheDocument();
      });
    });

    it("displays fallback pickup row when student has no pickup_time but has full access", async () => {
      // Use only one student without pickup_time
      const studentsNoPickup = [
        {
          id: "20",
          first_name: "NoPickup",
          second_name: "Student",
          school_class: "2a",
          group_name: "Gruppe A",
          current_location: "Raum 101",
          has_full_access: true,
        },
      ];

      const swrModule = await import("~/lib/swr");
      mockUseSWRAuthWithStudents(swrModule, {
        data: { students: studentsNoPickup },
        isLoading: false,
        error: null,
      } as ReturnType<typeof swrModule.useSWRAuth>);

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(
          screen.getByText(new RegExp(`^NoPickup\\b`)),
        ).toBeInTheDocument();
      });

      // Should show the "Gehzeit:" em-dash fallback for students with full access but no pickup time
      expect(screen.getByText("Gehzeit: –")).toBeInTheDocument();
    });
  });

  describe("Arrival Time Filtering", () => {
    it("filters students by specific arrival time", async () => {
      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByText(new RegExp(`^Max\\b`))).toBeInTheDocument();
      });

      fireEvent.change(screen.getByTestId("filter-arrivalTime"), {
        target: { value: "08:15" },
      });

      await waitFor(() => {
        expect(screen.getByText(new RegExp(`^Tom\\b`))).toBeInTheDocument();
        expect(
          screen.queryByText(new RegExp(`^Max\\b`)),
        ).not.toBeInTheDocument();
        expect(
          screen.queryByText(new RegExp(`^Anna\\b`)),
        ).not.toBeInTheDocument();
        expect(
          screen.queryByText(new RegExp(`^Lisa\\b`)),
        ).not.toBeInTheDocument();
      });
    });

    it("filters students with no arrival time while excluding redacted and exception-only rows", async () => {
      const studentsWithArrivalGaps = [
        {
          id: "30",
          first_name: "NoArrival",
          second_name: "Student",
          school_class: "1a",
          current_location: "Raum 101",
          has_full_access: true,
        },
        {
          id: "31",
          first_name: "Exception",
          second_name: "Student",
          school_class: "1a",
          current_location: "Raum 101",
          arrival_is_exception: true,
          has_full_access: true,
        },
        {
          id: "32",
          first_name: "RedactedArrival",
          second_name: "Student",
          school_class: "1a",
          current_location: "Raum 101",
          has_full_access: false,
        },
      ];

      const swrModule = await import("~/lib/swr");
      mockUseSWRAuthWithStudents(swrModule, {
        data: { students: studentsWithArrivalGaps },
        isLoading: false,
        error: null,
      } as ReturnType<typeof swrModule.useSWRAuth>);

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(
          screen.getByText(new RegExp(`^NoArrival\\b`)),
        ).toBeInTheDocument();
      });

      fireEvent.change(screen.getByTestId("filter-arrivalTime"), {
        target: { value: "none" },
      });

      await waitFor(() => {
        expect(
          screen.getByText(new RegExp(`^NoArrival\\b`)),
        ).toBeInTheDocument();
        expect(
          screen.queryByText(new RegExp(`^Exception\\b`)),
        ).not.toBeInTheDocument();
        expect(
          screen.queryByText(new RegExp(`^RedactedArrival\\b`)),
        ).not.toBeInTheDocument();
        expect(
          screen.getByTestId("active-filter-arrivalTime"),
        ).toHaveTextContent("Keine Ankunftszeit");
      });
    });
  });

  describe("Sorting and Grouping", () => {
    it("sorts students by pickup time before students without pickup times", async () => {
      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByText(new RegExp(`^Max\\b`))).toBeInTheDocument();
      });

      fireEvent.change(screen.getByTestId("filter-sort"), {
        target: { value: "pickup" },
      });

      await waitFor(() => {
        expect(screen.getByTestId("active-filter-sort")).toHaveTextContent(
          "Nächste Abholung",
        );
      });

      const renderedNames = screen
        .getAllByText(/^(Max|Tom|Lisa|Anna) /)
        .map((node) => node.textContent?.split(" ")[0]);
      expect(renderedNames).toEqual(["Max", "Tom", "Lisa", "Anna"]);
    });

    it("forwards the pickup status filter to the backend (#1492)", async () => {
      mockSearchParams.set("pickup_status", "self");
      const { studentService } = await import("~/lib/api");
      const swrModule = await import("~/lib/swr");
      const fetcher = captureStudentsFetcher(swrModule);

      render(<StudentSearchPage />);

      await waitFor(() => expect(fetcher.current).not.toBeNull());
      await fetcher.current!();

      expect(studentService.getStudents).toHaveBeenCalledWith(
        expect.objectContaining({ pickupStatus: "self" }),
      );
      expect(
        screen.getByTestId("active-filter-pickupStatus"),
      ).toHaveTextContent("Geht alleine nach Hause");
    });

    it("forwards the 'no pickup rule' filter to the backend (#1492)", async () => {
      mockSearchParams.set("pickup_status", "none");
      const { studentService } = await import("~/lib/api");
      const swrModule = await import("~/lib/swr");
      const fetcher = captureStudentsFetcher(swrModule);

      render(<StudentSearchPage />);

      await waitFor(() => expect(fetcher.current).not.toBeNull());
      await fetcher.current!();

      expect(studentService.getStudents).toHaveBeenCalledWith(
        expect.objectContaining({ pickupStatus: "none" }),
      );
      expect(
        screen.getByTestId("active-filter-pickupStatus"),
      ).toHaveTextContent("Keine Abholregelung");
    });

    // GDPR exclusion of redacted students from the pickup-status filter is
    // enforced and tested server-side now (see backend applyAdministrativeFilters).

    it("groups students by the selected day's departure modes", async () => {
      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByText(new RegExp(`^Max\\b`))).toBeInTheDocument();
      });

      fireEvent.change(screen.getByTestId("filter-groupMode"), {
        target: { value: "pickup-status" },
      });

      await waitFor(() => {
        expect(screen.getByTestId("active-filter-groupMode")).toHaveTextContent(
          "Ansicht: Nach Abholregelung",
        );
      });

      expect(
        screen.getAllByRole("heading", { level: 2 }).map((h) => h.textContent),
      ).toEqual(["Geht alleine nach Hause", "Wird abgeholt"]);
    });

    it("shows every allowed departure mode and a placeholder for a missing rule", async () => {
      const swrModule = await import("~/lib/swr");
      mockUseSWRAuthWithStudents(swrModule, {
        data: {
          students: [
            {
              ...mockStudents[0]!,
              departure_modes: ["alone", "bus", "pickup", "accompanied"],
            },
            {
              ...mockStudents[1]!,
              departure_modes: [],
            },
            {
              ...mockStudents[2]!,
              id: "restricted",
              first_name: "Restricted",
              second_name: "Child",
              departure_modes: ["pickup"],
              has_full_access: false,
            },
          ],
        },
        error: undefined,
        isLoading: false,
        mutate: vi.fn(),
      } as never);

      render(<StudentSearchPage />);

      expect(
        await screen.findByText(
          "Heimweg: Geht alleine nach Hause, Bus, Wird abgeholt, Mit anderem Kind",
        ),
      ).toBeInTheDocument();
      expect(screen.getByText("Heimweg: –")).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: /Restricted Child/ }),
      ).not.toHaveTextContent("Heimweg:");
    });

    it("shows an unknown legacy departure rule without treating it as self-going", async () => {
      const swrModule = await import("~/lib/swr");
      mockUseSWRAuthWithStudents(swrModule, {
        data: {
          students: [
            {
              ...mockStudents[0]!,
              departure_modes: [],
              departure_label: "Taxi mit Begleitperson",
            },
          ],
        },
        error: undefined,
        isLoading: false,
        mutate: vi.fn(),
      } as never);

      render(<StudentSearchPage />);

      expect(
        await screen.findByText("Heimweg: Taxi mit Begleitperson"),
      ).toBeInTheDocument();
      expect(screen.queryByText("Heimweg: Geht alleine nach Hause")).toBeNull();
    });

    it("groups students by status in operational order", async () => {
      const studentsByStatus = [
        { ...mockStudents[1]!, first_name: "HomeChild" },
        { ...mockStudents[3]!, first_name: "YardChild" },
        { ...mockStudents[2]!, first_name: "TransitChild" },
        { ...mockStudents[0]!, first_name: "PresentChild" },
        { ...mockStudents[0]!, id: "9", first_name: "SickChild", sick: true },
        {
          ...mockStudents[1]!,
          id: "10",
          first_name: "ExcusedChild",
          excused: true,
        },
      ];
      const swrModule = await import("~/lib/swr");
      mockUseSWRAuthWithStudents(swrModule, {
        data: { students: studentsByStatus },
        isLoading: false,
        error: null,
      } as ReturnType<typeof swrModule.useSWRAuth>);

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(
          screen.getByText(new RegExp(`^PresentChild\\b`)),
        ).toBeInTheDocument();
      });

      fireEvent.change(screen.getByTestId("filter-groupMode"), {
        target: { value: "status" },
      });

      await waitFor(() => {
        expect(screen.getByTestId("active-filter-groupMode")).toHaveTextContent(
          "Ansicht: Nach Status",
        );
      });

      expect(
        screen.getAllByRole("heading", { level: 2 }).map((h) => h.textContent),
      ).toEqual([
        "Anwesend",
        "Unterwegs",
        "Schulhof",
        "Krank",
        "Entschuldigt",
        "Abwesend",
      ]);
    });

    it("groups redacted and missing room/time values under explicit fallback labels", async () => {
      const groupedFixture = [
        {
          id: "40",
          first_name: "Hidden",
          second_name: "Student",
          school_class: "1a",
          current_location: "Raum 101",
          has_full_access: false,
        },
        {
          id: "41",
          first_name: "Home",
          second_name: "Student",
          school_class: "1a",
          current_location: "Zuhause",
          has_full_access: true,
        },
        {
          id: "42",
          first_name: "Room",
          second_name: "Student",
          school_class: "1a",
          current_location: "Gruppe A - Atelier",
          has_full_access: true,
        },
      ];
      const swrModule = await import("~/lib/swr");
      mockUseSWRAuthWithStudents(swrModule, {
        data: { students: groupedFixture },
        isLoading: false,
        error: null,
      } as ReturnType<typeof swrModule.useSWRAuth>);

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByText(new RegExp(`^Hidden\\b`))).toBeInTheDocument();
      });

      fireEvent.change(screen.getByTestId("filter-groupMode"), {
        target: { value: "room" },
      });

      await waitFor(() => {
        expect(
          screen.getByRole("heading", { name: "Atelier" }),
        ).toBeInTheDocument();
        expect(
          screen.getByRole("heading", { name: "Kein Raum" }),
        ).toBeInTheDocument();
        expect(
          screen.getByRole("heading", { name: "Nicht einsehbar" }),
        ).toBeInTheDocument();
      });
    });
  });

  describe("Tracking Filter (Aktivitäten heute)", () => {
    // Fixture: two students, two configured labels; student "1" completed
    // Hausaufgaben, student "2" has done nothing yet.
    const trackingStudents = [
      {
        id: "1",
        first_name: "Felix",
        second_name: "Schneider",
        school_class: "1a",
        group_name: "Sterne",
        current_location: "Raum 101",
        has_full_access: true,
      },
      {
        id: "2",
        first_name: "Emma",
        second_name: "Meyer",
        school_class: "1a",
        group_name: "Sterne",
        current_location: "Raum 101",
        has_full_access: true,
      },
    ];

    const trackingTwoLabels: TrackingIndicatorsResponse = {
      labels: ["Hausaufgaben", "Mensa"],
      results: {
        "1": [true, false],
        "2": [false, false],
      },
    };

    // Configures useSWRAuth to return students for the students key and
    // the provided tracking response for the tracking-indicators key.
    async function primeTracking(opts: {
      tracking?: TrackingIndicatorsResponse;
      students?: typeof trackingStudents;
    }) {
      const swrModule = await import("~/lib/swr");
      const studentsResult = {
        data: { students: opts.students ?? trackingStudents },
        isLoading: false,
        error: null,
      } as ReturnType<typeof swrModule.useSWRAuth>;
      const trackingResult = {
        data: opts.tracking,
        isLoading: false,
        error: null,
      } as ReturnType<typeof swrModule.useSWRAuth>;
      vi.mocked(swrModule.useSWRAuth).mockImplementation((key: unknown) => {
        if (key === "search-rooms-list") {
          return {
            data: [],
            isLoading: false,
            error: null,
          } as ReturnType<typeof swrModule.useSWRAuth>;
        }

        if (typeof key === "string" && key.startsWith("tracking-indicators-")) {
          return trackingResult;
        }
        return studentsResult;
      });
    }

    it("does not render the tracking dropdown when no labels are configured", async () => {
      await primeTracking({ tracking: { labels: [], results: {} } });
      render(<StudentSearchPage />);

      await waitFor(() =>
        expect(screen.getByText(new RegExp(`^Felix\\b`))).toBeInTheDocument(),
      );
      expect(screen.queryByTestId("filter-tracking")).not.toBeInTheDocument();
    });

    it("renders the dropdown with Alle / per-label / Noch nichts erledigt options", async () => {
      await primeTracking({ tracking: trackingTwoLabels });
      render(<StudentSearchPage />);

      await waitFor(() =>
        expect(screen.getByTestId("filter-tracking")).toBeInTheDocument(),
      );
      const select = screen.getByTestId("filter-tracking");
      expect(select.querySelector('option[value="all"]')).toHaveTextContent(
        "Alle Aktivitäten heute",
      );
      expect(
        select.querySelector('option[value="missing:0"]'),
      ).toHaveTextContent("Noch nicht in Hausaufgaben");
      expect(
        select.querySelector('option[value="missing:1"]'),
      ).toHaveTextContent("Noch nicht in Mensa");
      expect(
        select.querySelector('option[value="none_visited"]'),
      ).toHaveTextContent("Noch nichts erledigt");
    });

    it("selecting 'missing:0' hides students who already completed that label", async () => {
      await primeTracking({ tracking: trackingTwoLabels });
      render(<StudentSearchPage />);

      await waitFor(() =>
        expect(screen.getByText(new RegExp(`^Felix\\b`))).toBeInTheDocument(),
      );

      fireEvent.change(screen.getByTestId("filter-tracking"), {
        target: { value: "missing:0" },
      });

      await waitFor(() => {
        // Felix completed Hausaufgaben (results[0] === true) → hidden.
        expect(
          screen.queryByText(new RegExp(`^Felix\\b`)),
        ).not.toBeInTheDocument();
        // Emma hasn't → visible.
        expect(screen.getByText(new RegExp(`^Emma\\b`))).toBeInTheDocument();
      });
    });

    it("'none_visited' keeps only students missing every configured indicator", async () => {
      await primeTracking({ tracking: trackingTwoLabels });
      render(<StudentSearchPage />);

      await waitFor(() =>
        expect(screen.getByText(new RegExp(`^Felix\\b`))).toBeInTheDocument(),
      );

      fireEvent.change(screen.getByTestId("filter-tracking"), {
        target: { value: "none_visited" },
      });

      await waitFor(() => {
        expect(
          screen.queryByText(new RegExp(`^Felix\\b`)),
        ).not.toBeInTheDocument();
        expect(screen.getByText(new RegExp(`^Emma\\b`))).toBeInTheDocument();
      });
    });

    it("shows the chip with the correct label and clears it when clicked", async () => {
      await primeTracking({ tracking: trackingTwoLabels });
      render(<StudentSearchPage />);

      await waitFor(() =>
        expect(screen.getByTestId("filter-tracking")).toBeInTheDocument(),
      );

      fireEvent.change(screen.getByTestId("filter-tracking"), {
        target: { value: "missing:1" },
      });

      await waitFor(() => {
        expect(screen.getByTestId("active-filter-tracking")).toHaveTextContent(
          "Noch nicht in Mensa",
        );
      });

      fireEvent.click(screen.getByTestId("active-filter-tracking"));

      await waitFor(() => {
        expect(
          screen.queryByTestId("active-filter-tracking"),
        ).not.toBeInTheDocument();
        expect(screen.getByTestId("filter-tracking")).toHaveValue("all");
      });
    });

    it("'Alle zurücksetzen' clears the tracking filter", async () => {
      await primeTracking({ tracking: trackingTwoLabels });
      render(<StudentSearchPage />);

      await waitFor(() =>
        expect(screen.getByTestId("filter-tracking")).toBeInTheDocument(),
      );

      fireEvent.change(screen.getByTestId("filter-tracking"), {
        target: { value: "none_visited" },
      });
      await waitFor(() =>
        expect(
          screen.getByTestId("active-filter-tracking"),
        ).toBeInTheDocument(),
      );

      fireEvent.click(screen.getByTestId("clear-filters"));

      await waitFor(() => {
        expect(
          screen.queryByTestId("active-filter-tracking"),
        ).not.toBeInTheDocument();
        expect(screen.getByTestId("filter-tracking")).toHaveValue("all");
      });
    });

    it("hides redacted students (has_full_access === false) while a tracking filter is active", async () => {
      const mixed = [
        trackingStudents[0],
        {
          id: "9",
          first_name: "Redacted",
          second_name: "Kid",
          school_class: "1a",
          group_name: "Sterne",
          current_location: "Raum 101",
          has_full_access: false,
        },
      ];
      // Only include tracking data for id "1"; redacted student has no row.
      await primeTracking({
        tracking: {
          labels: ["Hausaufgaben"],
          results: { "1": [false] },
        },
        students: mixed as typeof trackingStudents,
      });
      render(<StudentSearchPage />);

      await waitFor(() =>
        expect(
          screen.getByText(new RegExp(`^Redacted\\b`)),
        ).toBeInTheDocument(),
      );

      // Initially both visible.
      expect(screen.getByText(new RegExp(`^Felix\\b`))).toBeInTheDocument();
      expect(screen.getByText(new RegExp(`^Redacted\\b`))).toBeInTheDocument();

      fireEvent.change(screen.getByTestId("filter-tracking"), {
        target: { value: "missing:0" },
      });

      await waitFor(() => {
        // Felix missing HA → shown.
        expect(screen.getByText(new RegExp(`^Felix\\b`))).toBeInTheDocument();
        // Redacted kid hidden: no tracking data AND has_full_access false.
        expect(
          screen.queryByText(new RegExp(`^Redacted\\b`)),
        ).not.toBeInTheDocument();
      });
    });
  });

  describe("Arrival sort mode", () => {
    it("re-sorts the list when switching sort=arrival", async () => {
      render(<StudentSearchPage />);

      await waitFor(() =>
        expect(screen.getByText(new RegExp(`^Max\\b`))).toBeInTheDocument(),
      );

      // Flip to arrival sort mode: component should re-render without error.
      fireEvent.change(screen.getByTestId("filter-sort"), {
        target: { value: "arrival" },
      });

      await waitFor(() =>
        expect(screen.getByTestId("filter-sort")).toHaveValue("arrival"),
      );

      // All four default students still present post-sort.
      expect(screen.getByText(new RegExp(`^Max\\b`))).toBeInTheDocument();
      expect(screen.getByText(new RegExp(`^Anna\\b`))).toBeInTheDocument();
      expect(screen.getByText(new RegExp(`^Tom\\b`))).toBeInTheDocument();
      expect(screen.getByText(new RegExp(`^Lisa\\b`))).toBeInTheDocument();
    });

    it("offers pickup sorting as a dedicated sort mode", async () => {
      render(<StudentSearchPage />);

      await waitFor(() =>
        expect(screen.getByText(new RegExp(`^Max\\b`))).toBeInTheDocument(),
      );

      fireEvent.change(screen.getByTestId("filter-sort"), {
        target: { value: "pickup" },
      });

      await waitFor(() =>
        expect(screen.getByTestId("filter-sort")).toHaveValue("pickup"),
      );
      expect(screen.getByText(new RegExp(`^Max\\b`))).toBeInTheDocument();
      expect(screen.getByText(new RegExp(`^Tom\\b`))).toBeInTheDocument();
    });

    it("filters by a specific arrival time", async () => {
      render(<StudentSearchPage />);

      await waitFor(() =>
        expect(screen.getByText(new RegExp(`^Max\\b`))).toBeInTheDocument(),
      );

      fireEvent.change(screen.getByTestId("filter-arrivalTime"), {
        target: { value: "08:15" },
      });

      await waitFor(() => {
        expect(screen.getByText(new RegExp(`^Tom\\b`))).toBeInTheDocument();
      });
      expect(screen.queryByText(new RegExp(`^Max\\b`))).not.toBeInTheDocument();
      expect(
        screen.queryByText(new RegExp(`^Anna\\b`)),
      ).not.toBeInTheDocument();
      expect(
        screen.queryByText(new RegExp(`^Lisa\\b`)),
      ).not.toBeInTheDocument();
    });

    it("groups the result list when a grouping mode is selected", async () => {
      render(<StudentSearchPage />);

      await waitFor(() =>
        expect(screen.getByText(new RegExp(`^Max\\b`))).toBeInTheDocument(),
      );

      fireEvent.change(screen.getByTestId("filter-groupMode"), {
        target: { value: "status" },
      });

      await waitFor(() => {
        expect(screen.getAllByTestId("student-group")).toHaveLength(4);
      });
      expect(
        screen.getAllByText(new RegExp(`^Anwesend\\b`)).length,
      ).toBeGreaterThan(0);
      expect(
        screen.getAllByText(new RegExp(`^Abwesend\\b`)).length,
      ).toBeGreaterThan(0);
    });

    it("shows the room filter even without a room deep-link", async () => {
      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByTestId("filter-room")).toBeInTheDocument();
      });
      expect(
        screen.getByRole("option", { name: "Raum 101" }),
      ).toBeInTheDocument();
    });

    it("requests a large first page for the room filter options", async () => {
      const swrModule = await import("~/lib/swr");
      vi.mocked(swrModule.useSWRAuth).mockImplementation((key, fetcher) => {
        if (key === "search-rooms-list") {
          void (fetcher as () => Promise<unknown>)();
          return {
            data: [],
            isLoading: false,
            error: null,
          } as ReturnType<typeof swrModule.useSWRAuth>;
        }

        return {
          data: { students: mockStudents },
          isLoading: false,
          error: null,
        } as ReturnType<typeof swrModule.useSWRAuth>;
      });

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(roomService.getRooms).toHaveBeenCalledWith({
          page: 1,
          pageSize: 1000,
        });
      });
    });

    it("syncs room selection changes into the URL", async () => {
      mockSearchParams.set("room_id", "42");
      mockSearchParams.set("room_name", "Alter Raum");
      window.history.replaceState(
        { preserved: true },
        "",
        "/students/search?room_id=42&room_name=Alter+Raum&status=anwesend",
      );

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByTestId("filter-room")).toHaveValue("42");
      });

      fireEvent.change(screen.getByTestId("filter-room"), {
        target: { value: "101" },
      });

      const url = new URL(window.location.href);
      expect(url.searchParams.get("room_id")).toBe("101");
      expect(url.searchParams.get("room_name")).toBe("Raum 101");
      expect(url.searchParams.get("status")).toBe("anwesend");
      expect(window.history.state).toEqual({ preserved: true });
    });

    it("removes only room params when clearing the room filter", async () => {
      mockSearchParams.set("room_id", "42");
      mockSearchParams.set("room_name", "Alter Raum");
      window.history.replaceState(
        { preserved: true },
        "",
        "/students/search?room_id=42&room_name=Alter+Raum&status=anwesend",
      );

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByTestId("filter-room")).toHaveValue("42");
      });

      fireEvent.change(screen.getByTestId("filter-room"), {
        target: { value: "" },
      });

      const url = new URL(window.location.href);
      expect(url.searchParams.has("room_id")).toBe(false);
      expect(url.searchParams.has("room_name")).toBe(false);
      expect(url.searchParams.get("status")).toBe("anwesend");
      expect(window.history.state).toEqual({ preserved: true });
    });

    it("does not group status-only locations as rooms", async () => {
      const swrModule = await import("~/lib/swr");
      mockUseSWRAuthWithStudents(swrModule, {
        data: {
          students: [
            {
              id: "5",
              first_name: "Nina",
              second_name: "Status",
              school_class: "1a",
              current_location: "Anwesend",
              has_full_access: true,
            },
            {
              id: "6",
              first_name: "Rita",
              second_name: "Redacted",
              school_class: "1a",
              current_location: "Anwesend",
              has_full_access: false,
            },
          ],
        },
        isLoading: false,
        error: null,
      } as ReturnType<typeof swrModule.useSWRAuth>);

      render(<StudentSearchPage />);

      await waitFor(() => {
        expect(screen.getByText(new RegExp(`^Nina\\b`))).toBeInTheDocument();
      });

      fireEvent.change(screen.getByTestId("filter-groupMode"), {
        target: { value: "room" },
      });

      await waitFor(() => {
        expect(screen.getByText("Kein Raum")).toBeInTheDocument();
        expect(screen.getByText("Nicht einsehbar")).toBeInTheDocument();
      });
      expect(
        screen.queryByRole("heading", { name: "Anwesend" }),
      ).not.toBeInTheDocument();
    });
  });
});
