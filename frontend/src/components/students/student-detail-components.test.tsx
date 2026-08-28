import { describe, it, expect, vi, afterEach, beforeEach } from "vitest";
import {
  render,
  screen,
  fireEvent,
  cleanup,
  within,
} from "@testing-library/react";

const { mockUseSWR } = vi.hoisted(() => ({
  mockUseSWR: vi.fn(),
}));

vi.mock("swr", () => ({
  default: mockUseSWR,
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: vi.fn() }),
}));

vi.mock("~/lib/student-companion-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/student-companion-api")>()),
  fetchStudentCompanions: vi.fn(() => new Promise(() => undefined)),
}));

import {
  PersonIcon,
  ChevronDownIcon,
  WarningIcon,
  StudentHeaderAvatar,
  StudentHeaderLocation,
  StudentHeaderStats,
  studentHeaderTitle,
  SupervisorsCard,
  PersonalInfoReadOnly,
  StudentHistorySection,
} from "./student-detail-components";
import { SectionCard } from "~/components/ui/section-card";
import type { ExtendedStudent } from "~/lib/hooks/use-student-data";
import { getLocationBadgeTone } from "~/lib/location-helper";
import type { SupervisorContact } from "~/lib/student-helpers";

beforeEach(() => {
  mockUseSWR.mockReturnValue({
    data: undefined,
    error: undefined,
    isLoading: true,
    isValidating: false,
    mutate: vi.fn(),
  });
});

/**
 * Format a date string the same way the component does.
 * This ensures tests are timezone-stable by using the same formatting logic.
 */
function formatDateLikeComponent(dateString: string): string {
  return new Date(dateString).toLocaleDateString("de-DE", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  });
}

// =============================================================================
// Icon Component Tests
// =============================================================================

describe("PersonIcon", () => {
  it("renders with default className", () => {
    render(<PersonIcon />);
    const svg = document.querySelector("svg");
    expect(svg).toBeInTheDocument();
    expect(svg).toHaveClass("h-5", "w-5");
  });

  it("accepts custom className", () => {
    render(<PersonIcon className="h-6 w-6 text-blue-500" />);
    const svg = document.querySelector("svg");
    expect(svg).toHaveClass("h-6", "w-6", "text-blue-500");
  });

  it("uses the central children concept", () => {
    render(<PersonIcon />);
    const svg = document.querySelector("svg");
    expect(svg).toHaveAttribute("data-moto-duotone-tone", "greenVivid");
  });
});

describe("ChevronDownIcon", () => {
  it("renders with default className", () => {
    render(<ChevronDownIcon />);
    const svg = document.querySelector("svg");
    expect(svg).toBeInTheDocument();
    expect(svg).toHaveClass("h-4", "w-4");
  });

  it("accepts custom className", () => {
    render(<ChevronDownIcon className="h-5 w-5 text-gray-400" />);
    const svg = document.querySelector("svg");
    expect(svg).toHaveClass("h-5", "w-5", "text-gray-400");
  });
});

describe("WarningIcon", () => {
  it("renders with default className", () => {
    render(<WarningIcon />);
    const svg = document.querySelector("svg");
    expect(svg).toBeInTheDocument();
    expect(svg).toHaveClass("h-5", "w-5");
  });

  it("accepts custom className", () => {
    render(<WarningIcon className="h-8 w-8 text-red-500" />);
    const svg = document.querySelector("svg");
    expect(svg).toHaveClass("h-8", "w-8", "text-red-500");
  });
});

// =============================================================================
// StudentDetailHeader Tests
// =============================================================================

/**
 * Der Kopf der Kindakte, so zusammengesetzt wie auf der Seite selbst: die
 * Kopfkarte liefert `TenantPage`, die drei Bausteine kommen von hier. Die
 * Attrappe ersetzt die frühere Wrapper-Komponente, die nur noch für Tests
 * existierte.
 */
