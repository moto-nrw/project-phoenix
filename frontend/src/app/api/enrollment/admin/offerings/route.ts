import { enrollmentChildPath } from "~/lib/enrollment-admin-child.server";
import { tenant } from "~/lib/route-wrapper.server";
import { parseRequestBody } from "~/lib/route-wrapper-utils.server";

interface UpdateOfferingsBody {
  request_id?: string | number;
  child_id?: string | number;
  offerings?: Array<{
    offering_id?: string | number;
    selected_days?: string[];
  }>;
  reason?: string;
  complete_withdrawal_confirmed?: boolean;
}

export const PUT = tenant.route(async (request) => {
  const body = await parseRequestBody<UpdateOfferingsBody>(request);
  return tenant.forwardToBackend(
    request,
    enrollmentChildPath(body, "offerings"),
    {
      method: "PUT",
      json: {
        offerings: body.offerings ?? [],
        reason: body.reason ?? "",
        complete_withdrawal_confirmed:
          body.complete_withdrawal_confirmed === true,
      },
    },
  );
});
