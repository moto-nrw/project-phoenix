/**
 * Behavior tests for the search page when the person clicks "Alle Kinder" in
 * the navigation while already on it, and for a student list that never
 * finishes loading (#3374).
 *
 * The App Router does not remount the page for such a click, so the tests
 * render the page once and click a plain link to the current path, the way the
 * sidebar entry reaches the document.
 */
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const {
  mockGetStudents,
  mockUseSWRAuth,
  mockUseImmutableSWR,
  mockPush,
  mockMutateStudents,
  mockUpdateSession,
  sessionState,
} = vi.hoisted(() => ({
  mockGetStudents: vi.fn(),
  mockUseSWRAuth: vi.fn(),
  mockUseImmutableSWR: vi.fn(),
  mockPush: vi.fn(),
  mockMutateStudents: vi.fn(),
  mockUpdateSession: vi.fn(),
  sessionState: { token: "t" as string | undefined },
}));

let currentSearch = new URLSearchParams();

vi.mock("next-auth/react", () => ({
  useSession: vi.fn(() => ({
    data: { user: { id: "3", tenantId: 2, token: sessionState.token } },
    status: "authenticated",
    update: mockUpdateSession,
  })),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, replace: vi.fn() }),
  useSearchParams: () => currentSearch,
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: mockPush }),
}));

vi.mock("~/lib/supervision-context", () => ({
  useOptionalSupervision: () => ({
    hasGroups: false,
    isSupervising: false,
    isLoadingGroups: false,
    isLoadingSupervision: false,
    overviewEnabled: false,
    supervisedRooms: [],
    groups: [],
    refresh: () => undefined,
  }),
}));