function StudentDetailHeader(
  props: Readonly<{
    student: ExtendedStudent;
    myGroups: never[];
    myGroupRooms: never[];
    mySupervisedRooms: never[];
    todayPickupPlannedTime?: string;
    todayPickupActualTime?: string;
    todayPickupNote?: string;
    isPickupException?: boolean;
    todayArrivalPlannedTime?: string;
    todayArrivalActualTime?: string;
    isArrivalException?: boolean;
    isArrivalAbsent?: boolean;
    todayArrivalNote?: string;
    sickReason?: string;
  }>,
) {
  return (
    <SectionCard
      headingLevel={1}
      leading={<StudentHeaderAvatar student={props.student} />}
      title={studentHeaderTitle(props.student)}
      description={
        <StudentHeaderStats
          student={props.student}
          todayPickupPlannedTime={props.todayPickupPlannedTime}
          todayPickupActualTime={props.todayPickupActualTime}
          todayPickupNote={props.todayPickupNote}
          isPickupException={props.isPickupException}
          todayArrivalPlannedTime={props.todayArrivalPlannedTime}
          todayArrivalActualTime={props.todayArrivalActualTime}
          isArrivalException={props.isArrivalException}
          isArrivalAbsent={props.isArrivalAbsent}
          todayArrivalNote={props.todayArrivalNote}
          sickReason={props.sickReason}
        />
      }
      actions={
        <StudentHeaderLocation
          student={props.student}
          myGroups={props.myGroups}
          myGroupRooms={props.myGroupRooms}
          mySupervisedRooms={props.mySupervisedRooms}
          todayArrivalPlannedTime={props.todayArrivalPlannedTime}
          isArrivalException={props.isArrivalException}
          isArrivalAbsent={props.isArrivalAbsent}
          todayArrivalNote={props.todayArrivalNote}
        />
      }
    />
  );
}

