import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { StaffSchedule } from "~/lib/staff-api";
import { ArbeitszeitmodellTab } from "./arbeitszeitmodell-tab";

const mocks = vi.hoisted(() => ({
  updateSchedule: vi.fn(),
  mutateSchedule: vi.fn(),
  mutate: vi.fn(),
  toastError: vi.fn(),
}));

const schedule: StaffSchedule = {
  mode: "custom",
  model: null,
  rotationLength: 1,
  rotationAnchorDate: "2026-08-10",
  entries: [{ weekIndex: 0, dayOfWeek: 0, targetMinutes: 267 }],
  weeklyTotals: [267],
  validFrom: "2026-08-10",
};

vi.mock("swr", () => ({
  useSWRConfig: () => ({ mutate: mocks.mutate }),
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string | null) =>
    key?.startsWith("staff-schedule-")
      ? { data: schedule, isLoading: false, mutate: mocks.mutateSchedule }
      : { data: [], isLoading: false, mutate: vi.fn() },
}));

vi.mock("~/lib/staff-api", () => ({
  staffScheduleService: {
    getSchedule: vi.fn(),
    updateSchedule: mocks.updateSchedule,
  },
  workTimeModelService: { list: vi.fn() },
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: vi.fn(),
    error: mocks.toastError,
  }),
}));

vi.mock("~/components/ui/modal", () => ({
  Modal: ({
    isOpen,
    title,
    children,
    footer,
  }: {
    isOpen: boolean;
    title: string;
    children: React.ReactNode;
    footer?: React.ReactNode;
  }) =>
    isOpen ? (
      <div role="dialog" aria-label={title}>
        {children}
        {footer}
      </div>
    ) : null,
}));

describe("Arbeitszeitmodell decimal-hours editor", () => {
  beforeEach(() => {
    mocks.updateSchedule.mockReset();
    mocks.updateSchedule.mockResolvedValue(schedule);
    mocks.mutateSchedule.mockReset();
    mocks.mutate.mockReset();
    mocks.toastError.mockReset();
  });

  function openEditor() {
    render(<ArbeitszeitmodellTab staffId="42" canEdit />);
    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
  }

  it("loads existing minutes as decimal hours and updates totals while typing", () => {
    openEditor();

    const input = screen.getByRole("textbox", {
      name: "Dezimalstunden Montag",
    });
    expect(input).toHaveValue("4,45");

    fireEvent.change(input, { target: { value: "0,33" } });

    expect(input).toHaveValue("0,33");
    expect(screen.getByText("20min (20 min)")).toBeInTheDocument();
    expect(
      screen.getByText("Wochensoll Woche A").nextSibling,
    ).toHaveTextContent("20min");
  });

  it("accepts decimal points and sends only integer target minutes", async () => {
    openEditor();

    fireEvent.change(
      screen.getByRole("textbox", { name: "Dezimalstunden Montag" }),
      { target: { value: "4.45" } },
    );
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(mocks.updateSchedule).toHaveBeenCalledWith("42", {
        mode: "custom",
        rotationLength: 1,
        entries: [{ weekIndex: 0, dayOfWeek: 0, targetMinutes: 267 }],
      }),
    );
  });

  it("treats an empty field as a zero-hour day", () => {
    openEditor();

    const input = screen.getByRole("textbox", {
      name: "Dezimalstunden Montag",
    });
    fireEvent.change(input, { target: { value: "" } });

    expect(input).toHaveAccessibleDescription("0min (0 min)");
    expect(
      screen.getByText("Wochensoll Woche A").nextSibling,
    ).toHaveTextContent("0min");
  });

  it("keeps invalid input visible, exposes its error, and blocks saving", () => {
    openEditor();

    const input = screen.getByRole("textbox", {
      name: "Dezimalstunden Montag",
    });
    fireEvent.change(input, { target: { value: "12,01" } });

    expect(input).toHaveValue("12,01");
    expect(input).toHaveAttribute("aria-invalid", "true");
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Bitte 0 bis 12 eingeben.",
    );

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    expect(mocks.updateSchedule).not.toHaveBeenCalled();
    expect(mocks.toastError).toHaveBeenCalledWith(
      "Bitte die ungültigen Dezimalstunden korrigieren.",
    );
  });

  it("clears removed rotation weeks from both visible and saved state", async () => {
    openEditor();

    fireEvent.click(screen.getByRole("combobox", { name: "Rotation" }));
    fireEvent.click(screen.getByText("A/B-Wochen (2 Wochen)"));
    fireEvent.click(screen.getByRole("button", { name: "Woche B" }));
    fireEvent.change(
      screen.getByRole("textbox", { name: "Dezimalstunden Montag" }),
      { target: { value: "4,45" } },
    );

    fireEvent.click(screen.getByRole("combobox", { name: "Rotation" }));
    fireEvent.click(screen.getByText("Standard (1 Woche)"));
    fireEvent.click(screen.getByRole("combobox", { name: "Rotation" }));
    fireEvent.click(screen.getByText("A/B-Wochen (2 Wochen)"));
    fireEvent.click(screen.getByRole("button", { name: "Woche B" }));

    expect(
      screen.getByRole("textbox", { name: "Dezimalstunden Montag" }),
    ).toHaveValue("");

    fireEvent.change(
      screen.getByRole("textbox", { name: "Dezimalstunden Dienstag" }),
      { target: { value: "1" } },
    );
    expect(
      screen.getByText("5h 27min gesamt · Ø 2h 44min / Woche"),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await waitFor(() =>
      expect(mocks.updateSchedule).toHaveBeenCalledWith("42", {
        mode: "custom",
        rotationLength: 2,
        entries: [
          { weekIndex: 0, dayOfWeek: 0, targetMinutes: 267 },
          { weekIndex: 1, dayOfWeek: 1, targetMinutes: 60 },
        ],
      }),
    );
  });
});
