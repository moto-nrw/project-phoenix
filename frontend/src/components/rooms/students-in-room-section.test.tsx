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

const mockUseSWRAuth = vi.fn();
vi.mock("~/lib/swr", () => ({
  useSWRAuth: (...args: unknown[]) => mockUseSWRAuth(...args),
}));

vi.mock("~/lib/hooks/use-user-context", () => ({
  useUserContext: () => ({
    userContext: {
      educationalGroupIds: [],
      educationalGroupRoomNames: [],
      supervisedRoomNames: [],
    },
    isLoading: false,
    error: null,
  }),
}));

vi.mock("~/lib/pickup-helpers", async () => {
  const actual = await vi.importActual<typeof import("~/lib/pickup-helpers")>(
    "~/lib/pickup-helpers",
  );
  return {
    ...actual,
    useMinuteClock: () => new Date("2026-01-01T10:00:00Z"),
  };
});

// Light mocks for visual primitives — we want to assert behaviors (text,
// button visibility, click handlers), not the exact rendering of every UI
// atom imported transitively.
vi.mock("~/components/ui/info-card", () => ({
  InfoCard: ({
    title,
    children,
  }: {
    title: string;
    children: React.ReactNode;
  }) => (
    <section data-testid="info-card" aria-label={title}>
      <h2>{title}</h2>
      {children}
    </section>
  ),
}));

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

vi.mock("~/components/ui/student-presence-badge", () => ({
  StudentPresenceBadge: () => <span data-testid="presence-badge" />,
}));

vi.mock("~/components/students/student-card", () => ({
  StudentCard: ({
    studentId,
    firstName,
    lastName,
    onClick,
    extraContent,
  }: {
    studentId: string;
    firstName: string;
    lastName: string;
    onClick: () => void;
    extraContent: React.ReactNode;
  }) => (
    <button
      type="button"
      data-testid={`student-card-${studentId}`}
      onClick={onClick}
    >
      <span>
        {firstName} {lastName}
      </span>
      <div>{extraContent}</div>
    </button>
  ),
  StudentInfoRow: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  SchoolClassIcon: () => <span />,
  GroupIcon: () => <span />,
  PickupTimeRow: ({ pickupTime }: { pickupTime?: string }) =>
    pickupTime ? <div data-testid="pickup-row">{pickupTime}</div> : null,
  ArrivalTimeRow: ({ arrivalTime }: { arrivalTime?: string }) =>
    arrivalTime ? <div data-testid="arrival-row">{arrivalTime}</div> : null,
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
  arrival_time?: string;
  pickup_time?: string;
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
  data?: { students: MockStudent[] };
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

      expect(screen.getByTestId("student-card-1")).toBeInTheDocument();
      expect(screen.getByTestId("student-card-2")).toBeInTheDocument();
      expect(screen.getByTestId("student-card-3")).toBeInTheDocument();
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

  describe("navigation", () => {
    it("navigates to the student detail page with from=/rooms/{id}", () => {
      setSWR({ data: { students: [makeStudent({ id: "7" })] } });

      render(<StudentsInRoomSection roomId="42" roomName="OGS-Raum 1" />);

      fireEvent.click(screen.getByTestId("student-card-7"));

      // The from= param is what wires up the breadcrumb + sidebar back to
      // Räume on the student detail page (#1323). If this regresses, the
      // child detail view will say "Kindersuche" instead.
      expect(mockPush).toHaveBeenCalledWith("/students/7?from=/rooms/42");
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