describe("Kopf der Kindakte", () => {
  const mockStudent: ExtendedStudent = {
    id: "1",
    first_name: "Max",
    second_name: "Mustermann",
    name: "Max Mustermann",
    birthday: "2015-05-15",
    school_class: "3a",
    group_id: "g1",
    group_name: "Gruppe 1",
    buskind: false,
    pickup_status: "selbst",
    health_info: undefined,
    supervisor_notes: undefined,
    extra_info: undefined,
    sick: false,
    sick_since: undefined,
    current_location: "Klassenraum",
    location_since: "2024-01-15T08:00:00Z",
    bus: false,
  };

  it("renders student name", () => {
    render(
      <StudentDetailHeader
        student={mockStudent}
        myGroups={[]}
        myGroupRooms={[]}
        mySupervisedRooms={[]}
      />,
    );
    expect(screen.getByText("Max Mustermann")).toBeInTheDocument();
  });

  it("renders group name when provided", () => {
    render(
      <StudentDetailHeader
        student={mockStudent}
        myGroups={[]}
        myGroupRooms={[]}
        mySupervisedRooms={[]}
      />,
    );
    expect(screen.getByText("Gruppe 1")).toBeInTheDocument();
  });

  it("does not render group when group_name is missing", () => {
    const studentWithoutGroup = { ...mockStudent, group_name: undefined };
    render(
      <StudentDetailHeader
        student={studentWithoutGroup}
        myGroups={[]}
        myGroupRooms={[]}
        mySupervisedRooms={[]}
      />,
    );
    expect(screen.queryByText("Gruppe 1")).not.toBeInTheDocument();
  });

  it("rendert den Entitätskopf ohne seitlichen Einzug", () => {
    const { container } = render(
      <StudentDetailHeader
        student={mockStudent}
        myGroups={[]}
        myGroupRooms={[]}
        mySupervisedRooms={[]}
      />,
    );

    // Der Kopf steht bündig wie auf der Mitarbeiter-Detailseite: kein
    // Einzug, auf keiner Breite. Seit der Kopfkarte (PageIntro) trägt die
    // SectionCard die Identitätszeile und den Aktionsbereich.
    const identitySlot = container.querySelector(".flex.min-w-0.gap-3");
    expect(identitySlot).not.toBeNull();
    expect(identitySlot?.className).not.toMatch(/\bml-/);

    const badgeSlot = container.querySelector(".shrink-0.flex-wrap");
    expect(badgeSlot).not.toBeNull();
    expect(badgeSlot?.className).not.toMatch(/\bmr-/);
  });

  // ---------------------------------------------------------------------------
  // Per-room color forwarding (Issue #1381) — the header builds a `badgeStudent`
  // object from the incoming student before passing it to LocationBadge. A
  // prior whitelist version of that object dropped `current_room_color`, which
  // left the detail-page badge stuck on the default Anwesend blue while
  // /students/search rendered the configured hex correctly. This regression
  // test fails if anyone reintroduces the whitelist (or any other field
  // filtering between `student` and the badge).
  // ---------------------------------------------------------------------------
  describe("LocationBadge field forwarding (Issue #1381)", () => {
    afterEach(() => cleanup());

    it("forwards current_room_color to the LocationBadge", () => {
      const studentInRoom: ExtendedStudent = {
        ...mockStudent,
        // contextAware mode only paints the per-room hex when the viewer has
        // detailed access. has_full_access bypasses the OGS-group check and
        // keeps the test independent of myGroups wiring.
        has_full_access: true,
        current_location: "Anwesend - Bibliothek",
        current_room_color: "#A3D977",
      };

      render(
        <StudentDetailHeader
          student={studentInRoom}
          myGroups={[]}
          myGroupRooms={[]}
          mySupervisedRooms={[]}
        />,
      );

      // The badge keeps custom room colors on its dot while the wrapper uses
      // the shared neutral surface for unknown legacy colors.
      const badgeContainer = screen.getByText("Bibliothek").closest("span");
      expect(badgeContainer).toHaveStyle({
        backgroundColor: getLocationBadgeTone("#A3D977").backgroundColor,
      });
      expect(badgeContainer?.querySelector("span")).toHaveStyle({
        backgroundColor: "#A3D977",
      });
    });

    it("falls back to the default room color when current_room_color is null", () => {
      const studentInRoom: ExtendedStudent = {
        ...mockStudent,
        has_full_access: true,
        current_location: "Anwesend - Bibliothek",
        current_room_color: null,
      };

      render(
        <StudentDetailHeader
          student={studentInRoom}
          myGroups={[]}
          myGroupRooms={[]}
          mySupervisedRooms={[]}
        />,
      );

      const badgeContainer = screen.getByText("Bibliothek").closest("span");
      // OTHER_ROOM blue is the documented fallback (see location-badge tests
      // for Issue #1324). Asserting on a non-default color guards against
      // accidentally passing the per-room override through as `undefined`.
      expect(badgeContainer).not.toHaveStyle({ backgroundColor: "#A3D977" });
    });
  });

  // ---------------------------------------------------------------------------
  // TodayTimeStatusInlineRow — covers planned/actual rendering, absent flow,
  // exception dot, and the "(geplant: …)" parenthetical.
  //
  // The row is private to student-detail-components, so we exercise it
  // through StudentDetailHeader's public time props.
  // ---------------------------------------------------------------------------
  describe("TodayTimeStatusInlineRow (via StudentDetailHeader)", () => {
    // The setup file does not register a global cleanup; without this,
    // each render leaks into the next test's document.body and breaks
    // single-element assertions like getByText.
    afterEach(() => cleanup());

    function renderHeaderWithTimes(
      props: Partial<React.ComponentProps<typeof StudentDetailHeader>>,
    ) {
      return render(
        <StudentDetailHeader
          student={mockStudent}
          myGroups={[]}
          myGroupRooms={[]}
          mySupervisedRooms={[]}
          {...props}
        />,
      );
    }

    /**
     * Scope queries to a single time row by data-testid. The header also
     * renders a LocationBadge that can produce overlapping strings ("Kommt
     * heute nicht"), so unscoped queries find duplicates and fail tests for
     * the wrong reason.
     */
    function rowFor(kind: "arrival" | "pickup") {
      return within(screen.getByTestId(`today-time-row-${kind}`));
    }

    it("renders both rows with a dash placeholder when no times are configured", () => {
      // Always-render contract: caregivers should see the structural labels
      // even when nothing is scheduled, otherwise it's ambiguous whether the
      // system rendered nothing on purpose or hit an error.
      renderHeaderWithTimes({});

      expect(rowFor("arrival").getByText("–")).toBeInTheDocument();
      expect(rowFor("pickup").getByText("–")).toBeInTheDocument();
    });

    it("shows the planned pickup time as the primary value when nothing is resolved yet", () => {
      renderHeaderWithTimes({
        todayPickupPlannedTime: "15:30",
      });

      const pickup = rowFor("pickup");
      // Primary value is the planned time. The "(geplant: …)" parenthetical
      // is suppressed until an actual time arrives — otherwise we'd be
      // duplicating the same value into both spots.
      expect(pickup.getByText("15:30")).toBeInTheDocument();
      expect(pickup.queryByText(/geplant:/)).not.toBeInTheDocument();
    });

    it("promotes actual to primary and demotes planned to a parenthetical when both exist", () => {
      // The killer feature of the rewrite — caregivers need the actual at a
      // glance, with the original schedule visible for context.
      renderHeaderWithTimes({
        todayPickupPlannedTime: "15:30",
        todayPickupActualTime: "15:42",
      });

      const pickup = rowFor("pickup");
      // Primary value — what actually happened.
      expect(pickup.getByText("15:42")).toBeInTheDocument();
      // Parenthetical — combines the planned value with the diff so the
      // caregiver doesn't have to subtract in their head.
      expect(pickup.getByText(/geplant: 15:30, \+12 min/)).toBeInTheDocument();
    });

    it("renders the exception dot in the affected row only", () => {
      renderHeaderWithTimes({
        todayPickupPlannedTime: "15:30",
        isPickupException: true,
      });

      // The dot uses the German tooltip "Ausnahme" via title= attribute —
      // assert via title rather than class so we're not coupled to styling.
      expect(rowFor("pickup").getByTitle("Ausnahme")).toBeInTheDocument();
      expect(rowFor("arrival").queryByTitle("Ausnahme")).toBeNull();
    });

    it("does not render the exception dot when isPickupException is false", () => {
      renderHeaderWithTimes({
        todayPickupPlannedTime: "15:30",
        isPickupException: false,
      });

      expect(rowFor("pickup").queryByTitle("Ausnahme")).toBeNull();
    });

    it("renders an optional pickup note inline in parentheses", () => {
      renderHeaderWithTimes({
        todayPickupPlannedTime: "15:30",
        todayPickupNote: "Wird von Oma abgeholt",
      });

      expect(
        rowFor("pickup").getByText("(Wird von Oma abgeholt)"),
      ).toBeInTheDocument();
    });

    it("switches the arrival row to 'Kommt heute nicht' when isArrivalAbsent is true", () => {
      renderHeaderWithTimes({
        todayArrivalPlannedTime: "08:00",
        isArrivalAbsent: true,
      });

      const arrival = rowFor("arrival");
      // Absent state is mutually exclusive with the planned value — showing
      // both would confuse caregivers about whether to expect the kid.
      expect(arrival.getByText("Kommt heute nicht")).toBeInTheDocument();
      expect(arrival.queryByText("08:00")).toBeNull();
      // Pickup row must remain unaffected.
      expect(rowFor("pickup").queryByText("Kommt heute nicht")).toBeNull();
    });

    it("renders the absence reason inside the absent row", () => {
      renderHeaderWithTimes({
        todayArrivalPlannedTime: "08:00",
        isArrivalAbsent: true,
        todayArrivalNote: "Krank",
      });

      const arrival = rowFor("arrival");
      expect(arrival.getByText("Kommt heute nicht")).toBeInTheDocument();
      expect(arrival.getByText("(Krank)")).toBeInTheDocument();
    });

    it("keeps a sick checked-in student out of the overdue pickup row", () => {
      renderHeaderWithTimes({
        student: { ...mockStudent, sick: true },
        todayArrivalPlannedTime: "08:00",
        todayArrivalActualTime: "08:05",
        todayPickupPlannedTime: "15:30",
        todayPickupActualTime: undefined,
      });

      expect(screen.getByTestId("today-absence-row")).toHaveTextContent(
        "Kommt heute nicht (krank gemeldet)",
      );
      expect(screen.queryByTestId("today-time-row-pickup")).toBeNull();
    });

    it("uses day planning status for the detail header absence row", () => {
      renderHeaderWithTimes({
        student: {
          ...mockStudent,
          current_location: "Zuhause",
          day_planning_status: "not_coming_today",
          day_planning_label: "kein Plan für heute",
        },
        todayArrivalPlannedTime: undefined,
        todayPickupPlannedTime: undefined,
      });

      expect(screen.getByTestId("today-absence-row")).toHaveTextContent(
        "Kommt heute nicht (kein Plan für heute)",
      );
      expect(screen.queryByTestId("today-time-row-arrival")).toBeNull();
      expect(screen.queryByTestId("today-time-row-pickup")).toBeNull();
    });

    it("renders the actual time even when no planned time was scheduled (walk-in)", () => {
      // Walk-in scenario: a student arrives without a planned arrival
      // configured. The row still renders the actual but suppresses the
      // "(geplant: …)" parenthetical because there's nothing to compare.
      renderHeaderWithTimes({
        todayArrivalActualTime: "07:55",
      });

      const arrival = rowFor("arrival");
      expect(arrival.getByText("07:55")).toBeInTheDocument();
      expect(arrival.queryByText(/geplant:/)).toBeNull();
    });

    it("folds the diff annotation into the planned parenthetical", () => {
      renderHeaderWithTimes({
        todayPickupPlannedTime: "15:00",
        todayPickupActualTime: "15:12",
      });

      // Inline form combines planned + diff in one parenthetical to keep
      // the row compact: "(geplant: 15:00, +12 min)".
      expect(
        rowFor("pickup").getByText(/geplant: 15:00, \+12 min/),
      ).toBeInTheDocument();
    });
  });
});

