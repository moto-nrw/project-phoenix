import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { StudentsInRoomSection } from "./students-in-room-section";

// ----------------------------------------------------------------------------
// Mocks
// ----------------------------------------------------------------------------

const mockPush = vi.fn();
vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: mockPush }),
}));

const mockUseSearchParams = vi.fn(() => ({
  get: vi.fn(() => null),
  toString: vi.fn(() => "room=42"),
}));
vi.mock("next/navigation", () => ({
  useSearchParams: () => mockUseSearchParams(),
}));

const mockUseSWRAuth = vi.fn();
vi.mock("~/lib/swr", () => ({
  useSWRAuth: (...args: unknown[]) => mockUseSWRAuth(...args),
}));

// Light mocks for visual primitives — we want to assert behaviors (text,
// button visibility, click handlers), not the exact rendering of every UI
// atom imported transitively. The "Kinder im Raum" section now renders
// its own <section aria-label> wrapper instead of going through InfoCard
// (#1323 review — quiet section headers across the slide-over), so no
// info-card mock is needed.

vi.mock("~/components/ui/button", () => ({
  Button: ({
    onClick,
    children,
    ...rest
  }: React.ButtonHTMLAttributes<HTMLButtonElement>) => (
    <button onClick={onClick} {...rest}>
      {children}
    </button>
  ),
}));

vi.mock("~/components/ui/alert", () => ({
  Alert: ({ message }: { message: string }) => (
    <div role="alert">{message}</div>
  ),
}));

vi.mock("~/components/ui/loading", () => ({
  Loading: ({ message }: { message: string }) => (
    <div data-testid="loading">{message}</div>
  ),
}));

vi.mock("~/components/students/compact-student-card", () => ({
  CompactStudentCard: ({
    studentId,
    firstName,
    lastName,
    schoolClass,
    groupName,
    onClick,
  }: {
    studentId: string;
    firstName?: string;
    lastName?: string;
    schoolClass?: string;
    groupName?: string;
    onClick?: () => void;
  }) => (
    <button
      type="button"
      data-testid={`compact-student-card-${studentId}`}
      data-school-class={schoolClass}
      data-group-name={groupName}
      onClick={onClick}
    >
      <span>
        {firstName} {lastName}
      </span>
    </button>
  ),
}));

// ----------------------------------------------------------------------------
// Helpers
// ----------------------------------------------------------------------------

interface MockStudent {
  id: string;
  first_name: string;
  second_name: string;
  school_class: string;
  group_name?: string;
}

const makeStudent = (overrides: Partial<MockStudent> = {}): MockStudent => ({
  id: "1",
  first_name: "Anna",
  second_name: "Müller",
  school_class: "1a",
  group_name: "Bären",
  ...overrides,
});

const setSWR = (state: {
  data?: {
    students: MockStudent[];
    pagination?: { total_records: number };
  };
  error?: unknown;
  isLoading?: boolean;
}) => {
  mockUseSWRAuth.mockReturnValue({
    data: state.data,
    error: state.error ?? null,
    isLoading: state.isLoading ?? false,
  });
};

beforeEach(() => {
  mockPush.mockReset();
  mockUseSWRAuth.mockReset();
});

// ----------------------------------------------------------------------------
// Tests
// ----------------------------------------------------------------------------

