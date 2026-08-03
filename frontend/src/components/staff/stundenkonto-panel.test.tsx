import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

// The date fields moved from native inputs to the kit picker; this stub keeps
// them settable via fireEvent.change and forwards min/max so the bound
// assertions below still pin what the component computes. Imported inside the
// factory because vi.mock is hoisted above the imports.
vi.mock("~/components/ui/date-picker", async (importOriginal) => {
  const { isoDatePickerMock } = await import("~/test/mocks/date-picker");
  return { ...(await importOriginal<object>()), ...isoDatePickerMock() };
});

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

vi.mock("~/components/ui/confirm-delete-modal", () => ({
  ConfirmDeleteModal: () => null,
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
  }),
}));

vi.mock("~/lib/staff-api", () => ({
  staffBalanceAdjustmentService: {
    create: vi.fn(),
    delete: vi.fn(),
    reset: vi.fn(),
    createOpening: vi.fn(),
  },
}));

import { staffBalanceAdjustmentService } from "~/lib/staff-api";
import { StundenkontoPanel } from "./stundenkonto-panel";

describe("StundenkontoPanel", () => {
  it("keeps scheduled adjustments visible in the management history", () => {
    render(
      <StundenkontoPanel
        staffId="4"
        balanceMinutes={180}
        accountStartKey="2026-01-01"
        todayKey="2026-07-24"
        adjustments={[
          {
            id: "17",
            type: "payout",
            minutesDelta: -120,
            effectiveDate: "2026-08-15",
            note: "Augustgehalt",
            decidedBy: "9",
            decidedAt: "2026-07-24T08:00:00Z",
          },
        ]}
        onChanged={vi.fn()}
      />,
    );

    expect(screen.getByText("Augustgehalt")).toBeInTheDocument();
    expect(screen.getAllByText("Auszahlung")).toHaveLength(2);
    expect(screen.getByText("15.08.2026")).toBeInTheDocument();
  });

  it("allows adjustments through the end of the supported future month", () => {
    render(
      <StundenkontoPanel
        staffId="4"
        balanceMinutes={180}
        accountStartKey="2026-01-01"
        todayKey="2026-07-24"
        adjustments={[]}
        onChanged={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Auszahlung" }));

    expect(screen.getByLabelText("Wirksam am")).toHaveAttribute(
      "max",
      "2027-07-31",
    );
  });

  it("limits resets to the last closed Berlin day and describes the historical cutoff", () => {
    render(
      <StundenkontoPanel
        staffId="4"
        balanceMinutes={180}
        accountStartKey="2026-01-01"
        todayKey="2026-07-24"
        adjustments={[]}
        onChanged={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Zurücksetzen" }));

    const resetDate = screen.getByLabelText("Stichtag");
    expect(resetDate).toHaveAttribute("max", "2026-07-23");
    expect(resetDate).toHaveValue("2026-07-23");
    expect(screen.getByText(/Aktueller Stand:/)).toBeInTheDocument();
    expect(
      screen.getByText(/Gewünschter Übertrag am Stichtag:/),
    ).toBeInTheDocument();
    expect(screen.queryByText(/danach:/)).not.toBeInTheDocument();
  });

  it("disables reset until the account has a closed day", () => {
    render(
      <StundenkontoPanel
        staffId="4"
        balanceMinutes={0}
        accountStartKey="2026-07-24"
        todayKey="2026-07-24"
        adjustments={[]}
        onChanged={vi.fn()}
      />,
    );

    expect(screen.getByRole("button", { name: "Zurücksetzen" })).toBeDisabled();
  });

  it("bucht einen negativen Eröffnungssaldo in Minuten zum Stichtag", async () => {
    const createOpening = vi.mocked(
      staffBalanceAdjustmentService.createOpening,
    );
    createOpening.mockResolvedValue(
      {} as Awaited<
        ReturnType<typeof staffBalanceAdjustmentService.createOpening>
      >,
    );

    render(
      <StundenkontoPanel
        staffId="4"
        balanceMinutes={0}
        accountStartKey="2026-01-01"
        todayKey="2026-07-24"
        adjustments={[]}
        onChanged={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Eröffnungssaldo" }));

    expect(
      screen.getByLabelText("Übernommener Saldo (Stunden)"),
    ).toHaveAttribute("type", "text");

    const openingDate = screen.getByLabelText("Stichtag");
    expect(openingDate).toHaveAttribute("min", "2026-01-01");
    expect(openingDate).toHaveAttribute("max", "2026-07-23");
    expect(openingDate).toHaveValue("2026-01-01");

    fireEvent.change(screen.getByLabelText("Übernommener Saldo (Stunden)"), {
      target: { value: "-2,5" },
    });
    fireEvent.change(screen.getByLabelText("Begründung (Pflicht)"), {
      target: { value: "Übernahme Altsystem" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Buchen" }));

    await waitFor(() => {
      expect(createOpening).toHaveBeenCalledWith("4", {
        effectiveDate: "2026-01-01",
        balanceMinutes: -150,
        note: "Übernahme Altsystem",
      });
    });
  });

  it("rejects opening balances with trailing text", () => {
    render(
      <StundenkontoPanel
        staffId="4"
        balanceMinutes={0}
        accountStartKey="2026-01-01"
        todayKey="2026-07-24"
        adjustments={[]}
        onChanged={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Eröffnungssaldo" }));
    fireEvent.change(screen.getByLabelText("Übernommener Saldo (Stunden)"), {
      target: { value: "-3,25x" },
    });
    fireEvent.change(screen.getByLabelText("Begründung (Pflicht)"), {
      target: { value: "Übernahme Altsystem" },
    });

    expect(screen.getByRole("button", { name: "Buchen" })).toBeDisabled();
  });

  it("sperrt einen zweiten Eröffnungssaldo", () => {
    render(
      <StundenkontoPanel
        staffId="4"
        balanceMinutes={180}
        accountStartKey="2026-01-01"
        todayKey="2026-07-24"
        adjustments={[
          {
            id: "21",
            type: "opening",
            minutesDelta: 180,
            effectiveDate: "2026-01-01",
            note: "Übernahme Altsystem",
            decidedBy: "9",
            decidedAt: "2026-01-02T08:00:00Z",
          },
        ]}
        onChanged={vi.fn()}
      />,
    );

    expect(
      screen.getByRole("button", { name: "Eröffnungssaldo" }),
    ).toBeDisabled();
    expect(screen.getAllByText("Eröffnungssaldo")).toHaveLength(2);
  });
});
