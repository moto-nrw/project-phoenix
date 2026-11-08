import { enrollmentChildPath } from "~/lib/enrollment-admin-child.server";
import { tenant } from "~/lib/route-wrapper.server";
import { parseRequestBody } from "~/lib/route-wrapper-utils.server";

interface DataCorrectionBody {
  request_id?: string | number;
  child_id?: string | number;
  first_name?: string;
  last_name?: string;
  date_of_birth?: string;
  target_grade_level?: number;
  target_school_class?: string;
  reason?: string;
}

export const PUT = tenant.route(async (request) => {
  const body = await parseRequestBody<DataCorrectionBody>(request);
  return tenant.forwardToBackend(
    request,
    enrollmentChildPath(body, "data-correction"),
    {
      method: "PUT",
      json: {
        first_name: body.first_name,
        last_name: body.last_name,
        date_of_birth: body.date_of_birth,
        target_grade_level: body.target_grade_level,
        target_school_class: body.target_school_class,
        reason: body.reason,
      },
    },
  );
});
