import "@testing-library/jest-dom/vitest";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { mockLoggerError } = vi.hoisted(() => ({
  mockLoggerError: vi.fn(),
}));

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({
    error: mockLoggerError,
    warn: vi.fn(),
    info: vi.fn(),
    debug: vi.fn(),
  }),
}));

vi.mock("~/components/ui/hooks/useIsMobile", () => ({
  useIsMobile: vi.fn(() => false),
}));

class MockResizeObserver {
  observe = vi.fn();
  unobserve = vi.fn();
  disconnect = vi.fn();
}

vi.stubGlobal("ResizeObserver", MockResizeObserver);

import { StaffMasterDetail } from "./staff-master-detail";
import type { GroupDefinition } from "~/components/database/grouped-list";
import type { Teacher } from "@/lib/teacher-api";

const baseTeacher: Teacher = {
  id: "1",
  name: "Anna Müller",
  first_name: "Anna",
  last_name: "Müller",
  email: "anna.mueller@example.com",
  role: "Lehrerin",
  account_role: "teacher",
  qualifications: "M.Ed. Mathematik",
  staff_notes: "",
  staff_id: "11",
  account_id: 99,
};

const teacherWithoutAccount: Teacher = {
  ...baseTeacher,
  id: "2",
  first_name: "Ben",
  last_name: "Schmidt",
  name: "Ben Schmidt",
  email: undefined,
  account_id: undefined,
  account_role: null,
  role: null,
  qualifications: null,
  staff_id: "12",
};

function flatGroup(teachers: Teacher[]): GroupDefinition<Teacher>[] {
  if (teachers.length === 0) return [];
  return [
    {
      id: "__flat__",
      title: `Alle Personal (${teachers.length})`,
      items: teachers,
    },
  ];
}

