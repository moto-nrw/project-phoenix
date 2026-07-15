import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
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
      moveShift: vi.fn(),
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

const inactiveShiftType: ShiftType = {
  ...shiftType,
  name: "Betreuung alt",
  isActive: false,
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

function renderDialog(
  overrides: {
    shift?: StaffShift;
    shiftTypes?: readonly ShiftType[];
  } = {},
) {
  const onClose = vi.fn();
  const onDataChanged = vi.fn();
  render(
    <ShiftMoveDialog
      isOpen
      shift={overrides.shift ?? baseShift()}
      sourceMember={source}
      staff={[source, other]}
      shiftTypes={overrides.shiftTypes ?? [shiftType]}
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
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: "Verschieben" }));
    await Promise.resolve();
  });
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

  it("issues exactly one atomic move and never uses CRUD composition", async () => {
    vi.mocked(staffShiftService.moveShift).mockResolvedValue(baseShift());
    const { onDataChanged, onClose } = renderDialog();

    await confirmMove();

    await waitFor(() =>
      expect(staffShiftService.moveShift).toHaveBeenCalledTimes(1),
    );
    expect(staffShiftService.moveShift).toHaveBeenCalledWith(
      "1",
      expect.objectContaining({
        sourceStaffId: "7",
        targetStaffId: "7",
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

  it("sends source and target to one atomic move request", async () => {
    vi.mocked(staffShiftService.moveShift).mockResolvedValue(
      baseShift({ staffId: other.id }),
    );
    const { onDataChanged, onClose } = renderDialog();

    selectPerson("Zielperson", "Yilmaz, Bo");
    await confirmMove();

    await waitFor(() =>
      expect(staffShiftService.moveShift).toHaveBeenCalledTimes(1),
    );
    expect(staffShiftService.moveShift).toHaveBeenCalledWith(
      "1",
      expect.objectContaining({
        sourceStaffId: "7",
        targetStaffId: "8",
        date: "2026-07-06",
        startTime: "08:00",
        endTime: "12:00",
      }),
    );
    expect(staffShiftService.updateShift).not.toHaveBeenCalled();
    expect(staffShiftService.createShift).not.toHaveBeenCalled();
    expect(staffShiftService.deleteShift).not.toHaveBeenCalled();
    expect(onDataChanged).toHaveBeenCalledTimes(1);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("shows an error without reporting a data change when the move fails", async () => {
    vi.mocked(staffShiftService.moveShift).mockRejectedValue(
      new ShiftApiError(500, "boom"),
    );
    const { onDataChanged, onClose } = renderDialog();

    selectPerson("Zielperson", "Yilmaz, Bo");
    await confirmMove();

    // moveErrorMessage falls through to the raw detail for a 500.
    await waitFor(() => expect(screen.getByText("boom")).toBeInTheDocument());
    expect(staffShiftService.moveShift).toHaveBeenCalledTimes(1);
    expect(staffShiftService.deleteShift).not.toHaveBeenCalled();
    expect(staffShiftService.createShift).not.toHaveBeenCalled();
    expect(
      screen.getByRole("combobox", { name: "Zielperson" }),
    ).toBeInTheDocument();
    expect(onDataChanged).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
  });

  it("maps an origin-link 400 to the Vertretung-specific message", async () => {
    vi.mocked(staffShiftService.moveShift).mockRejectedValue(
      new ShiftApiError(
        400,
        "replacement must be on the same date as the shift it covers",
      ),
    );
    renderDialog({ shift: baseShift({ originShiftId: "42" }) });

    selectPerson("Zielperson", "Yilmaz, Bo");
    await confirmMove();

    await waitFor(() =>
      expect(
        screen.getByText(/kann nur innerhalb des Tages und Zeitfensters/i),
      ).toBeInTheDocument(),
    );
    expect(staffShiftService.moveShift).toHaveBeenCalledTimes(1);
    expect(staffShiftService.deleteShift).not.toHaveBeenCalled();
  });

  it("maps a stale-move conflict to a reload message", async () => {
    vi.mocked(staffShiftService.moveShift).mockRejectedValue(
      new ShiftApiError(409, "shift changed concurrently"),
    );
    renderDialog();

    selectPerson("Zielperson", "Yilmaz, Bo");
    await confirmMove();

    expect(
      await screen.findByText(/zwischenzeitlich geändert/i),
    ).toBeInTheDocument();
  });

  it("requires an active or empty shift type when the person changes", () => {
    renderDialog({
      shift: baseShift({ shiftTypeName: inactiveShiftType.name }),
      shiftTypes: [inactiveShiftType],
    });

    selectPerson("Zielperson", "Yilmaz, Bo");

    expect(screen.getByRole("alert")).toHaveTextContent(
      "Diese Schichtart ist inaktiv",
    );
    expect(screen.getByRole("button", { name: "Verschieben" })).toBeDisabled();
    expect(staffShiftService.moveShift).not.toHaveBeenCalled();
  });

  it("executes only once for two confirmations in the same render", async () => {
    let resolveMove: ((shift: StaffShift) => void) | undefined;
    vi.mocked(staffShiftService.moveShift).mockImplementation(
      () =>
        new Promise<StaffShift>((resolve) => {
          resolveMove = resolve;
        }),
    );
    renderDialog();
    selectPerson("Zielperson", "Yilmaz, Bo");

    fireEvent.click(screen.getByRole("button", { name: "Verschieben" }));
    const confirm = screen.getByRole("button", { name: "Verschieben" });
    fireEvent.click(confirm);
    fireEvent.click(confirm);

    expect(staffShiftService.moveShift).toHaveBeenCalledTimes(1);

    await act(async () => {
      resolveMove?.(baseShift());
      await Promise.resolve();
    });
    expect(staffShiftService.moveShift).toHaveBeenCalledTimes(1);
  });
});

describe("ShiftMoveDialog series shift", () => {
  beforeEach(() => vi.clearAllMocks());

  it("uses the atomic concrete-shift move endpoint for a series row", async () => {
    vi.mocked(staffShiftService.moveShift).mockResolvedValue(baseShift());
    renderDialog({ shift: baseShift({ seriesId: "5" }) });

    selectPerson("Zielperson", "Yilmaz, Bo");
    await confirmMove();

    await waitFor(() =>
      expect(staffShiftService.moveShift).toHaveBeenCalledTimes(1),
    );
    expect(staffShiftService.moveShift).toHaveBeenCalledWith(
      "1",
      expect.objectContaining({ sourceStaffId: "7", targetStaffId: "8" }),
    );
    expect(staffShiftService.createShift).not.toHaveBeenCalled();
    expect(staffShiftService.deleteShift).not.toHaveBeenCalled();
  });
});