// =============================================================================
// SupervisorsCard Tests
// =============================================================================

describe("SupervisorsCard", () => {
  const mockSupervisors: SupervisorContact[] = [
    {
      id: 1,
      first_name: "Anna",
      last_name: "Schmidt",
      email: "anna.schmidt@example.com",
      role: "supervisor",
    },
  ];

  it("returns null when supervisors array is empty", () => {
    const { container } = render(
      <SupervisorsCard supervisors={[]} studentName="Max" />,
    );
    expect(container.firstChild).toBeNull();
  });

  it("renders supervisor name", () => {
    render(
      <SupervisorsCard
        supervisors={mockSupervisors}
        studentName="Max Mustermann"
      />,
    );
    expect(screen.getByText("Anna Schmidt")).toBeInTheDocument();
  });

  it("renders supervisor email", () => {
    render(
      <SupervisorsCard
        supervisors={mockSupervisors}
        studentName="Max Mustermann"
      />,
    );
    expect(screen.getByText("anna.schmidt@example.com")).toBeInTheDocument();
  });

  it("renders 'Gruppenleitung' badge", () => {
    render(
      <SupervisorsCard
        supervisors={mockSupervisors}
        studentName="Max Mustermann"
      />,
    );
    expect(screen.getByText("Gruppenleitung")).toBeInTheDocument();
  });

  it("renders contact button", () => {
    render(
      <SupervisorsCard
        supervisors={mockSupervisors}
        studentName="Max Mustermann"
      />,
    );
    expect(
      screen.getByRole("button", { name: /kontakt aufnehmen/i }),
    ).toBeInTheDocument();
  });

  it("opens email client when contact button is clicked", () => {
    // Mock globalThis.location
    const originalLocation = globalThis.location;
    Object.defineProperty(globalThis, "location", {
      value: { href: "" },
      writable: true,
    });

    render(
      <SupervisorsCard
        supervisors={mockSupervisors}
        studentName="Max Mustermann"
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /kontakt aufnehmen/i }));

    expect(globalThis.location.href).toBe(
      "mailto:anna.schmidt@example.com?subject=Anfrage zu Max Mustermann",
    );

    // Restore original location
    Object.defineProperty(globalThis, "location", {
      value: originalLocation,
      writable: true,
    });
  });

  it("renders multiple supervisors with divider", () => {
    const multipleSupervisors: SupervisorContact[] = [
      {
        id: 1,
        first_name: "Anna",
        last_name: "Schmidt",
        email: "anna@example.com",
        role: "supervisor",
      },
      {
        id: 2,
        first_name: "Peter",
        last_name: "Müller",
        email: "peter@example.com",
        role: "supervisor",
      },
    ];

    render(
      <SupervisorsCard
        supervisors={multipleSupervisors}
        studentName="Max Mustermann"
      />,
    );

    expect(screen.getByText("Anna Schmidt")).toBeInTheDocument();
    expect(screen.getByText("Peter Müller")).toBeInTheDocument();
  });

  it("renders section title", () => {
    render(
      <SupervisorsCard
        supervisors={mockSupervisors}
        studentName="Max Mustermann"
      />,
    );
    expect(screen.getByText("Ansprechpartner")).toBeInTheDocument();
  });

  it("does not render contact button when supervisor has no email", () => {
    const supervisorWithoutEmail: SupervisorContact[] = [
      {
        id: 1,
        first_name: "Anna",
        last_name: "Schmidt",
        email: undefined,
        role: "supervisor",
      },
    ];

    render(
      <SupervisorsCard
        supervisors={supervisorWithoutEmail}
        studentName="Max Mustermann"
      />,
    );

    expect(
      screen.queryByRole("button", { name: /kontakt aufnehmen/i }),
    ).not.toBeInTheDocument();
  });
});

