import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { SchoolInvoicesPanel } from "./school-invoices-panel";
import type { SchoolInvoice } from "~/lib/operator/school-invoices-api";

const { swrState, mutateMock, listMock, createMock, updateMock, deleteMock } =
  vi.hoisted(() => ({
    swrState: {
      data: undefined as SchoolInvoice[] | undefined,
      error: undefined as unknown,
      isLoading: false,
    },
    mutateMock: vi.fn(),
    listMock: vi.fn(),
    createMock: vi.fn(),
    updateMock: vi.fn(),
    deleteMock: vi.fn(),
  }));

vi.mock("swr", () => ({
  default: () => ({
    data: swrState.data,
    error: swrState.error,
    isLoading: swrState.isLoading,
    mutate: mutateMock,
  }),
}));

vi.mock("~/lib/operator/school-invoices-api", async () => {
  const actual = await vi.importActual<
    typeof import("~/lib/operator/school-invoices-api")
  >("~/lib/operator/school-invoices-api");
  return {
    ...actual,
    listSchoolInvoices: listMock,
    createSchoolInvoice: createMock,
    updateSchoolInvoice: updateMock,
    deleteSchoolInvoice: deleteMock,
  };
});

function invoice(overrides: Partial<SchoolInvoice> = {}): SchoolInvoice {
  return {
    id: "12",
    periodLabel: "Januar 2026",
    invoiceNumber: "R-1",
    amountCents: 19900,
    dueDate: "2026-01-31",
    status: "offen",
    overdue: false,
    paidOn: null,
    note: "",
    ...overrides,
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  swrState.data = [invoice()];
  swrState.error = undefined;
  swrState.isLoading = false;
  mutateMock.mockResolvedValue(undefined);
});

function openCreateForm() {
  fireEvent.click(screen.getByRole("button", { name: "Rechnung anlegen" }));
}

