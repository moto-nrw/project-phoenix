import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { OfferingGap } from "~/lib/offering-gap";

const {
  mockGaps,
  mockLoadError,
  mockSetAcknowledged,
  mockMutate,
  mockToastSuccess,
  mockShowError,
} = vi.hoisted(() => ({
  mockGaps: { current: [] as OfferingGap[] },
  mockLoadError: { current: undefined as unknown },
  mockSetAcknowledged: vi.fn(),
  mockMutate: vi.fn(() => Promise.resolve()),
  mockToastSuccess: vi.fn(),
  mockShowError: vi.fn(),
}));

vi.mock("~/lib/timetable-api", () => ({
  timetableService: { setOfferingGapAcknowledged: mockSetAcknowledged },
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string | null) => ({
    data: key === null || mockLoadError.current ? undefined : mockGaps.current,
    error: key === null ? undefined : mockLoadError.current,
  }),
  useTenantMutate: () => mockMutate,
}));

// Only the toasts are replaced; the load-error path stays real.
vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({ success: mockToastSuccess }),
  useApiErrorDisplay: () => ({ show: mockShowError }),
}));

vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: null, status: "unauthenticated" }),
}));

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({ error: vi.fn(), info: vi.fn(), warn: vi.fn() }),
}));

import { ApiError } from "~/lib/api-error";
import { OfferingGapHints } from "./offering-gap-hints";

const randstunde: OfferingGap = {
  offering: { id: "7", name: "Randstunde" },
  studentCount: 46,
  schoolClasses: ["3a", "3b", "4a"],
  weekdays: [1, 2, 3, 4, 5],
  acknowledged: false,
  acknowledgedClasses: [],
};

const mittag: OfferingGap = {
  offering: { id: "8", name: "Mittagessen" },
  studentCount: 1,
  schoolClasses: [""],
  weekdays: [3],
  acknowledged: true,
  acknowledgedClasses: [""],
};

function renderHints(canManage = true) {
  const onCreateSeries = vi.fn();
  render(
    <OfferingGapHints
      periodId="5"
      canManage={canManage}
      onCreateSeries={onCreateSeries}
      templatesVersion={undefined}
    />,
  );
  return { onCreateSeries };
}

