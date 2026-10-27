/**
 * Behaviour tests for StudentSearchPage behind one stubbed mock header. Each
 * describe is a former test file and keeps its name and tests:
 *
 * - room_id deep-link (#1323), was page.room-filter
 * - Meine Gruppe and Meine Aufsicht as tabs of Alle Kinder (#1321), was
 *   page.child-scope
 * - Laufgemeinschaft grouping (#1694), was page.companion-grouping
 * - planning date (#1939), was page.date-filter
 *
 * The page has many transitive dependencies; the mocks below are deliberately
 * minimal. A describe sets in its beforeEach what its former file mocked
 * differently: the session, the supervision and user context, and the header
 * stub (`mockPageHeader`).
 *
 * page.school-checkin, page.school-checkin-detailed and page.nav-reset need a
 * different mock header (check-in hook and FAB, real location and check-in
 * helpers, the real header), so they stay separate, as does page.test.tsx.
 */
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import {
  afterAll,
  afterEach,
  beforeEach,
  describe,
  expect,
  it,
  vi,
} from "vitest";
import type { FilterConfig } from "~/components/ui/page-header/types";
import { berlinTodayISO, parseISODate, toISODate } from "~/lib/date-helpers";

const {
  mockGetStudents,
  mockUseSWRAuth,
  mockUseImmutableSWR,
  mockPush,
  mockUseSession,
  mockUseOptionalSupervision,
  mockUseUserContext,
  mockPageHeader,
  flags,
} = vi.hoisted(() => ({
  mockGetStudents: vi.fn(),
  mockUseSWRAuth: vi.fn(),
  mockUseImmutableSWR: vi.fn(),
  mockPush: vi.fn(),
  mockUseSession: vi.fn(),
  mockUseOptionalSupervision: vi.fn(),
  mockUseUserContext: vi.fn(),
  mockPageHeader: vi.fn(),
  flags: { childSearchOnly: false },
}));

let currentSearch = new URLSearchParams();
// Mutable clock so the planning-date tests can advance the page across
// midnight via rerender; null reads the frozen test clock.
let clockNow: Date | null = null;

vi.mock("next-auth/react", () => ({
  useSession: () => mockUseSession(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, replace: vi.fn() }),
  useSearchParams: () => currentSearch,
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: mockPush }),
}));

vi.mock("~/lib/supervision-context", () => ({
  useOptionalSupervision: () => mockUseOptionalSupervision(),
}));

vi.mock("~/lib/tenant-context", () => ({
  useTenantRoutingModeSafe: () => "subdomain",
  useChildSearchOnlyNavigation: () => flags.childSearchOnly,
  useTenant: () => ({ tenantSlug: "t", tenant: null }),
  useTenantSafe: () => ({
    tenantSlug: "t",
    tenant: { studentPhotosEnabled: true },
  }),
  useTenantSlugSafe: () => "t",
  usePresenceMode: () => "detailed",
  useAttendanceWebEnabled: vi.fn(() => true),
  useNFCEnabled: () => true,
  useOpenCareGroupMode: () => false,
  TenantProvider: ({ children }: { children: React.ReactNode }) => children,
}));

vi.mock("~/lib/breadcrumb-context", () => ({
  useSetBreadcrumb: vi.fn(),
  useBreadcrumb: () => ({ breadcrumb: {}, setBreadcrumb: vi.fn() }),
}));

vi.mock("~/components/ui/page-header/PageHeaderWithSearch", () => ({
  PageHeaderWithSearch: (props: { filters?: FilterConfig[] }) =>
    mockPageHeader(props),
}));

vi.mock("~/components/ui/date-picker", () => ({
  DatePicker: () => <div data-testid="date-picker" />,
}));

vi.mock("~/components/ui/alert", () => ({
  Alert: ({ message }: { message: string }) => <div>{message}</div>,
}));

vi.mock("@/components/ui/location-badge", () => ({
  LocationBadge: () => <span />,
}));

vi.mock("@/components/ui/student-presence-badge", () => ({
  StudentPresenceBadge: () => <span data-testid="presence-badge" />,
}));

vi.mock("~/components/ui/data-table", () => ({
  DataTableStatusBadge: ({
    active,
    activeLabel,
    inactiveLabel,
  }: {
    active: boolean;
    activeLabel: string;
    inactiveLabel: string;
  }) => (
    <span data-testid="planning-badge">
      {active ? activeLabel : inactiveLabel}
    </span>
  ),
}));

vi.mock("~/components/students/tracking-indicators", () => ({
  TrackingIndicators: () => <span />,
}));

vi.mock("~/components/students/school-checkin-fab", () => ({
  SchoolCheckinFab: () => <span />,
}));