// =============================================================================
// PersonalInfoReadOnly Tests
// =============================================================================

describe("PersonalInfoReadOnly", () => {
  const mockStudent: ExtendedStudent = {
    id: "1",
    first_name: "Max",
    second_name: "Mustermann",
    name: "Max Mustermann",
    birthday: "2015-05-15",
    school_class: "3a",
    group_id: "g1",
    group_name: "Gruppe 1",
    buskind: true,
    pickup_status: "selbst",
    health_info: "Allergien: Erdnüsse",
    address_street: "Musterstraße 12",
    address_city: "Köln",
    address_postal_code: "50667",
    supervisor_notes: undefined,
    extra_info: undefined,
    sick: false,
    sick_since: undefined,
    current_location: "Nicht anwesend",
    location_since: undefined,
    bus: true,
    bus_days: { mon: true, fri: true },
  };

  it("renders section title", () => {
    render(<PersonalInfoReadOnly student={mockStudent} />);
    expect(screen.getByText("Persönliche Informationen")).toBeInTheDocument();
  });

  it("renders student name", () => {
    render(<PersonalInfoReadOnly student={mockStudent} />);
    expect(screen.getByText("Max Mustermann")).toBeInTheDocument();
  });

  it("renders school class", () => {
    render(<PersonalInfoReadOnly student={mockStudent} />);
    expect(screen.getByText("3a")).toBeInTheDocument();
  });

  it("renders group name", () => {
    render(<PersonalInfoReadOnly student={mockStudent} />);
    expect(screen.getByText("Gruppe 1")).toBeInTheDocument();
  });

  it("renders student address", () => {
    render(<PersonalInfoReadOnly student={mockStudent} />);
    expect(screen.getByText("Musterstraße 12, 50667 Köln")).toBeInTheDocument();
  });

  it("renders 'Nicht zugewiesen' when no group", () => {
    const studentWithoutGroup = { ...mockStudent, group_name: undefined };
    render(<PersonalInfoReadOnly student={studentWithoutGroup} />);
    expect(screen.getByText("Nicht zugewiesen")).toBeInTheDocument();
  });

  it("formats birthday correctly", () => {
    render(<PersonalInfoReadOnly student={mockStudent} />);
    // Use same formatting as component to ensure timezone-stable assertion
    const expectedDate = formatDateLikeComponent("2015-05-15");
    expect(screen.getByText(expectedDate)).toBeInTheDocument();
  });

  it("shows 'Nicht angegeben' when no birthday", () => {
    const studentWithoutBirthday = { ...mockStudent, birthday: undefined };
    render(<PersonalInfoReadOnly student={studentWithoutBirthday} />);
    expect(screen.getByText("Nicht angegeben")).toBeInTheDocument();
  });

  it("renders bus days as the weekday departure matrix", () => {
    // mockStudent has bus_days {mon, fri}; these fold into a "Bus" badge on Mo
    // and Fr, while the unconfigured weekdays fold to a "Zu Fuß" badge.
    render(<PersonalInfoReadOnly student={mockStudent} />);
    expect(screen.getByText("Mo")).toBeInTheDocument();
    expect(screen.getByText("Fr")).toBeInTheDocument();
    expect(screen.getAllByText("Bus")).toHaveLength(2);
    expect(screen.getAllByText("Zu Fuß")).toHaveLength(3);
    // The verbose single-line sentence is gone.
    expect(
      screen.queryByText("Mo: Fährt Bus; Fr: Fährt Bus"),
    ).not.toBeInTheDocument();
  });

  it("renders 'Geht immer alleine' when no bus/pickup days are set", () => {
    const goesAlone = {
      ...mockStudent,
      buskind: false,
      bus: false,
      bus_days: {},
      pickup_days: {},
    };
    render(<PersonalInfoReadOnly student={goesAlone} />);
    expect(screen.getByText("Geht immer alleine")).toBeInTheDocument();
  });

  it("renders pickup weekdays as 'Abgeholt' badges in the departure matrix", () => {
    const pickedUp = {
      ...mockStudent,
      bus_days: {},
      pickup_days: { mon: true, wed: true },
    };
    render(<PersonalInfoReadOnly student={pickedUp} />);
    expect(screen.getAllByText("Abgeholt")).toHaveLength(2);
    expect(screen.getAllByText("Zu Fuß")).toHaveLength(3);
    expect(
      screen.queryByText("Mo: Wird abgeholt; Mi: Wird abgeholt"),
    ).not.toBeInTheDocument();
  });

  it("renders health info when present", () => {
    render(<PersonalInfoReadOnly student={mockStudent} />);
    expect(screen.getByText("Allergien: Erdnüsse")).toBeInTheDocument();
  });

  it("renders enrollment extra fields as personal info rows", () => {
    render(
      <PersonalInfoReadOnly
        student={mockStudent}
        enrollmentExtraGroups={[
          {
            request_id: "77",
            phase_name: "Anmeldung 2026",
            submitted_at: "2026-06-01T12:00:00Z",
            fields: [
              {
                key: "swimming_level",
                label: "Schwimmfähigkeit",
                type: "select",
                options: [{ label: "Kann sicher schwimmen", value: "safe" }],
                value: "safe",
              },
            ],
          },
        ]}
      />,
    );

    expect(screen.getByText("Schwimmfähigkeit")).toBeInTheDocument();
    expect(screen.getByText("Kann sicher schwimmen")).toBeInTheDocument();
  });

  it("prefixes enrollment extra field labels when multiple requests are present", () => {
    render(
      <PersonalInfoReadOnly
        student={mockStudent}
        enrollmentExtraGroups={[
          {
            request_id: "77",
            phase_name: "Anmeldung 2026",
            submitted_at: "2026-06-01T12:00:00Z",
            fields: [
              {
                key: "note",
                label: "Hinweis",
                type: "text",
                value: "Erste Antwort",
              },
            ],
          },
          {
            request_id: "88",
            phase_name: "Sommerferien 2026",
            submitted_at: "2026-07-01T12:00:00Z",
            fields: [
              {
                key: "note",
                label: "Hinweis",
                type: "text",
                value: "Zweite Antwort",
              },
            ],
          },
        ]}
      />,
    );

    expect(screen.getByText("Anmeldung 2026 · Hinweis")).toBeInTheDocument();
    expect(screen.getByText("Sommerferien 2026 · Hinweis")).toBeInTheDocument();
    expect(screen.getByText("Erste Antwort")).toBeInTheDocument();
    expect(screen.getByText("Zweite Antwort")).toBeInTheDocument();
  });

  it("does not render health info when not present", () => {
    const studentWithoutHealth = { ...mockStudent, health_info: undefined };
    render(<PersonalInfoReadOnly student={studentWithoutHealth} />);
    expect(
      screen.queryByText("Gesundheitsinformationen"),
    ).not.toBeInTheDocument();
  });

  it("shows 'Nur Ansicht' badge by default", () => {
    render(<PersonalInfoReadOnly student={mockStudent} />);
    // Badge shows "Nur Ansicht" on desktop and "Ansicht" on mobile
    expect(screen.getByText("Nur Ansicht")).toBeInTheDocument();
  });

  it("shows edit button when showEditButton is true", () => {
    const onEditClick = vi.fn();
    render(
      <PersonalInfoReadOnly
        student={mockStudent}
        showEditButton={true}
        onEditClick={onEditClick}
      />,
    );
    expect(
      screen.getByRole("button", { name: "Bearbeiten" }),
    ).toBeInTheDocument();
  });

  it("calls onEditClick when edit button is clicked", () => {
    const onEditClick = vi.fn();
    render(
      <PersonalInfoReadOnly
        student={mockStudent}
        showEditButton={true}
        onEditClick={onEditClick}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    expect(onEditClick).toHaveBeenCalledTimes(1);
  });

  it("does not show edit button when showEditButton is true but onEditClick is undefined", () => {
    render(
      <PersonalInfoReadOnly student={mockStudent} showEditButton={true} />,
    );
    // Edit button should not be rendered when onEditClick is not provided
    expect(screen.queryByTitle("Bearbeiten")).not.toBeInTheDocument();
    // Should still show the "Nur Ansicht" badge
    expect(screen.getByText("Nur Ansicht")).toBeInTheDocument();
  });

  it("does not show edit button when showEditButton is false", () => {
    render(
      <PersonalInfoReadOnly
        student={mockStudent}
        showEditButton={false}
        onEditClick={vi.fn()}
      />,
    );
    expect(screen.queryByTitle("Bearbeiten")).not.toBeInTheDocument();
  });
});

// =============================================================================
// PersonalInfoReadOnly with Edit Button Tests
// =============================================================================

describe("PersonalInfoReadOnly with showEditButton", () => {
  const mockStudent: ExtendedStudent = {
    id: "1",
    first_name: "Max",
    second_name: "Mustermann",
    name: "Max Mustermann",
    birthday: "2015-05-15",
    school_class: "3a",
    group_id: "g1",
    group_name: "Gruppe 1",
    buskind: false,
    pickup_status: "selbst",
    health_info: "Allergien: Erdnüsse",
    supervisor_notes: "Benötigt extra Aufmerksamkeit",
    extra_info: "Elternnotiz hier",
    sick: false,
    sick_since: undefined,
    current_location: "Nicht anwesend",
    location_since: undefined,
    bus: false,
  };

  it("renders section title", () => {
    render(
      <PersonalInfoReadOnly
        student={mockStudent}
        showEditButton={true}
        onEditClick={vi.fn()}
      />,
    );
    expect(screen.getByText("Persönliche Informationen")).toBeInTheDocument();
  });

  it("renders edit button when showEditButton is true", () => {
    render(
      <PersonalInfoReadOnly
        student={mockStudent}
        showEditButton={true}
        onEditClick={vi.fn()}
      />,
    );
    expect(
      screen.getByRole("button", { name: "Bearbeiten" }),
    ).toBeInTheDocument();
  });

  it("calls onEditClick when edit button is clicked", () => {
    const onEditClick = vi.fn();
    render(
      <PersonalInfoReadOnly
        student={mockStudent}
        showEditButton={true}
        onEditClick={onEditClick}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    expect(onEditClick).toHaveBeenCalledTimes(1);
  });

  it("renders supervisor notes when present", () => {
    render(
      <PersonalInfoReadOnly
        student={mockStudent}
        showEditButton={true}
        onEditClick={vi.fn()}
      />,
    );
    expect(
      screen.getByText("Benötigt extra Aufmerksamkeit"),
    ).toBeInTheDocument();
  });

  it("renders extra info (parent notes) when present", () => {
    render(
      <PersonalInfoReadOnly
        student={mockStudent}
        showEditButton={true}
        onEditClick={vi.fn()}
      />,
    );
    expect(screen.getByText("Elternnotiz hier")).toBeInTheDocument();
  });

  it("shows change metadata for the departure companion note", () => {
    mockUseSWR.mockReturnValue({
      data: [
        {
          id: "42",
          field: "departure_companion_note",
          edited_by: "Erika Beispiel",
          changed_at: "2026-07-25T10:00:00Z",
        },
      ],
      error: undefined,
      isLoading: false,
      isValidating: false,
      mutate: vi.fn(),
    });

    render(
      <PersonalInfoReadOnly
        student={{
          ...mockStudent,
          departure_companion_note: "Geschwisterkind Mia",
        }}
        showEditButton={true}
        onEditClick={vi.fn()}
      />,
    );

    expect(screen.getByText("Geht außerdem mit")).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", {
        name: "Änderungsinformation anzeigen",
      }),
    );
    expect(screen.getByText("Erika Beispiel")).toBeInTheDocument();
  });

  // Note: Sick status display was moved to StudentSickReportSection (student-checkout-section.tsx)
  // Tests for sick functionality are in student-checkout-section.test.tsx and page.test.tsx
});

