import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ShiftApiError, staffShiftService } from "~/lib/shift-api";
import type { StaffScheduleStaff, StaffShift } from "~/lib/shift-helpers";
import type { ShiftType } from "~/lib/shift-type-helpers";

import { ShiftMoveDialog } from "./shift-move-dialog";

// Spy on the CRUD service while keeping the real ShiftApiError class (the
// dialog classifies errors against it). vitest hoists this above the imports,
// so `staffShiftService` above is the mocked singleton.
vi.mock("~/lib/shift-api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("~/lib/shift-api")>();
  return {
    ...actual,
    staffShiftService: {
      updateShift: vi.fn(),
      deleteShift: vi.fn(),
      createShift: vi.fn(),
    },
  };
});

const source: StaffScheduleStaff = {
  id: "7",
  firstName: "Ada",
  lastName: "Lovelace",
};
const other: StaffScheduleStaff = {
  id: "8",
  firstName: "Bo",
  lastName: "Yilmaz",
};

const shiftType: ShiftType = {
  id: "3",
  name: "Betreuung",
  color: "#83CD2D",
  description: "",
  isActive: true,
};

function baseShift(overrides: Partial<StaffShift> = {}): StaffShift {
  return {
    id: "1",
    staffId: source.id,
    date: "2026-07-06",
    startTime: "08:00",
    endTime: "12:00",
    breakMinutes: 0,
    shiftTypeId: "3",
    shiftTypeName: "Betreuung",
    shiftTypeColor: "#83CD2D",
    notes: "",
    seriesId: null,
    detached: false,
    cancelled: false,
    changeReason: null,
    originShiftId: null,
    ...overrides,
  };
}

function renderDialog(overrides: { shift?: StaffShift } = {}) {
  const onClose = vi.fn();
  const onDataChanged = vi.fn();
  render(
    <ShiftMoveDialog
      isOpen
      shift={overrides.shift ?? baseShift()}
      sourceMember={source}
      staff={[source, other]}
      shiftTypes={[shiftType]}
      onClose={onClose}
      onDataChanged={onDataChanged}
    />,
  );
  return { onClose, onDataChanged };
}

function selectPerson(comboboxName: string, optionName: string) {
  fireEvent.click(screen.getByRole("combobox", { name: comboboxName }));
  fireEvent.click(screen.getByRole("option", { name: optionName }));
}

async function confirmMove() {
  // Footer submit opens the confirmation step; confirm executes it. Only one
  // "Verschieben" button is visible at a time (the form modal is hidden while
  // the confirmation shows).
  fireEvent.click(screen.getByRole("button", { name: "Verschieben" }));
  fireEvent.click(screen.getByRole("button", { name: "Verschieben" }));
}

describe("ShiftMoveDialog prefill", () => {
  beforeEach(() => vi.clearAllMocks());

  it("prefills person, day, times and shift type from the source shift", () => {
    renderDialog();

    // Zielperson trigger shows the source person (last, first).
    expect(screen.getByText("Lovelace, Ada")).toBeInTheDocument();
    // Zieltag shows the source day.
    expect(screen.getByText("06.07.2026")).toBeInTheDocument();
    // Times.
    expect((screen.getByLabelText("Beginn") as HTMLInputElement).value).toBe(
      "08:00",
    );
    expect((screen.getByLabelText("Ende") as HTMLInputElement).value).toBe(
      "12:00",
    );
    // Schichtart trigger shows the attached type.
    expect(
      screen.getByRole("combobox", { name: "Schichtart" }),
    ).toHaveTextContent("Betreuung");
  });
});