vi.mock("~/components/students/school-checkin-mode-mobile", () => ({
  SchoolCheckinModeMobile: () => <span />,
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

vi.mock("~/lib/location-helper", async (importOriginal) => {
  const actual = await importOriginal<typeof import("~/lib/location-helper")>();
  return {
    ...actual,
    isHomeLocation: () => false,
    isPresentLocation: () => true,
    isTransitLocation: () => false,
    isSchoolyardLocation: () => false,
  };
});

vi.mock("~/lib/student-helpers", () => ({
  SCHOOL_YEAR_FILTER_OPTIONS: [{ value: "all", label: "Alle" }],
  getSchoolYear: () => "1",
}));

vi.mock("~/lib/pickup-helpers", () => ({
  useMinuteClock: () => clockNow ?? new Date(),
}));

vi.mock("~/lib/active-api", () => ({
  activeService: {
    getTrackingIndicators: vi.fn(() =>
      Promise.resolve({ labels: [], results: {} }),
    ),
  },
}));

vi.mock("~/lib/hooks/use-user-context", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/hooks/use-user-context")>()),
  useUserContext: () => mockUseUserContext(),
}));

vi.mock("~/lib/api", () => ({
  studentService: {
    getStudents: (...args: unknown[]) => mockGetStudents(...args),
  },
  groupService: {
    getGroups: vi.fn(() => Promise.resolve([])),
  },
  roomService: {
    getRooms: vi.fn(() => Promise.resolve([])),
  },
}));

vi.mock("~/lib/swr", () => ({
  useImmutableSWR: (...args: unknown[]) => mockUseImmutableSWR(...args),
  useSWRAuth: (key: string | null, fetcher?: () => Promise<unknown>) =>
    mockUseSWRAuth(key, fetcher),
  mutate: vi.fn(),
  useTenantMutate: () => vi.fn(),
}));

vi.mock("~/components/students/student-card", () => ({
  StudentCard: ({
    studentId,
    onClick,
    locationBadge,
  }: {
    studentId: string;
    onClick: () => void;
    locationBadge: React.ReactNode;
  }) => (
    <button
      type="button"
      data-testid={`student-card-${studentId}`}
      onClick={onClick}
    >
      card-{studentId}
      {locationBadge}
    </button>
  ),
  SchoolClassIcon: () => <span />,
  GroupIcon: () => <span />,
  DepartureModeIcon: () => <span />,
  StudentInfoRow: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  PickupTimeRow: () => <div />,
  ArrivalTimeRow: () => <div />,
  StudentAbsenceRow: ({ label }: { label: string }) => <div>{label}</div>,
  StudentPendingExcusedRow: () => <div />,
}));

// Die Karte „Unterwegs“ (#5026) prüft ihre Rechte selbst
// (student-search-transit-card.test.tsx); hier zählt, was die Seite ihr gibt.
vi.mock("~/components/students/student-search-transit-card", () => ({
  StudentSearchTransitCard: ({
    show,
    fromParam,
    students,
  }: {
    show: boolean;
    fromParam: string;
    students: readonly unknown[];
  }) => (
    <div
      data-testid="transit-card"
      data-show={String(show)}
      data-from={fromParam}
      data-count={students.length}
    />
  ),
}));

import StudentSearchPage from "./page";

// Defaults of the former files; a describe overrides what its file mocked
// differently. Each hook returns a fresh object per render, as the former
// inline mocks did.
beforeEach(() => {
  currentSearch = new URLSearchParams();
  clockNow = null;
  flags.childSearchOnly = false;
  mockUseSession.mockImplementation(() => ({
    data: { user: { token: "t" } },
    status: "authenticated",
  }));
  mockUseOptionalSupervision.mockImplementation(() => ({
    hasGroups: false,
    isSupervising: false,
    isLoadingGroups: false,
    isLoadingSupervision: false,
    overviewEnabled: false,
    supervisedRooms: [],
    groups: [],
    refresh: () => undefined,
  }));
  mockUseUserContext.mockImplementation(() => ({
    userContext: {
      educationalGroupIds: [],
      educationalGroupRoomNames: [],
      supervisedRoomNames: [],
    },
  }));
  mockPageHeader.mockImplementation(() => <div data-testid="header" />);
});

// Each describe was its own file and started on a fresh document: leave no
// stored filters or URL behind for the next one.
function resetDocumentAfterAll() {
  afterAll(() => {
    localStorage.clear();
    window.history.replaceState(null, "", "/");
  });
}

describe("StudentSearchPage — room_id deep-link (#1323)", () => {
  resetDocumentAfterAll();

  const mockStudent = {
    id: "7",
    first_name: "Max",
    second_name: "Mustermann",
    school_class: "1a",
    current_location: "Anwesend",
    has_full_access: true,
  };

  beforeEach(() => {
    vi.clearAllMocks();
    currentSearch = new URLSearchParams();
    localStorage.clear();

    // Default: no in-flight tracking-indicators result.
    mockUseImmutableSWR.mockReturnValue({
      data: [],
      isLoading: false,
      error: null,
    });

    // Capture the SWR fetcher and invoke it so the studentService spy actually
    // sees the call — useSWRAuth's real impl runs the fetcher synchronously
    // for the test path; we mimic that here.
    mockUseSWRAuth.mockImplementation(
      (key: string | null, fetcher?: () => Promise<unknown>) => {
        if (fetcher) {
          // Fire-and-forget — we only assert on mockGetStudents call shape.
          void fetcher().catch(() => undefined);
        }
        if (key === "search-rooms-list") {
          return {
            data: [],
            isLoading: false,
            error: null,
          };
        }
        return {
          data: { students: [mockStudent] },
          isLoading: false,
          error: null,
        };
      },
    );

    mockGetStudents.mockResolvedValue({ students: [mockStudent] });
  });

  afterEach(() => {
    cleanup();
    localStorage.clear();
  });

  it("forwards room_id from the URL into studentService.getStudents", async () => {
    currentSearch = new URLSearchParams({
      room_id: "42",
      room_name: "OGS-Raum 1",
    });

    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(mockGetStudents).toHaveBeenCalled();
    });

    // Find the call with our roomId — there may be multiple if the page
    // re-renders. The contract is: the first call after mount must carry
    // the URL-derived room_id, otherwise the page would render the
    // unfiltered list before correcting itself.
    const calls = mockGetStudents.mock.calls.map(
      (c) => c[0] as Record<string, unknown>,
    );
    const filteredCall = calls.find((c) => c.roomId === "42");
    expect(filteredCall).toBeDefined();
    expect(filteredCall?.roomId).toBe("42");
  });

  it("does not forward roomId when no room_id query param is present", async () => {
    currentSearch = new URLSearchParams();

    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(mockGetStudents).toHaveBeenCalled();
    });

    const calls = mockGetStudents.mock.calls.map(
      (c) => c[0] as Record<string, unknown>,
    );
    expect(calls.every((c) => c.roomId === undefined || c.roomId === "")).toBe(
      true,
    );
  });

  it("preserves room filter in the back-link when drilling into a child", async () => {
    currentSearch = new URLSearchParams({
      room_id: "42",
      room_name: "OGS-Raum 1",
    });

    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-7")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("student-card-7"));

    expect(mockPush).toHaveBeenCalledTimes(1);
    const target = mockPush.mock.calls[0]?.[0] as string;
    // Must be of the form: /students/7?from=<encoded /students/search?room_id=42&room_name=...>
    // Without this, hitting "back" from the detail page drops the filter.
    expect(target.startsWith("/students/7?from=")).toBe(true);
    const fromParam = decodeURIComponent(target.split("from=")[1] ?? "");
    expect(fromParam).toContain("/students/search");
    expect(fromParam).toContain("room_id=42");
    expect(fromParam).toContain("room_name=OGS-Raum");
  });

  it("uses the bare /students/search referrer when there is no room filter", async () => {
    currentSearch = new URLSearchParams();

    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-7")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("student-card-7"));

    const target = mockPush.mock.calls[0]?.[0] as string;
    // Without an active room filter the from= must NOT be encoded — the
    // existing call sites (and the breadcrumb) match on the literal path.
    expect(target).toBe("/students/7?from=/students/search");
  });
});

