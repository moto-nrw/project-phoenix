import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

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
  },
}));

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
});
