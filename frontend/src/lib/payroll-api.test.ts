import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("./session-cache", () => ({
  sessionFetch: vi.fn(),
}));

import { sessionFetch } from "./session-cache";
import {
  duplicateLohnartNumbers,
  fetchPayrollStatus,
  mapPayrollStatus,
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
