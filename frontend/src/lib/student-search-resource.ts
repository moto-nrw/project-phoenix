import type { studentService } from "~/lib/api";
import { createLogger } from "~/lib/logger";
const logger = createLogger({ component: "StudentSearch" });
/** Same bounded page walk for the destination and navigation intent. */
export async function fetchStudentSearch(
  filters: NonNullable<Parameters<typeof studentService.getStudents>[0]>,
  requestDate: string,
  requestIsToday: boolean,
  loadPage: typeof studentService.getStudents,
) {
  const MAX_STUDENT_SEARCH_PAGES = 20;
  const result = await loadPage(filters);
  const students = [...result.students];

  // Walk the remaining pages when the selection is larger than one page.
  // The first request stays untouched (no page parameter), so the common
  // single-page case is byte-identical with before.
  //
  // Separate requests only add up to the whole selection because the list
  // query carries a total order (repositories/users/student.go orders by id
  // unless a caller asks for something else). Without one, PostgreSQL may
  // answer two windows out of two different row orders, and children then
  // appear twice or not at all (#2218 review).
  const totalPages = result.pagination?.total_pages ?? 1;
  const lastPage = Math.min(totalPages, MAX_STUDENT_SEARCH_PAGES);
  for (let page = 2; page <= lastPage; page++) {
    const nextPage = await loadPage({ ...filters, page });
    students.push(...nextPage.students);
  }
  // Past the bound the list is incomplete, and everything downstream — the
  // count badge, the grouping headers, the export scope — reads exactly the
  // rows loaded here. Reporting that as a finished result would present a
  // truncated list as the complete one, so the number the backend counted
  // travels along and the page says on screen that it is showing a part
  // (#2218 review). The log line stays for the server-side view.
  let truncatedTotal: number | null = null;
  if (totalPages > lastPage) {
    truncatedTotal = result.pagination?.total_records ?? students.length;
    logger.warn("student_search_pages_truncated", {
      total_pages: totalPages,
      fetched_pages: lastPage,
    });
  }

  // Tag the response with the date AND today/planning mode it was fetched
  // for so date-sensitive rendering can tell a fresh result apart from
  // keepPreviousData holding the previous request's rows. The mode matters
  // on top of the date: when a selected future date rolls over to today at
  // midnight, requestDate is unchanged but the semantics flip from planning
  // to live — the stale planning rows must not be shown under live presence
  // badges and check-in controls (#1939).
  return {
    students,
    requestDate: requestDate,
    requestIsToday: requestIsToday,
    truncatedTotal,
  };
}
