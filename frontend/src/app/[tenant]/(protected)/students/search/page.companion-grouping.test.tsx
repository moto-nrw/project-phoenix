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
  }),
  deriveCheckinState: () => "unknown",
}));

vi.mock("~/lib/location-helper", () => ({
  isHomeLocation: () => false,
  isPresentLocation: () => true,
  isTransitLocation: () => false,
  isSchoolyardLocation: () => false,
}));

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

  it("ignores links pointing at children who are not on the page", async () => {
    // Anna links to Ben, but the current filter left Ben out of the list —
    // the group must not be invented from a name the page never received.
    renderWithStudents([anna, clara]);

    await waitFor(() =>
      expect(screen.getByTestId("student-card-1")).toBeDefined(),
    );

    fireEvent.click(screen.getByTestId("filter-groupMode-companions"));

    await waitFor(() =>
      expect(screen.getAllByTestId("student-group").length).toBe(1),
    );

    const groups = readGroups();
    expect(groups[0]!.label).toContain("Ohne Laufgemeinschaft");
    expect(groups[0]!.studentIds.sort()).toEqual(["1", "3"]);
  });
});
