import { schoolDateCases, setSchoolDateClock } from "~/test/school-date-cases";
import { afterEach as restoreZoneAfterEach, vi as zoneVi } from "vitest";
restoreZoneAfterEach(() => zoneVi.unstubAllEnvs());
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: {
      user: { id: "account-1", tenantId: 1, scope: "operator" },
      expires: "2099-01-01",
    },
    status: "authenticated",
  }),
}));
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { SWRConfig } from "~/lib/swr/testing";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { TraegerErasureOrder } from "~/lib/operator/provisioning-helpers";

const {
  mockGetOrder,
  mockRecordOrder,
  mockRecordRecords,
  mockWithdrawOrder,
  mockToastSuccess,
} = vi.hoisted(() => ({
  mockGetOrder: vi.fn(),
  mockRecordOrder: vi.fn(),
  mockRecordRecords: vi.fn(),
  mockWithdrawOrder: vi.fn(),
  mockToastSuccess: vi.fn(),
}));

vi.unmock("swr");

vi.mock("~/lib/operator/provisioning-api", () => ({
  operatorProvisioningService: {
    getTraegerErasureOrder: mockGetOrder,
    recordTraegerErasureOrder: mockRecordOrder,
    recordTraegerErasureOrderRecords: mockRecordRecords,
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
  recordsChoice: "erasure_chosen",
  recordsHandedOverOn: null,
  recordsExportSha256: null,
};

function fillOrderFields(reference: HTMLElement) {
  fireEvent.change(reference, { target: { value: "loesch-4657" } });
  fireEvent.change(screen.getByTestId("traeger-erasure-order-confirmed-on"), {
    target: { value: "2026-09-01" },
  });
  fireEvent.change(
    screen.getByTestId("traeger-erasure-order-erase-not-before"),
    { target: { value: "2026-12-31" } },
  );
}

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
    // The Träger-Büro loses its downloads with the deactivation (#4720).
    expect(screen.getByText(/Vor dem Deaktivieren/)).toBeInTheDocument();
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
    fillOrderFields(reference);
    // The order waits for the hint on the records and the customer's choice
    // (#4720, AVV Ziff. 8.4).
    expect(submit).toBeDisabled();
    fireEvent.click(
      screen.getByLabelText(/auf die Sicherung seiner Unterlagen hingewiesen/),
    );
    expect(submit).toBeDisabled();
    fireEvent.click(
      screen.getByRole("button", { name: "Kunde wählt Löschung" }),
    );
    fireEvent.click(submit);

    await waitFor(() => {
      expect(mockRecordOrder).toHaveBeenCalledWith("1", {
        order_reference: "LOESCH-4657",
        confirmed_on: "2026-09-01",
        erase_not_before: "2026-12-31",
        records_notice_acknowledged: true,
        records_choice: "erasure_chosen",
      });
    });
    expect(mockToastSuccess).toHaveBeenCalledWith(
      "Der Löschauftrag ist erfasst.",
    );
    expect(await screen.findByText("Löschauftrag erfasst")).toBeInTheDocument();
  });

  it("points to the records and offers the Rückgabe export before the order", async () => {
    mockGetOrder.mockResolvedValue(null);

    renderSection(false);

    expect(
      await screen.findByText(/aufbewahrungspflichtigen Unterlagen/),
    ).toBeInTheDocument();
    expect(screen.getByText(/AVV Ziff\. 8\.4/)).toBeInTheDocument();
    expect(
      screen.getByText(/tenant export --organization 1 --operator .* --out/),
    ).toBeInTheDocument();
  });

  it("records the handover of the Rückgabe export with its day and checksum", async () => {
    mockGetOrder.mockResolvedValueOnce(null).mockResolvedValue(openOrder);
    mockRecordOrder.mockResolvedValue(openOrder);

    renderSection(false);

    fillOrderFields(await screen.findByLabelText(/Vorgangsnummer/));
    fireEvent.click(
      screen.getByLabelText(/auf die Sicherung seiner Unterlagen hingewiesen/),
    );
    fireEvent.click(screen.getByRole("button", { name: "Rückgabe übergeben" }));
    const submit = screen.getByRole("button", {
      name: "Löschauftrag erfassen",
    });
    expect(submit).toBeDisabled();
    expect(
      screen.getByTestId("traeger-erasure-order-records-handed-over-on"),
    ).toHaveAttribute("max", "2026-09-09");
    fireEvent.change(
      screen.getByTestId("traeger-erasure-order-records-handed-over-on"),
      { target: { value: "2026-09-05" } },
    );
    fireEvent.change(screen.getByLabelText(/Prüfsumme/), {
      target: { value: ` ${"AB".repeat(32)} ` },
    });
    fireEvent.click(submit);

    await waitFor(() => {
      expect(mockRecordOrder).toHaveBeenCalledWith("1", {
        order_reference: "LOESCH-4657",
        confirmed_on: "2026-09-01",
        erase_not_before: "2026-12-31",
        records_notice_acknowledged: true,
        records_choice: "handed_over",
        records_handed_over_on: "2026-09-05",
        records_export_sha256: "ab".repeat(32),
      });
    });
  });

  it("ignores an invalid handover checksum once the customer chooses erasure", async () => {
    mockGetOrder.mockResolvedValueOnce(null).mockResolvedValue(openOrder);
    mockRecordOrder.mockResolvedValue(openOrder);

    renderSection(false);

    fillOrderFields(await screen.findByLabelText(/Vorgangsnummer/));
    fireEvent.click(
      screen.getByLabelText(/auf die Sicherung seiner Unterlagen hingewiesen/),
    );
    fireEvent.click(screen.getByRole("button", { name: "Rückgabe übergeben" }));
    fireEvent.change(screen.getByLabelText(/Prüfsumme/), {
      target: { value: "keine-pruefsumme" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Kunde wählt Löschung" }),
    );
    const submit = screen.getByRole("button", {
      name: "Löschauftrag erfassen",
    });
    expect(submit).toBeEnabled();
    fireEvent.click(submit);

    await waitFor(() => {
      expect(mockRecordOrder).toHaveBeenCalledWith("1", {
        order_reference: "LOESCH-4657",
        confirmed_on: "2026-09-01",
        erase_not_before: "2026-12-31",
        records_notice_acknowledged: true,
        records_choice: "erasure_chosen",
      });
    });
  });

  it("warns that an open order without the customer's choice is not executed", async () => {
    mockGetOrder.mockResolvedValue({ ...openOrder, recordsChoice: null });

    renderSection(false);

    expect(
      await screen.findByText(/ohne Hinweis auf die Unterlagen erfasst/),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Tragen Sie Hinweis und Wahl hier nach/),
    ).toBeInTheDocument();
  });

  it("records the hint and the choice on a started order without them", async () => {
    const started = {
      ...openOrder,
      startedAt: "2027-01-02T09:00:00Z",
      recordsChoice: null,
    };
    mockGetOrder
      .mockResolvedValueOnce(started)
      .mockResolvedValue({ ...started, recordsChoice: "erasure_chosen" });
    mockRecordRecords.mockResolvedValue({
      ...started,
      recordsChoice: "erasure_chosen",
    });

    renderSection(false);

    expect(
      await screen.findByText(/läuft erst weiter, wenn Hinweis und Wahl/),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Löschauftrag zurücknehmen" }),
    ).not.toBeInTheDocument();
    const submit = screen.getByRole("button", { name: "Wahl eintragen" });
    expect(submit).toBeDisabled();
    fireEvent.click(
      screen.getByLabelText(/auf die Sicherung seiner Unterlagen hingewiesen/),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Kunde wählt Löschung" }),
    );
    expect(submit).toBeEnabled();
    fireEvent.click(submit);

    await waitFor(() => {
      expect(mockRecordRecords).toHaveBeenCalledWith("1", {
        records_notice_acknowledged: true,
        records_choice: "erasure_chosen",
      });
    });
    expect(mockToastSuccess).toHaveBeenCalledWith("Die Wahl ist eingetragen.");
    expect(await screen.findByText("Kunde wählt Löschung")).toBeInTheDocument();
  });

  it("shows the recorded handover of an open order", async () => {
    mockGetOrder.mockResolvedValue({
      ...openOrder,
      recordsChoice: "handed_over",
      recordsHandedOverOn: "2026-09-05",
    });

    renderSection(false);

    expect(
      await screen.findByText("Rückgabe übergeben am 05.09.2026"),
    ).toBeInTheDocument();
  });

  it("shows the open order and withdraws it only after the confirmation", async () => {
    mockGetOrder.mockResolvedValueOnce(openOrder).mockResolvedValue(null);
    mockWithdrawOrder.mockResolvedValue(undefined);

    renderSection(false);

    expect(await screen.findByText("LOESCH-4657")).toBeInTheDocument();
    expect(screen.getByText("01.09.2026")).toBeInTheDocument();
    expect(screen.getByText("Kunde wählt Löschung")).toBeInTheDocument();
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