describe("OfferingGapHints", () => {
  beforeEach(() => {
    mockGaps.current = [randstunde, mittag];
    mockLoadError.current = undefined;
    mockSetAcknowledged.mockReset();
    mockToastSuccess.mockClear();
    mockShowError.mockClear();
    mockMutate.mockClear();
    mockSetAcknowledged.mockResolvedValue(undefined);
  });

  it("collects every open offering as a row of one list", () => {
    mockGaps.current = [
      randstunde,
      { ...randstunde, offering: { id: "9", name: "Kurzbetreuung" } },
      mittag,
    ];
    renderHints();

    expect(
      screen.getByRole("heading", { name: "Angebote ohne Regeltermin" }),
    ).toBeInTheDocument();
    expect(screen.getByText("2 offen")).toBeInTheDocument();
    // A list in the page, not an alert that interrupts.
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    const rows = within(
      screen
        .getByRole("heading", { name: "Angebote ohne Regeltermin" })
        .closest("section")!,
    ).getAllByRole("listitem");
    expect(rows).toHaveLength(2);
    expect(within(rows[0]!).getByText("„Randstunde“")).toBeInTheDocument();
    expect(within(rows[1]!).getByText("„Kurzbetreuung“")).toBeInTheDocument();
    expect(
      screen.getAllByText(/46 Kinder · Klassen 3a, 3b, 4a · Mo–Fr/),
    ).toHaveLength(2);
    expect(
      screen.getAllByRole("button", { name: /^Kein Regeltermin nötig: / }),
    ).toHaveLength(2);
    // The decided offering waits folded below, not as a warning.
    expect(
      screen.getByRole("heading", { name: "Ohne eigenen Regeltermin (1)" }),
    ).toBeInTheDocument();
    expect(screen.queryByText("„Mittagessen“")).not.toBeInTheDocument();
  });

  it("shortens many classes and keeps the full list in the tooltip", () => {
    mockGaps.current = [
      {
        ...randstunde,
        schoolClasses: ["1a", "1b", "2a", "2b", "3a", "3b", "4a", "4b"],
        weekdays: [1, 3],
      },
    ];
    renderHints();

    expect(
      screen.getByText("Klassen 1a, 1b und 6 weitere"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Klassen 1a, 1b, 2a, 2b, 3a, 3b, 4a, 4b"),
    ).toBeInTheDocument();
    expect(screen.getByText(/· Mo, Mi/)).toBeInTheDocument();
  });

  it("offers both ways as equal buttons named after their offering", () => {
    renderHints();

    const create = screen.getByRole("button", {
      name: "Regeltermin anlegen: „Randstunde“",
    });
    const none = screen.getByRole("button", {
      name: "Kein Regeltermin nötig: „Randstunde“",
    });
    expect(create).toHaveTextContent("Regeltermin anlegen");
    expect(none).toHaveTextContent("Kein Regeltermin nötig");
    expect(create.className).toBe(none.className);
  });

  it("folds the list after five offerings", () => {
    mockGaps.current = Array.from({ length: 7 }, (_, index) => ({
      ...randstunde,
      offering: { id: String(100 + index), name: `Angebot ${index + 1}` },
    }));
    renderHints();

    expect(screen.getByText("7 offen")).toBeInTheDocument();
    expect(screen.getByText("„Angebot 5“")).toBeInTheDocument();
    expect(screen.queryByText("„Angebot 6“")).not.toBeInTheDocument();
    const more = screen.getByRole("button", { name: "2 weitere anzeigen" });
    expect(more).toHaveAttribute("aria-expanded", "false");
    const list = document.getElementById(more.getAttribute("aria-controls")!);
    expect(list?.tagName).toBe("UL");

    fireEvent.click(more);

    expect(screen.getByText("„Angebot 7“")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Weniger anzeigen" }),
    ).toHaveAttribute("aria-expanded", "true");
  });

  it("shows no warning when every offering is decided", () => {
    mockGaps.current = [mittag];
    renderHints();

    expect(
      screen.getByRole("heading", { name: "Ohne eigenen Regeltermin (1)" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/Angebote? ohne Regeltermin/),
    ).not.toBeInTheDocument();
  });

  it("creates a Regeltermin or records that none is needed", async () => {
    const { onCreateSeries } = renderHints();

    fireEvent.click(
      screen.getByRole("button", { name: "Regeltermin anlegen: „Randstunde“" }),
    );
    expect(onCreateSeries).toHaveBeenCalledWith(randstunde);

    fireEvent.click(
      screen.getByRole("button", {
        name: "Kein Regeltermin nötig: „Randstunde“",
      }),
    );
    await waitFor(() =>
      expect(mockSetAcknowledged).toHaveBeenCalledWith("7", "5", true),
    );
    await waitFor(() =>
      expect(mockToastSuccess).toHaveBeenCalledWith(
        "Der Hinweis zu „Randstunde“ ist ausgeblendet.",
      ),
    );
    expect(mockMutate).toHaveBeenCalledWith("offering-gaps-5");
  });

  it("brings a decided hint back", async () => {
    renderHints();

    fireEvent.click(
      screen.getByRole("button", {
        name: /Ohne eigenen Regeltermin.*ausklappen/,
      }),
    );
    expect(
      screen.getByText(/1 Kind · Kinder ohne Klasse · Mi/),
    ).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", {
        name: "Hinweis wieder zeigen: „Mittagessen“",
      }),
    );

    await waitFor(() =>
      expect(mockSetAcknowledged).toHaveBeenCalledWith("8", "5", false),
    );
  });

  it("shows a failed decision through the error path", async () => {
    const failure = new Error("offline");
    mockSetAcknowledged.mockRejectedValue(failure);
    renderHints();

    fireEvent.click(
      screen.getByRole("button", {
        name: "Kein Regeltermin nötig: „Randstunde“",
      }),
    );

    await waitFor(() =>
      expect(mockShowError).toHaveBeenCalledWith(failure, {
        object: "das Ausblenden des Hinweises",
      }),
    );
    expect(mockToastSuccess).not.toHaveBeenCalled();
  });

  it("only shows open hints to readers without planning rights", () => {
    renderHints(false);

    expect(screen.getByText("„Randstunde“")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /^Regeltermin anlegen/ }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /^Kein Regeltermin nötig/ }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("„Mittagessen“")).not.toBeInTheDocument();
  });
  it("shows a failed check with a retry instead of an empty list", async () => {
    mockLoadError.current = new ApiError("boom", 500);
    renderHints();

    expect(
      await screen.findByText(/Prüfung der Angebote ohne Regeltermin/),
    ).toBeInTheDocument();
    expect(screen.queryByText("„Randstunde“")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    await waitFor(() =>
      expect(mockMutate).toHaveBeenCalledWith("offering-gaps-5"),
    );
  });
});
