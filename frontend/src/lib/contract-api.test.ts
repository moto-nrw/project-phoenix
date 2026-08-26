import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";

import {
  fetchContractOverview,
  formatCents,
  invoiceStatusLabel,
  invoiceStatusTone,
  mapContractOverview,
  mapInvoice,
  type Invoice,
} from "./contract-api";

const { sessionFetchMock } = vi.hoisted(() => ({
  sessionFetchMock: vi.fn(),
}));

vi.mock("./session-cache", () => ({
  sessionFetch: sessionFetchMock,
}));

function backendInvoice(overrides: Record<string, unknown> = {}) {
  return {
    id: 7,
    period_label: "Januar 2026",
    invoice_number: "R-2026-001",
    amount_cents: 19900,
    due_date: "2026-01-31",
    status: "offen" as const,
    overdue: false,
    paid_on: null,
    ...overrides,
  };
}

function backendOverview(overrides: Record<string, unknown> = {}) {
  return {
    tier: "plus",
    tier_label: "Plus",
    booked_children: 150,
    active_children: 163,
    price_per_child_cents: 200,
    billing_cycle: "monatlich",
    billing_cycle_label: "Monatlich",
    term_start: "2026-01-01",
    term_end: null,
    invoice_recipient: "buchhaltung@schule.test",
    customer_number: "K-10023",
    support_email: "rechnung@moto.test",
    note: "Hinweis",
    configured: true,
    reference_date: "2026-02-15",
    invoices: [backendInvoice()],
    open_amount_cents: 19900,
    next_due: backendInvoice(),
    ...overrides,
  };
}

describe("mapInvoice", () => {
  it("turns the int64 id into a string (Projektkonvention)", () => {
    expect(mapInvoice(backendInvoice()).id).toBe("7");
  });

  it("keeps calendar dates as YYYY-MM-DD strings", () => {
    const invoice = mapInvoice(
      backendInvoice({ paid_on: "2026-02-03", status: "bezahlt" }),
    );
    expect(invoice.dueDate).toBe("2026-01-31");
    expect(invoice.paidOn).toBe("2026-02-03");
  });

  it("maps a missing payment date to null instead of undefined", () => {
    expect(
      mapInvoice(backendInvoice({ paid_on: undefined })).paidOn,
    ).toBeNull();
  });

  it("does not expose operator-only notes on school invoices", () => {
    expect(
      mapInvoice(backendInvoice({ note: "nur intern" })),
    ).not.toHaveProperty("note");
  });
});

describe("mapContractOverview", () => {
  it("maps every snake_case field to camelCase", () => {
    const overview = mapContractOverview(backendOverview());

    expect(overview.tierLabel).toBe("Plus");
    expect(overview.bookedChildren).toBe(150);
    expect(overview.activeChildren).toBe(163);
    expect(overview.pricePerChildCents).toBe(200);
    expect(overview.billingCycleLabel).toBe("Monatlich");
    expect(overview.termStart).toBe("2026-01-01");
    expect(overview.invoiceRecipient).toBe("buchhaltung@schule.test");
    expect(overview.customerNumber).toBe("K-10023");
    expect(overview.supportEmail).toBe("rechnung@moto.test");
    expect(overview.openAmountCents).toBe(19900);
    expect(overview.referenceDate).toBe("2026-02-15");
  });

  it("turns absent optional dates into null", () => {
    const overview = mapContractOverview(
      backendOverview({ term_start: undefined, term_end: undefined }),
    );
    expect(overview.termStart).toBeNull();
    expect(overview.termEnd).toBeNull();
  });

  it("treats a null invoice list as empty so the page never crashes", () => {
    const overview = mapContractOverview(
      backendOverview({ invoices: null, next_due: null }),
    );
    expect(overview.invoices).toEqual([]);
    expect(overview.nextDue).toBeNull();
  });

  it("maps the next-due invoice when one is present", () => {
    const overview = mapContractOverview(backendOverview());
    expect(overview.nextDue?.id).toBe("7");
  });
});

describe("fetchContractOverview", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("requests the contract endpoint and unwraps the envelope", async () => {
    sessionFetchMock.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ data: backendOverview() }),
    });

    const overview = await fetchContractOverview();

    expect(sessionFetchMock).toHaveBeenCalledWith("/api/contract");
    expect(overview.tier).toBe("plus");
  });

  it("throws on a failed response instead of returning an empty contract", async () => {
    sessionFetchMock.mockResolvedValue({
      ok: false,
      statusText: "Forbidden",
    });

    await expect(fetchContractOverview()).rejects.toThrow("Forbidden");
  });
});

describe("formatCents", () => {
  it("renders integer cents as German euro amounts", () => {
    // Non-breaking space before the currency symbol; normalise for the assert.
    expect(formatCents(19900).replace(/\s/g, " ")).toBe("199,00 €");
    expect(formatCents(0).replace(/\s/g, " ")).toBe("0,00 €");
    expect(formatCents(1).replace(/\s/g, " ")).toBe("0,01 €");
  });
});

function invoice(overrides: Partial<Invoice> = {}): Invoice {
  return {
    id: "1",
    periodLabel: "Januar 2026",
    invoiceNumber: "",
    amountCents: 100,
    dueDate: "2026-01-31",
    status: "offen",
    overdue: false,
    paidOn: null,
    ...overrides,
  };
}

describe("invoiceStatusLabel", () => {
  it("names the three stored statuses", () => {
    expect(invoiceStatusLabel(invoice({ status: "offen" }))).toBe("Offen");
    expect(invoiceStatusLabel(invoice({ status: "bezahlt" }))).toBe("Bezahlt");
    expect(invoiceStatusLabel(invoice({ status: "storniert" }))).toBe(
      "Storniert",
    );
  });

  it("shows the derived overdue state for an open, late invoice", () => {
    expect(invoiceStatusLabel(invoice({ overdue: true }))).toBe("Überfällig");
  });

  it("never calls a paid or cancelled invoice overdue", () => {
    expect(
      invoiceStatusLabel(invoice({ status: "bezahlt", overdue: true })),
    ).toBe("Bezahlt");
    expect(
      invoiceStatusLabel(invoice({ status: "storniert", overdue: true })),
    ).toBe("Storniert");
  });
});

describe("invoiceStatusTone", () => {
  it("gives every status a distinct tone", () => {
    const tones = [
      invoiceStatusTone(invoice({ status: "offen" })),
      invoiceStatusTone(invoice({ status: "offen", overdue: true })),
      invoiceStatusTone(invoice({ status: "bezahlt" })),
      invoiceStatusTone(invoice({ status: "storniert" })),
    ];
    expect(tones).toEqual(["orange", "red", "green", "gray"]);
    expect(new Set(tones).size).toBe(4);
  });
});
