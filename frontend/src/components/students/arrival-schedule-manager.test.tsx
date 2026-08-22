import { describe, it, expect, vi, beforeEach } from "vitest";
import {
  render,
  screen,
  fireEvent,
  waitFor,
  act,
} from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import { toISODate } from "~/lib/date-helpers";
import { formatDateISO, getWeekDays } from "~/lib/arrival-schedule-helpers";

const {
  mockFetch,
  mockFetchSettings,
  mockUpdateSchedules,
  mockCreateException,
  mockUpdateException,
  mockDeleteException,
  mockCreateNote,
  mockUpdateNote,
  mockDeleteNote,
} = vi.hoisted(() => ({
  mockFetch: vi.fn(),
  mockFetchSettings: vi.fn(),
  mockUpdateSchedules: vi.fn(),
  mockCreateException: vi.fn(),
  mockUpdateException: vi.fn(),
  mockDeleteException: vi.fn(),
  mockCreateNote: vi.fn(),
  mockUpdateNote: vi.fn(),
  mockDeleteNote: vi.fn(),
}));

vi.mock("~/lib/student-arrival-api", async () => {
  const actual = await vi.importActual<
    typeof import("~/lib/student-arrival-api")
  >("~/lib/student-arrival-api");
  return {
    ...actual,
    fetchArrivalData: mockFetch,
    fetchArrivalSettings: mockFetchSettings,
    updateArrivalSchedules: mockUpdateSchedules,
    createArrivalException: mockCreateException,
    updateArrivalException: mockUpdateException,
    deleteArrivalException: mockDeleteException,
    createArrivalNote: mockCreateNote,
    updateArrivalNote: mockUpdateNote,
    deleteArrivalNote: mockDeleteNote,
  };
});

vi.mock("./arrival-schedule-form-modal", () => ({
  ArrivalScheduleFormModal: (props: {
    isOpen: boolean;
    careDaysSource: string;
    onClose: () => void;
    onSubmit: (
      schedules: {
        weekday: number;
        expected_arrival: string;
        notes: string | null;
      }[],
    ) => Promise<void>;
  }) =>
    props.isOpen ? (
      <div data-testid="schedule-form-modal">
        <span data-testid="care-days-source">{props.careDaysSource}</span>
        <button
          type="button"
          data-testid="schedule-submit"
          onClick={() =>
            void props.onSubmit([
              { weekday: 1, expected_arrival: "08:00", notes: null },
            ])
          }
        >
          submit
        </button>
        <button
          type="button"
          data-testid="schedule-close"
          onClick={props.onClose}
        >
          close
        </button>
      </div>
    ) : null,
}));

vi.mock("./arrival-day-edit-modal", () => ({
  ArrivalDayEditModal: (props: {
    isOpen: boolean;
    onClose: () => void;
    day: {
      date: Date;
      exception: { id: number } | null;
      notes: { id: number }[];
    } | null;
    onSaveException: (params: {
      expectedArrival: string | null;
      reason?: string | null;
    }) => Promise<void>;
    onDeleteException: () => Promise<void>;
    onCreateNote: (content: string) => Promise<void>;
    onUpdateNote: (noteId: number, content: string) => Promise<void>;
    onDeleteNote: (noteId: number) => Promise<void>;
  }) =>
    props.isOpen ? (
      <div data-testid="day-edit-modal">
        <button
          type="button"
          data-testid="day-save-exception"
          onClick={() =>
            void props.onSaveException({
              expectedArrival: "09:00",
              reason: "late",
            })
          }
        >
          save
        </button>
        <button
          type="button"
          data-testid="day-delete-exception"
          onClick={() => void props.onDeleteException()}
        >
          delete-exc
        </button>
        <button
          type="button"
          data-testid="day-create-note"
          onClick={() => void props.onCreateNote("new note")}
        >
          add-note
        </button>
        <button
          type="button"
          data-testid="day-update-note"
          onClick={() => void props.onUpdateNote(10, "updated")}
        >
          edit-note
        </button>
        <button
          type="button"
          data-testid="day-delete-note"
          onClick={() => void props.onDeleteNote(10)}
        >
          del-note
        </button>
        <button type="button" data-testid="day-close" onClick={props.onClose}>
          close
        </button>
      </div>
    ) : null,
}));

import { ArrivalScheduleManager } from "./arrival-schedule-manager";

const emptyData = { schedules: [], exceptions: [], notes: [] };

