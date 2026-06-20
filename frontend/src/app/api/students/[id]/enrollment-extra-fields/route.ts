import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler, isStringParam } from "~/lib/route-wrapper.server";
import type { StudentEnrollmentExtraFieldGroup } from "~/lib/student-api";

interface EnrollmentExtraFieldsResponse {
  data: StudentEnrollmentExtraFieldGroup[];
}

export const GET = createGetHandler<StudentEnrollmentExtraFieldGroup[]>(
  async (_request, token, params) => {
    if (!isStringParam(params.id)) {
      throw new Error("Student ID is required");
    }

    const response = await apiGet<EnrollmentExtraFieldsResponse>(
      `/api/students/${encodeURIComponent(params.id)}/enrollment-extra-fields`,
      token,
    );
    return response.data;
  },
);
