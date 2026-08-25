import { describe, expect, it, vi, beforeEach } from "vitest";

import {
  centsToEuroInput,
  createSchoolInvoice,
  deleteSchoolInvoice,
  formatCents,
  invoiceStatusLabel,
  invoiceStatusTone,
  listSchoolInvoices,
  mapSchoolInvoice,
  parseEuroToCents,
  updateSchoolInvoice,
  type SchoolInvoice,
  type SchoolInvoiceInput,
} from "./school-invoices-api";

const { operatorFetchMock } = vi.hoisted(() => ({
  operatorFetchMock: vi.fn(),
}));

vi.mock("./api-helpers", () => ({
  operatorFetch: operatorFetchMock,
}));

function backendInvoice(overrides: Record<string, unknown> = {}) {
  return {
    id: 12,
    period_label: "Januar 2026",
    invoice_number: "R-1",
    amount_cents: 19900,
    due_date: "2026-01-31",
    status: "offen" as const,
    overdue: true,
    paid_on: null,
    note: "intern",
    ...overrides,
  };
}

function input(
  overrides: Partial<SchoolInvoiceInput> = {},
): SchoolInvoiceInput {
  return {
    periodLabel: "Januar 2026",
    invoiceNumber: "R-1",
    amountCents: 19900,
    dueDate: "2026-01-31",
    status: "offen",
    paidOn: null,
    note: "intern",
    ...overrides,
  };
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe("mapSchoolInvoice", () => {
  it("maps snake_case to camelCase and stringifies the id", () => {
    const invoice = mapSchoolInvoice(backendInvoice());

    expect(invoice.id).toBe("12");
    expect(invoice.periodLabel).toBe("Januar 2026");
    expect(invoice.amountCents).toBe(19900);
    expect(invoice.dueDate).toBe("2026-01-31");
    expect(invoice.overdue).toBe(true);
    expect(invoice.paidOn).toBeNull();
  });
});

describe("listSchoolInvoices", () => {
  it("calls the school-scoped endpoint", async () => {
    operatorFetchMock.mockResolvedValue([backendInvoice()]);

    const invoices = await listSchoolInvoices("42");

    expect(operatorFetchMock).toHaveBeenCalledWith(
      "/api/operator/provisioning/schools/42/invoices",
    );
    expect(invoices).toHaveLength(1);
  });

  it("treats a null payload as an empty schedule", async () => {
    operatorFetchMock.mockResolvedValue(null);

    await expect(listSchoolInvoices("42")).resolves.toEqual([]);
  });
});

describe("createSchoolInvoice", () => {
  it("POSTs the snake_case body the backend expects", async () => {
    operatorFetchMock.mockResolvedValue(backendInvoice());

    await createSchoolInvoice("42", input());

    expect(operatorFetchMock).toHaveBeenCalledWith(
      "/api/operator/provisioning/schools/42/invoices",
      {
        method: "POST",
        body: {
          period_label: "Januar 2026",
          invoice_number: "R-1",
          amount_cents: 19900,
          due_date: "2026-01-31",
          status: "offen",
          paid_on: "",
          note: "intern",
        },
      },
    );
  });

  it("sends an empty string for a missing payment date so the backend clears it", async () => {
    operatorFetchMock.mockResolvedValue(backendInvoice());

    await createSchoolInvoice("42", input({ paidOn: null }));

    const call = operatorFetchMock.mock.calls[0] as [
      string,
      { body: Record<string, unknown> },
    ];
    expect(call[1].body.paid_on).toBe("");
  });
});

describe("updateSchoolInvoice", () => {
  it("PUTs to the invoice-scoped endpoint", async () => {
    operatorFetchMock.mockResolvedValue(
      backendInvoice({ status: "bezahlt", paid_on: "2026-02-03" }),
    );

    const invoice = await updateSchoolInvoice(
      "42",
      "12",
      input({ status: "bezahlt", paidOn: "2026-02-03" }),
    );

    expect(operatorFetchMock).toHaveBeenCalledWith(
      "/api/operator/provisioning/schools/42/invoices/12",
      expect.objectContaining({ method: "PUT" }),
    );
    expect(invoice.paidOn).toBe("2026-02-03");
  });
});

describe("deleteSchoolInvoice", () => {
  it("DELETEs the invoice-scoped endpoint", async () => {
    operatorFetchMock.mockResolvedValue({});

    await deleteSchoolInvoice("42", "12");

    expect(operatorFetchMock).toHaveBeenCalledWith(
      "/api/operator/provisioning/schools/42/invoices/12",
      { method: "DELETE" },
    );
  });
});

describe("parseEuroToCents", () => {
  it("accepts German and English decimal separators", () => {
    expect(parseEuroToCents("19,90")).toBe(1990);
    expect(parseEuroToCents("19.90")).toBe(1990);
    expect(parseEuroToCents(" 19,9 ")).toBe(1990);
    expect(parseEuroToCents("199")).toBe(19900);
    expect(parseEuroToCents("0")).toBe(0);
  });

  // Rejecting instead of coercing matters: a silent 0 would tell the school
  // it owes nothing.
  it("rejects anything that is not an amount", () => {
    expect(parseEuroToCents("")).toBeNull();
    expect(parseEuroToCents("abc")).toBeNull();
    expect(parseEuroToCents("-5")).toBeNull();
    expect(parseEuroToCents("19,905")).toBeNull();
    expect(parseEuroToCents("1,2,3")).toBeNull();
  });

  it("rounds to whole cents rather than storing a float", () => {
    expect(parseEuroToCents("0.07")).toBe(7);
    expect(parseEuroToCents("1234.56")).toBe(123456);
  });
});

describe("centsToEuroInput", () => {
  it("round-trips with parseEuroToCents", () => {
    for (const cents of [0, 1, 1990, 123456]) {
      expect(parseEuroToCents(centsToEuroInput(cents))).toBe(cents);
    }
  });

  it("uses the German decimal comma", () => {
    expect(centsToEuroInput(1990)).toBe("19,90");
  });
});

describe("formatCents", () => {
  it("renders euro amounts", () => {
    expect(formatCents(1990).replace(/\s/g, " ")).toBe("19,90 €");
  });
});

function invoice(overrides: Partial<SchoolInvoice> = {}): SchoolInvoice {
  return {
    id: "1",
    periodLabel: "Januar 2026",
    invoiceNumber: "",
    amountCents: 100,
    dueDate: "2026-01-31",
    status: "offen",
    overdue: false,
    paidOn: null,
    note: "",
    ...overrides,
  };
}

describe("invoice status display", () => {
  it("matches the tenant-side labels and tones", () => {
    expect(invoiceStatusLabel(invoice())).toBe("Offen");
    expect(invoiceStatusLabel(invoice({ overdue: true }))).toBe("Überfällig");
    expect(invoiceStatusLabel(invoice({ status: "bezahlt" }))).toBe("Bezahlt");
    expect(invoiceStatusLabel(invoice({ status: "storniert" }))).toBe(
      "Storniert",
    );

    expect(invoiceStatusTone(invoice())).toBe("orange");
    expect(invoiceStatusTone(invoice({ overdue: true }))).toBe("red");
    expect(invoiceStatusTone(invoice({ status: "bezahlt" }))).toBe("green");
    expect(invoiceStatusTone(invoice({ status: "storniert" }))).toBe("gray");
  });
});