// =============================================================================
// StudentHistorySection Tests
// =============================================================================

describe("StudentHistorySection", () => {
  const defaultProps = {
    studentId: "123",
    attendanceLogEnabled: true,
    feedbackEnabled: true,
    onNavigate: vi.fn(),
  };

  it("renders section title", () => {
    render(<StudentHistorySection {...defaultProps} />);
    expect(screen.getByText("Historien")).toBeInTheDocument();
  });

  it("renders room history button as enabled when feature is on", () => {
    render(<StudentHistorySection {...defaultProps} />);
    expect(screen.getByText("Anwesenheitsprotokoll")).toBeInTheDocument();
    expect(
      screen.getByText("Anwesenheit je Betreuungsangebot und besuchte Räume"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Anwesenheitsprotokoll").closest("button"),
    ).not.toBeDisabled();
  });

  it("renders room history button as disabled when feature is off", () => {
    render(
      <StudentHistorySection {...defaultProps} attendanceLogEnabled={false} />,
    );
    expect(screen.getByText("Anwesenheitsprotokoll")).toBeInTheDocument();
    expect(
      screen.getByText("Anwesenheitsprotokoll").closest("button"),
    ).toBeDisabled();
  });

  it("renders feedback history button as enabled when feature is on", () => {
    render(<StudentHistorySection {...defaultProps} />);
    expect(screen.getByText("Feedbackhistorie")).toBeInTheDocument();
    expect(screen.getByText("Feedback und Bewertungen")).toBeInTheDocument();
    const feedbackButton = screen
      .getByText("Feedbackhistorie")
      .closest("button");
    expect(feedbackButton).not.toBeDisabled();
  });

  it("renders feedback history button as disabled when feature is off", () => {
    render(<StudentHistorySection {...defaultProps} feedbackEnabled={false} />);
    expect(screen.getByText("Feedbackhistorie")).toBeInTheDocument();
    expect(
      screen.getByText("Feedbackhistorie").closest("button"),
    ).toBeDisabled();
  });

  it("renders meal history button as disabled", () => {
    render(<StudentHistorySection {...defaultProps} />);
    expect(screen.getByText("Mensaverlauf")).toBeInTheDocument();
    expect(screen.getByText("Mahlzeiten und Bestellungen")).toBeInTheDocument();
    expect(screen.getByText("Mensaverlauf").closest("button")).toBeDisabled();
  });

  it("disables change history without supervisor access", () => {
    render(
      <StudentHistorySection {...defaultProps} canViewChangeHistory={false} />,
    );

    expect(
      screen.getByText("Änderungsverlauf").closest("button"),
    ).toBeDisabled();
    expect(
      screen.getByText("Anwesenheitsprotokoll").closest("button"),
    ).not.toBeDisabled();
  });

  it("raumverlauf button navigates on click when enabled", () => {
    const onNavigate = vi.fn();
    render(
      <StudentHistorySection
        studentId="456"
        attendanceLogEnabled={true}
        feedbackEnabled={true}
        onNavigate={onNavigate}
      />,
    );
    const raumButton = screen
      .getByText("Anwesenheitsprotokoll")
      .closest("button");
    fireEvent.click(raumButton!);
    expect(onNavigate).toHaveBeenCalledWith("/students/456/room-history");
  });

  it("raumverlauf button does not navigate when disabled", () => {
    const onNavigate = vi.fn();
    render(
      <StudentHistorySection
        studentId="456"
        attendanceLogEnabled={false}
        feedbackEnabled={true}
        onNavigate={onNavigate}
      />,
    );
    const raumButton = screen
      .getByText("Anwesenheitsprotokoll")
      .closest("button");
    fireEvent.click(raumButton!);
    expect(onNavigate).not.toHaveBeenCalled();
  });

  it("feedback button navigates on click when enabled", () => {
    const onNavigate = vi.fn();
    render(
      <StudentHistorySection
        studentId="456"
        attendanceLogEnabled={true}
        feedbackEnabled={true}
        onNavigate={onNavigate}
      />,
    );
    const feedbackButton = screen
      .getByText("Feedbackhistorie")
      .closest("button");
    fireEvent.click(feedbackButton!);
    expect(onNavigate).toHaveBeenCalledWith("/students/456/feedback-history");
  });
});
