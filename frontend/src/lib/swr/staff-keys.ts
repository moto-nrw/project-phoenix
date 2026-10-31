import { previewRow, previewText, type ListPreview } from "./list-preview";
import { SWR_FAMILIES } from "./shell-keys";
import type { staffService } from "~/lib/staff-api";
import { resourceKey } from "./resource-key";
export type StaffFilters = NonNullable<
  Parameters<typeof staffService.getAllStaff>[0]
>;
export const staffKeys = {
  previewSources: (key: string) =>
    key.startsWith("staff-directory-list-") ||
    key.startsWith("staff-directory-teachers-"),
  preview(data: unknown, id: string): ListPreview | undefined {
    const row = previewRow(data, id);
    const name = previewText(row?.name);
    return name ? { id, name, context: previewText(row?.role) } : undefined;
  },
  recordKey: (id: string) => `staff-record-${encodeURIComponent(id)}`,
  family: SWR_FAMILIES.staff,
  key: (params: StaffFilters = {}, options: { strict?: boolean } = {}) =>
    resourceKey("staff-directory-list-", {
      ...params,
      strict: options.strict ?? false,
    }),
  teachersKey: () =>
    resourceKey("staff-directory-teachers-", { page: 1, pageSize: 1000 }),
} as const;
