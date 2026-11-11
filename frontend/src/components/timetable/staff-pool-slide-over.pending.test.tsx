// #6593: Der Personal-Move läuft aus der Rückfrage über dem Panel; bis zur
// Antwort bleibt auch das Panel gesperrt, und eine späte Antwort zeigt nichts
// im Panel eines anderen Blocks. Echter SlideOver und echte Rückfrage.
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import { useSWRAuth } from "~/lib/swr";
import { timetableService } from "~/lib/timetable-api";
import type {
  EnrichedInstance,
  MoveStaffResponse,
  StaffPoolEntry,
} from "~/lib/timetable-types";
import { catalogText } from "~/test/error-catalog-text";
import {
  deferred,
  expectSlideOverSaveLocked,
  expectSlideOverUnlocked,
} from "~/test/helpers/form-modal-pending";

const { mockToastSuccess } = vi.hoisted(() => ({ mockToastSuccess: vi.fn() }));

vi.mock("vaul", () => import("~/test/mocks/vaul"));

vi.mock("~/lib/swr", () => ({ useSWRAuth: vi.fn() }));

vi.mock("~/lib/timetable-api", () => ({
  timetableService: { getStaffPool: vi.fn(), moveStaff: vi.fn() },
}));

vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({
    success: mockToastSuccess,
    error: vi.fn(),
    warning: vi.fn(),
    info: vi.fn(),
  }),
}));

import { StaffPoolSlideOver } from "./staff-pool-slide-over";

const TITLE = "Personalpool";
const FUTURE_DATE = "2099-09-21";

function instance(id: string, title: string): EnrichedInstance {
  return {
    id,
    date: FUTURE_DATE,
    startTime: "12:30",
    endTime: "13:30",
    title,
    status: "planned",
    staff: [],
  } as unknown as EnrichedInstance;
}

const ENTRY: StaffPoolEntry = {
  staffId: "7",
  displayName: "Ina Umzieherin",
  category: "assigned_elsewhere",
  onShift: true,
  coversWindow: true,
  shiftWindows: ["08:00–16:00"],
  assignments: [
    {
      instanceId: "41",
      title: "Schulhof",
      startTime: "12:00",
      endTime: "14:00",
      isSubstitute: false,
    },
  ],
};

const MOVED: MoveStaffResponse = {
  targetInstanceId: "42",
  sourceInstanceId: "41",
  action: "moved",
  timeConflicts: [],
  coverageWarnings: [],
};

const mutatePool = vi.fn(() => Promise.resolve());

function renderAndMove() {
  vi.mocked(useSWRAuth).mockReturnValue({
    data: {
      instanceId: "42",
      title: "Mensa",
      date: FUTURE_DATE,
      startTime: "12:30",
      endTime: "13:30",
      dienstplanInUse: true,
      entries: [ENTRY],
    },
    isLoading: false,
    error: undefined,
    mutate: mutatePool,
    isValidating: false,
  } as unknown as ReturnType<typeof useSWRAuth>);
  const answer = deferred<MoveStaffResponse>();
  vi.mocked(timetableService.moveStaff).mockReturnValue(answer.promise);
  const onClose = vi.fn();
  const onMoved = vi.fn();
  const panel = (target: EnrichedInstance) => (
    <StaffPoolSlideOver
      open
      instance={target}
      canManage
      onClose={onClose}
      onMoved={onMoved}
    />
  );
  const view = render(panel(instance("42", "Mensa")));
  fireEvent.click(screen.getByRole("button", { name: /Hierher verschieben/ }));
  fireEvent.click(screen.getByRole("button", { name: "Verschieben" }));
  expect(timetableService.moveStaff).toHaveBeenCalledWith("42", {
    staffId: "7",
    sourceInstanceId: "41",
  });
  return {
    answer,
    onClose,
    onMoved,
    switchBlock: () => view.rerender(panel(instance("43", "Lernzeit"))),
  };
}

