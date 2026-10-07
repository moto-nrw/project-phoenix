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
  // `base` may already be a full sentence ("Die Aktivität ist abgesagt.").
  const head = base.replace(/\.$/, "");
  if (notice.familyCount === 0) {
    return `${head}. Keine betroffene Familie nutzt das Elternportal.`;
  }
  if (notice.familyCount === 1) {
    return `${head}. 1 Familie wurde informiert.`;
  }
  return `${head}. ${notice.familyCount} Familien wurden informiert.`;
}