describe("ShiftMoveDialog same person", () => {
  beforeEach(() => vi.clearAllMocks());

  it("issues exactly one updateShift and never deletes or creates", async () => {
    vi.mocked(staffShiftService.updateShift).mockResolvedValue(baseShift());
    const { onDataChanged, onClose } = renderDialog();

    await confirmMove();

    await waitFor(() =>
      expect(staffShiftService.updateShift).toHaveBeenCalledTimes(1),
    );
    expect(staffShiftService.updateShift).toHaveBeenCalledWith(
      "1",
      expect.objectContaining({
        staffId: "7",
        date: "2026-07-06",
        startTime: "08:00",
        endTime: "12:00",
        shiftTypeId: "3",
      }),
    );
    expect(staffShiftService.deleteShift).not.toHaveBeenCalled();
    expect(staffShiftService.createShift).not.toHaveBeenCalled();
    expect(onDataChanged).toHaveBeenCalledTimes(1);
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});

describe("ShiftMoveDialog person change", () => {
  beforeEach(() => vi.clearAllMocks());

  it("deletes the source shift before creating it for the target person", async () => {
    vi.mocked(staffShiftService.deleteShift).mockResolvedValue(undefined);
    vi.mocked(staffShiftService.createShift).mockResolvedValue(baseShift());
    const { onDataChanged, onClose } = renderDialog();

    selectPerson("Zielperson", "Yilmaz, Bo");
    await confirmMove();

    await waitFor(() =>
      expect(staffShiftService.createShift).toHaveBeenCalledTimes(1),
    );
    expect(staffShiftService.deleteShift).toHaveBeenCalledWith("1");
    // DELETE strictly before POST.
    const deleteOrder = vi.mocked(staffShiftService.deleteShift).mock
      .invocationCallOrder[0]!;
    const createOrder = vi.mocked(staffShiftService.createShift).mock
      .invocationCallOrder[0]!;
    expect(deleteOrder).toBeLessThan(createOrder);
    // The create carries the target person, target day, and times.
    expect(staffShiftService.createShift).toHaveBeenCalledWith(
      expect.objectContaining({
        staffId: "8",
        date: "2026-07-06",
        startTime: "08:00",
        endTime: "12:00",
      }),
    );
    expect(staffShiftService.updateShift).not.toHaveBeenCalled();
    expect(onDataChanged).toHaveBeenCalledTimes(1);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("keeps the dialog open with a restore instruction when the create fails after the delete", async () => {
    vi.mocked(staffShiftService.deleteShift).mockResolvedValue(undefined);
    vi.mocked(staffShiftService.createShift).mockRejectedValue(
      new ShiftApiError(500, "boom"),
    );
    const { onDataChanged, onClose } = renderDialog();

    selectPerson("Zielperson", "Yilmaz, Bo");
    await confirmMove();

    await waitFor(() =>
      expect(
        screen.getByText(/ursprüngliche Schicht wurde gelöscht/i),
      ).toBeInTheDocument(),
    );
    // The full source data is shown so the admin can restore it by hand.
    expect(screen.getByText("Ada Lovelace")).toBeInTheDocument();
    expect(screen.getByText("08:00–12:00")).toBeInTheDocument();
    expect(screen.getByText("Montag, 06. Juli 2026")).toBeInTheDocument();
    // Dialog is still open; the caller was told to revalidate (the DELETE
    // went through, so the source shift must vanish from the grid) but the
    // dialog itself was not closed.
    expect(screen.getByText("Schicht verschieben")).toBeInTheDocument();
    expect(onDataChanged).toHaveBeenCalledTimes(1);
    expect(onClose).not.toHaveBeenCalled();
  });
});

describe("ShiftMoveDialog series shift", () => {
  beforeEach(() => vi.clearAllMocks());

  it("uses the single-shift DELETE (never a series endpoint) for a series row", async () => {
    vi.mocked(staffShiftService.deleteShift).mockResolvedValue(undefined);
    vi.mocked(staffShiftService.createShift).mockResolvedValue(baseShift());
    renderDialog({ shift: baseShift({ seriesId: "5" }) });

    selectPerson("Zielperson", "Yilmaz, Bo");
    await confirmMove();

    await waitFor(() =>
      expect(staffShiftService.createShift).toHaveBeenCalledTimes(1),
    );
    // Single-shift delete by the concrete shift id — the service exposes no
    // series-delete here, so the series stays intact via a server exception.
    expect(staffShiftService.deleteShift).toHaveBeenCalledWith("1");
    expect(staffShiftService.deleteShift).toHaveBeenCalledTimes(1);
  });
});
