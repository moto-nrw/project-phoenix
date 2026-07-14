import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { ActivityCategory } from "~/lib/activity-helpers";
import type { ShiftType } from "~/lib/shift-type-helpers";

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

function shiftType(id: string, name: string): ShiftType {
  return { id, name, color: "#83CD2D", description: "", isActive: true };
}

function renderModal(shiftTypes: ShiftType[] = []) {
  return render(
    <ShiftTypeManageModal
      isOpen
      shiftTypes={shiftTypes}
      isLoading={false}
      loadError={false}
      onClose={vi.fn()}
      onChanged={vi.fn()}
    />,
  );
}

describe("ShiftTypeManageModal — category mapping safety (#1837)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("preserves linked categories when saving an unrelated field on an existing shift type", async () => {
    // Category 1 is already mapped to shift type 99. Editing 99 and saving only
    // a name change must NOT clear the mapping (must resend category_ids: ["1"]).
    mockGetCategories.mockResolvedValue([category("1", "99"), category("2")]);
    mockUpdateShiftType.mockResolvedValue(shiftType("99", "Betreuung X"));

    renderModal([shiftType("99", "Betreuung")]);
    await waitFor(() => expect(mockGetCategories).toHaveBeenCalledTimes(1));

    fireEvent.click(
      screen.getByRole("button", { name: "Betreuung bearbeiten" }),
    );
    fireEvent.change(screen.getByLabelText("Name*"), {
      target: { value: "Betreuung X" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Änderungen speichern" }),
    );

    await waitFor(() => expect(mockUpdateShiftType).toHaveBeenCalledTimes(1));
    const [id, payload] = mockUpdateShiftType.mock.calls[0] as [
      string,
      { categoryIds?: string[] },
    ];
    expect(id).toBe("99");
    expect(payload.categoryIds).toEqual(["1"]);
  });

  it("omits category_ids when the category list failed to load (never clears mappings)", async () => {
    mockGetCategories.mockRejectedValue(new Error("network down"));
    mockCreateShiftType.mockResolvedValue(shiftType("5", "Neu"));

    renderModal([]);
    // Let the failed load settle.
    await waitFor(() => expect(mockGetCategories).toHaveBeenCalledTimes(1));

    fireEvent.click(screen.getByRole("button", { name: "Eigene Schichtart" }));
    fireEvent.change(screen.getByLabelText("Name*"), {
      target: { value: "Neu" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Schichtart anlegen" }));

    await waitFor(() => expect(mockCreateShiftType).toHaveBeenCalledTimes(1));
    const [payload] = mockCreateShiftType.mock.calls[0] as [
      { categoryIds?: string[] },
    ];
    // Omitted (undefined) so the backend leaves existing links untouched —
    // never an empty array that would clear them.
    expect(payload.categoryIds).toBeUndefined();
  });

  it("omits category_ids on a follow-up edit when the post-save reload failed", async () => {
    // Open load succeeds; the post-save refresh then fails, so the local
    // category snapshot is stale. A subsequent edit must not resubmit mappings
    // from that stale snapshot.
    mockGetCategories
      .mockResolvedValueOnce([category("1", "99"), category("2")])
      .mockRejectedValue(new Error("reload failed"));
    mockUpdateShiftType.mockResolvedValue(shiftType("99", "Betreuung"));

    renderModal([shiftType("99", "Betreuung")]);
    await waitFor(() => expect(mockGetCategories).toHaveBeenCalledTimes(1));

    // First edit + save → triggers the post-save reload, which rejects.
    fireEvent.click(
      screen.getByRole("button", { name: "Betreuung bearbeiten" }),
    );
    fireEvent.change(screen.getByLabelText("Name*"), {
      target: { value: "Betreuung A" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Änderungen speichern" }),
    );
    await waitFor(() => expect(mockUpdateShiftType).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(mockGetCategories).toHaveBeenCalledTimes(2));

    // Second edit in the same open modal + save.
    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Betreuung bearbeiten" }),
      ).toBeInTheDocument(),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Betreuung bearbeiten" }),
    );
    fireEvent.change(screen.getByLabelText("Name*"), {
      target: { value: "Betreuung B" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Änderungen speichern" }),
    );

    await waitFor(() => expect(mockUpdateShiftType).toHaveBeenCalledTimes(2));
    const [, payload] = mockUpdateShiftType.mock.calls[1] as [
      string,
      { categoryIds?: string[] },
    ];
    expect(payload.categoryIds).toBeUndefined();
  });

  it("discards a slow initial load that resolves after the post-save refresh (#1837)", async () => {
    // The initial (open) load is deferred and resolves LATE with a stale
    // snapshot; the post-save refresh resolves first with fresh mappings. The
    // stale late resolution must be ignored (token guard).
    let resolveStaleInitial!: (v: ActivityCategory[]) => void;
    const staleInitial = new Promise<ActivityCategory[]>((res) => {
      resolveStaleInitial = res;
    });
    mockGetCategories
      .mockReturnValueOnce(staleInitial) // open load — stale, pending
      .mockResolvedValue([category("1", "99"), category("2")]); // fresh
    mockUpdateShiftType.mockResolvedValue(shiftType("99", "Betreuung"));

    renderModal([shiftType("99", "Betreuung")]);
    await waitFor(() => expect(mockGetCategories).toHaveBeenCalledTimes(1));

    // Edit + save while the initial load is still pending → post-save refresh
    // (call 2) resolves fresh.
    fireEvent.click(
      screen.getByRole("button", { name: "Betreuung bearbeiten" }),
    );
    fireEvent.change(screen.getByLabelText("Name*"), {
      target: { value: "Betreuung A" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Änderungen speichern" }),
    );
    await waitFor(() => expect(mockGetCategories).toHaveBeenCalledTimes(2));

    // The stale initial load finally resolves — must be discarded.
    resolveStaleInitial([category("1"), category("2")]); // cat1 NOT mapped

    // A follow-up edit must preselect from the FRESH snapshot (cat1 → 99).
    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Betreuung bearbeiten" }),
      ).toBeInTheDocument(),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Betreuung bearbeiten" }),
    );
    fireEvent.change(screen.getByLabelText("Name*"), {
      target: { value: "Betreuung B" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Änderungen speichern" }),
    );

    await waitFor(() => expect(mockUpdateShiftType).toHaveBeenCalledTimes(2));
    const [, payload] = mockUpdateShiftType.mock.calls[1] as [
      string,
      { categoryIds?: string[] },
    ];
    expect(payload.categoryIds).toEqual(["1"]);
  });

  it("disables the category selector until categories have loaded", async () => {
    let resolveLoad!: (v: ActivityCategory[]) => void;
    mockGetCategories.mockReturnValueOnce(
      new Promise<ActivityCategory[]>((res) => {
        resolveLoad = res;
      }),
    );

    renderModal([shiftType("99", "Betreuung")]);
    fireEvent.click(
      screen.getByRole("button", { name: "Betreuung bearbeiten" }),
    );

    const trigger = screen.getByRole("button", {
      name: "Timetable-Kategorien",
    });
    expect(trigger).toBeDisabled();
    expect(
      screen.getAllByText(/Kategorien werden geladen/).length,
    ).toBeGreaterThan(0);

    resolveLoad([category("1", "99")]);
    await waitFor(() => expect(trigger).toBeEnabled());
  });

  it("refetches categories after a successful save so preselection is not stale", async () => {
    mockGetCategories
      .mockResolvedValueOnce([category("1"), category("2")])
      .mockResolvedValue([category("1", "99"), category("2")]);
    mockCreateShiftType.mockResolvedValue(shiftType("99", "Betreuung"));

    renderModal([]);
    await waitFor(() => expect(mockGetCategories).toHaveBeenCalledTimes(1));

    fireEvent.click(screen.getByRole("button", { name: "Eigene Schichtart" }));
    fireEvent.change(screen.getByLabelText("Name*"), {
      target: { value: "Betreuung" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Schichtart anlegen" }));

    await waitFor(() => expect(mockCreateShiftType).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(mockGetCategories).toHaveBeenCalledTimes(2));
  });
});
