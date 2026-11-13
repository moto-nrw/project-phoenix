import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { AbsencePlanGap } from "~/lib/staff-api";

const {
  mockGaps,
  mockLoadError,
  mockSyncPlan,
  mockMutate,
  mockToastSuccess,
  mockShowError,
} = vi.hoisted(() => ({
  mockGaps: { current: [] as AbsencePlanGap[] },
  mockLoadError: { current: undefined as unknown },
  mockSyncPlan: vi.fn(),
  mockMutate: vi.fn(() => Promise.resolve()),
  mockToastSuccess: vi.fn(),
  mockShowError: vi.fn(),
}));

vi.mock("~/lib/staff-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/staff-api")>()),
  staffAbsenceService: { syncPlan: mockSyncPlan, listPlanGaps: vi.fn() },
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: () => ({
    data: mockLoadError.current ? undefined : mockGaps.current,
    error: mockLoadError.current,
  }),
  useTenantMutate: () => mockMutate,
}));

// Only the toasts are replaced; the load-error path stays real.
vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({ success: mockToastSuccess }),
  useApiErrorDisplay: () => ({ show: mockShowError }),
}));

vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: () => "sonnenhof",
}));

vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: null, status: "unauthenticated" }),
}));

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({ error: vi.fn(), info: vi.fn(), warn: vi.fn() }),
}));

import { ApiError } from "~/lib/api-error";
import { AbsencePlanGaps, describePlanRows } from "./absence-plan-gaps";

function gap(
  id: string,
  staffId: string,
  overrides: Partial<AbsencePlanGap> = {},
): AbsencePlanGap {
  return {
    absenceId: id,
    staffId,
    absenceType: "vacation",
    dateStart: "2026-12-14",
    dateEnd: "2026-12-18",
    openShifts: 4,
    openBlocks: 6,
    ...overrides,
  };
}

const staffById = new Map([
  ["11", { firstName: "Mia", lastName: "Krüger" }],
  ["12", { firstName: "Jonas", lastName: "Weber" }],
]);

const onShowWeek = vi.fn();

function renderGaps(onPlansChanged = vi.fn()) {
  render(
    <AbsencePlanGaps
      enabled
      staffById={staffById}
      onPlansChanged={onPlansChanged}
      onShowWeek={onShowWeek}
    />,
  );
  return onPlansChanged;
}

