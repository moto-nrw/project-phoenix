import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("./session-cache", () => ({
  sessionFetch: vi.fn(),
}));

import { sessionFetch } from "./session-cache";
import {
  duplicateLohnartNumbers,
  fetchPayrollDispatch,
  fetchPayrollStatus,
  mapPayrollStatus,
  PAYROLL_DISPATCH_EMAIL_PATTERN,
  payrollDispatchReasonText,
  payrollMonthLabel,
  savePayrollSetting,
  type PayrollStatus,
} from "./payroll-api";

const mockedSessionFetch = vi.mocked(sessionFetch);

const backendStatus = {
  categories: [
    {
      id: "regelarbeit",
      label: "Regelarbeit",
      number: "9001",
      unit_required: false,
      setting_key: "payroll.lohnart_regelarbeit",
    },
    {
      id: "krank",
      label: "Krank",
      number: "9002",
      unit: "tage",
      unit_required: true,
      setting_key: "payroll.lohnart_krank",
      unit_setting_key: "payroll.einheit_krank",
    },
  ],
  beraternummer: "9999999",
  mandantennummer: "",
  lodas_header_complete: false,
  configured_categories: 2,
  total_categories: 7,
  staff_total: 21,
  staff_without_personnel_number: 19,
  dispatch: {
    auto: true,
    channel: "email",
    format: "datev_lodas",
    email: "lohn@example.de",
  },
};

describe("mapPayrollStatus", () => {
  it("maps snake_case wire fields and defaults missing units", () => {
    const status = mapPayrollStatus(backendStatus);

    expect(status.categories).toHaveLength(2);
    expect(status.categories[0]).toEqual({
      id: "regelarbeit",
      label: "Regelarbeit",
      number: "9001",
      unit: "",
      unitRequired: false,
      settingKey: "payroll.lohnart_regelarbeit",
      unitSettingKey: null,
    });
    expect(status.categories[1]?.unit).toBe("tage");
    expect(status.categories[1]?.unitSettingKey).toBe("payroll.einheit_krank");
    expect(status.lodasHeaderComplete).toBe(false);
    expect(status.staffWithoutPersonnelNumber).toBe(19);
  });
});

describe("fetchPayrollStatus", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("fetches and maps the payroll status", async () => {
    mockedSessionFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ data: backendStatus }),
    } as Response);

    const status = await fetchPayrollStatus();

    expect(mockedSessionFetch).toHaveBeenCalledWith(
      "/api/settings/payroll-status",
    );
    expect(status.categories[0]?.settingKey).toBe(
      "payroll.lohnart_regelarbeit",
    );
    expect(status.staffTotal).toBe(21);
  });

  it("reports a failed status request", async () => {
    mockedSessionFetch.mockResolvedValue({
      ok: false,
      statusText: "Forbidden",
    } as Response);

    await expect(fetchPayrollStatus()).rejects.toThrow(
      "Failed to fetch payroll status: Forbidden",
    );
  });
});

describe("duplicateLohnartNumbers", () => {
  const base = mapPayrollStatus(backendStatus);

  const withNumbers = (numbers: string[]): PayrollStatus => ({
    ...base,
    categories: numbers.map((number, i) => ({
      id: `cat-${i}`,
      label: `Kategorie ${i}`,
      number,
      unit: "",
      unitRequired: false,
      settingKey: `payroll.cat_${i}`,
      unitSettingKey: null,
    })),
  });

  it("reports numbers assigned to more than one category", () => {
    expect(
      duplicateLohnartNumbers(withNumbers(["9001", "9002", "9001"])),
    ).toEqual(["9001"]);
  });

  it("ignores empty (unconfigured) numbers", () => {
    expect(duplicateLohnartNumbers(withNumbers(["", "", "9001"]))).toEqual([]);
  });
});

describe("mapPayrollStatus dispatch (#4805)", () => {
  it("maps the Lohnbüro settings", () => {
    expect(mapPayrollStatus(backendStatus).dispatch).toEqual({
      auto: true,
      channel: "email",
      format: "datev_lodas",
      email: "lohn@example.de",
    });
  });

  it("falls back to the backend defaults when the block is missing or unknown", () => {
    const { dispatch: _dispatch, ...withoutDispatch } = backendStatus;
    expect(mapPayrollStatus(withoutDispatch).dispatch).toEqual({
      auto: false,
      channel: "sftp",
      format: "csv",
      email: "",
    });
    expect(
      mapPayrollStatus({
        ...backendStatus,
        dispatch: { auto: false, channel: "fax", format: "pdf", email: "" },
      }).dispatch,
    ).toMatchObject({ channel: "sftp", format: "csv" });
  });
});

