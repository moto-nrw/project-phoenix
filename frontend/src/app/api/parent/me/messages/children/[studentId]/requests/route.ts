import { proxyPost } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

interface CreateRequestBody {
  request_type: string;
  payload: Record<string, unknown>;
}

/**
 * Proxy POST /api/parent/me/messages/children/{studentId}/requests → backend.
 * Submits a structured guardian request tied to the child's conversation.
 */
export const POST = proxyPost<unknown, CreateRequestBody>(
  (params) =>
    `/parent/me/messages/children/${requirePathSegmentParam(params, "studentId")}/requests`,
);