describe("StudentsInRoomSection", () => {
  describe("loading and error states", () => {
    it("shows the loading indicator while fetching", () => {
      setSWR({ data: undefined, isLoading: true });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      expect(screen.getByTestId("loading")).toBeInTheDocument();
    });

    it("renders an error alert if the fetch fails", () => {
      setSWR({ data: undefined, error: new Error("network down") });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      const alert = screen.getByRole("alert");
      expect(alert).toHaveTextContent(/Liste der Kinder/);
      // Error path must not also render the empty-state placeholder —
      // those two states are mutually exclusive and showing both would
      // confuse the user about whether a retry would help.
      expect(
        screen.queryByText(/Aktuell keine Kinder/),
      ).not.toBeInTheDocument();
    });

    it("shows the empty placeholder when no children are present", () => {
      setSWR({ data: { students: [] } });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      expect(
        screen.getByText(/Aktuell keine Kinder im Raum/),
      ).toBeInTheDocument();
      // "In Kindersuche öffnen" must NOT render when the list is empty —
      // otherwise the deep-link would land on an empty filtered Kindersuche
      // with no obvious affordance for the user to recover.
      expect(
        screen.queryByRole("button", { name: /Kindersuche/ }),
      ).not.toBeInTheDocument();
    });
  });

  describe("populated list", () => {
    it("renders one card per student", () => {
      setSWR({
        data: {
          students: [
            makeStudent({ id: "1", first_name: "Anna" }),
            makeStudent({ id: "2", first_name: "Ben" }),
            makeStudent({ id: "3", first_name: "Carla" }),
          ],
        },
      });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      expect(screen.getByTestId("compact-student-card-1")).toBeInTheDocument();
      expect(screen.getByTestId("compact-student-card-2")).toBeInTheDocument();
      expect(screen.getByTestId("compact-student-card-3")).toBeInTheDocument();
    });

    it("forwards school_class and group_name to the compact card", () => {
      // The minimal card shows just identity (name + class + group). The
      // section must pass those fields through unmodified — earlier
      // versions used StudentCard with extraContent and arrival/pickup
      // rows; that complexity is gone, but the identifiers must remain
      // visible so staff can recognise the child.
      setSWR({
        data: {
          students: [
            makeStudent({
              id: "1",
              school_class: "2b",
              group_name: "Füchse",
            }),
          ],
        },
      });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      const card = screen.getByTestId("compact-student-card-1");
      expect(card.getAttribute("data-school-class")).toBe("2b");
      expect(card.getAttribute("data-group-name")).toBe("Füchse");
    });

    it("uses the singular noun when exactly one child is present", () => {
      setSWR({ data: { students: [makeStudent({ id: "1" })] } });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      // Pluralization regression guard: "1 Kind" not "1 Kinder".
      // The count and noun render in sibling nodes, so search by parent text.
      const subline = screen.getByText((_, node) => {
        const text = node?.textContent ?? "";
        return /\b1\s+Kind\s+aktuell\s+anwesend\b/.test(text);
      });
      expect(subline).toBeInTheDocument();
    });

    it("uses the plural noun when more than one child is present", () => {
      setSWR({
        data: {
          students: [makeStudent({ id: "1" }), makeStudent({ id: "2" })],
        },
      });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      const subline = screen.getByText((_, node) => {
        const text = node?.textContent ?? "";
        return /\b2\s+Kinder\s+aktuell\s+anwesend\b/.test(text);
      });
      expect(subline).toBeInTheDocument();
    });
  });

  describe("count source", () => {
    it("uses pagination.total_records over the rendered length", () => {
      // Even if the API ever truncates the response (default backend
      // page size is 50), the count badge must stay honest. The visible
      // card list can be shorter than the count without lying about it.
      setSWR({
        data: {
          students: [makeStudent({ id: "1" }), makeStudent({ id: "2" })],
          pagination: { total_records: 87 },
        },
      });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      const subline = screen.getByText((_, node) => {
        const text = node?.textContent ?? "";
        return /\b87\s+Kinder\s+aktuell\s+anwesend\b/.test(text);
      });
      expect(subline).toBeInTheDocument();
    });

    it("falls back to students.length when pagination metadata is missing", () => {
      setSWR({
        data: {
          students: [makeStudent({ id: "1" }), makeStudent({ id: "2" })],
        },
      });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      const subline = screen.getByText((_, node) => {
        const text = node?.textContent ?? "";
        return /\b2\s+Kinder\s+aktuell\s+anwesend\b/.test(text);
      });
      expect(subline).toBeInTheDocument();
    });
  });

  describe("truncation notice", () => {
    it("renders an overflow status when total_records exceeds rendered count", () => {
      // Hard-coded pageSize=200 caps the response. When a room ever
      // exceeds it, the section must surface the gap and point at the
      // Kindersuche escape hatch — otherwise staff can't see or open
      // the missing children (#1374).
      setSWR({
        data: {
          students: [
            makeStudent({ id: "1" }),
            makeStudent({ id: "2" }),
            makeStudent({ id: "3" }),
          ],
          pagination: { total_records: 250 },
        },
      });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      const notice = screen.getByRole("status");
      expect(notice).toHaveTextContent(/3 von 250 Kindern/);
      expect(notice).toHaveTextContent(/247/);
      expect(notice).toHaveTextContent(/Kindersuche/);
    });

    it("does NOT render the notice when total_records matches rendered count", () => {
      setSWR({
        data: {
          students: [makeStudent({ id: "1" }), makeStudent({ id: "2" })],
          pagination: { total_records: 2 },
        },
      });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      expect(screen.queryByRole("status")).not.toBeInTheDocument();
    });
  });

  describe("navigation", () => {
    it("encodes the full URL as from= in the slide-over context (preserves filters)", () => {
      // The section now only renders inside the slide-over (legacy
      // /rooms/{id} subpage is a server-side redirect). The from= URL
      // must carry the COMPLETE current query string — including any
      // grid filters (search, building, status) — so the user lands
      // back on their narrowed view, not a reset grid.
      mockUseSearchParams.mockReturnValueOnce({
        get: vi.fn(() => null),
        toString: vi.fn(
          () => "search=foo&building=Main&status=occupied&room=42",
        ),
      });
      setSWR({ data: { students: [makeStudent({ id: "7" })] } });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      fireEvent.click(screen.getByTestId("compact-student-card-7"));

      // ? inside from= must be URL-encoded; & inside likewise. Without
      // encoding the outer query parser splits at the inner '?' and
      // sees only the first param.
      expect(mockPush).toHaveBeenCalledWith(
        `/students/7?from=${encodeURIComponent(
          "/rooms?search=foo&building=Main&status=occupied&room=42",
        )}`,
      );
    });

    it("falls back to /rooms?room={id} when no query string is present", () => {
      mockUseSearchParams.mockReturnValueOnce({
        get: vi.fn(() => null),
        toString: vi.fn(() => ""),
      });
      setSWR({ data: { students: [makeStudent({ id: "7" })] } });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      fireEvent.click(screen.getByTestId("compact-student-card-7"));

      expect(mockPush).toHaveBeenCalledWith(
        `/students/7?from=${encodeURIComponent("/rooms?room=42")}`,
      );
    });

    it("opens Kindersuche pre-filtered with both room_id and room_name", () => {
      setSWR({
        data: {
          students: [makeStudent({ id: "1" }), makeStudent({ id: "2" })],
        },
      });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      fireEvent.click(
        screen.getByRole("button", { name: /In Kindersuche öffnen/ }),
      );

      expect(mockPush).toHaveBeenCalledTimes(1);
      const target = mockPush.mock.calls[0]?.[0] as string;
      expect(target).toMatch(/^\/students\/search\?/);
      // room_name must be URL-encoded — the seed value "OGS-Raum 1" contains
      // a space and a hyphen and would break a naive concatenation.
      expect(target).toContain("room_id=42");
      expect(target).toContain("room_name=OGS-Raum+1");
    });
  });
});
