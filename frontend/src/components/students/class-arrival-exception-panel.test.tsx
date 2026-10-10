import { describe, it, expect, vi, beforeEach } from "vitest";
import { ApiError } from "~/lib/api-error";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import { toISODate } from "~/lib/date-helpers";

const {
  mockFetchExceptions,
  mockUpsert,
  mockDelete,
  mockGetWeek,
  mockGetTemplates,
  mockToastSuccess,
  mockToastError,
} = vi.hoisted(() => ({
  mockFetchExceptions: vi.fn(),
  mockUpsert: vi.fn(),
  mockDelete: vi.fn(),
  mockGetWeek: vi.fn(),
  mockGetTemplates: vi.fn(),
  mockToastSuccess: vi.fn(),
  mockToastError: vi.fn(),
}));

vi.mock("~/lib/student-arrival-api", async () => {
  const actual = await vi.importActual<
    typeof import("~/lib/student-arrival-api")
  >("~/lib/student-arrival-api");
  return {
    ...actual,
    fetchClassArrivalExceptions: mockFetchExceptions,
    upsertClassArrivalException: mockUpsert,
    deleteClassArrivalException: mockDelete,
  };
});

vi.mock("~/lib/timetable-api", () => ({
  timetableService: { getWeek: mockGetWeek, getTemplates: mockGetTemplates },
}));

vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({
    success: mockToastSuccess,
    error: mockToastError,
    info: vi.fn(),
  }),
}));

// The kit DatePicker is a portal calendar; a plain date input keeps the test
// on the panel's own behaviour.
vi.mock("~/components/ui/date-picker", () => ({
  DatePicker: ({
    id,
    value,
    onChange,
    maxDate,
    disabledDay,
  }: {
    id?: string;
    value: Date | null;
    onChange: (date: Date | null) => void;
    maxDate?: Date;
    disabledDay?: (date: Date) => boolean;
  }) => (
    <input
      id={id}
      type="date"
      value={value ? toISODate(value) : ""}
      data-max-date={maxDate ? toISODate(maxDate) : undefined}
      data-saturday-disabled={String(
        disabledDay?.(new Date("2026-09-05T00:00:00")) ?? false,
      )}
      onChange={(event) =>
        onChange(
          event.target.value
            ? new Date(`${event.target.value}T00:00:00`)
            : null,
        )
      }
    />
  ),
}));

import { ClassArrivalExceptionPanel } from "./class-arrival-exception-panel";

const savedException = {
  school_class: "4a",
  date: "2099-03-02",
  arrival_time: "12:45",
  reason: "Unterricht fällt aus",
  created_at: "2099-03-01T10:00:00Z",
};

