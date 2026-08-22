import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { CareWeeklyPlanModal } from "./care-weekly-plan-modal";

const toastSuccess = vi.fn();
const toastError = vi.fn();

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: toastSuccess,
    error: toastError,
  }),
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
    footer?: React.ReactNode;
  }) =>
    isOpen ? (
      <div role="dialog" aria-label={title}>
        <h2>{title}</h2>
        {children}
        <div>{footer}</div>
      </div>
    ) : null,
}));

const initialArrivalSchedules = [
  { weekday: 1, inCare: true, expected_arrival: "08:00", notes: "kommt frueh" },
  { weekday: 2, inCare: true, expected_arrival: "09:00", notes: null },
];

const initialPickupSchedules = [
  { weekday: 1, pickupTime: "15:00", notes: "Mama" },
  { weekday: 3, pickupTime: "16:00" },
];

describe("CareWeeklyPlanModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("loads existing weekly rows and submits changed schedules", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    const onClose = vi.fn();

    render(
      <CareWeeklyPlanModal
        isOpen
        onClose={onClose}
        careDaysSource="weekly_plan"
        initialArrivalSchedules={initialArrivalSchedules}
        initialPickupSchedules={initialPickupSchedules}
        onSubmit={onSubmit}
      />,
    );

    expect(
      screen.getByRole("dialog", { name: "Wochenplan bearbeiten" }),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("checkbox", { name: "Donnerstag" }));
    fireEvent.change(document.getElementById("weekly-arrival-4")!, {
      target: { value: "10:15" },
    });
    fireEvent.change(document.getElementById("weekly-pickup-4")!, {
      target: { value: "15:45" },
    });
    fireEvent.change(document.getElementById("weekly-arrival-notes-1")!, {
      target: { value: "Bitte vorne warten" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Wochenplan speichern" }),
    );

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith({
        arrivalSchedules: expect.arrayContaining([
          {
            weekday: 1,
            inCare: true,
            expected_arrival: "08:00",
            notes: "Bitte vorne warten",
          },
          { weekday: 4, inCare: true, expected_arrival: "10:15", notes: null },
        ]),
        pickupData: {
          schedules: expect.arrayContaining([
            { weekday: 1, pickupTime: "15:00", notes: "Mama" },
            { weekday: 4, pickupTime: "15:45", notes: undefined },
          ]),
        },
      });
    });
    expect(toastSuccess).toHaveBeenCalledWith("Wochenplan wurde gespeichert");
    expect(onClose).toHaveBeenCalled();
  });

  it("uses the overridden successMessage when provided", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    const onClose = vi.fn();

    render(
      <CareWeeklyPlanModal
        isOpen
        onClose={onClose}
        careDaysSource="weekly_plan"
        initialArrivalSchedules={initialArrivalSchedules}
        initialPickupSchedules={initialPickupSchedules}
        onSubmit={onSubmit}
        successMessage="Betreuungszeiten übernommen"
      />,
    );

    fireEvent.click(
      screen.getByRole("button", { name: "Wochenplan speichern" }),
    );

    await waitFor(() => {
      expect(toastSuccess).toHaveBeenCalledWith("Betreuungszeiten übernommen");
    });
    // The default wording must not appear when overridden.
    expect(toastSuccess).not.toHaveBeenCalledWith(
      "Wochenplan wurde gespeichert",
    );
    expect(onClose).toHaveBeenCalled();
  });

  it("shows validation errors for invalid times", async () => {
    render(
      <CareWeeklyPlanModal
        isOpen
        onClose={vi.fn()}
        careDaysSource="weekly_plan"
        initialArrivalSchedules={[
          { weekday: 1, inCare: true, expected_arrival: "99:99" },
        ]}
        initialPickupSchedules={[]}
        onSubmit={vi.fn()}
      />,
    );

    fireEvent.click(
      screen.getByRole("button", { name: "Wochenplan speichern" }),
    );

    expect(
      screen.getByText("Ungültige Ankunftszeit für Montag."),
    ).toBeInTheDocument();
  });

  it("shows validation errors for invalid pickup times", async () => {
    render(
      <CareWeeklyPlanModal
        isOpen
        onClose={vi.fn()}
        careDaysSource="weekly_plan"
        initialArrivalSchedules={[]}
        initialPickupSchedules={[{ weekday: 2, pickupTime: "99:99" }]}
        onSubmit={vi.fn()}
      />,
    );

    fireEvent.click(
      screen.getByRole("button", { name: "Wochenplan speichern" }),
    );

    expect(
      screen.getByText("Ungültige Abholzeit für Dienstag."),
    ).toBeInTheDocument();
  });

  it("can open and clear note fields", () => {
    render(
      <CareWeeklyPlanModal
        isOpen
        onClose={vi.fn()}
        careDaysSource="weekly_plan"
        initialArrivalSchedules={[]}
        initialPickupSchedules={[]}
        onSubmit={vi.fn()}
      />,
    );

    fireEvent.click(screen.getAllByRole("button", { name: /Notizen/i })[0]!);
    fireEvent.change(document.getElementById("weekly-pickup-notes-1")!, {
      target: { value: "Neue Notiz" },
    });
    fireEvent.change(document.getElementById("weekly-pickup-notes-1")!, {
      target: { value: " " },
    });

    expect(document.getElementById("weekly-pickup-notes-1")).toHaveValue("");
  });

  it("shows submit errors in the modal", async () => {
    const onSubmit = vi.fn().mockRejectedValue(new Error("Backend kaputt"));

    render(
      <CareWeeklyPlanModal
        isOpen
        onClose={vi.fn()}
        careDaysSource="weekly_plan"
        initialArrivalSchedules={initialArrivalSchedules}
        initialPickupSchedules={initialPickupSchedules}
        onSubmit={onSubmit}
      />,
    );

    fireEvent.click(
      screen.getByRole("button", { name: "Wochenplan speichern" }),
    );

    await waitFor(() => {
      expect(screen.getByText("Backend kaputt")).toBeInTheDocument();
    });
    expect(toastError).toHaveBeenCalledWith("Backend kaputt");
  });

  it("stores a selected care day without an own arrival time", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);

    render(
      <CareWeeklyPlanModal
        isOpen
        onClose={vi.fn()}
        careDaysSource="weekly_plan"
        initialArrivalSchedules={[]}
        initialPickupSchedules={[]}
        onSubmit={onSubmit}
      />,
    );

    const monday = screen.getByRole("checkbox", { name: "Montag" });
    expect(document.getElementById("weekly-arrival-1")).toBeDisabled();
    fireEvent.click(monday);
    expect(document.getElementById("weekly-arrival-1")).toBeEnabled();

    fireEvent.click(
      screen.getByRole("button", { name: "Wochenplan speichern" }),
    );

    await waitFor(() =>
      expect(onSubmit).toHaveBeenCalledWith({
        arrivalSchedules: [
          {
            weekday: 1,
            inCare: true,
            expected_arrival: "",
            notes: null,
          },
        ],
        pickupData: { schedules: [] },
      }),
    );
  });

  it("shows booking care days without editable checkboxes", () => {
    render(
      <CareWeeklyPlanModal
        isOpen
        onClose={vi.fn()}
        careDaysSource="bookings"
        initialArrivalSchedules={initialArrivalSchedules}
        initialPickupSchedules={[]}
        onSubmit={vi.fn()}
      />,
    );

    expect(
      screen.getByText(
        /Für ein neues Kind gibt es noch keine Buchungen\. Ankunftszeiten tragen Sie später im Wochenplan ein\./,
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("checkbox", { name: "Montag" })).toBeDisabled();
    expect(screen.getByRole("checkbox", { name: "Mittwoch" })).toBeDisabled();
    expect(document.getElementById("weekly-arrival-3")).toBeDisabled();
  });
});
