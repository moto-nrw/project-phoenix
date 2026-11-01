import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { BookingLeftover } from "~/lib/booking-leftover";

const mocks = vi.hoisted(() => ({
  leftovers: { current: [] as BookingLeftover[] },
  loadError: { current: undefined as unknown },
  setKept: vi.fn(),
  remove: vi.fn(),
  mutate: vi.fn(() => Promise.resolve()),
  toastSuccess: vi.fn(),
  showError: vi.fn(),
}));

vi.mock("~/lib/timetable-api", () => ({
  timetableService: { setBookingLeftoverKept: mocks.setKept },
}));

vi.mock("~/lib/roster-impact", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/roster-impact")>()),
  removeStudentTemplateDays: mocks.remove,
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string | null) => ({
    data:
      key === null || mocks.loadError.current
        ? undefined
        : mocks.leftovers.current,
    error: key === null ? undefined : mocks.loadError.current,
  }),
  useTenantMutate: () => mocks.mutate,
}));

// Only the toasts are replaced; the form and load error paths stay real.
vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({ success: mocks.toastSuccess }),
  useApiErrorDisplay: () => ({ show: mocks.showError }),
}));

vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: null, status: "unauthenticated" }),
}));

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({ error: vi.fn(), info: vi.fn(), warn: vi.fn() }),
}));

import { ApiError } from "~/lib/api-error";
import { BookingLeftoverHints } from "./booking-leftover-hints";

const hannah: BookingLeftover = {
  templateId: "9",
  templateName: "Hausaufgabenbetreuung",
  studentId: "42",
  studentName: "Hannah Schmitz",
  weekdays: [3],
  bookingEndedOn: "2026-09-07",
  from: "2026-09-10",
  until: null,
  kept: false,
};

const kept: BookingLeftover = {
  ...hannah,
  studentId: "43",
  studentName: "Elias Sommerfeld",
  kept: true,
};

function renderHints(canManage = true) {
  render(
    <BookingLeftoverHints
      periodId="5"
      canManage={canManage}
      templatesVersion={undefined}
    />,
  );
}

function openSection() {
  return screen
    .getByRole("heading", { name: "Ohne passende Buchung eingeplant" })
    .closest("section")!;
}

