import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "./api-error";
import {
  cancelCareInvoice,
  downloadCareLetter,
  fetchCareLetters,
  mailCareLetter,
  mailCareLetters,
  printCareLetters,
} from "./traeger-care-billing-api";

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

const INVOICE = {
  invoice_id: 41,
  kind: "invoice",
  number: "MB-2027-0001",
  invoice_number: "MB-2027-0001",
  issued_on: "2027-01-04",
  child_first_name: "Mia",
  child_last_name: "Beispiel",
  school_name: "OGS Am Lindenhof",
  recipient_first_name: "Eva",
  recipient_last_name: "Beispiel",
  recipient_email: "eva@example.org",
  total_cents: 9600,
  cancelled: true,
  cancellation_number: "MB-2027-0009",
  has_letter: true,
  status: "sent",
  sends: 1,
  attempts: 1,
  requested_at: "2027-01-05T08:00:00Z",
  sent_at: "2027-01-05T08:01:00Z",
  mail_outcome: "delivered",
};

const STORNO = {
  ...INVOICE,
  kind: "cancellation",
  number: "MB-2027-0009",
  total_cents: -9600,
  reason: "Falscher Empfänger",
  cancelled: false,
  cancellation_number: undefined,
  status: "not_sent",
  sends: 0,
  attempts: 0,
  requested_at: null,
  sent_at: null,
  mail_outcome: undefined,
};

describe("traeger-care-billing-api letters (#4684)", () => {
  const fetchMock = vi.fn<typeof fetch>();

  beforeEach(() => {
    fetchMock.mockReset();
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("maps the letters, each invoice and its Storno, and the counts", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(200, {
        data: {
          letters: {
            payment_due_on: "2027-01-20",
            notes: "",
            created_at: "2027-01-04T09:00:00Z",
          },
          documents: [INVOICE, STORNO],
          summary: {
            letters: 2,
            sent: 1,
            queued: 0,
            failed: 0,
            not_sent: 1,
            without_email: 0,
          },
        },
      }),
    );

    const letters = await fetchCareLetters("7");

    expect(String(fetchMock.mock.calls[0]![0])).toBe(
      "/api/traeger/care-billing-runs/7/letters",
    );
    expect(letters.letters).toEqual({
      paymentDueOn: "2027-01-20",
      notes: "",
      createdAt: "2027-01-04T09:00:00Z",
    });
    expect(letters.documents[0]).toMatchObject({
      invoiceId: "41",
      kind: "invoice",
      cancelled: true,
      cancellationNumber: "MB-2027-0009",
      reason: "",
      mailOutcome: "delivered",
    });
    expect(letters.documents[1]).toMatchObject({
      invoiceId: "41",
      kind: "cancellation",
      number: "MB-2027-0009",
      invoiceNumber: "MB-2027-0001",
      totalCents: -9600,
      reason: "Falscher Empfänger",
      cancellationNumber: "",
    });
    expect(letters.summary).toEqual({
      letters: 2,
      sent: 1,
      queued: 0,
      failed: 0,
      notSent: 1,
      withoutEmail: 0,
    });
  });

  it("reads a run without letters yet", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(200, {
        data: { letters: null, documents: null, summary: null },
      }),
    );

    const letters = await fetchCareLetters("7");

    expect(letters.letters).toBeNull();
    expect(letters.documents).toEqual([]);
    expect(letters.summary.letters).toBe(0);
  });

  it("sends the letter of a Storno through its own route", async () => {
    fetchMock.mockResolvedValue(jsonResponse(202, { data: STORNO }));

    const mailing = await mailCareLetter("7", "41", "cancellation");

    const [url, init] = fetchMock.mock.calls[0]!;
    expect(String(url)).toBe(
      "/api/traeger/care-billing-runs/7/invoices/41/cancellation/mail",
    );
    expect(init?.method).toBe("POST");
    expect(mailing.kind).toBe("cancellation");
  });

  it("sends the letter of an invoice", async () => {
    fetchMock.mockResolvedValue(jsonResponse(202, { data: INVOICE }));

    await mailCareLetter("7", "41", "invoice");

    expect(String(fetchMock.mock.calls[0]![0])).toBe(
      "/api/traeger/care-billing-runs/7/invoices/41/mail",
    );
  });

  it("counts the letters a send to all held back", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(202, {
        data: { queued: 3, already: 1, without_email: 2, recipient_changed: 1 },
      }),
    );

    expect(await mailCareLetters("7")).toEqual({
      queued: 3,
      already: 1,
      withoutEmail: 2,
      recipientChanged: 1,
    });
    expect(String(fetchMock.mock.calls[0]![0])).toBe(
      "/api/traeger/care-billing-runs/7/letters/mail",
    );
  });

  it("cancels with the reason", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(201, {
        data: { letters: null, documents: [], summary: null },
      }),
    );

    await cancelCareInvoice("7", "41", "Falscher Empfänger");

    const [url, init] = fetchMock.mock.calls[0]!;
    expect(String(url)).toBe(
      "/api/traeger/care-billing-runs/7/invoices/41/cancellation",
    );
    expect(JSON.parse(String(init?.body))).toEqual({
      reason: "Falscher Empfänger",
    });
  });

  it("names the files to print and to show", async () => {
    fetchMock.mockImplementation(async () => new Response("%PDF"));

    await printCareLetters("7");
    await printCareLetters("7", true);
    await downloadCareLetter("7", "41", "invoice");
    await downloadCareLetter("7", "41", "cancellation");

    expect(fetchMock.mock.calls.map(([url]) => String(url))).toEqual([
      "/api/traeger/care-billing-runs/7/letters/print",
      "/api/traeger/care-billing-runs/7/letters/print?only=without_email",
      "/api/traeger/care-billing-runs/7/invoices/41/letter",
      "/api/traeger/care-billing-runs/7/invoices/41/letter?kind=cancellation",
    ]);
  });

  it("keeps the code of a refused send", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(409, {
        status: 409,
        code: "parent_billing.care_recipient_changed",
      }),
    );

    const failure = await mailCareLetter("7", "41", "invoice").catch(
      (err: unknown) => err,
    );

    expect(failure).toBeInstanceOf(ApiError);
    expect((failure as ApiError).code).toBe(
      "parent_billing.care_recipient_changed",
    );
  });
});