describe("AbsencePlanGaps", () => {
  beforeEach(() => {
    globalThis.localStorage.clear();
    mockGaps.current = [];
    mockLoadError.current = undefined;
    mockSyncPlan.mockReset();
    mockMutate.mockClear();
    mockToastSuccess.mockClear();
    mockShowError.mockClear();
  });

  it("shows nothing while every absence is in the plan", () => {
    const { container } = render(
      <AbsencePlanGaps
        enabled
        staffById={staffById}
        onPlansChanged={vi.fn()}
        onShowWeek={vi.fn()}
      />,
    );
    expect(container.firstChild).toBeNull();
  });

  it("lists each absence with its person, type, days and the open plan rows, soonest first", () => {
    mockGaps.current = [
      gap("1", "11"),
      gap("2", "12", {
        absenceType: "training",
        dateStart: "2026-12-07",
        dateEnd: "2026-12-07",
        openShifts: 0,
        openBlocks: 1,
      }),
    ];
    renderGaps();

    expect(
      screen.getByText("Abwesenheiten fehlen im Plan"),
    ).toBeInTheDocument();
    expect(screen.getByText("2 offen")).toBeInTheDocument();
    const names = screen.getAllByText(/^(Mia Krüger|Jonas Weber)$/);
    expect(names.map((node) => node.textContent)).toEqual([
      "Jonas Weber",
      "Mia Krüger",
    ]);
    expect(
      screen.getByText(
        "Urlaub · 14.12.2026 bis 18.12.2026 · noch im Plan: 4 Schichten und 6 Blöcke",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Fortbildung · 07.12.2026 · noch im Plan: 1 Block"),
    ).toBeInTheDocument();
    // Der gemeinsame Hinweis steht einmal, nicht je Zeile.
    expect(
      screen.getAllByText(
        /Abgesagte Schichten und Vertretungen bleiben, wie sie sind/,
      ),
    ).toHaveLength(1);
  });

  it("shows the first week of an absence in the Dienstplan", () => {
    mockGaps.current = [gap("1", "11")];
    renderGaps();
    fireEvent.click(
      screen.getByRole("button", {
        name: "Woche der Abwesenheit von Mia Krüger ansehen",
      }),
    );
    expect(onShowWeek).toHaveBeenCalledWith("2026-12-14");
  });

  it("takes one absence over and reloads the list and the plans", async () => {
    mockGaps.current = [gap("1", "11")];
    mockSyncPlan.mockResolvedValue(true);
    const onPlansChanged = renderGaps();

    fireEvent.click(
      screen.getByRole("button", {
        name: "Abwesenheit von Mia Krüger in den Plan übernehmen",
      }),
    );

    await waitFor(() => expect(onPlansChanged).toHaveBeenCalled());
    expect(mockSyncPlan).toHaveBeenCalledWith("1");
    expect(mockToastSuccess).toHaveBeenCalledWith(
      "Die Abwesenheit von Mia Krüger steht jetzt im Plan.",
    );
    expect(mockMutate).toHaveBeenCalledWith("absence-plan-gaps");
    expect(
      screen.queryByRole("button", { name: "Alle übernehmen" }),
    ).toBeNull();
    expect(screen.getByText("Mia Krüger")).toBeInTheDocument();
  });

  it("asks before taking all over and goes through them one by one", async () => {
    mockGaps.current = [gap("1", "11"), gap("2", "12")];
    mockSyncPlan.mockResolvedValue(true);
    const onPlansChanged = renderGaps();

    fireEvent.click(screen.getByRole("button", { name: "Alle übernehmen" }));
    const dialog = await screen.findByRole("dialog");
    expect(dialog).toHaveTextContent(
      "Das betrifft 2 Abwesenheiten: 8 Schichten und 12 Blöcke.",
    );
    expect(mockSyncPlan).not.toHaveBeenCalled();

    const confirm = screen
      .getAllByRole("button", { name: "Alle übernehmen" })
      .find((button) => dialog.contains(button));
    fireEvent.click(confirm!);

    await waitFor(() =>
      expect(mockToastSuccess).toHaveBeenCalledWith(
        "2 Abwesenheiten stehen jetzt im Plan.",
      ),
    );
    expect(mockSyncPlan.mock.calls).toEqual([["1"], ["2"]]);
    expect(onPlansChanged).toHaveBeenCalled();
  });

  it("stops at a failed absence, names it and keeps what was taken over", async () => {
    mockGaps.current = [gap("1", "11"), gap("2", "12")];
    const failure = new ApiError("nope", 500);
    mockSyncPlan.mockResolvedValueOnce(true).mockRejectedValueOnce(failure);
    const onPlansChanged = renderGaps();

    fireEvent.click(screen.getByRole("button", { name: "Alle übernehmen" }));
    const dialog = await screen.findByRole("dialog");
    const confirm = screen
      .getAllByRole("button", { name: "Alle übernehmen" })
      .find((button) => dialog.contains(button));
    fireEvent.click(confirm!);

    await waitFor(() =>
      expect(mockShowError).toHaveBeenCalledWith(failure, {
        object: "die Übernahme der Abwesenheit von Jonas Weber",
        messageSuffix: "1 von 2 stehen schon im Plan.",
      }),
    );
    expect(mockToastSuccess).not.toHaveBeenCalled();
    expect(onPlansChanged).toHaveBeenCalled();
  });

  it("hides an absence the Leitung keeps in the plan and shows it again", () => {
    mockGaps.current = [gap("1", "11"), gap("2", "12")];
    renderGaps();

    fireEvent.click(
      screen.getByRole("button", {
        name: "Abwesenheit von Mia Krüger ausblenden",
      }),
    );

    expect(screen.getByText("1 offen")).toBeInTheDocument();
    expect(
      screen.getByText("Ausgeblendete Abwesenheiten (1)"),
    ).toBeInTheDocument();
    // Ein einzelner offener Eintrag braucht kein „Alle übernehmen“.
    expect(
      screen.queryByRole("button", { name: "Alle übernehmen" }),
    ).toBeNull();
    expect(
      globalThis.localStorage.getItem(
        "moto:absence-plan-gaps:hidden:sonnenhof",
      ),
    ).toBe('["1"]');

    fireEvent.click(
      screen.getByRole("button", { name: /Ausgeblendete Abwesenheiten/ }),
    );
    fireEvent.click(
      screen.getByRole("button", {
        name: "Abwesenheit von Mia Krüger wieder zeigen",
      }),
    );
    expect(screen.getByText("2 offen")).toBeInTheDocument();
    expect(
      globalThis.localStorage.getItem(
        "moto:absence-plan-gaps:hidden:sonnenhof",
      ),
    ).toBeNull();
  });

  it("takes only the shown absences over and names the skipped ones", async () => {
    mockGaps.current = [gap("1", "11"), gap("2", "12"), gap("3", "11")];
    globalThis.localStorage.setItem(
      "moto:absence-plan-gaps:hidden:sonnenhof",
      '["3"]',
    );
    mockSyncPlan.mockResolvedValueOnce(true).mockResolvedValueOnce(false);
    renderGaps();

    fireEvent.click(screen.getByRole("button", { name: "Alle übernehmen" }));
    const dialog = await screen.findByRole("dialog");
    const confirm = screen
      .getAllByRole("button", { name: "Alle übernehmen" })
      .find((button) => dialog.contains(button));
    fireEvent.click(confirm!);

    await waitFor(() =>
      expect(mockToastSuccess).toHaveBeenCalledWith(
        "1 Abwesenheit steht jetzt im Plan. 1 gilt nicht mehr.",
      ),
    );
    expect(mockSyncPlan.mock.calls).toEqual([["1"], ["2"]]);
  });

  it("shows a load error instead of an empty list", async () => {
    mockLoadError.current = new ApiError("down", 500);
    renderGaps();
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(screen.queryByTestId("absence-plan-gaps")).toBeNull();
  });
});

describe("describePlanRows", () => {
  it("leaves a zero part out", () => {
    expect(
      describePlanRows(gap("1", "11", { openShifts: 1, openBlocks: 0 })),
    ).toBe("1 Schicht");
  });
});
