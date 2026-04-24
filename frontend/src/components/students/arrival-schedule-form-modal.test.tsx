import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import type { ArrivalScheduleFormEntry } from "~/lib/arrival-schedule-helpers";

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
        {children}
        {footer}
      </div>
    ) : null,
}));

vi.mock("~/components/ui/alert", () => ({
  Alert: ({ type, message }: { type: string; message: string }) => (
    <div data-testid="alert" data-type={type}>
      {message}
    </div>
  ),
}));

import { ArrivalScheduleFormModal } from "./arrival-schedule-form-modal";

function emptyWeek(): ArrivalScheduleFormEntry[] {
  return [1, 2, 3, 4, 5].map((w) => ({
    weekday: w,
    expected_arrival: "",
    notes: null,
  }));
}

describe("ArrivalScheduleFormModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("does not render when closed", () => {
    render(
      <ArrivalScheduleFormModal
        isOpen={false}
        onClose={vi.fn()}
        onSubmit={vi.fn()}
        initialSchedules={emptyWeek()}
      />,
    );

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("renders time + notes inputs for each weekday", () => {
    render(
      <ArrivalScheduleFormModal
        isOpen={true}
        onClose={vi.fn()}
        onSubmit={vi.fn()}
        initialSchedules={emptyWeek()}
      />,
    );

    expect(screen.getAllByLabelText("Ankunftszeit")).toHaveLength(5);
    expect(screen.getAllByLabelText("Notiz (optional)")).toHaveLength(5);
  });

  it("prefills inputs from initialSchedules", () => {
    render(
      <ArrivalScheduleFormModal
        isOpen={true}
        onClose={vi.fn()}
        onSubmit={vi.fn()}
        initialSchedules={[
          { weekday: 1, expected_arrival: "08:00", notes: "early" },
          ...emptyWeek().slice(1),
        ]}
      />,
    );

    const timeInputs = screen.getAllByLabelText(
      "Ankunftszeit",
    ) as HTMLInputElement[];
    const notesInputs = screen.getAllByLabelText(
      "Notiz (optional)",
    ) as HTMLInputElement[];
    expect(timeInputs[0]!.value).toBe("08:00");
    expect(notesInputs[0]!.value).toBe("early");
  });

  it("updates time and notes via onChange", () => {
    render(
      <ArrivalScheduleFormModal
        isOpen={true}
        onClose={vi.fn()}
        onSubmit={vi.fn()}
        initialSchedules={emptyWeek()}
      />,
    );

    const timeInputs = screen.getAllByLabelText(
      "Ankunftszeit",
    ) as HTMLInputElement[];
    const notesInputs = screen.getAllByLabelText(
      "Notiz (optional)",
    ) as HTMLInputElement[];

    fireEvent.change(timeInputs[0]!, { target: { value: "08:15" } });
    fireEvent.change(notesInputs[0]!, { target: { value: "Neue Notiz" } });

    expect(timeInputs[0]!.value).toBe("08:15");
    expect(notesInputs[0]!.value).toBe("Neue Notiz");
  });

  it("submits only filled days", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(
      <ArrivalScheduleFormModal
        isOpen={true}
        onClose={vi.fn()}
        onSubmit={onSubmit}
        initialSchedules={emptyWeek()}
      />,
    );

    fireEvent.change(screen.getAllByLabelText("Ankunftszeit")[0]!, {
      target: { value: "08:00" },
    });

    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));

    await waitFor(() => expect(onSubmit).toHaveBeenCalled());
    const args = onSubmit.mock.calls[0]![0] as ArrivalScheduleFormEntry[];
    expect(args).toHaveLength(1);
    expect(args[0]!.expected_arrival).toBe("08:00");
  });

  it("shows error when submit throws", async () => {
    const onSubmit = vi.fn().mockRejectedValue(new Error("Save failure"));

    render(
      <ArrivalScheduleFormModal
        isOpen={true}
        onClose={vi.fn()}
        onSubmit={onSubmit}
        initialSchedules={emptyWeek()}
      />,
    );

    fireEvent.change(screen.getAllByLabelText("Ankunftszeit")[0]!, {
      target: { value: "08:00" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));

    await waitFor(() =>
      expect(screen.getByText("Save failure")).toBeInTheDocument(),
    );
  });

  it("clears notes to null when user empties the field", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(
      <ArrivalScheduleFormModal
        isOpen={true}
        onClose={vi.fn()}
        onSubmit={onSubmit}
        initialSchedules={[
          { weekday: 1, expected_arrival: "08:00", notes: "prev" },
          ...emptyWeek().slice(1),
        ]}
      />,
    );

    fireEvent.change(screen.getAllByLabelText("Notiz (optional)")[0]!, {
      target: { value: "" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));

    await waitFor(() => expect(onSubmit).toHaveBeenCalled());
    const args = onSubmit.mock.calls[0]![0] as ArrivalScheduleFormEntry[];
    expect(args[0]!.notes).toBeNull();
  });

  it("calls onClose when cancel clicked", () => {
    const onClose = vi.fn();
    render(
      <ArrivalScheduleFormModal
        isOpen={true}
        onClose={onClose}
        onSubmit={vi.fn()}
        initialSchedules={emptyWeek()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));
    expect(onClose).toHaveBeenCalled();
  });

  it("rebuilds schedules when initialSchedules prop changes while open", () => {
    const { rerender } = render(
      <ArrivalScheduleFormModal
        isOpen={true}
        onClose={vi.fn()}
        onSubmit={vi.fn()}
        initialSchedules={emptyWeek()}
      />,
    );

    rerender(
      <ArrivalScheduleFormModal
        isOpen={true}
        onClose={vi.fn()}
        onSubmit={vi.fn()}
        initialSchedules={[
          { weekday: 1, expected_arrival: "10:00", notes: null },
          ...emptyWeek().slice(1),
        ]}
      />,
    );

    const timeInputs = screen.getAllByLabelText(
      "Ankunftszeit",
    ) as HTMLInputElement[];
    expect(timeInputs[0]!.value).toBe("10:00");
  });

  it("passes non-Error rejection message to state", async () => {
    const onSubmit = vi.fn().mockRejectedValue("plain string");

    render(
      <ArrivalScheduleFormModal
        isOpen={true}
        onClose={vi.fn()}
        onSubmit={onSubmit}
        initialSchedules={emptyWeek()}
      />,
    );

    fireEvent.change(screen.getAllByLabelText("Ankunftszeit")[0]!, {
      target: { value: "08:00" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));

    await waitFor(() =>
      expect(
        screen.getByText("Fehler beim Speichern des Ankunftsplans"),
      ).toBeInTheDocument(),
    );
  });
});
