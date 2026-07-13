import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { ActivityCategory } from "~/lib/activity-helpers";

const { mockGetCategories, mockCreateShiftType, mockUpdateShiftType } =
  vi.hoisted(() => ({
    mockGetCategories: vi.fn(),
    mockCreateShiftType: vi.fn(),
    mockUpdateShiftType: vi.fn(),
  }));

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({ error: vi.fn(), info: vi.fn(), warn: vi.fn() }),
}));

vi.mock("~/lib/activity-api", () => ({
  getCategories: mockGetCategories,
}));

vi.mock("~/lib/shift-type-api", () => ({
  ShiftTypeApiError: class ShiftTypeApiError extends Error {},
  shiftTypeService: {
    getShiftTypes: vi.fn(),
    createShiftType: mockCreateShiftType,
    updateShiftType: mockUpdateShiftType,
    deleteShiftType: vi.fn(),
    createDefaults: vi.fn(),
  },
}));

import { ShiftTypeManageModal } from "./shift-type-manage-modal";

function category(id: string, shiftTypeId?: string): ActivityCategory {
  return {
    id,
    name: `Kategorie ${id}`,
    shiftTypeId,
    created_at: new Date(),
    updated_at: new Date(),
  };
}

describe("ShiftTypeManageModal — category mapping refresh (#1837)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("refetches categories after a successful save so preselection is not stale", async () => {
    // First load returns the mapping-free list; after the save the server has
    // rewritten shift_type_id, so the modal must refetch to avoid a stale
    // preselection that could clear the mapping on the next edit.
    mockGetCategories
      .mockResolvedValueOnce([category("1"), category("2")])
      .mockResolvedValue([category("1", "99"), category("2")]);
    mockCreateShiftType.mockResolvedValue({
      id: "99",
      name: "Betreuung",
      color: "#83CD2D",
      description: "",
      isActive: true,
    });

    render(
      <ShiftTypeManageModal
        isOpen
        shiftTypes={[]}
        isLoading={false}
        loadError={false}
        onClose={vi.fn()}
        onChanged={vi.fn()}
      />,
    );

    // Initial categories load on open.
    await waitFor(() => expect(mockGetCategories).toHaveBeenCalledTimes(1));

    // Open the create form (empty state exposes "Eigene Schichtart").
    fireEvent.click(screen.getByRole("button", { name: "Eigene Schichtart" }));
    fireEvent.change(screen.getByLabelText("Name*"), {
      target: { value: "Betreuung" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Schichtart anlegen" }));

    await waitFor(() => expect(mockCreateShiftType).toHaveBeenCalledTimes(1));
    // The fix: categories are refetched after the write completes.
    await waitFor(() => expect(mockGetCategories).toHaveBeenCalledTimes(2));
  });
});