describe("StaffPoolSlideOver pending lock (#6593)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("sperrt das Panel, solange der Move läuft, und gibt es nach einem Fehler frei", async () => {
    const { answer, onClose } = renderAndMove();

    expectSlideOverSaveLocked(TITLE, [], "Schließen");
    expect(
      screen.getByRole("button", { name: /Hierher verschieben/ }),
    ).toBeDisabled();

    await act(async () => {
      answer.reject(
        new ApiError("conflict", 409, {
          code: "timetable.staff_already_on_target",
        }),
      );
    });

    // Der Fehler steht in der offenen Rückfrage; nach ihrem Schließen ist
    // das Panel wieder frei.
    // Anker: die Rückfrage ist offen und zeigt den Fehler.
    expect(
      await screen.findByText(
        catalogText(
          "timetable.staff_already_on_target",
          "das Verschieben der Person",
        ),
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));
    await waitFor(() =>
      expect(screen.queryByText("Person verschieben?")).not.toBeInTheDocument(),
    );
    expectSlideOverUnlocked(TITLE, "Schließen");
    expect(onClose).not.toHaveBeenCalled();
  });

  // #6594: Ließe sich die Rückfrage während des Moves schließen, hätte ein
  // Fehler keine Fläche mehr: er steht nur in der Rückfrage.
  it("hält die Rückfrage offen, solange der Move läuft, und zeigt danach den Fehler darin", async () => {
    const { answer } = renderAndMove();

    const cancel = screen.getByRole("button", { name: "Abbrechen" });
    expect(cancel).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Modal schließen" }),
    ).toBeDisabled();
    fireEvent.click(cancel);
    fireEvent.click(screen.getByRole("button", { name: "Modal schließen" }));
    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.getByText("Person verschieben?")).toBeInTheDocument();

    await act(async () => {
      answer.reject(new ApiError("down", 503, { code: "general.unavailable" }));
    });

    expect(await screen.findByRole("alert")).toBeVisible();
    expect(screen.getByText("Person verschieben?")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Abbrechen" })).toBeEnabled();
  });

  it("schließt nach dem Move die Rückfrage und frischt auf", async () => {
    const { answer, onClose, onMoved } = renderAndMove();
    // Anker: die Rückfrage ist offen, solange der Move läuft.
    expect(screen.getByText("Person verschieben?")).toBeInTheDocument();

    await act(async () => {
      answer.resolve(MOVED);
    });

    await waitFor(() =>
      expect(screen.queryByText("Person verschieben?")).not.toBeInTheDocument(),
    );
    expectSlideOverUnlocked(TITLE, "Schließen");
    expect(mutatePool).toHaveBeenCalledOnce();
    expect(onMoved).toHaveBeenCalledOnce();
    expect(onClose).not.toHaveBeenCalled();
  });

  it("frischt nach einer späten Antwort auf, zeigt im Panel des neuen Blocks aber keine Rückfrage", async () => {
    const { answer, onMoved, switchBlock } = renderAndMove();

    switchBlock();
    expectSlideOverUnlocked(TITLE, "Schließen");
    // Anker: das Panel zeigt den neuen Block.
    expect(screen.getByText(/Lernzeit/)).toBeInTheDocument();
    expect(screen.queryByText("Person verschieben?")).not.toBeInTheDocument();

    await act(async () => {
      answer.resolve(MOVED);
    });

    expect(mockToastSuccess).toHaveBeenCalledWith(
      "Ina Umzieherin wurde verschoben.",
    );
    expect(onMoved).toHaveBeenCalledOnce();
    // Anker: das Panel zeigt weiter den neuen Block.
    expect(screen.getByText(/Lernzeit/)).toBeInTheDocument();
    expect(screen.queryByText("Person verschieben?")).not.toBeInTheDocument();
  });

  it("zeigt einen späten Fehler nicht im Panel des neuen Blocks", async () => {
    const { answer, switchBlock } = renderAndMove();

    switchBlock();
    await act(async () => {
      answer.reject(new ApiError("down", 503, { code: "general.unavailable" }));
    });

    // Anker: das Panel zeigt den neuen Block.
    expect(screen.getByText(/Lernzeit/)).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(screen.queryByText("Person verschieben?")).not.toBeInTheDocument();
  });
});