describe("BookingLeftoverHints", () => {
  beforeEach(() => {
    mocks.leftovers.current = [hannah, kept];
    mocks.loadError.current = undefined;
    mocks.setKept.mockReset().mockResolvedValue(undefined);
    mocks.remove.mockReset().mockResolvedValue(undefined);
    mocks.mutate.mockClear();
    mocks.toastSuccess.mockClear();
    mocks.showError.mockClear();
  });

  it("names the child, the Regeltermin, the weekdays and since when", () => {
    renderHints();

    expect(screen.getByText("1 offen")).toBeInTheDocument();
    const rows = within(openSection()).getAllByRole("listitem");
    expect(rows).toHaveLength(1);
    expect(within(rows[0]!).getByText("Hannah Schmitz")).toBeInTheDocument();
    expect(
      within(rows[0]!).getByText(
        "„Hausaufgabenbetreuung“ · mittwochs · ohne passende Buchung ab 10.09.2026",
      ),
    ).toBeInTheDocument();
    // A list in the page, not an alert that interrupts.
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("removes from the given day only after the two-step confirmation", async () => {
    renderHints();

    fireEvent.click(
      screen.getByRole("button", {
        name: "Entfernen: Hannah Schmitz in „Hausaufgabenbetreuung“",
      }),
    );
    const dialog = await screen.findByRole("dialog");
    expect(
      within(dialog).getByText(/ab 10\.09\.2026 mittwochs/),
    ).toBeInTheDocument();
    expect(mocks.remove).not.toHaveBeenCalled();

    fireEvent.click(within(dialog).getByRole("button", { name: "Entfernen" }));
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Jetzt entfernen" }),
    );

    await waitFor(() =>
      expect(mocks.remove).toHaveBeenCalledWith({
        templateId: "9",
        studentId: "42",
        weekdays: [3],
        effectiveFrom: "2026-09-10",
        effectiveUntil: null,
      }),
    );
    expect(mocks.toastSuccess).toHaveBeenCalledWith(
      "Hannah Schmitz ist aus „Hausaufgabenbetreuung“ entfernt.",
    );
    expect(mocks.mutate).toHaveBeenCalled();
  });

  it("removes only the gap a later booking closes (#5497)", async () => {
    mocks.leftovers.current = [
      { ...hannah, weekdays: [4], from: "2026-11-05", until: "2026-11-16" },
    ];
    renderHints();

    const row = within(openSection()).getByRole("listitem");
    expect(
      within(row).getByText(
        "„Hausaufgabenbetreuung“ · donnerstags · ohne passende Buchung vom 05.11.2026 bis 15.11.2026",
      ),
    ).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", {
        name: "Entfernen: Hannah Schmitz in „Hausaufgabenbetreuung“",
      }),
    );
    const dialog = await screen.findByRole("dialog");
    expect(
      within(dialog).getByText(/vom 05\.11\.2026 bis 15\.11\.2026 donnerstags/),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByText(
        /Ab 16\.11\.2026 passt die neue Buchung wieder\. Dann bleibt Hannah Schmitz eingeplant\./,
      ),
    ).toBeInTheDocument();

    fireEvent.click(within(dialog).getByRole("button", { name: "Entfernen" }));
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Jetzt entfernen" }),
    );
    await waitFor(() =>
      expect(mocks.remove).toHaveBeenCalledWith({
        templateId: "9",
        studentId: "42",
        weekdays: [4],
        effectiveFrom: "2026-11-05",
        effectiveUntil: "2026-11-16",
      }),
    );
  });

  it("keeps the failed removal open with its error", async () => {
    mocks.remove.mockRejectedValue(new ApiError("boom", 400));
    renderHints();

    fireEvent.click(
      screen.getByRole("button", {
        name: "Entfernen: Hannah Schmitz in „Hausaufgabenbetreuung“",
      }),
    );
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: "Entfernen" }));
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Jetzt entfernen" }),
    );

    expect(await within(dialog).findByRole("alert")).toBeInTheDocument();
    expect(mocks.toastSuccess).not.toHaveBeenCalled();
  });

  it("keeps the child with „Bleibt so“ and can show the hint again", async () => {
    renderHints();

    fireEvent.click(
      screen.getByRole("button", {
        name: "Bleibt so: Hannah Schmitz in „Hausaufgabenbetreuung“",
      }),
    );
    await waitFor(() =>
      expect(mocks.setKept).toHaveBeenCalledWith(hannah, true),
    );
    expect(mocks.toastSuccess).toHaveBeenCalledWith(
      "Hannah Schmitz bleibt in „Hausaufgabenbetreuung“.",
    );

    fireEvent.click(
      screen.getByRole("button", {
        name: /Bleibt ohne Buchung eingeplant.*ausklappen/,
      }),
    );
    fireEvent.click(
      screen.getByRole("button", {
        name: "Hinweis wieder zeigen: Elias Sommerfeld in „Hausaufgabenbetreuung“",
      }),
    );
    await waitFor(() =>
      expect(mocks.setKept).toHaveBeenCalledWith(kept, false),
    );
  });

  it("shows readers without planning rights the open rows without actions", () => {
    renderHints(false);

    expect(screen.getByText("Hannah Schmitz")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /^Entfernen/ }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /^Bleibt so/ }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByText(
        "Ändern kann nur, wer den Betreuungsplan bearbeiten darf.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("Elias Sommerfeld")).not.toBeInTheDocument();
  });

  it("shows nothing without leftovers and the load error instead of an empty list", async () => {
    mocks.leftovers.current = [];
    const { unmount } = render(
      <BookingLeftoverHints
        periodId="5"
        canManage
        templatesVersion={undefined}
      />,
    );
    expect(
      screen.queryByTestId("booking-leftover-hints"),
    ).not.toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    unmount();

    mocks.loadError.current = new ApiError("boom", 500);
    renderHints();
    expect(
      await screen.findByText(/Prüfung der Kinder ohne passende Buchung/),
    ).toBeInTheDocument();
    expect(screen.queryByText("Hannah Schmitz")).not.toBeInTheDocument();
  });
});
