import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";
import type { EnrollmentAnswerFilter } from "~/lib/student-helpers";

interface ApiEnrollmentAnswerFiltersResponse {
  data: Array<{
    key: string;
    label: string;
    type: string;
    options: Array<{ value: string; label: string; count: number }>;
  }>;
}

/**
 * The questions of the registration form the child list can filter by
 * (#6954): yes/no and choice questions with at least one answer. Readable
 * with users:read, like the answers on the child's page.
 */
export const GET = createGetHandler(
  async (_request, token): Promise<EnrollmentAnswerFilter[]> => {
    const response = await apiGet<ApiEnrollmentAnswerFiltersResponse>(
      "/api/students/enrollment-answer-filters",
      token,
    );
    return response.data.map((filter) => ({
      key: filter.key,
      label: filter.label,
      options: filter.options.map((option) => ({ ...option })),
    }));
  },
);
