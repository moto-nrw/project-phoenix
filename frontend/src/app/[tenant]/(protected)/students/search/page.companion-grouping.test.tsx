/**
 * Behavior tests for the "Nach Laufgemeinschaft" grouping on the Kindersuche
 * page (#1694). Focus: the client-side bucketing of the child-to-child
 * "läuft mit" links — connected children share one bucket, everyone else lands
 * in "Ohne Laufgemeinschaft".
 *
 * The page has many transitive dependencies; mocks below are deliberately
 * minimal — only enough to render and exercise the grouping code path.
 */
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { FilterConfig } from "~/components/ui/page-header/types";

const { mockGetStudents, mockUseSWRAuth, mockUseImmutableSWR, mockPush } =
  vi.hoisted(() => ({
    mockGetStudents: vi.fn(),
    mockUseSWRAuth: vi.fn(),
    mockUseImmutableSWR: vi.fn(),
    mockPush: vi.fn(),
  }));

let currentSearch = new URLSearchParams();

vi.mock("next-auth/react", () => ({
  useSession: vi.fn(() => ({
    data: { user: { token: "t" } },
    status: "authenticated",
  })),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, replace: vi.fn() }),
  useSearchParams: () => currentSearch,
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: mockPush }),
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
  TenantProvider: ({ children }: { children: React.ReactNode }) => children,
}));

vi.mock("~/lib/breadcrumb-context", () => ({
  useSetBreadcrumb: vi.fn(),
  useBreadcrumb: () => ({ breadcrumb: {}, setBreadcrumb: vi.fn() }),
}));

// Renders the page's filter configs as plain buttons so a test can pick a
// grouping option without driving the real header's popover/sheet UI.
vi.mock("~/components/ui/page-header/PageHeaderWithSearch", () => ({
  PageHeaderWithSearch: ({ filters }: { filters?: FilterConfig[] }) => (
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
  ),
}));

vi.mock("~/components/ui/loading", () => ({
  Loading: () => <div data-testid="loading" />,
}));

vi.mock("~/components/ui/alert", () => ({
  Alert: ({ message }: { message: string }) => <div>{message}</div>,
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
  StudentCard: ({
    studentId,
    onClick,
  }: {
    studentId: string;
    onClick: () => void;
  }) => (
    <button
      type="button"
      data-testid={`student-card-${studentId}`}
      onClick={onClick}
    >
      card-{studentId}
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
}));

import StudentSearchPage from "./page";

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
          card.getAttribute("data-testid")?.replace("student-card-", "") ?? "",
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

describe("StudentSearchPage — Laufgemeinschaft grouping (#1694)", () => {
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