describe("ClassArrivalExceptionPanel", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockFetchExceptions.mockResolvedValue({
      school_class: "4a",
      can_edit: true,
      exceptions: [],
    });
    mockGetWeek.mockResolvedValue({
      from: "2099-03-02",
      to: "2099-03-02",
      instances: [],
    });
    mockGetTemplates.mockResolvedValue({ templates: [] });
  });

  it("shows the empty list and the form for editors", async () => {
    render(
      <ClassArrivalExceptionPanel schoolClass="4a" classLabel="Klasse 4a" />,
    );

    expect(
      await screen.findByText("Keine Abweichung eingetragen"),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Kommt um")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Unterricht fällt aus" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Ändern kann das die Koordination."),
    ).not.toBeInTheDocument();
  });

  it("hides the form and names who may change it for non-editors", async () => {
    mockFetchExceptions.mockResolvedValue({
      school_class: "4a",
      can_edit: false,
      exceptions: [savedException],
    });

    render(
      <ClassArrivalExceptionPanel schoolClass="4a" classLabel="Klasse 4a" />,
    );

    expect(
      await screen.findByText("Ändern kann das die Koordination."),
    ).toBeInTheDocument();
    expect(screen.getByText("Unterricht fällt aus")).toBeInTheDocument();
    expect(screen.queryByLabelText("Kommt um")).not.toBeInTheDocument();
    // Ohne Bearbeitungsrecht gibt es kein Zeilenmenü (Bauart 1 Regel 4).
    expect(
      screen.queryByRole("button", { name: /^Aktionen für/ }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Entfernen" }),
    ).not.toBeInTheDocument();
  });

  it("saves date, time and reason for the class", async () => {
    mockUpsert.mockResolvedValue(savedException);
    const onChanged = vi.fn();

    render(
      <ClassArrivalExceptionPanel
        schoolClass="4a"
        classLabel="Klasse 4a"
        onChanged={onChanged}
      />,
    );
    await screen.findByLabelText("Kommt um");

    fireEvent.change(screen.getByLabelText("Datum"), {
      target: { value: "2099-03-02" },
    });
    fireEvent.change(screen.getByLabelText("Kommt um"), {
      target: { value: "12:45" },
    });
    fireEvent.change(screen.getByLabelText("Grund (optional)"), {
      target: { value: " Wandertag " },
    });
    fireEvent.click(screen.getByRole("button", { name: "Tag speichern" }));

    await waitFor(() => {
      expect(mockUpsert).toHaveBeenCalledWith("4a", "2099-03-02", {
        arrival_time: "12:45",
        reason: "Wandertag",
      });
    });
    expect(mockToastSuccess).toHaveBeenCalledWith(
      "Klasse 4a kommt am 02.03.2099 um 12:45 Uhr.",
    );
    expect(onChanged).toHaveBeenCalled();
    expect(mockFetchExceptions).toHaveBeenCalledTimes(2);
  });

  it("keeps the save button disabled without date and time", async () => {
    render(
      <ClassArrivalExceptionPanel schoolClass="4a" classLabel="Klasse 4a" />,
    );
    const save = await screen.findByRole("button", { name: "Tag speichern" });
    expect(save).toBeDisabled();

    fireEvent.change(screen.getByLabelText("Kommt um"), {
      target: { value: "12:45" },
    });
    expect(save).toBeDisabled();
  });

  it("limits dates to weekdays in the loaded list horizon", async () => {
    render(
      <ClassArrivalExceptionPanel schoolClass="4a" classLabel="Klasse 4a" />,
    );

    const picker = await screen.findByLabelText("Datum");
    expect(picker).toHaveAttribute("id", "class-arrival-exception-date");
    const expectedMax = new Date();
    expectedMax.setHours(0, 0, 0, 0);
    expectedMax.setDate(expectedMax.getDate() + 60);
    expect(picker).toHaveAttribute("data-max-date", toISODate(expectedMax));
    expect(picker).toHaveAttribute("data-saturday-disabled", "true");
  });

  it("opens weekend dates when the school runs the weekend on Friday's plan (#3921)", async () => {
    render(
      <ClassArrivalExceptionPanel
        schoolClass="4a"
        classLabel="Klasse 4a"
        weekendOpen
      />,
    );

    const picker = await screen.findByLabelText("Datum");
    expect(picker).toHaveAttribute("data-saturday-disabled", "false");
  });

  it("presets the earliest block start and the reason for Unterrichtsausfall", async () => {
    mockGetWeek.mockResolvedValue({
      from: "2099-03-02",
      to: "2099-03-02",
      instances: [
        {
          date: "2099-03-02",
          startTime: "12:45",
          status: "planned",
          activityGroupId: "selected-class",
        },
        {
          date: "2099-03-02",
          startTime: "11:45",
          status: "planned",
          activityGroupId: "selected-class",
        },
        { date: "2099-03-02", startTime: "08:00", status: "cancelled" },
      ],
    });
    mockGetTemplates.mockResolvedValue({
      templates: [{ id: "selected-class", targetSchoolClass: "4a" }],
    });

    render(
      <ClassArrivalExceptionPanel schoolClass="4a" classLabel="Klasse 4a" />,
    );
    await screen.findByLabelText("Kommt um");

    fireEvent.change(screen.getByLabelText("Datum"), {
      target: { value: "2099-03-02" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Unterricht fällt aus" }),
    );

    await waitFor(() => {
      expect(screen.getByLabelText("Kommt um")).toHaveValue("11:45");
    });
    expect(screen.getByLabelText("Grund (optional)")).toHaveValue(
      "Unterricht fällt aus",
    );
    expect(mockGetWeek).toHaveBeenCalledWith("2099-03-02", "2099-03-02");
  });

  it("uses only blocks that apply to the selected normalized class for the preset", async () => {
    mockGetWeek.mockResolvedValue({
      from: "2099-03-02",
      to: "2099-03-02",
      instances: [
        {
          date: "2099-03-02",
          startTime: "08:00",
          status: "planned",
          activityGroupId: "other-class",
        },
        {
          date: "2099-03-02",
          startTime: "11:45",
          status: "planned",
          activityGroupId: "selected-class",
        },
      ],
    });
    mockGetTemplates.mockResolvedValue({
      templates: [
        { id: "other-class", targetSchoolClass: "4b" },
        { id: "selected-class", targetSchoolClass: " 4A " },
      ],
    });

    render(
      <ClassArrivalExceptionPanel schoolClass="4a" classLabel="Klasse 4a" />,
    );
    await screen.findByLabelText("Kommt um");

    fireEvent.change(screen.getByLabelText("Datum"), {
      target: { value: "2099-03-02" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Unterricht fällt aus" }),
    );

    await waitFor(() => {
      expect(screen.getByLabelText("Kommt um")).toHaveValue("11:45");
    });
  });

  it("ignores blocks whose activity group is no longer in the templates response", async () => {
    mockGetWeek.mockResolvedValue({
      from: "2099-03-02",
      to: "2099-03-02",
      instances: [
        {
          date: "2099-03-02",
          startTime: "08:00",
          status: "planned",
          activityGroupId: "missing-template",
        },
        {
          date: "2099-03-02",
          startTime: "11:45",
          status: "planned",
          activityGroupId: "selected-class",
        },
      ],
    });
    mockGetTemplates.mockResolvedValue({
      templates: [{ id: "selected-class", targetSchoolClass: "4a" }],
    });

    render(
      <ClassArrivalExceptionPanel schoolClass="4a" classLabel="Klasse 4a" />,
    );
    await screen.findByLabelText("Kommt um");

    fireEvent.change(screen.getByLabelText("Datum"), {
      target: { value: "2099-03-02" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Unterricht fällt aus" }),
    );

    await waitFor(() => {
      expect(screen.getByLabelText("Kommt um")).toHaveValue("11:45");
    });
  });

  it("uses only blocks with an explicit matching class", async () => {
    mockGetWeek.mockResolvedValue({
      from: "2099-03-02",
      to: "2099-03-02",
      instances: [
        { date: "2099-03-02", startTime: "08:00", status: "planned" },
        {
          date: "2099-03-02",
          startTime: "09:00",
          status: "planned",
          activityGroupId: "without-class",
        },
        {
          date: "2099-03-02",
          startTime: "11:45",
          status: "planned",
          activityGroupId: "selected-class",
        },
      ],
    });
    mockGetTemplates.mockResolvedValue({
      templates: [
        { id: "without-class" },
        { id: "selected-class", targetSchoolClass: "4a" },
      ],
    });

    render(
      <ClassArrivalExceptionPanel schoolClass="4a" classLabel="Klasse 4a" />,
    );
    await screen.findByLabelText("Kommt um");

    fireEvent.change(screen.getByLabelText("Datum"), {
      target: { value: "2099-03-02" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Unterricht fällt aus" }),
    );

    await waitFor(() => {
      expect(screen.getByLabelText("Kommt um")).toHaveValue("11:45");
    });
  });

  it("asks for a time when the day has no block", async () => {
    render(
      <ClassArrivalExceptionPanel schoolClass="4a" classLabel="Klasse 4a" />,
    );
    await screen.findByLabelText("Kommt um");

    fireEvent.change(screen.getByLabelText("Datum"), {
      target: { value: "2099-03-02" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Unterricht fällt aus" }),
    );

    expect(
      await screen.findByText(
        "Für diesen Tag ist kein Betreuungsblock geplant. Bitte tragen Sie die Uhrzeit selbst ein.",
      ),
    ).toBeInTheDocument();
    const time = screen.getByLabelText("Kommt um");
    expect(time).toHaveValue("");
    expect(time).toHaveAttribute("aria-invalid", "true");
    await waitFor(() => expect(time).toHaveFocus());
    expect(mockToastError).not.toHaveBeenCalled();
  });

  it("removes an entered day", async () => {
    mockFetchExceptions.mockResolvedValue({
      school_class: "4a",
      can_edit: true,
      exceptions: [savedException],
    });
    mockDelete.mockResolvedValue(undefined);

    render(
      <ClassArrivalExceptionPanel schoolClass="4a" classLabel="Klasse 4a" />,
    );

    // #3109: the row button opens the ConfirmDeleteModal; the day is only
    // removed after the two-step confirmation inside the dialog.
    fireEvent.click(
      await screen.findByRole("button", { name: /^Aktionen für/ }),
    );
    fireEvent.click(screen.getByRole("menuitem", { name: "Entfernen" }));
    expect(mockDelete).not.toHaveBeenCalled();
    expect(
      screen.getByRole("heading", { name: "Abweichung entfernen?" }),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Ja, entfernen" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Endgültig entfernen" }),
    );

    await waitFor(() => {
      expect(mockDelete).toHaveBeenCalledWith("4a", "2099-03-02");
    });
    expect(mockToastSuccess).toHaveBeenCalledWith(
      "Die Abweichung am 02.03.2099 ist entfernt.",
    );
    await waitFor(() => {
      expect(
        screen.queryByRole("heading", { name: "Abweichung entfernen?" }),
      ).not.toBeInTheDocument();
    });
  });

  it("asks its parent dialog to suspend while confirming a removal", async () => {
    const onConfirmationVisibilityChange = vi.fn();
    mockFetchExceptions.mockResolvedValue({
      school_class: "4a",
      can_edit: true,
      exceptions: [savedException],
    });

    render(
      <ClassArrivalExceptionPanel
        schoolClass="4a"
        classLabel="Klasse 4a"
        onConfirmationVisibilityChange={onConfirmationVisibilityChange}
      />,
    );

    fireEvent.click(
      await screen.findByRole("button", { name: /^Aktionen für/ }),
    );
    fireEvent.click(screen.getByRole("menuitem", { name: "Entfernen" }));
    expect(onConfirmationVisibilityChange).toHaveBeenLastCalledWith(true);

    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));
    expect(onConfirmationVisibilityChange).toHaveBeenLastCalledWith(false);
  });

  it("keeps a failed removal inside the dialog", async () => {
    mockFetchExceptions.mockResolvedValue({
      school_class: "4a",
      can_edit: true,
      exceptions: [savedException],
    });
    mockDelete.mockRejectedValue(
      new ApiError("boom", 409, {
        code: "classday.arrival_exception_class_not_found",
      }),
    );

    render(
      <ClassArrivalExceptionPanel schoolClass="4a" classLabel="Klasse 4a" />,
    );

    fireEvent.click(
      await screen.findByRole("button", { name: /^Aktionen für/ }),
    );
    fireEvent.click(screen.getByRole("menuitem", { name: "Entfernen" }));
    fireEvent.click(screen.getByRole("button", { name: "Ja, entfernen" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Endgültig entfernen" }),
    );

    expect(
      await screen.findByText(
        "Die Abweichung wurde nicht gefunden. Bitte laden Sie die Seite neu.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("boom")).not.toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Abweichung entfernen?" }),
    ).toBeInTheDocument();
    expect(mockToastError).not.toHaveBeenCalled();
  });

  it("shows a failed save in the form with the catalog text and retries", async () => {
    mockFetchExceptions.mockResolvedValue({
      school_class: "4a",
      can_edit: true,
      exceptions: [],
    });
    mockUpsert
      .mockRejectedValueOnce(
        new ApiError("boom", 500, {
          code: "general.server",
          instance: "req-class",
        }),
      )
      .mockResolvedValueOnce(savedException);

    render(
      <ClassArrivalExceptionPanel schoolClass="4a" classLabel="Klasse 4a" />,
    );
    await screen.findByLabelText("Kommt um");
    fireEvent.change(screen.getByLabelText("Datum"), {
      target: { value: "2099-03-02" },
    });
    fireEvent.change(screen.getByLabelText("Kommt um"), {
      target: { value: "12:45" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Tag speichern" }));

    expect(
      await screen.findByText(
        "Die Abweichung konnte nicht bearbeitet werden. Bitte versuchen Sie es später erneut.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText(/req-class/)).toBeInTheDocument();
    expect(mockToastError).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    await waitFor(() => expect(mockUpsert).toHaveBeenCalledTimes(2));
  });

  it("shows a failed load in place with a retry", async () => {
    mockFetchExceptions
      .mockRejectedValueOnce(
        new ApiError("offline", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValueOnce({
        school_class: "4a",
        can_edit: true,
        exceptions: [],
      });

    render(
      <ClassArrivalExceptionPanel schoolClass="4a" classLabel="Klasse 4a" />,
    );

    expect(
      await screen.findByText(
        "Die Liste der Abweichungen ist gerade nicht erreichbar. Bitte versuchen Sie es erneut.",
      ),
    ).toBeInTheDocument();
    expect(mockToastError).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    expect(await screen.findByLabelText("Kommt um")).toBeInTheDocument();
  });
});