describe("savePayrollSetting", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("sends the value in its schema type", async () => {
    mockedSessionFetch.mockResolvedValue({ ok: true } as Response);

    await savePayrollSetting("payroll.dispatch_auto", true);
    await savePayrollSetting("payroll.dispatch_channel", "email");

    expect(mockedSessionFetch).toHaveBeenNthCalledWith(
      1,
      "/api/settings/values/payroll.dispatch_auto",
      expect.objectContaining({
        method: "PUT",
        body: JSON.stringify({ value: true }),
      }),
    );
    expect(mockedSessionFetch).toHaveBeenNthCalledWith(
      2,
      "/api/settings/values/payroll.dispatch_channel",
      expect.objectContaining({ body: JSON.stringify({ value: "email" }) }),
    );
  });
});

describe("fetchPayrollDispatch", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("maps the latest dispatch", async () => {
    mockedSessionFetch.mockResolvedValue({
      ok: true,
      json: () =>
        Promise.resolve({
          data: {
            latest: {
              year: 2026,
              month: 9,
              correction: true,
              channel: "email",
              format: "xlsx",
              recipient: "lohn@example.de",
              state: "sent",
              attempts: 1,
              filename: "zeitkonten_2026-09_korrektur-1.xlsx",
              created_at: "2026-10-01T08:00:00Z",
              finished_at: "2026-10-01T08:01:00Z",
              download_expires_at: "2026-10-15T08:00:00Z",
            },
          },
        }),
    } as Response);

    const latest = await fetchPayrollDispatch();

    expect(mockedSessionFetch).toHaveBeenCalledWith(
      "/api/staff/time-tracking/payroll-dispatch",
    );
    expect(latest).toEqual({
      year: 2026,
      month: 9,
      correction: true,
      channel: "email",
      format: "xlsx",
      recipient: "lohn@example.de",
      state: "sent",
      reason: null,
      attempts: 1,
      filename: "zeitkonten_2026-09_korrektur-1.xlsx",
      createdAt: "2026-10-01T08:00:00Z",
      finishedAt: "2026-10-01T08:01:00Z",
      downloadExpiresAt: "2026-10-15T08:00:00Z",
      downloadedAt: null,
    });
  });

  it("answers null before the first dispatch", async () => {
    mockedSessionFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ data: { latest: null } }),
    } as Response);

    await expect(fetchPayrollDispatch()).resolves.toBeNull();
  });
});

describe("payrollDispatchReasonText", () => {
  it("names the reason of a blocked dispatch in the e-mail's words", () => {
    expect(
      payrollDispatchReasonText({
        state: "blocked",
        reason: "datev_incomplete",
        channel: "sftp",
      }),
    ).toBe(
      "Die DATEV-Angaben sind nicht vollständig. Zum Beispiel fehlt eine Personalnummer oder die Nummer einer Lohnart.",
    );
    expect(
      payrollDispatchReasonText({
        state: "blocked",
        reason: "email_missing",
        channel: "email",
      }),
    ).toBe("Für den Versand per E-Mail fehlt die Adresse des Lohnbüros.");
  });

  it("reuses the SFTP transfer text after four failed attempts", () => {
    expect(
      payrollDispatchReasonText({
        state: "failed",
        reason: "connection_failed",
        channel: "sftp",
      }),
    ).toBe(
      "Die Übertragung an das Lohnbüro hat viermal nicht geklappt. Die Gegenstelle war nicht erreichbar. Bitte versuchen Sie es später noch einmal.",
    );
  });

  it("says nothing while nothing went wrong", () => {
    expect(
      payrollDispatchReasonText({
        state: "sent",
        reason: null,
        channel: "email",
      }),
    ).toBeNull();
  });
});

describe("payrollMonthLabel", () => {
  it("names the month in German", () => {
    expect(payrollMonthLabel(2026, 9)).toBe("September 2026");
    expect(payrollMonthLabel(2027, 1)).toBe("Januar 2027");
  });
});

describe("PAYROLL_DISPATCH_EMAIL_PATTERN", () => {
  it("matches the backend pattern", () => {
    expect(PAYROLL_DISPATCH_EMAIL_PATTERN.test("lohn@example.de")).toBe(true);
    expect(PAYROLL_DISPATCH_EMAIL_PATTERN.test("lohn@example")).toBe(false);
    expect(PAYROLL_DISPATCH_EMAIL_PATTERN.test("lohn büro@example.de")).toBe(
      false,
    );
  });
});
