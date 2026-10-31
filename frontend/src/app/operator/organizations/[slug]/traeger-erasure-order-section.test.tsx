import { schoolDateCases, setSchoolDateClock } from "~/test/school-date-cases";
import { afterEach as restoreZoneAfterEach, vi as zoneVi } from "vitest";
restoreZoneAfterEach(() => zoneVi.unstubAllEnvs());
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { SWRConfig } from "swr";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { TraegerErasureOrder } from "~/lib/operator/provisioning-helpers";

const { mockGetOrder, mockRecordOrder, mockWithdrawOrder, mockToastSuccess } =
  vi.hoisted(() => ({
    mockGetOrder: vi.fn(),
    mockRecordOrder: vi.fn(),
    mockWithdrawOrder: vi.fn(),
    mockToastSuccess: vi.fn(),
  }));

vi.unmock("swr");

vi.mock("~/lib/operator/provisioning-api", () => ({
  operatorProvisioningService: {
    getTraegerErasureOrder: mockGetOrder,
    recordTraegerErasureOrder: mockRecordOrder,
    withdrawTraegerErasureOrder: mockWithdrawOrder,
  },
}));

vi.mock("~/components/ui/date-picker", async () => {
  const { toISODate, parseISODate } = await import("~/lib/date-helpers");
  return {
    DatePicker: ({ id, value, onChange, minDate, maxDate }: any) => (
      <input
        id={id}
        data-testid={id}
        min={minDate ? toISODate(minDate) : undefined}
        max={maxDate ? toISODate(maxDate) : undefined}
        type="date"
        value={value ? toISODate(value) : ""}
        onChange={(e) =>
          onChange(e.target.value ? parseISODate(e.target.value) : null)
        }
      />
    ),
  };
});

vi.mock("~/components/ui/modal", () => ({
  ConfirmationModal: ({
    isOpen,
    children,
    title,
    onConfirm,
    confirmText = "Bestätigen",
  }: any) =>
    isOpen ? (
      <div role="dialog" aria-label={title}>
        {children}
        <button type="button" onClick={onConfirm}>
          {confirmText}
        </button>
      </div>
    ) : null,
}));

vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({
    success: mockToastSuccess,
    error: vi.fn(),
    info: vi.fn(),
    warning: vi.fn(),
  }),
}));

import { ToastProvider } from "~/contexts/ToastContext";
import { TraegerErasureOrderSection } from "./traeger-erasure-order-section";

const openOrder: TraegerErasureOrder = {
  id: "7",
  organizationId: "1",
  orderReference: "LOESCH-4657",
  confirmedOn: "2026-09-01",
  eraseNotBefore: "2026-12-31",
  recordedAt: "2026-09-02T10:00:00Z",
  startedAt: null,
};

function renderSection(organizationActive: boolean) {
  return render(
    <ToastProvider>
      <SWRConfig value={{ provider: () => new Map(), dedupingInterval: 0 }}>
        <TraegerErasureOrderSection
          organizationId="1"
          organizationActive={organizationActive}
        />
      </SWRConfig>
    </ToastProvider>,
  );
}

describe("TraegerErasureOrderSection", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("asks to deactivate an active Träger before an erasure order", async () => {
    mockGetOrder.mockResolvedValue(null);

    renderSection(true);

    expect(
      await screen.findByText(/erst, wenn der Träger deaktiviert ist/),
    ).toBeInTheDocument();
    expect(screen.queryByLabelText(/Vorgangsnummer/)).not.toBeInTheDocument();
  });

  it("records the order of a deactivated Träger with its grace period", async () => {
    mockGetOrder.mockResolvedValueOnce(null).mockResolvedValue(openOrder);
    mockRecordOrder.mockResolvedValue(openOrder);

    renderSection(false);

    const reference = await screen.findByLabelText(/Vorgangsnummer/);
    const submit = screen.getByRole("button", {
      name: "Löschauftrag erfassen",
    });
    expect(submit).toBeDisabled();
    fireEvent.change(reference, { target: { value: "loesch-4657" } });
    fireEvent.change(screen.getByTestId("traeger-erasure-order-confirmed-on"), {
      target: { value: "2026-09-01" },
    });
    fireEvent.change(
      screen.getByTestId("traeger-erasure-order-erase-not-before"),
      { target: { value: "2026-12-31" } },
    );
    fireEvent.click(submit);

    await waitFor(() => {
      expect(mockRecordOrder).toHaveBeenCalledWith("1", {
        order_reference: "LOESCH-4657",
        confirmed_on: "2026-09-01",
        erase_not_before: "2026-12-31",
      });
    });
    expect(mockToastSuccess).toHaveBeenCalledWith(
      "Der Löschauftrag ist erfasst.",
    );
    expect(await screen.findByText("Löschauftrag erfasst")).toBeInTheDocument();
  });

  it("shows the open order and withdraws it only after the confirmation", async () => {
    mockGetOrder.mockResolvedValueOnce(openOrder).mockResolvedValue(null);
    mockWithdrawOrder.mockResolvedValue(undefined);

    renderSection(false);

    expect(await screen.findByText("LOESCH-4657")).toBeInTheDocument();
    expect(screen.getByText("01.09.2026")).toBeInTheDocument();
    expect(screen.getAllByText(/31\.12\.2026/).length).toBeGreaterThan(0);

    fireEvent.click(
      screen.getByRole("button", { name: "Löschauftrag zurücknehmen" }),
    );
    expect(mockWithdrawOrder).not.toHaveBeenCalled();
    const dialog = await screen.findByRole("dialog", {
      name: "Löschauftrag zurücknehmen?",
    });
    expect(dialog).toHaveTextContent("Der Träger bleibt deaktiviert.");
    fireEvent.click(screen.getByRole("button", { name: "Zurücknehmen" }));

    await waitFor(() => {
      expect(mockWithdrawOrder).toHaveBeenCalledWith("1");
    });
    expect(mockToastSuccess).toHaveBeenCalledWith(
      "Der Löschauftrag ist zurückgenommen.",
    );
  });

  it("offers no withdrawal once the erasure has started", async () => {
    mockGetOrder.mockResolvedValue({
      ...openOrder,
      startedAt: "2027-01-02T09:00:00Z",
    });

    renderSection(false);

    expect(await screen.findByText("Löschung läuft")).toBeInTheDocument();
    expect(
      screen.getByText(/kann nicht mehr zurückgenommen werden/),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Löschauftrag zurücknehmen" }),
    ).not.toBeInTheDocument();
  });

  it.each(schoolDateCases("2026-09-09"))(
    "uses Berlin today and tomorrow for erasure bounds in %s",
    async (zone, instant) => {
      setSchoolDateClock(zone, instant, "2026-09-09");
      mockGetOrder.mockResolvedValue(null);
      renderSection(false);
      expect(
        await screen.findByTestId("traeger-erasure-order-confirmed-on"),
      ).toHaveAttribute("max", "2026-09-09");
      expect(
        screen.getByTestId("traeger-erasure-order-erase-not-before"),
      ).toHaveAttribute("min", "2026-09-10");
    },
  );
});
