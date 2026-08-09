import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { SelectionBulkPickupModal } from "./selection-bulk-pickup-modal";

const { bulkUpsert } = vi.hoisted(() => ({ bulkUpsert: vi.fn() }));
vi.mock("~/lib/pickup-schedule-api", () => ({
  bulkUpsertPickupSchedules: bulkUpsert,
}));
vi.mock("~/contexts/ToastContext", () => ({
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
      ),
    );
    expect(onClose).toHaveBeenCalledOnce();
  });
});
