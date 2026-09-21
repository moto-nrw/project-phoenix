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
    error,
    isBackdropDismissDisabled,
  }: {
    isOpen: boolean;
    title: string;
    children: React.ReactNode;
    footer?: React.ReactNode;
    error?: string | { message: string } | null;
    isBackdropDismissDisabled?: boolean;
  }) =>
    isOpen ? (
      <div
        role="dialog"
        aria-label={title}
        data-backdrop-dismiss-disabled={String(!!isBackdropDismissDisabled)}
      >
        <h2>{title}</h2>
        {error ? (
          <div role="alert">
            {typeof error === "string" ? error : error.message}
          </div>
        ) : null}
        {children}
        <div>{footer}</div>
      </div>
    ) : null,
}));

const initialArrivalSchedules = [
  { weekday: 1, inCare: true, expected_arrival: "08:00", notes: "kommt früh" },
  { weekday: 2, inCare: true, expected_arrival: "09:00", notes: null },
];

const initialPickupSchedules = [
  { weekday: 1, pickupTime: "15:00", notes: "Mama" },
  { weekday: 3, pickupTime: "16:00" },
];

function submit(): void {
  fireEvent.click(screen.getByRole("button", { name: "Übernehmen" }));
}

describe("CareWeeklyPlanModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("keeps the staged plan on a click next to the dialog (#3370)", () => {
    render(
      <CareWeeklyPlanModal
        isOpen
        onClose={vi.fn()}
        careDaysSource="weekly_plan"
        initialArrivalSchedules={initialArrivalSchedules}
        initialPickupSchedules={initialPickupSchedules}
        onSubmit={vi.fn()}
        successMessage="Betreuungszeiten übernommen"
      />,
    );

    // Das Verhalten selbst prüft form-modal.test.tsx.
    expect(
      screen.getByRole("dialog", { name: "Wochenplan festlegen" }),
    ).toHaveAttribute("data-backdrop-dismiss-disabled", "true");
  });

  it("loads existing weekly rows and hands over the changed schedules", async () => {
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

    // Beim Anlegen gibt es noch kein Objekt: der Dialog legt fest, er
    // bearbeitet nicht (bauart/no-edit-overlay).
    expect(
      screen.getByRole("dialog", { name: "Wochenplan festlegen" }),
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
    submit();

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
    expect(toastSuccess).toHaveBeenCalledWith("Betreuungszeiten übernommen");
    expect(onClose).toHaveBeenCalled();
  });

  it("shows the caller's successMessage, never a generic saved wording", async () => {
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
        successMessage="Zeiten vorgemerkt"
      />,
    );

    submit();

    await waitFor(() => {
      expect(toastSuccess).toHaveBeenCalledWith("Zeiten vorgemerkt");
    });
    expect(toastSuccess).toHaveBeenCalledTimes(1);
    expect(onClose).toHaveBeenCalled();
  });

  it("closes without handing anything over on Abbrechen", () => {
    const onSubmit = vi.fn();
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

    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));

    expect(onClose).toHaveBeenCalledTimes(1);
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("renders nothing while closed", () => {
    render(
      <CareWeeklyPlanModal
        isOpen={false}
        onClose={vi.fn()}
        careDaysSource="weekly_plan"
        initialArrivalSchedules={initialArrivalSchedules}
        initialPickupSchedules={initialPickupSchedules}
        onSubmit={vi.fn()}
        successMessage="Betreuungszeiten übernommen"
      />,
    );

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
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
        successMessage="Betreuungszeiten übernommen"
      />,
    );

    submit();

    expect(
      await screen.findByText("Ungültige Ankunftszeit für Montag."),
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
        successMessage="Betreuungszeiten übernommen"
      />,
    );

    submit();

    expect(
      await screen.findByText("Ungültige Abholzeit für Dienstag."),
    ).toBeInTheDocument();
  });

  it("drops a note that is only whitespace instead of handing it over", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);

    render(
      <CareWeeklyPlanModal
        isOpen
        onClose={vi.fn()}
        careDaysSource="weekly_plan"
        initialArrivalSchedules={[]}
        initialPickupSchedules={[{ weekday: 1, pickupTime: "15:00" }]}
        onSubmit={onSubmit}
        successMessage="Betreuungszeiten übernommen"
      />,
    );

    fireEvent.click(screen.getAllByRole("button", { name: /Notizen/i })[0]!);
    fireEvent.change(document.getElementById("weekly-pickup-notes-1")!, {
      target: { value: "Neue Notiz" },
    });
    fireEvent.change(document.getElementById("weekly-pickup-notes-1")!, {
      target: { value: " " },
    });
    submit();

    await waitFor(() =>
      expect(onSubmit).toHaveBeenCalledWith({
        arrivalSchedules: [],
        pickupData: {
          schedules: [{ weekday: 1, pickupTime: "15:00", notes: undefined }],
        },
      }),
    );
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
        successMessage="Betreuungszeiten übernommen"
      />,
    );

    submit();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Backend kaputt",
    );
    expect(toastError).not.toHaveBeenCalled();
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
        successMessage="Betreuungszeiten übernommen"
      />,
    );

    const monday = screen.getByRole("checkbox", { name: "Montag" });
    expect(document.getElementById("weekly-arrival-1")).toBeDisabled();
    fireEvent.click(monday.nextElementSibling!);
    expect(document.getElementById("weekly-arrival-1")).toBeEnabled();

    submit();

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

  it("only allows and persists pickup times on booked care days", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);

    render(
      <CareWeeklyPlanModal
        isOpen
        onClose={vi.fn()}
        careDaysSource="bookings"
        initialArrivalSchedules={initialArrivalSchedules}
        initialPickupSchedules={initialPickupSchedules}
        onSubmit={onSubmit}
        successMessage="Betreuungszeiten übernommen"
      />,
    );

    expect(
      screen.getByText(
        /Die Betreuungstage kommen aus den Buchungen\. Abholzeiten können Sie nur an gebuchten Tagen eintragen\./,
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("checkbox", { name: "Montag" })).toBeDisabled();
    expect(screen.getByRole("checkbox", { name: "Mittwoch" })).toBeDisabled();
    expect(document.getElementById("weekly-arrival-3")).toBeDisabled();
    expect(document.getElementById("weekly-pickup-1")).toBeEnabled();
    expect(document.getElementById("weekly-pickup-3")).toBeDisabled();

    submit();

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith({
        arrivalSchedules: expect.arrayContaining([
          expect.objectContaining({ weekday: 1 }),
          expect.objectContaining({ weekday: 2 }),
        ]),
        pickupData: {
          schedules: [{ weekday: 1, pickupTime: "15:00", notes: "Mama" }],
        },
      });
    });
  });
});
