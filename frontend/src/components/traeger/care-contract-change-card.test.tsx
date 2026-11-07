import {
  act,
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
import {
  contractChangeReview,
  type ContractChanges,
} from "~/lib/care-contract-changes";

function render(ui: React.ReactElement) {
  return renderPlain(ui, { wrapper: ToastProvider });
}

const mocks = vi.hoisted(() => ({
  fetchContractChanges: vi.fn(),
  decideContractChange: vi.fn(),
}));

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { permissions: ["admin:*"] } },
    status: "authenticated",
  }),
}));

vi.mock("~/lib/care-contract-changes-api", () => ({
  fetchContractChanges: mocks.fetchContractChanges,
  decideContractChange: mocks.decideContractChange,
}));

import { ApiError } from "~/lib/api-error";
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

describe("contractChangeReview", () => {
  it("distinguishes fixed-day additions and removals without inventing weekdays", () => {
    const row = contractChangeReview({
      ...pending.pending[0]!,
      lines: [
        {
          careOfferingId: "8",
          moduleName: "Neu",
          state: "added",
          beforeDays: [],
          afterDays: [],
        },
        {
          careOfferingId: "9",
          moduleName: "Alt",
          state: "removed",
          beforeDays: [],
          afterDays: [],
        },
      ],
    });
    expect(row.diff.map((line) => [line.old, line.new])).toEqual([
      ["Nicht gebucht", "Gebucht (feste Tage)"],
      ["Gebucht (feste Tage)", "Nicht gebucht"],
    ]);
  });
});

describe("CareContractChangeCard (#5979)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows the shared module comparison with both decisions", async () => {
    mocks.fetchContractChanges.mockResolvedValue(pending);
    render(<CareContractChangeCard contractId="25" onDecided={vi.fn()} />);

    expect(await screen.findByText("05.10.2026")).toBeInTheDocument();
    expect(screen.getByText("Wartet auf Entscheidung")).toBeInTheDocument();
    expect(screen.getByText("Offene Ganztagsschule:")).toBeInTheDocument();
    expect(screen.getByText("Mo, Mi")).toBeInTheDocument();
    expect(screen.getByText("Mo, Di")).toBeInTheDocument();
    expect(
      screen.getByText(/Ab Oktober arbeite ich dienstags länger./),
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
    await waitFor(() =>
      expect(
        within(dialog).getByRole("button", { name: "Freigeben" }),
      ).toBeEnabled(),
    );
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

  it("requires the selected date's preview before approving", async () => {
    let resolvePreview!: (changes: ContractChanges) => void;
    mocks.fetchContractChanges
      .mockResolvedValueOnce(pending)
      .mockImplementation(
        () =>
          new Promise<ContractChanges>((resolve) => {
            resolvePreview = resolve;
          }),
      );
    mocks.decideContractChange.mockResolvedValue({ ...pending, pending: [] });
    render(<CareContractChangeCard contractId="25" onDecided={vi.fn()} />);
    fireEvent.click(await screen.findByRole("button", { name: "Freigeben" }));
    const dialog = await screen.findByRole("dialog");
    const approve = within(dialog).getByRole("button", { name: "Freigeben" });
    expect(approve).toBeDisabled();
    fireEvent.change(within(dialog).getByLabelText(/Gilt ab/), {
      target: { value: "2026-11-02" },
    });
    await waitFor(() =>
      expect(mocks.fetchContractChanges).toHaveBeenLastCalledWith(
        "25",
        "2026-11-02",
      ),
    );
    expect(approve).toBeDisabled();
    await act(async () =>
      resolvePreview({
        ...pending,
        pending: [
          {
            ...pending.pending[0]!,
            lines: [
              {
                ...pending.pending[0]!.lines[0]!,
                beforeDays: ["thu"],
                afterDays: ["fri"],
              },
            ],
          },
        ],
      }),
    );
    expect(await within(dialog).findByText("Do")).toBeInTheDocument();
    expect(within(dialog).getByText("Fr")).toBeInTheDocument();
    await waitFor(() => expect(approve).toBeEnabled());
    fireEvent.click(approve);
    await waitFor(() =>
      expect(mocks.decideContractChange).toHaveBeenCalledWith(
        "25",
        "41",
        expect.objectContaining({ effectiveFrom: "2026-11-02" }),
      ),
    );
  });

  it.each([
    "students.change_request_stale",
    "students.change_request_not_pending",
  ])("refreshes and dismisses an obsolete form on %s", async (code) => {
    mocks.fetchContractChanges.mockResolvedValue(pending);
    mocks.decideContractChange.mockRejectedValueOnce(
      new ApiError("conflict", 409, { code }),
    );
    render(<CareContractChangeCard contractId="25" onDecided={vi.fn()} />);
    fireEvent.click(await screen.findByRole("button", { name: "Ablehnen" }));
    const dialog = await screen.findByRole("dialog");
    fireEvent.change(within(dialog).getByLabelText(/Begründung/), {
      target: { value: "Kein Platz." },
    });
    mocks.fetchContractChanges.mockResolvedValue({
      ...pending,
      pending: [{ ...pending.pending[0]!, version: "v-42" }],
    });
    fireEvent.click(within(dialog).getByRole("button", { name: "Ablehnen" }));
    await waitFor(() =>
      expect(mocks.fetchContractChanges).toHaveBeenCalledTimes(2),
    );
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
    fireEvent.click(await screen.findByRole("button", { name: "Ablehnen" }));
    const reopened = await screen.findByRole("dialog");
    fireEvent.change(within(reopened).getByLabelText(/Begründung/), {
      target: { value: "Kein Platz." },
    });
    mocks.decideContractChange.mockResolvedValue({ ...pending, pending: [] });
    fireEvent.click(within(reopened).getByRole("button", { name: "Ablehnen" }));
    await waitFor(() =>
      expect(mocks.decideContractChange).toHaveBeenLastCalledWith(
        "25",
        "41",
        expect.objectContaining({ expectedVersion: "v-42" }),
      ),
    );
  });

  it("keeps manual planning and pickup consequences visible for school follow-up", async () => {
    mocks.fetchContractChanges.mockResolvedValue(pending);
    mocks.decideContractChange.mockResolvedValue({
      ...pending,
      pending: [],
      rosterImpact: {
        effectiveFrom: "2026-10-05",
        adjusted: [],
        notAdjusted: [
          {
            templateId: "12",
            name: "Handgruppe",
            weekdays: [2],
            viaTarget: false,
            until: null,
          },
        ],
        pickupOverrides: [
          {
            weekday: 2,
            date: "2026-10-06",
            storedTime: "15:00",
            bookedTime: "16:00",
            resetNow: true,
          },
        ],
      },
    });
    render(<CareContractChangeCard contractId="25" onDecided={vi.fn()} />);
    fireEvent.click(await screen.findByRole("button", { name: "Freigeben" }));
    const dialog = await screen.findByRole("dialog");
    const approve = within(dialog).getByRole("button", { name: "Freigeben" });
    await waitFor(() => expect(approve).toBeEnabled());
    fireEvent.click(approve);
    expect(
      await screen.findByText(/Hinweise an die Schule/),
    ).toBeInTheDocument();
    expect(screen.getByText(/Handgruppe/)).toBeInTheDocument();
    expect(screen.getByText(/eingetragen 15:00 Uhr/)).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: /Im Betreuungsplan ansehen/ }),
    ).not.toBeInTheDocument();
  });
});