describe("StudentSearchPage — Kinder nur über Alle Kinder (#1321)", () => {
  resetDocumentAfterAll();

  // Own groups and own supervision, as the former file mocked them.
  beforeEach(() => {
    mockUseSession.mockImplementation(() => ({
      data: { user: { token: "t", roles: ["user"] } },
      status: "authenticated",
    }));
    mockUseOptionalSupervision.mockImplementation(() => ({
      hasGroups: true,
      isSupervising: true,
      isLoadingGroups: false,
      isLoadingSupervision: false,
      overviewEnabled: false,
      supervisedRooms: [{ id: "42", name: "Werkraum", groupId: "900" }],
      ownSupervisedRooms: [{ id: "42", name: "Werkraum", groupId: "900" }],
      groups: [{ id: "5", name: "Bären", is_personal: true }],
      refresh: () => undefined,
    }));
    mockUseUserContext.mockImplementation(() => ({
      userContext: {
        educationalGroupIds: ["5"],
        educationalGroupRoomNames: [],
        supervisedRoomNames: ["Veralteter Raum"],
        supervisedGroups: [
          {
            id: "900",
            name: "Werken",
            roomId: "91",
            room: { id: "91", name: "Veralteter Raum" },
          },
        ],
      },
    }));
  });

  const mockStudent = {
    id: "7",
    first_name: "Max",
    second_name: "Mustermann",
    school_class: "1a",
    current_location: "Anwesend",
    has_full_access: true,
  };

  beforeEach(() => {
    vi.clearAllMocks();
    currentSearch = new URLSearchParams();
    localStorage.clear();
    flags.childSearchOnly = true;

    // Default: no in-flight tracking-indicators result.
    mockUseImmutableSWR.mockReturnValue({
      data: [],
      isLoading: false,
      error: null,
    });

    // Capture the SWR fetcher and invoke it so the studentService spy actually
    // sees the call — useSWRAuth's real impl runs the fetcher synchronously
    // for the test path; we mimic that here.
    mockUseSWRAuth.mockImplementation(
      (key: string | null, fetcher?: () => Promise<unknown>) => {
        if (fetcher) {
          // Fire-and-forget — we only assert on mockGetStudents call shape.
          void fetcher().catch(() => undefined);
        }
        if (key === "search-rooms-list") {
          return {
            data: [],
            isLoading: false,
            error: null,
          };
        }
        return {
          data: { students: [mockStudent] },
          isLoading: false,
          error: null,
        };
      },
    );

    mockGetStudents.mockResolvedValue({ students: [mockStudent] });
  });

  afterEach(() => {
    cleanup();
    localStorage.clear();
  });

  function lastStudentFilters(): Record<string, unknown> | undefined {
    const calls = mockGetStudents.mock.calls;
    return calls[calls.length - 1]?.[0] as Record<string, unknown> | undefined;
  }

  it("shows no tabs while the school keeps the separate entries", () => {
    flags.childSearchOnly = false;

    render(<StudentSearchPage />);

    // Anker: die Kinderliste ist geladen, nur die Reiter fehlen.
    expect(screen.getByTestId("student-card-7")).toBeInTheDocument();
    expect(screen.queryByRole("tab", { name: "Meine Gruppe" })).toBeNull();
    expect(screen.queryByRole("tab", { name: "Meine Aufsicht" })).toBeNull();
  });

  it("offers Alle, Meine Gruppe and Meine Aufsicht, starting on Alle", () => {
    render(<StudentSearchPage />);

    expect(screen.getByRole("tab", { name: "Alle" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(screen.getByRole("tab", { name: "Meine Gruppe" })).toHaveAttribute(
      "aria-selected",
      "false",
    );
    expect(screen.getByRole("tab", { name: "Meine Aufsicht" })).toBeTruthy();
  });

  it("filters to the own groups on Meine Gruppe", async () => {
    render(<StudentSearchPage />);

    fireEvent.click(screen.getByRole("tab", { name: "Meine Gruppe" }));

    await waitFor(() => {
      expect(lastStudentFilters()?.groupId).toEqual(["5"]);
    });
    expect(lastStudentFilters()?.roomId).toBeUndefined();
    expect(window.location.search).toContain("group_id=5");
    expect(screen.getByRole("tab", { name: "Meine Gruppe" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  });

  it("filters to the current supervised room instead of cached user context", async () => {
    render(<StudentSearchPage />);

    fireEvent.click(screen.getByRole("tab", { name: "Meine Aufsicht" }));

    await waitFor(() => {
      expect(lastStudentFilters()?.roomId).toBe("42");
    });
    expect(lastStudentFilters()?.groupId).toEqual([]);

    fireEvent.click(screen.getByRole("tab", { name: "Alle" }));

    await waitFor(() => {
      expect(lastStudentFilters()?.roomId).toBeUndefined();
    });
    expect(lastStudentFilters()?.groupId).toEqual([]);
  });

  it("marks the tab that matches a filter restored from the URL", () => {
    currentSearch = new URLSearchParams({
      room_id: "42",
      room_name: "Werkraum",
    });

    render(<StudentSearchPage />);

    expect(screen.getByRole("tab", { name: "Meine Aufsicht" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  });

  it("keeps the scope tabs keyboard reachable for a stale room filter", () => {
    currentSearch = new URLSearchParams({
      room_id: "91",
      room_name: "Veralteter Raum",
    });

    render(<StudentSearchPage />);

    const all = screen.getByRole("tab", { name: "Alle" });
    expect(all).toHaveAttribute("aria-selected", "false");
    expect(all).toHaveAttribute("tabindex", "0");
    expect(screen.getByRole("tab", { name: "Meine Gruppe" })).toHaveAttribute(
      "aria-selected",
      "false",
    );
    expect(screen.getByRole("tab", { name: "Meine Aufsicht" })).toHaveAttribute(
      "aria-selected",
      "false",
    );
  });

  it("keeps the group and supervision pages reachable from the menu", async () => {
    render(<StudentSearchPage />);

    fireEvent.click(screen.getByRole("button", { name: "Weitere Aktionen" }));

    expect(
      await screen.findByRole("menuitem", { name: /Zur Aufsicht/ }),
    ).toHaveAttribute("href", "/active-supervisions");
    expect(
      screen.getByRole("menuitem", { name: /Zur Gruppenseite/ }),
    ).toHaveAttribute("href", "/ogs-groups");
  });

  it("adds no menu entries while the school keeps the separate entries", async () => {
    flags.childSearchOnly = false;

    render(<StudentSearchPage />);
    fireEvent.click(screen.getByRole("button", { name: "Weitere Aktionen" }));

    expect(
      await screen.findByRole("menuitem", { name: /Abwesenheiten/ }),
    ).toBeTruthy();
    expect(screen.queryByRole("menuitem", { name: /Zur Aufsicht/ })).toBeNull();
    expect(
      screen.queryByRole("menuitem", { name: /Zur Gruppenseite/ }),
    ).toBeNull();
  });

  it("counts the children in the OGS in the status line", async () => {
    render(<StudentSearchPage />);

    // Das Kind der Mocks ist „Anwesend“, also in einem Raum der OGS.
    expect(
      await screen.findByText(
        "1 Kind · 1 in der OGS · 0 in der Schule · 0 zuhause · 0 krank",
      ),
    ).toBeTruthy();
  });
});

describe("StudentSearchPage — Laufgemeinschaft grouping (#1694)", () => {
  resetDocumentAfterAll();

  // Renders the page's filter configs as plain buttons so a test can pick a
  // grouping option without driving the real header's popover/sheet UI.
  function FilterButtonsHeader({ filters }: { filters?: FilterConfig[] }) {
    return (
      <div data-testid="header">
        {(filters ?? []).map((filter) => (
          <div key={filter.id} data-testid={`filter-${filter.id}`}>
            {filter.options.map((option) => (
              <button
                key={option.value}
                type="button"
                data-testid={`filter-${filter.id}-${option.value}`}
                onClick={() => filter.onChange(option.value)}
              >
                {option.label}
              </button>
            ))}
          </div>
        ))}
      </div>
    );
  }

  beforeEach(() => {
    mockPageHeader.mockImplementation(FilterButtonsHeader);
  });

  interface MockStudent {
    id: string;
    name: string;
    first_name: string;
    second_name: string;
    school_class: string;
    current_location: string;
    has_full_access: boolean;
    companion_student_ids?: number[];
  }

  function student(
    id: string,
    first: string,
    last: string,
    companions?: number[],
  ): MockStudent {
    return {
      id,
      name: `${first} ${last}`,
      first_name: first,
      second_name: last,
      school_class: "1a",
      current_location: "Anwesend",
      has_full_access: true,
      ...(companions ? { companion_student_ids: companions } : {}),
    };
  }

  // Anna (1) <-> Ben (2) walk together; Clara (3) walks alone.
  const anna = student("1", "Anna", "Adler", [2]);
  const ben = student("2", "Ben", "Bauer", [1]);
  const clara = student("3", "Clara", "Cordes");

  function renderWithStudents(students: MockStudent[]) {
    mockUseSWRAuth.mockImplementation(
      (key: string | null, fetcher?: () => Promise<unknown>) => {
        if (fetcher) {
          // Fire-and-forget — the grouping only reads the returned data.
          void fetcher().catch(() => undefined);
        }
        if (key === "search-rooms-list") {
          return { data: [], isLoading: false, error: null };
        }
        return { data: { students }, isLoading: false, error: null };
      },
    );
    mockGetStudents.mockResolvedValue({ students });
    return render(<StudentSearchPage />);
  }

  /** Reads the rendered grouping sections as { label, studentIds }. */
  function readGroups() {
    return screen.getAllByTestId("student-group").map((section) => ({
      label: within(section).getAllByRole("heading")[0]?.textContent ?? "",
      studentIds: within(section)
        .getAllByRole("button")
        .map(
          (card) =>
            card.getAttribute("data-testid")?.replace("student-card-", "") ??
            "",
        ),
    }));
  }

  beforeEach(() => {
    vi.clearAllMocks();
    currentSearch = new URLSearchParams();
    localStorage.clear();

    mockUseImmutableSWR.mockReturnValue({
      data: [],
      isLoading: false,
      error: null,
    });
  });

  afterEach(() => {
    cleanup();
    localStorage.clear();
  });

  it("offers 'Nach Laufgemeinschaft' in the grouping dropdown", async () => {
    renderWithStudents([anna, ben, clara]);

    await waitFor(() =>
      expect(screen.getByTestId("student-card-1")).toBeDefined(),
    );

    const groupModeFilter = screen.getByTestId("filter-groupMode");
    expect(
      within(groupModeFilter).getByText("Nach Laufgemeinschaft"),
    ).toBeDefined();
  });

  it("buckets two linked children together under a label naming both", async () => {
    renderWithStudents([anna, ben, clara]);

    await waitFor(() =>
      expect(screen.getByTestId("student-card-1")).toBeDefined(),
    );

    fireEvent.click(screen.getByTestId("filter-groupMode-companions"));

    await waitFor(() =>
      expect(screen.getAllByTestId("student-group").length).toBeGreaterThan(0),
    );

    const groups = readGroups();
    const walkingGroup = groups.find((group) =>
      group.label.includes("Anna Adler"),
    );
    expect(walkingGroup).toBeDefined();
    // Names are joined alphabetically (German collation), not by id order.
    expect(walkingGroup!.label).toContain("Anna Adler, Ben Bauer");
    expect(walkingGroup!.studentIds.sort()).toEqual(["1", "2"]);
  });

  it("puts a child without a companion into 'Ohne Laufgemeinschaft', sorted last", async () => {
    renderWithStudents([anna, ben, clara]);

    await waitFor(() =>
      expect(screen.getByTestId("student-card-3")).toBeDefined(),
    );

    fireEvent.click(screen.getByTestId("filter-groupMode-companions"));

    await waitFor(() =>
      expect(screen.getAllByTestId("student-group").length).toBe(2),
    );

    const groups = readGroups();
    expect(groups.at(-1)!.label).toContain("Ohne Laufgemeinschaft");
    expect(groups.at(-1)!.studentIds).toEqual(["3"]);
  });

  it("merges a transitive chain (A-B, B-C) into ONE group of three", async () => {
    // Only B names both links; A and C know just their neighbour. The union
    // over the links must still produce a single Laufgemeinschaft.
    const a = student("1", "Anna", "Adler", [2]);
    const b = student("2", "Ben", "Bauer", [1, 3]);
    const c = student("3", "Clara", "Cordes", [2]);

    renderWithStudents([a, b, c]);

    await waitFor(() =>
      expect(screen.getByTestId("student-card-1")).toBeDefined(),
    );

    fireEvent.click(screen.getByTestId("filter-groupMode-companions"));

    await waitFor(() =>
      expect(screen.getAllByTestId("student-group").length).toBe(1),
    );

    const groups = readGroups();
    expect(groups[0]!.label).toContain("Anna Adler, Ben Bauer, Clara Cordes");
    expect(groups[0]!.studentIds.sort()).toEqual(["1", "2", "3"]);
  });

  it("counts a companion who is not on the page instead of naming them", async () => {
    // Anna links to Ben, but the current filter left Ben out of the list. Anna
    // still has a Laufgemeinschaft and must not be filed under "walks alone" —
    // that is the one reading a supervisor must never get wrong. Ben's NAME
    // stays out: the page never received it, and shipping it here would hand
    // back exactly the children the filter removed.
    renderWithStudents([anna, clara]);

    await waitFor(() =>
      expect(screen.getByTestId("student-card-1")).toBeDefined(),
    );

    fireEvent.click(screen.getByTestId("filter-groupMode-companions"));

    await waitFor(() =>
      expect(screen.getAllByTestId("student-group").length).toBe(2),
    );

    const groups = readGroups();
    const annasGroup = groups.find((group) =>
      group.label.includes("Anna Adler"),
    );
    expect(annasGroup).toBeDefined();
    expect(annasGroup!.label).toContain("1 weiteres Kind");
    expect(annasGroup!.label).not.toContain("Ben");
    expect(annasGroup!.studentIds).toEqual(["1"]);

    // Clara has no link at all — she is the one who really walks alone.
    const claraGroup = groups.find((group) =>
      group.label.includes("Ohne Laufgemeinschaft"),
    );
    expect(claraGroup).toBeDefined();
    expect(claraGroup!.studentIds).toEqual(["3"]);
  });

  it("keeps a group together when the child bridging it is filtered away", async () => {
    // A-B-C where only B knows both neighbours. With B off the page, A and C
    // have no link between them — they would fall into separate buckets and
    // the Laufgemeinschaft would read as two, even though it is one group of
    // three walking home together.
    const a = student("1", "Anna", "Adler", [2]);
    const c = student("3", "Clara", "Cordes", [2]);

    renderWithStudents([a, c]);

    await waitFor(() =>
      expect(screen.getByTestId("student-card-1")).toBeDefined(),
    );

    fireEvent.click(screen.getByTestId("filter-groupMode-companions"));

    await waitFor(() =>
      expect(screen.getAllByTestId("student-group").length).toBe(1),
    );

    const groups = readGroups();
    expect(groups[0]!.label).toContain("Anna Adler, Clara Cordes");
    expect(groups[0]!.label).toContain("1 weiteres Kind");
    expect(groups[0]!.studentIds.sort()).toEqual(["1", "3"]);
  });

  it("pluralizes the count of children the page is not showing", async () => {
    // Two hidden members, one German sentence — "1 weiteres Kind" vs
    // "2 weitere Kinder"; the wrong form reads as a bug to a school user.
    const a = student("1", "Anna", "Adler", [2, 4]);

    renderWithStudents([a, clara]);

    await waitFor(() =>
      expect(screen.getByTestId("student-card-1")).toBeDefined(),
    );

    fireEvent.click(screen.getByTestId("filter-groupMode-companions"));

    await waitFor(() =>
      expect(screen.getAllByTestId("student-group").length).toBe(2),
    );

    const groups = readGroups();
    const annasGroup = groups.find((group) =>
      group.label.includes("Anna Adler"),
    );
    expect(annasGroup!.label).toContain("2 weitere Kinder");
  });

  it("files a child whose links are redacted under 'Nicht einsehbar', not 'Ohne Laufgemeinschaft'", async () => {
    // The backend resolves the Laufgemeinschaft only for full-access children,
    // so a restricted child arrives WITHOUT companion ids whether or not they
    // walk with someone. Claiming they walk alone would state as fact the one
    // thing this page cannot know.
    const restricted = {
      ...student("4", "Dana", "Dreyer"),
      has_full_access: false,
    };

    renderWithStudents([anna, ben, restricted]);

    await waitFor(() =>
      expect(screen.getByTestId("student-card-4")).toBeDefined(),
    );

    fireEvent.click(screen.getByTestId("filter-groupMode-companions"));

    await waitFor(() =>
      expect(screen.getAllByTestId("student-group").length).toBe(2),
    );

    const groups = readGroups();
    // Last, after the real Laufgemeinschaften.
    expect(groups.at(-1)!.label).toContain("Nicht einsehbar");
    expect(groups.at(-1)!.studentIds).toEqual(["4"]);
    expect(
      groups.some((group) => group.label.includes("Ohne Laufgemeinschaft")),
    ).toBe(false);
  });

  it("keeps a restricted child in the group a full-access child links them into", async () => {
    // Here the link IS readable — it came from Anna's own list. The restricted
    // child's membership is a fact, not a guess, so they belong in the group.
    const restricted = {
      ...student("4", "Dana", "Dreyer"),
      has_full_access: false,
    };
    const annaLinksDana = student("1", "Anna", "Adler", [4]);

    renderWithStudents([annaLinksDana, restricted]);

    await waitFor(() =>
      expect(screen.getByTestId("student-card-4")).toBeDefined(),
    );

    fireEvent.click(screen.getByTestId("filter-groupMode-companions"));

    await waitFor(() =>
      expect(screen.getAllByTestId("student-group").length).toBe(1),
    );

    const groups = readGroups();
    expect(groups[0]!.label).toContain("Anna Adler, Dana Dreyer");
    expect(groups[0]!.studentIds.sort()).toEqual(["1", "4"]);
  });
});

describe("StudentSearchPage — planning date (#1939)", () => {
  resetDocumentAfterAll();

  // The header is stubbed, but the filter configs it receives are the page's
  // contract for which filters exist on the selected day — exposed as data so
  // a test can read them without rendering the real panel.
  function FilterDataHeader({
    filters,
  }: {
    filters?: {
      id: string;
      value?: unknown;
      options?: { value: string }[];
    }[];
  }) {
    return (
      <div
        data-testid="header"
        data-filters={JSON.stringify(
          (filters ?? []).map((filter) => ({
            id: filter.id,
            value: filter.value,
            options: filter.options?.map((option) => option.value),
          })),
        )}
      />
    );
  }

  beforeEach(() => {
    mockPageHeader.mockImplementation(FilterDataHeader);
  });

  // The planning horizon ends with the Sunday closing the CURRENT week (#1939),
  // so the number of selectable future days depends on today's weekday: on a
  // Sunday not even tomorrow is selectable. Anchoring on the real date would
  // therefore flake this suite every Sunday, so it pins the clock to a fixed
  // Monday (2026-06-01, 10:00 Berlin). Both the system time and the mocked
  // useMinuteClock must be pinned: the page reads "today" from the clock for
  // rendering, but the explicit-date initializer calls berlinTodayISO() with no
  // argument, i.e. straight off the system clock.
  const FIXED_NOW = new Date("2026-06-01T08:00:00Z");

  // Anchored on the Berlin day — the page's "today" — so the tests stay stable
  // when the runner's timezone disagrees with Europe/Berlin around midnight.
  function isoDaysFromToday(days: number): string {
    const date = parseISODate(berlinTodayISO(FIXED_NOW));
    date.setDate(date.getDate() + days);
    return toISODate(date);
  }

  const mockStudent = {
    id: "7",
    first_name: "Max",
    second_name: "Mustermann",
    school_class: "1a",
    current_location: "Anwesend",
    has_full_access: true,
    day_planning_status: "comes_today",
  };

  /**
   * An instant whose Berlin calendar day is `days` after today, independent of
   * the runner's timezone: local midnight of the target day plus 12 hours is
   * within the same Berlin day for any runner within UTC±10.
   */
  function clockAtBerlinDay(days: number): Date {
    const target = parseISODate(isoDaysFromToday(days));
    return new Date(target.getTime() + 12 * 3600 * 1000);
  }

  beforeEach(() => {
    vi.clearAllMocks();
    currentSearch = new URLSearchParams();
    localStorage.clear();
    // shouldAdvanceTime keeps timers running so waitFor / SWR still settle.
    vi.useFakeTimers({ shouldAdvanceTime: true });
    vi.setSystemTime(FIXED_NOW);
    clockNow = FIXED_NOW;

    mockUseImmutableSWR.mockReturnValue({
      data: [],
      isLoading: false,
      error: null,
    });

    mockUseSWRAuth.mockImplementation(
      (key: string | null, fetcher?: () => Promise<unknown>) => {
        if (fetcher) {
          void fetcher().catch(() => undefined);
        }
        if (key === "search-rooms-list") {
          return { data: [], isLoading: false, error: null };
        }
        return {
          data: { students: [mockStudent] },
          isLoading: false,
          error: null,
        };
      },
    );

    mockGetStudents.mockResolvedValue({ students: [mockStudent] });
  });

  afterEach(() => {
    cleanup();
    localStorage.clear();
    vi.useRealTimers();
  });

  it("forwards a non-today date from the URL into studentService.getStudents", async () => {
    const tomorrow = isoDaysFromToday(1);
    currentSearch = new URLSearchParams({ date: tomorrow });

    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(mockGetStudents).toHaveBeenCalled();
    });

    const calls = mockGetStudents.mock.calls.map(
      (c) => c[0] as Record<string, unknown>,
    );
    expect(calls.some((c) => c.date === tomorrow)).toBe(true);
  });

  it("omits the date for today so today's requests stay unchanged", async () => {
    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(mockGetStudents).toHaveBeenCalled();
    });

    const calls = mockGetStudents.mock.calls.map(
      (c) => c[0] as Record<string, unknown>,
    );
    expect(calls.every((c) => c.date === undefined)).toBe(true);
  });

  it("falls back to today when the date param is malformed", async () => {
    currentSearch = new URLSearchParams({ date: "18.07.2026" });

    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(mockGetStudents).toHaveBeenCalled();
    });

    const calls = mockGetStudents.mock.calls.map(
      (c) => c[0] as Record<string, unknown>,
    );
    expect(calls.every((c) => c.date === undefined)).toBe(true);
  });

  it("rejects a date beyond the planning horizon and strips it from the URL", async () => {
    // The backend answers dates past the Sunday closing the CURRENT calendar
    // week with 400 (materialized timetable horizon, #1939). The initializer
    // must collapse such a URL date to today instead of firing a request the
    // backend rejects. Today is pinned to a Monday, so the horizon ends at +6
    // and +7 (the next Monday) is the first rejected day.
    const beyondHorizon = isoDaysFromToday(7);
    window.history.replaceState(
      {},
      "",
      `/students/search?date=${beyondHorizon}`,
    );
    currentSearch = new URLSearchParams({ date: beyondHorizon });

    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(mockGetStudents).toHaveBeenCalled();
    });

    const calls = mockGetStudents.mock.calls.map(
      (c) => c[0] as Record<string, unknown>,
    );
    expect(calls.every((c) => c.date === undefined)).toBe(true);
    await waitFor(() => {
      expect(new URLSearchParams(window.location.search).has("date")).toBe(
        false,
      );
    });

    window.history.replaceState({}, "", "/");
  });

  it("accepts a date at the end of the planning horizon", async () => {
    // Today is pinned to a Monday, so +6 is the Sunday closing the current
    // week — the far edge of the horizon must stay selectable.
    const withinHorizon = isoDaysFromToday(6);
    currentSearch = new URLSearchParams({ date: withinHorizon });

    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(mockGetStudents).toHaveBeenCalled();
    });

    const calls = mockGetStudents.mock.calls.map(
      (c) => c[0] as Record<string, unknown>,
    );
    expect(calls.some((c) => c.date === withinHorizon)).toBe(true);
  });

  it("strips a rejected past/aged date from the address bar on mount", async () => {
    // A bookmark or hand-edited URL carrying a past date. The initializer
    // rejects it (planning is future-only) and the page uses today; the mount
    // reconcile must also remove the stale param so the address bar and copied
    // links stop advertising a date the page no longer uses (#1939).
    const past = isoDaysFromToday(-3);
    window.history.replaceState({}, "", `/students/search?date=${past}`);
    currentSearch = new URLSearchParams({ date: past });

    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(new URLSearchParams(window.location.search).has("date")).toBe(
        false,
      );
    });

    window.history.replaceState({}, "", "/");
  });

  it("shows the planning badge instead of the live presence badge on other days", async () => {
    const tomorrow = isoDaysFromToday(1);
    currentSearch = new URLSearchParams({ date: tomorrow });

    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-7")).toBeInTheDocument();
    });

    expect(screen.getByTestId("planning-badge")).toHaveTextContent("Kommt");
    expect(screen.queryByTestId("presence-badge")).not.toBeInTheDocument();

    // The date-context hint names the selected day, never "heute".
    expect(screen.getByText(/Geplante Anwesenheit für/)).toBeInTheDocument();
  });

  it("keeps the live presence badge for today", async () => {
    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-7")).toBeInTheDocument();
    });

    expect(screen.getByTestId("presence-badge")).toBeInTheDocument();
    expect(screen.queryByTestId("planning-badge")).not.toBeInTheDocument();
  });

  it("preserves the date in the back-link when drilling into a child", async () => {
    const tomorrow = isoDaysFromToday(1);
    currentSearch = new URLSearchParams({ date: tomorrow });

    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-7")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("student-card-7"));

    const target = mockPush.mock.calls[0]?.[0] as string;
    expect(target.startsWith("/students/7?from=")).toBe(true);
    const fromParam = decodeURIComponent(target.split("from=")[1] ?? "");
    expect(fromParam).toContain(`date=${tomorrow}`);
  });

  it("keeps following today when the implicit view crosses midnight", async () => {
    const { rerender } = render(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-7")).toBeInTheDocument();
    });

    clockNow = clockAtBerlinDay(1);
    rerender(<StudentSearchPage />);

    // Still the implicit today view: no date sent, no planning hint, live badge.
    const calls = mockGetStudents.mock.calls.map(
      (c) => c[0] as Record<string, unknown>,
    );
    expect(calls.every((c) => c.date === undefined)).toBe(true);
    expect(
      screen.queryByText(/Geplante Anwesenheit für/),
    ).not.toBeInTheDocument();
    expect(screen.getByTestId("presence-badge")).toBeInTheDocument();
  });

  it("switches to live semantics when an explicit future date becomes today", async () => {
    const tomorrow = isoDaysFromToday(1);
    currentSearch = new URLSearchParams({ date: tomorrow });

    const { rerender } = render(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("planning-badge")).toBeInTheDocument();
    });
    expect(
      mockGetStudents.mock.calls.some(
        (c) => (c[0] as Record<string, unknown>).date === tomorrow,
      ),
    ).toBe(true);

    clockNow = clockAtBerlinDay(1);
    rerender(<StudentSearchPage />);

    // The selected day is now today: a fresh request without the date param
    // (today semantics) and the live presence badge replace the planning view.
    await waitFor(() => {
      expect(
        mockGetStudents.mock.calls.some(
          (c) => (c[0] as Record<string, unknown>).date === undefined,
        ),
      ).toBe(true);
    });
    expect(screen.getByTestId("presence-badge")).toBeInTheDocument();
    expect(screen.queryByTestId("planning-badge")).not.toBeInTheDocument();
  });

  it("drops an explicit date once it has aged into the past (tab open across midnight)", async () => {
    const tomorrow = isoDaysFromToday(1);
    currentSearch = new URLSearchParams({ date: tomorrow });

    const { rerender } = render(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("planning-badge")).toBeInTheDocument();
    });

    // Advance two Berlin days: the selected day (tomorrow) is now firmly in the
    // past. The stale date must not keep driving the backend request or the
    // view — the page falls back to live "Heute" semantics (#1939).
    clockNow = clockAtBerlinDay(2);
    rerender(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("presence-badge")).toBeInTheDocument();
    });
    expect(screen.queryByTestId("planning-badge")).not.toBeInTheDocument();

    // The last request carries no date (today semantics), so the aged past date
    // is no longer driving the backend query.
    const dates = mockGetStudents.mock.calls.map(
      (c) => (c[0] as Record<string, unknown>).date,
    );
    expect(dates.at(-1)).toBeUndefined();
  });

  // The mocked useSWRAuth runs every fetcher regardless of the key, so the
  // assertion is on the SWR key itself — a null key is what disables the
  // request in production (#1939).
  const trackingKeys = () =>
    mockUseSWRAuth.mock.calls
      .map((c) => c[0] as string | null)
      .filter(
        (key) =>
          typeof key === "string" && key.startsWith("tracking-indicators-"),
      );

  it("requests tracking indicators for today", async () => {
    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-7")).toBeInTheDocument();
    });

    expect(trackingKeys().length).toBeGreaterThan(0);
  });

  it("skips the tracking-indicator request on a planning date", async () => {
    currentSearch = new URLSearchParams({ date: isoDaysFromToday(1) });

    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-7")).toBeInTheDocument();
    });

    // Tracking indicators describe today's activity participation and are not
    // rendered on a planning date, so the request must not be made at all.
    expect(trackingKeys()).toEqual([]);
  });

  function statusFilterOptions(): string[] {
    const raw =
      screen.getByTestId("header").getAttribute("data-filters") ?? "[]";
    const filters = JSON.parse(raw) as {
      id: string;
      options?: string[];
    }[];
    return filters.find((filter) => filter.id === "attendance")?.options ?? [];
  }

  it("offers every status for today", async () => {
    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-7")).toBeInTheDocument();
    });

    expect(statusFilterOptions()).toEqual([
      "all",
      "anwesend",
      "schule",
      "abwesend",
      "unterwegs",
      "schulhof",
      "krank",
      "klassenfahrt",
      "entschuldigt",
      "entschuldigt-klassenfahrt",
    ]);
  });

  it("keeps only the planned absence statuses on a planning date", async () => {
    currentSearch = new URLSearchParams({ date: isoDaysFromToday(1) });

    render(<StudentSearchPage />);

    await waitFor(() => {
      expect(screen.getByTestId("student-card-7")).toBeInTheDocument();
    });

    // krank/klassenfahrt/entschuldigt come from the status days of the selected
    // date; the location-derived buckets would answer for today only (#1939).
    expect(statusFilterOptions()).toEqual([
      "all",
      "krank",
      "klassenfahrt",
      "entschuldigt",
      "entschuldigt-klassenfahrt",
    ]);
  });
});