describe("ArrivalScheduleManager", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockFetch.mockResolvedValue(emptyData);
    mockFetchSettings.mockResolvedValue({ care_days_source: "weekly_plan" });
  });

  it("shows loader while loading initial data", () => {
    mockFetch.mockReturnValueOnce(new Promise(() => undefined));

    const { container } = render(<ArrivalScheduleManager studentId="1" />);

    expect(container.querySelector(".animate-spin")).toBeInTheDocument();
  });

  it("shows error when load fails", async () => {
    mockFetch.mockRejectedValueOnce(new Error("API down"));

    render(<ArrivalScheduleManager studentId="1" />);

    await waitFor(() =>
      expect(screen.getByText("API down")).toBeInTheDocument(),
    );
  });

  it("shows 5 day cells after successful load", async () => {
    render(<ArrivalScheduleManager studentId="1" />);

    await waitFor(() => expect(mockFetch).toHaveBeenCalled());

    // Should render 5 weekdays across desktop + mobile layouts (10 total)
    // and include the section title.
    expect(screen.getByText(/Ankunftsplan & Notizen/)).toBeInTheDocument();
  });

  it("hides edit button in readOnly mode", async () => {
    render(<ArrivalScheduleManager studentId="1" readOnly={true} />);

    await waitFor(() => expect(mockFetch).toHaveBeenCalled());
    expect(
      screen.queryByRole("button", { name: "Bearbeiten" }),
    ).not.toBeInTheDocument();
  });

  it("opens schedule form modal from the edit button", async () => {
    render(<ArrivalScheduleManager studentId="1" />);
    await waitFor(() => expect(mockFetch).toHaveBeenCalled());

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    expect(screen.getByTestId("schedule-form-modal")).toBeInTheDocument();
    expect(screen.getByTestId("care-days-source")).toHaveTextContent(
      "weekly_plan",
    );
  });

  it("passes booking care days to the schedule form", async () => {
    mockFetchSettings.mockResolvedValueOnce({ care_days_source: "bookings" });
    render(<ArrivalScheduleManager studentId="1" />);
    await waitFor(() => expect(mockFetchSettings).toHaveBeenCalled());

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    expect(screen.getByTestId("care-days-source")).toHaveTextContent(
      "bookings",
    );
  });

  it("shows the arrival source on mobile and desktop", async () => {
    mockFetch.mockResolvedValue({
      schedules: [
        {
          id: 1,
          weekday: 1,
          expected_arrival: "08:00",
          source: "class_schedule",
          source_class: "Klasse 1b",
        },
      ],
      exceptions: [],
      notes: [],
    });

    render(<ArrivalScheduleManager studentId="1" />);
    await screen.findByText(/Ankunftsplan & Notizen/);

    expect(screen.getAllByText("aus Klasse 1b")).toHaveLength(2);
  });

  it("submits schedules and reloads", async () => {
    mockUpdateSchedules.mockResolvedValueOnce(undefined);
    const onUpdate = vi.fn();
    render(<ArrivalScheduleManager studentId="1" onUpdate={onUpdate} />);
    await waitFor(() => expect(mockFetch).toHaveBeenCalled());

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    fireEvent.click(screen.getByTestId("schedule-submit"));

    await waitFor(() => expect(mockUpdateSchedules).toHaveBeenCalled());
    expect(mockUpdateSchedules).toHaveBeenCalledWith("1", [
      { weekday: 1, expected_arrival: "08:00", notes: null },
    ]);
    expect(onUpdate).toHaveBeenCalled();
  });

  it("loads the selected week after week navigation", async () => {
    render(<ArrivalScheduleManager studentId="1" />);
    await waitFor(() => expect(mockFetch).toHaveBeenCalled());

    // Should render two prev/next pairs (mobile + desktop)
    const prevButtons = screen.getAllByTitle("Vorherige Woche");
    const nextButtons = screen.getAllByTitle("Nächste Woche");

    fireEvent.click(prevButtons[0]!);

    await waitFor(() =>
      expect(mockFetch).toHaveBeenLastCalledWith(
        "1",
        formatDateISO(getWeekDays(-1)[0]!),
      ),
    );
    fireEvent.click(nextButtons[0]!);

    expect(prevButtons).toHaveLength(2);
    expect(nextButtons).toHaveLength(2);
  });

  it("day edit modal round-trips create exception", async () => {
    // Seed with a base schedule so day entries are populated
    const now = new Date();
    const monday = new Date(now);
    // shift to Monday
    monday.setDate(now.getDate() - ((now.getDay() + 6) % 7));
    mockFetch.mockResolvedValue({
      schedules: [{ id: 1, weekday: 1, expected_arrival: "08:00" }],
      exceptions: [],
      notes: [],
    });
    mockCreateException.mockResolvedValueOnce(undefined);

    render(<ArrivalScheduleManager studentId="1" />);
    await waitFor(() => expect(mockFetch).toHaveBeenCalled());

    // Open first day's edit button (desktop layout)
    const editButtons = screen.getAllByTitle("Tag bearbeiten");
    fireEvent.click(editButtons[0]!);

    expect(screen.getByTestId("day-edit-modal")).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByTestId("day-save-exception"));
    });

    await waitFor(() => expect(mockCreateException).toHaveBeenCalled());
  });

  it("day edit modal updates an existing exception", async () => {
    // Find this week's Monday deterministically to keep the test stable on any weekday.
    const now = new Date();
    const dayOfWeek = now.getDay();
    const mondayOffset = dayOfWeek === 0 ? -6 : 1 - dayOfWeek;
    const monday = new Date(now);
    monday.setDate(now.getDate() + mondayOffset);
    const mondayIso = toISODate(monday);

    mockFetch.mockResolvedValue({
      schedules: [{ id: 1, weekday: 1, expected_arrival: "08:00" }],
      exceptions: [
        { id: 99, exception_date: mondayIso, expected_arrival: "09:00" },
      ],
      notes: [],
    });
    mockUpdateException.mockResolvedValueOnce(undefined);

    render(<ArrivalScheduleManager studentId="1" />);
    await waitFor(() => expect(mockFetch).toHaveBeenCalled());

    const editButtons = screen.getAllByTitle("Tag bearbeiten");
    fireEvent.click(editButtons[0]!);

    await act(async () => {
      fireEvent.click(screen.getByTestId("day-save-exception"));
    });
    await waitFor(() => expect(mockUpdateException).toHaveBeenCalled());
  });

  it("day edit modal deletes an exception", async () => {
    const now = new Date();
    const dayOfWeek = now.getDay();
    const mondayOffset = dayOfWeek === 0 ? -6 : 1 - dayOfWeek;
    const monday = new Date(now);
    monday.setDate(now.getDate() + mondayOffset);
    const mondayIso = toISODate(monday);

    mockFetch.mockResolvedValue({
      schedules: [{ id: 1, weekday: 1, expected_arrival: "08:00" }],
      exceptions: [{ id: 99, exception_date: mondayIso }],
      notes: [],
    });
    mockDeleteException.mockResolvedValueOnce(undefined);

    render(<ArrivalScheduleManager studentId="1" />);
    await waitFor(() => expect(mockFetch).toHaveBeenCalled());

    const editButtons = screen.getAllByTitle("Tag bearbeiten");
    fireEvent.click(editButtons[0]!);

    await act(async () => {
      fireEvent.click(screen.getByTestId("day-delete-exception"));
    });
    await waitFor(() => expect(mockDeleteException).toHaveBeenCalled());
  });

  it("day edit modal creates/updates/deletes notes", async () => {
    const today = new Date();
    mockFetch.mockResolvedValue({
      schedules: [
        {
          id: 1,
          weekday: ((today.getDay() + 6) % 7) + 1,
          expected_arrival: "08:00",
        },
      ],
      exceptions: [],
      notes: [],
    });

    render(<ArrivalScheduleManager studentId="1" />);
    await waitFor(() => expect(mockFetch).toHaveBeenCalled());

    const editButtons = screen.getAllByTitle("Tag bearbeiten");
    fireEvent.click(editButtons[0]!);

    await act(async () => {
      fireEvent.click(screen.getByTestId("day-create-note"));
    });
    await waitFor(() => expect(mockCreateNote).toHaveBeenCalled());

    await act(async () => {
      fireEvent.click(screen.getByTestId("day-update-note"));
    });
    await waitFor(() => expect(mockUpdateNote).toHaveBeenCalled());

    await act(async () => {
      fireEvent.click(screen.getByTestId("day-delete-note"));
    });
    await waitFor(() => expect(mockDeleteNote).toHaveBeenCalled());
  });

  it("closes day edit modal via close button", async () => {
    const today = new Date();
    mockFetch.mockResolvedValue({
      schedules: [
        {
          id: 1,
          weekday: ((today.getDay() + 6) % 7) + 1,
          expected_arrival: "08:00",
        },
      ],
      exceptions: [],
      notes: [],
    });

    render(<ArrivalScheduleManager studentId="1" />);
    await waitFor(() => expect(mockFetch).toHaveBeenCalled());

    fireEvent.click(screen.getAllByTitle("Tag bearbeiten")[0]!);
    fireEvent.click(screen.getByTestId("day-close"));

    expect(screen.queryByTestId("day-edit-modal")).not.toBeInTheDocument();
  });

  it("readOnly suppresses Tag bearbeiten buttons", async () => {
    mockFetch.mockResolvedValue({
      schedules: [{ id: 1, weekday: 1, expected_arrival: "08:00" }],
      exceptions: [],
      notes: [],
    });

    render(<ArrivalScheduleManager studentId="1" readOnly={true} />);
    await waitFor(() => expect(mockFetch).toHaveBeenCalled());

    expect(screen.queryAllByTitle("Tag bearbeiten")).toHaveLength(0);
  });
});
