import type { GuardianNoticeResult } from "~/lib/timetable-types";

/**
 * Success line after a cancellation (#2601): says whether families were told,
 * so the person cancelling never has to guess whether the notice went out.
 */
export function cancelledToast(
  base: string,
  notice: GuardianNoticeResult | undefined,
): string {
  if (!notice) return base;
  if (notice.familyCount === 0) {
    return `${base}. Keine Familie mit Elternportal-Zugang betroffen.`;
  }
  if (notice.familyCount === 1) {
    return `${base}. 1 Familie wurde informiert.`;
  }
  return `${base}. ${notice.familyCount} Familien wurden informiert.`;
}
