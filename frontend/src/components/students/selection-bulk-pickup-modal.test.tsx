import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "~/lib/api-error";
import { releaseFakeTimers } from "~/test/clock";
import { SelectionBulkPickupModal } from "./selection-bulk-pickup-modal";

const { bulkUpsert } = vi.hoisted(() => ({ bulkUpsert: vi.fn() }));
vi.mock("~/lib/pickup-schedule-api", () => ({
  bulkUpsertPickupSchedules: bulkUpsert,
}));
vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({ success: vi.fn(), error: vi.fn() }),
}));

describe("SelectionBulkPickupModal", () => {
  beforeEach(() => vi.clearAllMocks());

  it("sends only filled weekdays for the selected student IDs", async () => {
    bulkUpsert.mockResolvedValue({ students_affected: 2 });
    const onClose = vi.fn();
    render(
      <SelectionBulkPickupModal
        isOpen
        onClose={onClose}
        studentIds={["4", "9"]}
      />,
    );

    fireEvent.change(screen.getByLabelText("Dienstag"), {
      target: { value: "16:10" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Für 2 setzen" }));

    await waitFor(() =>
      expect(bulkUpsert).toHaveBeenCalledWith(
        ["4", "9"],
        [{ weekday: 2, pickup_time: "16:10" }],
        false,
      ),
    );
    expect(onClose).toHaveBeenCalledOnce();
  });

  it("requires an explicit exception confirmation when the setting is enabled", async () => {
    bulkUpsert
      .mockRejectedValueOnce(
        Object.assign(new Error("Bestätigung erforderlich"), {
          code: "pickup.bulk_exception_confirmation_required",
        }),
      )
      .mockResolvedValueOnce({ students_affected: 2 });
    const onClose = vi.fn();
    render(
      <SelectionBulkPickupModal
        isOpen
        onClose={onClose}
        studentIds={["4", "9"]}
      />,
    );

    fireEvent.change(screen.getByLabelText("Dienstag"), {
      target: { value: "16:10" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Für 2 setzen" }));

    expect(
      await screen.findByText(
        /Die Zeiten werden für alle ausgewählten Kinder als dauerhafte/,
      ),
    ).toBeInTheDocument();
    const save = screen.getByRole("button", { name: "Ausnahmen speichern" });
    expect(save).toHaveAttribute("aria-disabled", "true");

    fireEvent.click(
      screen.getByRole("checkbox", {
        name: "Ich bestätige die dauerhaften Ausnahmen.",
      }),
    );
    fireEvent.click(save);

    await waitFor(() =>
      expect(bulkUpsert).toHaveBeenLastCalledWith(
        ["4", "9"],
        [{ weekday: 2, pickup_time: "16:10" }],
        true,
      ),
    );
    expect(onClose).toHaveBeenCalledOnce();
  });

  it("shows a failed save in the dialog with the catalog text and retries", async () => {
    bulkUpsert
      .mockRejectedValueOnce(
        new ApiError("boom", 500, { code: "general.server" }),
      )
      .mockResolvedValueOnce({ students_affected: 2 });
    const onClose = vi.fn();
    render(
      <SelectionBulkPickupModal
        isOpen
        onClose={onClose}
        studentIds={["4", "9"]}
      />,
    );

    fireEvent.change(screen.getByLabelText("Dienstag"), {
      target: { value: "16:10" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Für 2 setzen" }));

    expect(
      await screen.findByText(
        "Die Gehzeit konnte nicht bearbeitet werden. Bitte versuchen Sie es später erneut.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("boom")).not.toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    await waitFor(() => expect(onClose).toHaveBeenCalledOnce());
    expect(bulkUpsert).toHaveBeenCalledTimes(2);
  });

  it("keeps every close path locked while the save runs (#6503)", async () => {
    let finish!: (value: { students_affected: number }) => void;
    bulkUpsert.mockReturnValueOnce(
      new Promise((resolve) => {
        finish = resolve;
      }),
    );
    const onClose = vi.fn();
    render(
      <SelectionBulkPickupModal
        isOpen
        onClose={onClose}
        studentIds={["4", "9"]}
      />,
    );
    fireEvent.change(screen.getByLabelText("Dienstag"), {
      target: { value: "16:10" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Für 2 setzen" }));

    // Anker: das Speichern läuft, der Dialog ist offen.
    expect(
      await screen.findByRole("button", { name: "Speichern..." }),
    ).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Modal schließen" }),
    ).toBeDisabled();
    // Escape schließt sonst nach der Ausblend-Animation (250 ms).
    vi.useFakeTimers();
    fireEvent.keyDown(document, { key: "Escape" });
    vi.advanceTimersByTime(1000);
    releaseFakeTimers();
    expect(onClose).not.toHaveBeenCalled();

    finish({ students_affected: 2 });
    await waitFor(() => expect(onClose).toHaveBeenCalledOnce());
  });

  it("does not close the next dialog when a save finishes after unmount (#6503)", async () => {
    let finish!: (value: { students_affected: number }) => void;
    bulkUpsert.mockReturnValueOnce(
      new Promise((resolve) => {
        finish = resolve;
      }),
    );
    const onClose = vi.fn();
    const onSuccess = vi.fn();
    const { unmount } = render(
      <SelectionBulkPickupModal
        isOpen
        onClose={onClose}
        onSuccess={onSuccess}
        studentIds={["4", "9"]}
      />,
    );
    fireEvent.change(screen.getByLabelText("Dienstag"), {
      target: { value: "16:10" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Für 2 setzen" }));
    expect(
      await screen.findByRole("button", { name: "Speichern..." }),
    ).toBeDisabled();

    unmount();
    finish({ students_affected: 2 });

    // Anker: die Speicherung hat stattgefunden, die Liste lädt neu …
    await waitFor(() => expect(onSuccess).toHaveBeenCalledOnce());
    // … aber das Schließen gehört dem Dialog von damals, nicht dem nächsten.
    expect(onClose).not.toHaveBeenCalled();
  });
});
