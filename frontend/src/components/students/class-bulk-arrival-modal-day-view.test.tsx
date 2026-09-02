import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import type { Student } from "~/lib/student-helpers";

// The class modal carries two areas (#2962): the weekly Unterrichtsschluss
// and the single-day deviation. The switch shows the day panel and takes the
// weekly save button out of the footer; a group filter never gets the switch.

const { mockFetchBulkArrivalScheduleStatus, mockFetchClassArrivalTimes } =
  vi.hoisted(() => ({
    mockFetchBulkArrivalScheduleStatus: vi.fn(),
    mockFetchClassArrivalTimes: vi.fn(),
  }));

vi.mock("~/lib/student-arrival-api", async () => {
  const actual = await vi.importActual<
    typeof import("~/lib/student-arrival-api")
  >("~/lib/student-arrival-api");
  return {
    ...actual,
    fetchBulkArrivalScheduleStatus: mockFetchBulkArrivalScheduleStatus,
    fetchClassArrivalTimes: mockFetchClassArrivalTimes,
    bulkUpsertArrivalSchedules: vi.fn(),
  };
});

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({ success: vi.fn(), error: vi.fn(), info: vi.fn() }),
}));

vi.mock("./class-arrival-exception-panel", () => ({
  ClassArrivalExceptionPanel: ({
    schoolClass,
    classLabel,
  }: {
    schoolClass: string;
    classLabel: string;
  }) => (
    <div data-testid="class-arrival-exception-panel">
      {schoolClass} / {classLabel}
    </div>
  ),
}));

vi.mock("~/components/ui/form-modal", () => ({
  FormModal: ({
    isOpen,
    title,
    children,
    footer,
  }: {
    isOpen: boolean;
    title: string;
    children: React.ReactNode;
    footer: React.ReactNode;
  }) =>
    isOpen ? (
      <div role="dialog" aria-label={title}>
        <div>{children}</div>
        <div>{footer}</div>
      </div>
    ) : null,
}));

import { FilteredBulkArrivalModal } from "./class-bulk-arrival-modal";

function makeStudent(id: string): Student {
  return {
    id,
    name: `S ${id}`,
    first_name: "S",
    second_name: id,
    school_class: "4a",
    current_location: "class",
  } as Student;
}

describe("FilteredBulkArrivalModal day view", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockFetchBulkArrivalScheduleStatus.mockResolvedValue(0);
    mockFetchClassArrivalTimes.mockResolvedValue({
      school_class: "4a",
      times: { mon: "12:45" },
    });
  });

  it("switches to the day panel and swaps the footer", async () => {
    render(
      <FilteredBulkArrivalModal
        isOpen
        onClose={vi.fn()}
        filter={{ type: "school_class", schoolClass: "4a" }}
        filterLabel="4a"
        studentsInFilter={[makeStudent("1")]}
      />,
    );

    expect(await screen.findByLabelText("Montag")).toBeInTheDocument();
    expect(
      screen.getByRole("dialog", {
        name: "Unterrichtsschluss für Klasse 4a",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Speichern" }),
    ).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "An einem Tag abweichend" }),
    );

    expect(
      screen.getByTestId("class-arrival-exception-panel"),
    ).toHaveTextContent("4a / Klasse 4a");
    expect(
      screen.getByRole("dialog", {
        name: "Ankunftszeit an einem Tag für Klasse 4a",
      }),
    ).toBeInTheDocument();
    expect(screen.queryByLabelText("Montag")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Speichern" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Schließen" }),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Jede Woche" }));
    expect(await screen.findByLabelText("Montag")).toBeInTheDocument();
    expect(
      screen.queryByTestId("class-arrival-exception-panel"),
    ).not.toBeInTheDocument();
  });

  it("offers no day view for a group", async () => {
    render(
      <FilteredBulkArrivalModal
        isOpen
        onClose={vi.fn()}
        filter={{ type: "group", groupId: "7" }}
        filterLabel="Sonnengruppe"
        studentsInFilter={[makeStudent("1")]}
      />,
    );

    expect(await screen.findByLabelText("Montag")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "An einem Tag abweichend" }),
    ).not.toBeInTheDocument();
  });
});