describe("SchoolInvoicesPanel", () => {
  it("lists the schedule with amount and status", () => {
    render(<SchoolInvoicesPanel schoolId="42" />);

    expect(screen.getByText("Januar 2026")).toBeInTheDocument();
    expect(screen.getByText("Offen")).toBeInTheDocument();
    expect(screen.getByText("31.01.2026")).toBeInTheDocument();
  });

  it("tells the operator where tier and contingent live", () => {
    render(<SchoolInvoicesPanel schoolId="42" />);

    expect(
      screen.getByText(/Tarif und gebuchte Kinderzahl/),
    ).toBeInTheDocument();
  });

  it("shows an empty state before the first invoice", () => {
    swrState.data = [];

    render(<SchoolInvoicesPanel schoolId="42" />);

    expect(screen.getByText("Noch keine Rechnungen")).toBeInTheDocument();
  });

  it("surfaces a load failure", () => {
    swrState.data = undefined;
    swrState.error = new Error("kaputt");

    render(<SchoolInvoicesPanel schoolId="42" />);

    expect(screen.getByText("kaputt")).toBeInTheDocument();
  });

  it("creates an invoice from the form", async () => {
    createMock.mockResolvedValue(invoice());

    render(<SchoolInvoicesPanel schoolId="42" />);
    openCreateForm();

    fireEvent.change(screen.getByLabelText("Zeitraum"), {
      target: { value: "Februar 2026" },
    });
    fireEvent.change(screen.getByLabelText("Betrag in Euro"), {
      target: { value: "19,90" },
    });
    fireEvent.change(screen.getByLabelText("Fällig am"), {
      target: { value: "2026-02-28" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(createMock).toHaveBeenCalledWith("42", {
        periodLabel: "Februar 2026",
        // Das Anlege-Formular startet leer — die Rechnungsnummer wird oft
        // erst später nachgetragen.
        invoiceNumber: "",
        amountCents: 1990,
        dueDate: "2026-02-28",
        status: "offen",
        paidOn: null,
        note: "",
      });
    });
    expect(mutateMock).toHaveBeenCalled();
  });

  it("refuses to save without a period label", async () => {
    render(<SchoolInvoicesPanel schoolId="42" />);
    openCreateForm();

    fireEvent.change(screen.getByLabelText("Betrag in Euro"), {
      target: { value: "19,90" },
    });
    fireEvent.change(screen.getByLabelText("Fällig am"), {
      target: { value: "2026-02-28" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(await screen.findByText(/Zeitraum eintragen/)).toBeInTheDocument();
    expect(createMock).not.toHaveBeenCalled();
  });

  // Ein stiller 0-€-Betrag würde der Schule sagen, sie schulde nichts.
  it("refuses to save an amount it cannot parse", async () => {
    render(<SchoolInvoicesPanel schoolId="42" />);
    openCreateForm();

    fireEvent.change(screen.getByLabelText("Zeitraum"), {
      target: { value: "Februar 2026" },
    });
    fireEvent.change(screen.getByLabelText("Betrag in Euro"), {
      target: { value: "ungefähr zwanzig" },
    });
    fireEvent.change(screen.getByLabelText("Fällig am"), {
      target: { value: "2026-02-28" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(await screen.findByText(/Betrag wie 19,90/)).toBeInTheDocument();
    expect(createMock).not.toHaveBeenCalled();
  });

  it("refuses to save without a due date", async () => {
    render(<SchoolInvoicesPanel schoolId="42" />);
    openCreateForm();

    fireEvent.change(screen.getByLabelText("Zeitraum"), {
      target: { value: "Februar 2026" },
    });
    fireEvent.change(screen.getByLabelText("Betrag in Euro"), {
      target: { value: "19,90" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(/Fälligkeitsdatum wählen/),
    ).toBeInTheDocument();
    expect(createMock).not.toHaveBeenCalled();
  });

  it("prefills the form when editing an existing invoice", () => {
    render(<SchoolInvoicesPanel schoolId="42" />);

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));

    expect(screen.getByLabelText("Zeitraum")).toHaveValue("Januar 2026");
    expect(screen.getByLabelText("Betrag in Euro")).toHaveValue("199,00");
    expect(screen.getByLabelText("Fällig am")).toHaveValue("2026-01-31");
  });

  it("updates the invoice it was opened with", async () => {
    updateMock.mockResolvedValue(invoice());

    render(<SchoolInvoicesPanel schoolId="42" />);
    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));

    fireEvent.change(screen.getByLabelText("Betrag in Euro"), {
      target: { value: "250,00" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(updateMock).toHaveBeenCalledWith(
        "42",
        "12",
        expect.objectContaining({ amountCents: 25000 }),
      );
    });
  });

  // Das Zahlungsdatum ist nur beim Status "bezahlt" sichtbar — sonst würde man
  // ein Feld ausfüllen, das nichts bewirkt.
  it("hides the payment-date field unless the status is bezahlt", () => {
    render(<SchoolInvoicesPanel schoolId="42" />);
    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));

    expect(screen.queryByLabelText("Zahlung eingegangen am")).toBeNull();
  });

  it("shows the payment-date field for a paid invoice", () => {
    swrState.data = [invoice({ status: "bezahlt", paidOn: "2026-02-03" })];

    render(<SchoolInvoicesPanel schoolId="42" />);
    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));

    expect(screen.getByLabelText("Zahlung eingegangen am")).toHaveValue(
      "2026-02-03",
    );
  });

  it("refuses to mark an invoice paid without a payment date", async () => {
    swrState.data = [invoice({ status: "bezahlt", paidOn: null })];

    render(<SchoolInvoicesPanel schoolId="42" />);
    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(/Zahlung eingegangen ist/),
    ).toBeInTheDocument();
    expect(updateMock).not.toHaveBeenCalled();
  });

  it("reports a save failure instead of closing silently", async () => {
    createMock.mockRejectedValue(new Error("Rechnungsnummer schon vergeben"));

    render(<SchoolInvoicesPanel schoolId="42" />);
    openCreateForm();

    fireEvent.change(screen.getByLabelText("Zeitraum"), {
      target: { value: "Februar 2026" },
    });
    fireEvent.change(screen.getByLabelText("Betrag in Euro"), {
      target: { value: "19,90" },
    });
    fireEvent.change(screen.getByLabelText("Fällig am"), {
      target: { value: "2026-02-28" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText("Rechnungsnummer schon vergeben"),
    ).toBeInTheDocument();
  });

  it("deletes an invoice after the confirmation", async () => {
    deleteMock.mockResolvedValue(undefined);

    render(<SchoolInvoicesPanel schoolId="42" />);
    fireEvent.click(screen.getByRole("button", { name: "Löschen" }));

    const dialog = await screen.findByRole("dialog", {
      name: "Rechnung löschen",
    });
    fireEvent.click(within(dialog).getByRole("button", { name: "Löschen" }));

    await waitFor(() => {
      expect(deleteMock).toHaveBeenCalledWith("42", "12");
    });
  });

  // Eine zurückgezogene, aber echte Rechnung gehört auf "Storniert" — sonst
  // fehlt sie in der Nachvollziehbarkeit.
  it("suggests cancelling instead of deleting a real invoice", async () => {
    render(<SchoolInvoicesPanel schoolId="42" />);
    fireEvent.click(screen.getByRole("button", { name: "Löschen" }));

    expect(await screen.findByText(/auf „Storniert“/)).toBeInTheDocument();
  });

  it("reports a delete failure", async () => {
    deleteMock.mockRejectedValue(new Error("geht nicht"));

    render(<SchoolInvoicesPanel schoolId="42" />);
    fireEvent.click(screen.getByRole("button", { name: "Löschen" }));

    const dialog = await screen.findByRole("dialog", {
      name: "Rechnung löschen",
    });
    fireEvent.click(within(dialog).getByRole("button", { name: "Löschen" }));

    expect(await screen.findByText("geht nicht")).toBeInTheDocument();
  });
});
