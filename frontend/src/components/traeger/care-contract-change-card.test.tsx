import {
  fireEvent,
  render as renderPlain,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import type React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { changeWeek, type ContractChanges } from "~/lib/care-contract-changes";

function render(ui: React.ReactElement) {
  return renderPlain(ui, { wrapper: ToastProvider });
}

const mocks = vi.hoisted(() => ({
  fetchContractChanges: vi.fn(),
  decideContractChange: vi.fn(),
}));

vi.mock("~/lib/care-contract-changes-api", () => ({
  fetchContractChanges: mocks.fetchContractChanges,
  decideContractChange: mocks.decideContractChange,
}));

import { parseISODate, toISODate } from "~/lib/date-helpers";
vi.mock("~/components/ui/date-picker", () => ({
  DatePicker: ({
    id,
    value,
    onChange,
  }: {
    id: string;
    value?: Date;
    onChange: (value: Date | undefined) => void;
  }) => (
    <input
      id={id}
      type="date"
      value={value ? toISODate(value) : ""}
      onChange={(event) => onChange(parseISODate(event.target.value))}
    />
  ),
}));

import { CareContractChangeCard } from "./care-contract-change-card";

const pending: ContractChanges = {
  decidable: true,
  reasonRequired: false,
  lastDecision: null,
  pending: [
    {
      id: "41",
      version: "v-41",
      submittedAt: "2026-09-01T08:15:00Z",
      effectiveFrom: "2026-10-05",
      parentNote: "Ab Oktober arbeite ich dienstags länger.",
      endsAllCare: false,
      lines: [
        {
          careOfferingId: "7",
          moduleName: "Offene Ganztagsschule",
          state: "changed",
          beforeDays: ["mon", "wed"],
          afterDays: ["mon", "tue"],
        },
      ],
    },
  ],
};

describe("changeWeek (#5979)", () => {
  it("names each weekday of each module as kept, added, removed or free", () => {
    const week = changeWeek(pending.pending[0]!.lines);
    expect(week.weekdays).toEqual(["mon", "tue", "wed", "thu", "fri"]);
    expect(week.rows[0]!.days.map((day) => day.state)).toEqual([
      "kept",
      "added",
      "removed",
      "none",
      "none",
    ]);
  });

  it("shows the weekend only when a module books it", () => {
    const week = changeWeek([
      {
        careOfferingId: "8",
        moduleName: "Ferien",
        state: "added",
        beforeDays: [],
        afterDays: ["sat"],
      },
    ]);
    expect(week.weekdays).toEqual(["mon", "tue", "wed", "thu", "fri", "sat"]);
  });
});

describe("CareContractChangeCard (#5979)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows the parents' change day by day with both decisions", async () => {
    mocks.fetchContractChanges.mockResolvedValue(pending);
    render(<CareContractChangeCard contractId="25" onDecided={vi.fn()} />);

    expect(await screen.findByText("05.10.2026")).toBeInTheDocument();
    expect(screen.getByText("Wartet auf Entscheidung")).toBeInTheDocument();
    const week = screen.getByRole("list", { name: "Änderung je Wochentag" });
    expect(within(week).getByText("Offene Ganztagsschule")).toBeInTheDocument();
    expect(within(week).getByText("Montag: bleibt")).toBeInTheDocument();
    expect(within(week).getByText("Dienstag: kommt dazu")).toBeInTheDocument();
    expect(within(week).getByText("Mittwoch: fällt weg")).toBeInTheDocument();
    expect(
      screen.getByText("Ab Oktober arbeite ich dienstags länger."),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Freigeben" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Ablehnen" })).toBeEnabled();
  });

  it("approves from the confirmed day and reads the contract again", async () => {
    mocks.fetchContractChanges.mockResolvedValue(pending);
    const answered: ContractChanges = { ...pending, pending: [] };
    mocks.decideContractChange.mockResolvedValue(answered);
    const onDecided = vi.fn();
    render(<CareContractChangeCard contractId="25" onDecided={onDecided} />);

    fireEvent.click(await screen.findByRole("button", { name: "Freigeben" }));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByLabelText(/Gilt ab/)).toHaveValue("2026-10-05");
    fireEvent.click(within(dialog).getByRole("button", { name: "Freigeben" }));

    await waitFor(() =>
      expect(mocks.decideContractChange).toHaveBeenCalledWith("25", "41", {
        approve: true,
        reason: "",
        effectiveFrom: "2026-10-05",
        expectedVersion: "v-41",
      }),
    );
    await waitFor(() => expect(onDecided).toHaveBeenCalled());
    expect(
      await screen.findByText("Kein offener Änderungswunsch."),
    ).toBeInTheDocument();
  });

  it("rejects only with a reason for the parents", async () => {
    mocks.fetchContractChanges.mockResolvedValue(pending);
    mocks.decideContractChange.mockResolvedValue({ ...pending, pending: [] });
    render(<CareContractChangeCard contractId="25" onDecided={vi.fn()} />);

    fireEvent.click(await screen.findByRole("button", { name: "Ablehnen" }));
    const dialog = await screen.findByRole("dialog");
    const submit = within(dialog).getByRole("button", { name: "Ablehnen" });
    expect(submit).toBeDisabled();
    fireEvent.change(within(dialog).getByLabelText(/Begründung/), {
      target: { value: "Am Dienstag ist die Gruppe voll." },
    });
    fireEvent.click(submit);

    await waitFor(() =>
      expect(mocks.decideContractChange).toHaveBeenCalledWith("25", "41", {
        approve: false,
        reason: "Am Dienstag ist die Gruppe voll.",
        effectiveFrom: undefined,
        expectedVersion: "v-41",
      }),
    );
  });

  it("offers no approval for a wish that ends all care", async () => {
    mocks.fetchContractChanges.mockResolvedValue({
      ...pending,
      pending: [{ ...pending.pending[0]!, endsAllCare: true }],
    });
    render(<CareContractChangeCard contractId="25" onDecided={vi.fn()} />);

    expect(
      await screen.findByText(/Das ist eine Kündigung/),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Ablehnen" })).toBeEnabled();
    expect(
      screen.getAllByRole("button").map((button) => button.textContent),
    ).not.toContain("Freigeben");
  });

  it("shows the last decision when nothing is open", async () => {
    mocks.fetchContractChanges.mockResolvedValue({
      ...pending,
      pending: [],
      lastDecision: {
        approved: true,
        decidedAt: "2026-09-02T09:00:00Z",
        effectiveFrom: "2026-10-05",
        reason: "",
      },
    });
    render(<CareContractChangeCard contractId="25" onDecided={vi.fn()} />);

    expect(await screen.findByText("Freigegeben")).toBeInTheDocument();
    expect(screen.getByText("Gilt ab 05.10.2026")).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Änderungswunsch der Eltern" }),
    ).toBeInTheDocument();
  });
});
