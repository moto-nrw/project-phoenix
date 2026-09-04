import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import { toISODate } from "~/lib/date-helpers";

// Der Dialog der Klassenansicht (#2970) nutzt den geteilten Baustein mit den
// /school-Routen und meldet im Dialog statt per Toast.

const { mockList, mockUpsert, mockRemove, mockBlockStart } = vi.hoisted(() => ({
  mockList: vi.fn(),
  mockUpsert: vi.fn(),
  mockRemove: vi.fn(),
  mockBlockStart: vi.fn(),
}));

vi.mock("~/lib/school-class-day-api", () => ({
  fetchClassArrivalExceptionsSchool: mockList,
  upsertClassArrivalExceptionSchool: mockUpsert,
  deleteClassArrivalExceptionSchool: mockRemove,
  fetchClassBlockStartSchool: mockBlockStart,
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

// Der Kit-DatePicker ist ein Portal-Kalender; ein einfaches Datumsfeld hält
// den Test beim Verhalten des Dialogs.
vi.mock("~/components/ui/date-picker", () => ({
  DatePicker: ({
    id,
    value,
    onChange,
  }: {
    id?: string;
    value: Date | null;
    onChange: (date: Date | null) => void;
  }) => (
    <input
      id={id}
      type="date"
      value={value ? toISODate(value) : ""}
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

import { ClassArrivalExceptionDialog } from "./class-arrival-exception-dialog";

const savedException = {
  school_class: "4a",
  date: "2099-03-02",
  arrival_time: "12:45",
  reason: "Unterricht fällt aus",
  created_at: "2099-03-01T10:00:00Z",
  origin: "school" as const,
};

describe("ClassArrivalExceptionDialog", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockList.mockResolvedValue({
      school_class: "4a",
      can_edit: true,
      exceptions: [],
    });
  });

  it("names the effect and prefills the shown day", async () => {
    render(
      <ClassArrivalExceptionDialog
        isOpen
        onClose={vi.fn()}
        schoolClass="4a"
        defaultDate={new Date("2099-03-02T00:00:00")}
      />,
    );

    expect(
      screen.getByRole("dialog", {
        name: "Ankunftszeit an einem Tag für Klasse 4a",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Die OGS sieht die neue Zeit sofort. Sie gilt nur für Kinder mit Betreuung an diesem Tag.",
      ),
    ).toBeInTheDocument();
    expect(await screen.findByLabelText("Datum")).toHaveValue("2099-03-02");
    expect(mockList).toHaveBeenCalledWith("4a");
  });

  it("saves through the school routes and confirms inside the dialog", async () => {
    mockUpsert.mockResolvedValue(savedException);
    const onChanged = vi.fn();

    render(
      <ClassArrivalExceptionDialog
        isOpen
        onClose={vi.fn()}
        schoolClass="4a"
        defaultDate={new Date("2099-03-02T00:00:00")}
        onChanged={onChanged}
      />,
    );
    await screen.findByLabelText("Kommt um");

    fireEvent.change(screen.getByLabelText("Kommt um"), {
      target: { value: "12:45" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Tag speichern" }));

    await waitFor(() => {
      expect(mockUpsert).toHaveBeenCalledWith("4a", "2099-03-02", {
        arrival_time: "12:45",
        reason: null,
      });
    });
    expect(
      await screen.findByText("Klasse 4a kommt am 02.03.2099 um 12:45 Uhr"),
    ).toBeInTheDocument();
    expect(onChanged).toHaveBeenCalled();
  });

  it("presets the block start from the school route", async () => {
    mockBlockStart.mockResolvedValue("12:45");

    render(
      <ClassArrivalExceptionDialog
        isOpen
        onClose={vi.fn()}
        schoolClass="4a"
        defaultDate={new Date("2099-03-02T00:00:00")}
      />,
    );
    await screen.findByLabelText("Kommt um");

    fireEvent.click(
      screen.getByRole("button", { name: "Unterricht fällt aus" }),
    );

    await waitFor(() => {
      expect(screen.getByLabelText("Kommt um")).toHaveValue("12:45");
    });
    expect(mockBlockStart).toHaveBeenCalledWith("4a", "2099-03-02");
    expect(screen.getByLabelText("Grund (optional)")).toHaveValue(
      "Unterricht fällt aus",
    );
  });

  it("lists OGS entries with their origin and removes them", async () => {
    mockList.mockResolvedValue({
      school_class: "4a",
      can_edit: true,
      exceptions: [{ ...savedException, origin: "ogs" }],
    });
    mockRemove.mockResolvedValue(undefined);

    render(
      <ClassArrivalExceptionDialog
        isOpen
        onClose={vi.fn()}
        schoolClass="4a"
        defaultDate={null}
      />,
    );

    expect(
      await screen.findByText("Eingetragen von der OGS"),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Entfernen" }));

    await waitFor(() => {
      expect(mockRemove).toHaveBeenCalledWith("4a", "2099-03-02");
    });
    expect(
      await screen.findByText("Abweichung am 02.03.2099 entfernt"),
    ).toBeInTheDocument();
  });
});
