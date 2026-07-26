// Payroll configuration status (#1417 Tranche 2b): the /payroll page's data
// source. Requires config:manage — callers gate rendering on the permission
// so no request fires without it. Values are written through the generic
// settings API (setSettingValue) using the setting keys this status carries.

import { sessionFetch } from "./session-cache";

interface BackendPayrollCategoryStatus {
  id: string;
  label: string;
  number: string;
  unit?: string;
  unit_required: boolean;
  setting_key: string;
  unit_setting_key?: string;
}

interface BackendPayrollStatus {
  categories: BackendPayrollCategoryStatus[];
  beraternummer: string;
  mandantennummer: string;
  lodas_header_complete: boolean;
  configured_categories: number;
  total_categories: number;
  staff_total: number;
  staff_without_personnel_number: number;
}

interface PayrollCategoryStatus {
  id: string;
  label: string;
  number: string;
  unit: string;
  unitRequired: boolean;
  settingKey: string;
  unitSettingKey: string | null;
}

export interface PayrollStatus {
  categories: PayrollCategoryStatus[];
  beraternummer: string;
  mandantennummer: string;
  lodasHeaderComplete: boolean;
  configuredCategories: number;
  totalCategories: number;
  staffTotal: number;
  staffWithoutPersonnelNumber: number;
}

export function mapPayrollStatus(data: BackendPayrollStatus): PayrollStatus {
  return {
    categories: (data.categories ?? []).map((cat) => ({
      id: cat.id,
      label: cat.label,
      number: cat.number,
      unit: cat.unit ?? "",
      unitRequired: cat.unit_required,
      settingKey: cat.setting_key,
      unitSettingKey: cat.unit_setting_key ?? null,
    })),
    beraternummer: data.beraternummer,
    mandantennummer: data.mandantennummer,
    lodasHeaderComplete: data.lodas_header_complete,
    configuredCategories: data.configured_categories,
    totalCategories: data.total_categories,
    staffTotal: data.staff_total,
    staffWithoutPersonnelNumber: data.staff_without_personnel_number,
  };
}

export async function fetchPayrollStatus(): Promise<PayrollStatus> {
  const response = await sessionFetch("/api/settings/payroll-status");
  if (!response.ok) {
    throw new Error(`Failed to fetch payroll status: ${response.statusText}`);
  }
  const json = (await response.json()) as { data: BackendPayrollStatus };
  return mapPayrollStatus(json.data);
}

/**
 * Lohnartnummern that appear on more than one category. Not an error — a
 * Träger may legitimately book two categories on one Lohnart — but worth a
 * warning, because a duplicate usually is a typo that would double-book.
 */
export function duplicateLohnartNumbers(status: PayrollStatus): string[] {
  const seen = new Map<string, number>();
  for (const cat of status.categories) {
    if (cat.number === "") continue;
    seen.set(cat.number, (seen.get(cat.number) ?? 0) + 1);
  }
  return [...seen.entries()]
    .filter(([, count]) => count > 1)
    .map(([number]) => number);
}