describe("StudentSearchPage — Unterwegs-Karte (#5026)", () => {
  resetDocumentAfterAll();

  const mockStudent = {
    id: "7",
    first_name: "Max",
    second_name: "Mustermann",
    school_class: "1a",
    current_location: "Anwesend",
    has_full_access: true,
  };

  beforeEach(() => {
    vi.clearAllMocks();
    currentSearch = new URLSearchParams();
    localStorage.clear();
    mockUseImmutableSWR.mockReturnValue({
      data: [],
      isLoading: false,
      error: null,
    });
    mockUseSWRAuth.mockImplementation((key: string | null) =>
      key === "search-rooms-list"
        ? { data: [], isLoading: false, error: null }
        : { data: { students: [mockStudent] }, isLoading: false, error: null },
    );
  });

  afterEach(() => {
    cleanup();
    localStorage.clear();
  });

  it("offers the card while the list is grouped by status, with the way back", async () => {
    currentSearch = new URLSearchParams({ view: "status" });

    render(<StudentSearchPage />);

    await screen.findByTestId("student-card-7");
    const card = screen.getByTestId("transit-card");
    expect(card).toHaveAttribute("data-show", "true");
    expect(card).toHaveAttribute("data-count", "1");
    expect(decodeURIComponent(card.getAttribute("data-from") ?? "")).toBe(
      "/students/search?view=status",
    );
  });

  it("keeps the card closed while the list neither filters nor groups by Unterwegs", async () => {
    render(<StudentSearchPage />);

    await screen.findByTestId("student-card-7");
    expect(screen.getByTestId("transit-card")).toHaveAttribute(
      "data-show",
      "false",
    );
  });
});