describe("StaffMasterDetail", () => {
  const onSelect = vi.fn();
  const onEditClick = vi.fn();
  const onDeleteClick = vi.fn();
  const onUpdateNotes = vi.fn(() => Promise.resolve());
  const onManageCaregiver = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows the empty detail state when nothing is selected", () => {
    render(
      <StaffMasterDetail
        groupDefinitions={flatGroup([baseTeacher])}
        selectedId={null}
        selectedTeacher={null}
        onSelect={onSelect}
        onEditClick={onEditClick}
        onDeleteClick={onDeleteClick}
        onUpdateNotes={onUpdateNotes}
      />,
    );

    expect(screen.queryByText("Bearbeiten")).not.toBeInTheDocument();
    expect(screen.getByText("Anna Müller")).toBeInTheDocument();
  });

  it("renders the detail header and stammdaten sections for the selected teacher", () => {
    render(
      <StaffMasterDetail
        groupDefinitions={flatGroup([baseTeacher])}
        selectedId="1"
        selectedTeacher={baseTeacher}
        onSelect={onSelect}
        onEditClick={onEditClick}
        onDeleteClick={onDeleteClick}
        onUpdateNotes={onUpdateNotes}
      />,
    );

    expect(screen.getAllByText("Anna Müller").length).toBeGreaterThan(0);
    expect(screen.getByText("Persönliche Daten")).toBeInTheDocument();
    expect(screen.getByText("anna.mueller@example.com")).toBeInTheDocument();
    expect(screen.getByText("M.Ed. Mathematik")).toBeInTheDocument();
  });

  it("triggers edit and delete callbacks from the header", () => {
    render(
      <StaffMasterDetail
        groupDefinitions={flatGroup([baseTeacher])}
        selectedId="1"
        selectedTeacher={baseTeacher}
        onSelect={onSelect}
        onEditClick={onEditClick}
        onDeleteClick={onDeleteClick}
        onUpdateNotes={onUpdateNotes}
      />,
    );

    fireEvent.click(screen.getByText("Bearbeiten"));
    expect(onEditClick).toHaveBeenCalled();

    fireEvent.click(screen.getByText("Löschen"));
    expect(onDeleteClick).toHaveBeenCalled();
  });

  it("shows the manage caregiver button only when the callback is provided", () => {
    const { rerender } = render(
      <StaffMasterDetail
        groupDefinitions={flatGroup([baseTeacher])}
        selectedId="1"
        selectedTeacher={baseTeacher}
        onSelect={onSelect}
        onEditClick={onEditClick}
        onDeleteClick={onDeleteClick}
        onUpdateNotes={onUpdateNotes}
      />,
    );

    expect(screen.queryByText("Betreuung verwalten")).not.toBeInTheDocument();

    rerender(
      <StaffMasterDetail
        groupDefinitions={flatGroup([baseTeacher])}
        selectedId="1"
        selectedTeacher={baseTeacher}
        onSelect={onSelect}
        onEditClick={onEditClick}
        onDeleteClick={onDeleteClick}
        onUpdateNotes={onUpdateNotes}
        onManageCaregiver={onManageCaregiver}
      />,
    );

    fireEvent.click(screen.getByText("Betreuung verwalten"));
    expect(onManageCaregiver).toHaveBeenCalled();
  });

  it("shows the manage role button only when the callback is provided", () => {
    const onManageRole = vi.fn();
    const { rerender } = render(
      <StaffMasterDetail
        groupDefinitions={flatGroup([baseTeacher])}
        selectedId="1"
        selectedTeacher={baseTeacher}
        onSelect={onSelect}
        onEditClick={onEditClick}
        onDeleteClick={onDeleteClick}
        onUpdateNotes={onUpdateNotes}
      />,
    );

    expect(screen.queryByText("Rolle verwalten")).not.toBeInTheDocument();

    rerender(
      <StaffMasterDetail
        groupDefinitions={flatGroup([baseTeacher])}
        selectedId="1"
        selectedTeacher={baseTeacher}
        onSelect={onSelect}
        onEditClick={onEditClick}
        onDeleteClick={onDeleteClick}
        onUpdateNotes={onUpdateNotes}
        onManageRole={onManageRole}
      />,
    );

    fireEvent.click(screen.getByText("Rolle verwalten"));
    expect(onManageRole).toHaveBeenCalled();
  });

  it("hides the email and qualifications sections when fields are missing", () => {
    render(
      <StaffMasterDetail
        groupDefinitions={flatGroup([teacherWithoutAccount])}
        selectedId="2"
        selectedTeacher={teacherWithoutAccount}
        onSelect={onSelect}
        onEditClick={onEditClick}
        onDeleteClick={onDeleteClick}
        onUpdateNotes={onUpdateNotes}
      />,
    );

    expect(screen.queryByText("E-Mail")).not.toBeInTheDocument();
    expect(
      screen.queryByText("Berufliche Informationen"),
    ).not.toBeInTheDocument();
  });

  it("renders the group titles supplied by the page and a Stammdaten tab", () => {
    render(
      <StaffMasterDetail
        groupDefinitions={[
          {
            id: "teacher",
            title: "Lehrkraft (1)",
            items: [baseTeacher],
          },
          {
            id: "__no_role__",
            title: "Ohne Rolle (1)",
            items: [teacherWithoutAccount],
          },
        ]}
        selectedId="1"
        selectedTeacher={baseTeacher}
        onSelect={onSelect}
        onEditClick={onEditClick}
        onDeleteClick={onDeleteClick}
        onUpdateNotes={onUpdateNotes}
      />,
    );

    expect(screen.getByText("Lehrkraft (1)")).toBeInTheDocument();
    expect(screen.getByText("Ohne Rolle (1)")).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Stammdaten" })).toBeInTheDocument();
  });

  it("forwards notes edits to onUpdateNotes when the inline editor is saved", async () => {
    onUpdateNotes.mockResolvedValueOnce(undefined);

    render(
      <StaffMasterDetail
        groupDefinitions={flatGroup([baseTeacher])}
        selectedId="1"
        selectedTeacher={baseTeacher}
        onSelect={onSelect}
        onEditClick={onEditClick}
        onDeleteClick={onDeleteClick}
        onUpdateNotes={onUpdateNotes}
      />,
    );

    fireEvent.click(screen.getByText("Notizen hinzufügen..."));

    const textarea = screen.getByPlaceholderText("Notizen hinzufügen...");
    fireEvent.change(textarea, { target: { value: "Important note" } });
    // Save resolves asynchronously and updates the editor's local state on
    // success — wrap the click + microtask flush in act() so the resulting
    // post-render doesn't escape the test scope.
    await act(async () => {
      fireEvent.click(screen.getByText("Speichern"));
    });

    expect(onUpdateNotes).toHaveBeenCalledWith("Important note");
  });

  it("logs note save failures without closing the inline editor", async () => {
    const failingUpdate = vi.fn().mockRejectedValue(new Error("save failed"));

    render(
      <StaffMasterDetail
        groupDefinitions={flatGroup([baseTeacher])}
        selectedId="1"
        selectedTeacher={{ ...baseTeacher, staff_notes: "Existing notes" }}
        onSelect={onSelect}
        onEditClick={onEditClick}
        onDeleteClick={onDeleteClick}
        onUpdateNotes={failingUpdate}
      />,
    );

    fireEvent.click(screen.getByText("Existing notes"));
    fireEvent.click(screen.getByText("Speichern"));

    await waitFor(() => {
      expect(mockLoggerError).toHaveBeenCalledWith(
        "notes_save_failed",
        expect.objectContaining({ error: "save failed" }),
      );
    });
    expect(
      screen.getByPlaceholderText("Notizen hinzufügen..."),
    ).toBeInTheDocument();
  });

  it("resyncs inline notes when parent props change after cancel", () => {
    const { rerender } = render(
      <StaffMasterDetail
        groupDefinitions={flatGroup([baseTeacher])}
        selectedId="1"
        selectedTeacher={{ ...baseTeacher, staff_notes: "Initial notes" }}
        onSelect={onSelect}
        onEditClick={onEditClick}
        onDeleteClick={onDeleteClick}
        onUpdateNotes={onUpdateNotes}
      />,
    );

    fireEvent.click(screen.getByText("Initial notes"));
    fireEvent.change(screen.getByPlaceholderText("Notizen hinzufügen..."), {
      target: { value: "Unsaved local edit" },
    });
    fireEvent.click(screen.getByText("Abbrechen"));

    rerender(
      <StaffMasterDetail
        groupDefinitions={flatGroup([baseTeacher])}
        selectedId="1"
        selectedTeacher={{
          ...baseTeacher,
          staff_notes: "Fresh notes from parent",
        }}
        onSelect={onSelect}
        onEditClick={onEditClick}
        onDeleteClick={onDeleteClick}
        onUpdateNotes={onUpdateNotes}
      />,
    );

    fireEvent.click(screen.getByText("Fresh notes from parent"));
    expect(
      screen.getByDisplayValue("Fresh notes from parent"),
    ).toBeInTheDocument();
  });

  it("opens mailto with the staff display name as the subject when 'Schreiben' is clicked", () => {
    const originalLocation = globalThis.location;
    const setHref = vi.fn();
    Object.defineProperty(globalThis, "location", {
      configurable: true,
      value: {
        ...originalLocation,
        set href(value: string) {
          setHref(value);
        },
        get href() {
          return originalLocation.href;
        },
      },
    });

    render(
      <StaffMasterDetail
        groupDefinitions={flatGroup([baseTeacher])}
        selectedId="1"
        selectedTeacher={baseTeacher}
        onSelect={onSelect}
        onEditClick={onEditClick}
        onDeleteClick={onDeleteClick}
        onUpdateNotes={onUpdateNotes}
      />,
    );

    fireEvent.click(screen.getByTitle("E-Mail schreiben"));

    expect(setHref).toHaveBeenCalledTimes(1);
    const url = setHref.mock.calls[0]?.[0] as string;
    expect(url).toMatch(/^mailto:anna\.mueller@example\.com\?subject=/);
    // The subject must be URL-encoded so umlauts survive transit.
    expect(url).toContain(encodeURIComponent("Betreff: Anna Müller"));

    Object.defineProperty(globalThis, "location", {
      configurable: true,
      value: originalLocation,
    });
  });

  it("copies the email to the clipboard and flips the button label to 'Kopiert'", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText },
      configurable: true,
    });

    render(
      <StaffMasterDetail
        groupDefinitions={flatGroup([baseTeacher])}
        selectedId="1"
        selectedTeacher={baseTeacher}
        onSelect={onSelect}
        onEditClick={onEditClick}
        onDeleteClick={onDeleteClick}
        onUpdateNotes={onUpdateNotes}
      />,
    );

    fireEvent.click(screen.getByTitle("E-Mail kopieren"));

    expect(writeText).toHaveBeenCalledWith("anna.mueller@example.com");
    await waitFor(() => {
      expect(screen.getByText("Kopiert")).toBeInTheDocument();
    });
  });
});
