import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

const sessionFetch = vi.fn();
vi.mock("./session-cache", () => ({
  sessionFetch: (...args: unknown[]) =>
    (sessionFetch as (...a: unknown[]) => unknown)(...args),
}));

const downloadBlob = vi.fn();
vi.mock("./file-download", () => ({
  downloadBlob: (...args: unknown[]) =>
    (downloadBlob as (...a: unknown[]) => unknown)(...args),
  filenameFromDisposition: (response: Response) => {
    const match = /filename="([^"]+)"/.exec(
      response.headers.get("content-disposition") ?? "",
    );
    return match?.[1] ?? null;
  },
}));

import {
  exportPaymentOverview,
  fetchGuardianPayment,
  fetchPaymentOverview,
  revealGuardianPayment,
  setStudentPayer,
  updateGuardianPayment,
} from "./guardian-payment-api";

function jsonResponse(body: unknown, init?: ResponseInit): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
    ...init,
  });
}

describe("guardian-payment-api", () => {
  beforeEach(() => {
    sessionFetch.mockReset();
    downloadBlob.mockReset();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("maps the masked read", async () => {
    sessionFetch.mockResolvedValue(
      jsonResponse({
        data: {
          guardian_id: "42",
          iban_masked: "•••• 3000",
          account_holder: null,
        },
      }),
    );

    await expect(fetchGuardianPayment("42")).resolves.toEqual({
      guardianId: "42",
      ibanMasked: "•••• 3000",
      accountHolder: null,
    });
    expect(sessionFetch).toHaveBeenCalledWith("/api/guardians/42/payment");
  });

  it("reveals via POST, because the backend logs every unmasked read", async () => {
    sessionFetch.mockResolvedValue(
      jsonResponse({
        data: {
          guardian_id: "42",
          iban: "DE89370400440532013000",
          account_holder: "Sabine Schneider",
        },
      }),
    );

    const result = await revealGuardianPayment("42");

    expect(result.iban).toBe("DE89370400440532013000");
    expect(sessionFetch).toHaveBeenCalledWith(
      "/api/guardians/42/payment/reveal",
      expect.objectContaining({ method: "POST" }),
    );
  });

  it("sends the bank fields in snake_case", async () => {
    sessionFetch.mockResolvedValue(jsonResponse({ data: null }));

    await updateGuardianPayment("42", {
      iban: "DE89370400440532013000",
      accountHolder: null,
    });

    const [, init] = sessionFetch.mock.calls[0] as [string, RequestInit];
    expect(init.method).toBe("PUT");
    expect(JSON.parse(init.body as string)).toEqual({
      iban: "DE89370400440532013000",
      account_holder: null,
    });
  });

  it("surfaces the backend message instead of a bare status", async () => {
    sessionFetch.mockResolvedValue(
      jsonResponse(
        { error: "invalid payment value: malformed IBAN" },
        {
          status: 400,
        },
      ),
    );

    await expect(
      updateGuardianPayment("42", { iban: "DE00", accountHolder: null }),
    ).rejects.toThrow("malformed IBAN");
  });

  it("clears the payer with a null guardian id", async () => {
    sessionFetch.mockResolvedValue(jsonResponse({ data: null }));

    await setStudentPayer("7", null);

    const [url, init] = sessionFetch.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("/api/guardians/students/7/payer");
    expect(JSON.parse(init.body as string)).toEqual({ guardian_id: null });
  });

  it("maps overview rows and tolerates a null payload", async () => {
    sessionFetch.mockResolvedValue(
      jsonResponse({
        data: [
          {
            student_id: "1",
            student_name: "Mia Schneider",
            school_class: "1a",
            guardian_id: null,
            guardian_name: "",
            relationship_type: "",
            account_holder: "",
            iban_masked: "",
          },
        ],
      }),
    );
    await expect(fetchPaymentOverview()).resolves.toEqual([
      {
        studentId: "1",
        studentName: "Mia Schneider",
        schoolClass: "1a",
        guardianId: null,
        guardianName: "",
        relationshipType: "",
        accountHolder: "",
        ibanMasked: "",
      },
    ]);

    sessionFetch.mockResolvedValue(jsonResponse({ data: null }));
    await expect(fetchPaymentOverview()).resolves.toEqual([]);
  });

  it("saves the export under the filename the backend chose", async () => {
    sessionFetch.mockResolvedValue(
      new Response("binary", {
        status: 200,
        headers: {
          "content-disposition": 'attachment; filename="bankverbindungen.xlsx"',
        },
      }),
    );

    await exportPaymentOverview("xlsx");

    expect(downloadBlob).toHaveBeenCalledWith(
      expect.any(Blob),
      "bankverbindungen.xlsx",
    );
  });
});