vi.mock("~/lib/tenant-context", () => ({
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

vi.mock("@/components/ui/location-badge", () => ({
  LocationBadge: () => <span />,
}));

vi.mock("@/components/ui/student-presence-badge", () => ({
  StudentPresenceBadge: () => <span />,
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

vi.mock("~/lib/pickup-helpers", () => ({
  useMinuteClock: () => new Date(),
}));

vi.mock("~/lib/active-api", () => ({
  activeService: {
    getTrackingIndicators: vi.fn(() =>
      Promise.resolve({ labels: [], results: {} }),
    ),
  },
}));

vi.mock("~/lib/hooks/use-user-context", () => ({
  useUserContext: () => ({
    userContext: {
      educationalGroupIds: [],
      educationalGroupRoomNames: [],
      supervisedRoomNames: [],
    },
  }),
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
  StudentCard: ({ studentId }: { studentId: string }) => (
    <div data-testid={`student-card-${studentId}`}>card-{studentId}</div>
  ),
  SchoolClassIcon: () => <span />,
  GroupIcon: () => <span />,
  DepartureModeIcon: () => <span />,
  StudentInfoRow: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  PickupTimeRow: () => <div />,
  ArrivalTimeRow: () => <div />,
}));

import StudentSearchPage from "./page";

const STORAGE_KEY = "student-search:last-filters:tenant-2:3";
const STALL_MESSAGE =
  "Die Kinder laden gerade sehr lange. Bitte laden Sie die Liste noch einmal.";

const mockStudent = {
  id: "7",
  first_name: "Max",
  second_name: "Mustermann",
  school_class: "1a",
  current_location: "Anwesend",
  has_full_access: true,
  bus: true,
};

let studentsResponse: {
  data: unknown;
  isLoading: boolean;
  error: unknown;
};
let studentKeys: string[] = [];

function storedFilters() {
  return localStorage.getItem(STORAGE_KEY);
}

function renderOnSearchPage(search: string) {
  window.history.replaceState(null, "", `/students/search${search}`);
  currentSearch = new URLSearchParams(search);
  const navLink = document.createElement("a");
  navLink.href = "/students/search";
  navLink.textContent = "Alle Kinder (Navigation)";
  document.body.appendChild(navLink);
  const view = render(<StudentSearchPage />);
  return { ...view, navLink };
}

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.clear();
  sessionState.token = "t";
  studentKeys = [];
  studentsResponse = {
    data: { students: [mockStudent] },
    isLoading: false,
    error: undefined,
  };

  mockUseImmutableSWR.mockReturnValue({
    data: [],
    isLoading: false,
    error: null,
  });
  mockUseSWRAuth.mockImplementation((key: string | null) => {
    if (key?.startsWith("search-students-")) {
      studentKeys.push(key);
      return { ...studentsResponse, mutate: mockMutateStudents };
    }
    return { data: undefined, isLoading: false, error: undefined };
  });
  mockGetStudents.mockResolvedValue({ students: [mockStudent] });
});

afterEach(() => {
  cleanup();
  document.body.innerHTML = "";
  localStorage.clear();
  window.history.replaceState(null, "", "/");
});

describe("StudentSearchPage: click on the navigation entry of this page (#3374)", () => {
  it("drops filters, URL parameters and stored filters", async () => {
    const { navLink } = renderOnSearchPage("?bus=yes&sort=class");

    await waitFor(() => {
      expect(storedFilters()).toContain("yes");
    });
    const filteredKey = studentKeys.at(-1);

    fireEvent.click(navLink);

    await waitFor(() => {
      expect(window.location.search).toBe("");
    });
    expect(storedFilters()).toBeNull();
    // The request key is what SWR fetches by: a changed key is the fresh,
    // unfiltered request, so no forced revalidation of the old one on top.
    expect(studentKeys.at(-1)).not.toBe(filteredKey);
    expect(mockMutateStudents).not.toHaveBeenCalled();
  });

  it("clears a name search, which lives outside the URL", async () => {
    const { navLink } = renderOnSearchPage("");
    const unfilteredKey = studentKeys.at(-1);

    const [searchInput] = screen.getAllByPlaceholderText("Name suchen…");
    fireEvent.change(searchInput!, { target: { value: "Mila" } });
    await waitFor(() => {
      expect(studentKeys.at(-1)).toContain("Mila");
    });

    fireEvent.click(navLink);

    await waitFor(() => {
      expect(studentKeys.at(-1)).toBe(unfilteredKey);
    });
    expect(
      (screen.getAllByPlaceholderText("Name suchen…")[0] as HTMLInputElement)
        .value,
    ).toBe("");
  });

  it("reloads the list when nothing was filtered", async () => {
    const { navLink } = renderOnSearchPage("");

    fireEvent.click(navLink);

    await waitFor(() => {
      expect(mockMutateStudents).toHaveBeenCalledTimes(1);
    });
  });

  it("leaves the page alone for a link to another page", () => {
    renderOnSearchPage("?bus=yes");
    const otherLink = document.createElement("a");
    otherLink.href = "/rooms";
    document.body.appendChild(otherLink);

    const filteredKey = studentKeys.at(-1);

    fireEvent.click(otherLink);

    expect(storedFilters()).toContain("yes");
    expect(studentKeys.at(-1)).toBe(filteredKey);
    expect(mockMutateStudents).not.toHaveBeenCalled();
  });
});

describe("StudentSearchPage: student list that does not finish loading (#3374)", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("replaces the endless skeleton with a hint and a reload", () => {
    studentsResponse = { data: undefined, isLoading: true, error: undefined };
    renderOnSearchPage("");

    act(() => {
      vi.advanceTimersByTime(9_000);
    });
    expect(screen.queryByText(STALL_MESSAGE)).toBeNull();

    act(() => {
      vi.advanceTimersByTime(1_000);
    });
    expect(screen.getByText(STALL_MESSAGE)).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "Erneut laden" }));

    expect(mockMutateStudents).toHaveBeenCalledTimes(1);
    // A session with a token needs no second look; only the list is asked for.
    expect(mockUpdateSession).not.toHaveBeenCalled();
    // The retry gets the full waiting time again before the hint returns.
    expect(screen.queryByText(STALL_MESSAGE)).toBeNull();
    act(() => {
      vi.advanceTimersByTime(10_000);
    });
    expect(screen.getByText(STALL_MESSAGE)).toBeTruthy();
  });

  it("asks for the session again when the token is missing", () => {
    sessionState.token = undefined;
    studentsResponse = { data: undefined, isLoading: false, error: undefined };
    renderOnSearchPage("");

    act(() => {
      vi.advanceTimersByTime(10_000);
    });
    fireEvent.click(screen.getByRole("button", { name: "Erneut laden" }));

    expect(mockUpdateSession).toHaveBeenCalledTimes(1);
  });

  it("shows no hint once the list has loaded", () => {
    renderOnSearchPage("");

    act(() => {
      vi.advanceTimersByTime(30_000);
    });

    expect(screen.queryByText(STALL_MESSAGE)).toBeNull();
    expect(screen.getByTestId("student-card-7")).toBeTruthy();
  });

  it("offers the reload next to a load error", () => {
    studentsResponse = {
      data: undefined,
      isLoading: false,
      error: new Error("Failed to fetch"),
    };
    renderOnSearchPage("");

    expect(screen.getByText("Fehler beim Laden der Kinderdaten.")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Erneut laden" }));

    expect(mockMutateStudents).toHaveBeenCalledTimes(1);
  });
});
