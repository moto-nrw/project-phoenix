import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import {
  PersonIcon,
  ChevronDownIcon,
  WarningIcon,
  StudentDetailHeader,
  SupervisorsCard,
  PersonalInfoReadOnly,
  FullAccessPersonalInfoReadOnly,
  StudentHistorySection,
} from "./student-detail-components";
import type { ExtendedStudent } from "~/lib/hooks/use-student-data";
import type { SupervisorContact } from "~/lib/student-helpers";

/**
 * Format a date string the same way the component does.
 * This ensures tests are timezone-stable by using the same formatting logic.
 */
function formatDateLikeComponent(dateString: string): string {
  return new Date(dateString).toLocaleDateString("de-DE");
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

  it("renders path element", () => {
    render(<PersonIcon />);
    const path = document.querySelector("path");
    expect(path).toBeInTheDocument();
    expect(path).toHaveAttribute("stroke-linecap", "round");
    expect(path).toHaveAttribute("stroke-linejoin", "round");
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

describe("StudentDetailHeader", () => {
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
    supervisor_notes: undefined,
    extra_info: undefined,
    sick: false,
    sick_since: undefined,
    current_location: "Nicht anwesend",
    location_since: undefined,
    bus: false,
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

  it("renders buskind status", () => {
    render(<PersonalInfoReadOnly student={mockStudent} />);
    expect(screen.getByText("Ja")).toBeInTheDocument();
  });

  it("renders 'Nein' when not buskind", () => {
    const studentNotBuskind = { ...mockStudent, buskind: false };
    render(<PersonalInfoReadOnly student={studentNotBuskind} />);
    expect(screen.getByText("Nein")).toBeInTheDocument();
  });

  it("renders pickup status", () => {
    render(<PersonalInfoReadOnly student={mockStudent} />);
    expect(screen.getByText("selbst")).toBeInTheDocument();
  });

  it("renders health info when present", () => {
    render(<PersonalInfoReadOnly student={mockStudent} />);
    expect(screen.getByText("Allergien: Erdnüsse")).toBeInTheDocument();
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
    expect(screen.getByTitle("Bearbeiten")).toBeInTheDocument();
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
    fireEvent.click(screen.getByTitle("Bearbeiten"));
    expect(onEditClick).toHaveBeenCalledTimes(1);
  });
});

// =============================================================================
// FullAccessPersonalInfoReadOnly Tests
// =============================================================================

describe("FullAccessPersonalInfoReadOnly", () => {
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
      <FullAccessPersonalInfoReadOnly
        student={mockStudent}
        onEditClick={vi.fn()}
      />,
    );
    expect(screen.getByText("Persönliche Informationen")).toBeInTheDocument();
  });

  it("renders edit button", () => {
    render(
      <FullAccessPersonalInfoReadOnly
        student={mockStudent}
        onEditClick={vi.fn()}
      />,
    );
    expect(screen.getByTitle("Bearbeiten")).toBeInTheDocument();
  });

  it("calls onEditClick when edit button is clicked", () => {
    const onEditClick = vi.fn();
    render(
      <FullAccessPersonalInfoReadOnly
        student={mockStudent}
        onEditClick={onEditClick}
      />,
    );
    fireEvent.click(screen.getByTitle("Bearbeiten"));
    expect(onEditClick).toHaveBeenCalledTimes(1);
  });

  it("renders supervisor notes when present", () => {
    render(
      <FullAccessPersonalInfoReadOnly
        student={mockStudent}
        onEditClick={vi.fn()}
      />,
    );
    expect(
      screen.getByText("Benötigt extra Aufmerksamkeit"),
    ).toBeInTheDocument();
  });

  it("renders extra info (parent notes) when present", () => {
    render(
      <FullAccessPersonalInfoReadOnly
        student={mockStudent}
        onEditClick={vi.fn()}
      />,
    );
    expect(screen.getByText("Elternnotiz hier")).toBeInTheDocument();
  });

  it("shows 'Nicht krankgemeldet' when student is not sick", () => {
    render(
      <FullAccessPersonalInfoReadOnly
        student={mockStudent}
        onEditClick={vi.fn()}
      />,
    );
    expect(screen.getByText("Nicht krankgemeldet")).toBeInTheDocument();
  });

  it("shows 'Krank gemeldet' when student is sick", () => {
    const sickStudent = {
      ...mockStudent,
      sick: true,
      sick_since: "2024-01-10",
    };
    render(
      <FullAccessPersonalInfoReadOnly
        student={sickStudent}
        onEditClick={vi.fn()}
      />,
    );
    expect(screen.getByText("Krank gemeldet")).toBeInTheDocument();
  });

  it("shows sick since date when student is sick", () => {
    const sickSinceDate = "2024-01-10";
    const sickStudent = {
      ...mockStudent,
      sick: true,
      sick_since: sickSinceDate,
    };
    render(
      <FullAccessPersonalInfoReadOnly
        student={sickStudent}
        onEditClick={vi.fn()}
      />,
    );
    expect(screen.getByText(/seit/)).toBeInTheDocument();
    // Use same formatting as component to ensure timezone-stable assertion
    const expectedDate = formatDateLikeComponent(sickSinceDate);
    expect(screen.getByText(new RegExp(expectedDate))).toBeInTheDocument();
  });
});

// =============================================================================
// StudentHistorySection Tests
// =============================================================================

describe("StudentHistorySection", () => {
  it("renders section title", () => {
    render(<StudentHistorySection />);
    expect(screen.getByText("Historien")).toBeInTheDocument();
  });

  it("renders room history button", () => {
    render(<StudentHistorySection />);
    expect(screen.getByText("Raumverlauf")).toBeInTheDocument();
    expect(screen.getByText("Verlauf der Raumbesuche")).toBeInTheDocument();
  });

  it("renders feedback history button", () => {
    render(<StudentHistorySection />);
    expect(screen.getByText("Feedbackhistorie")).toBeInTheDocument();
    expect(screen.getByText("Feedback und Bewertungen")).toBeInTheDocument();
  });

  it("renders meal history button", () => {
    render(<StudentHistorySection />);
    expect(screen.getByText("Mensaverlauf")).toBeInTheDocument();
    expect(screen.getByText("Mahlzeiten und Bestellungen")).toBeInTheDocument();
  });

  it("all history buttons are disabled", () => {
    render(<StudentHistorySection />);
    const buttons = screen.getAllByRole("button");
    buttons.forEach((button) => {
      expect(button).toBeDisabled();
    });
  });
});
