// app/api/active/visits/[id]/route.ts
import {
  createProxyDeleteHandler,
  createProxyGetByIdHandler,
  createProxyPutHandler,
} from "~/lib/route-wrapper";

/**
 * Type definition for visit update request
 */
interface VisitUpdateRequest {
  student_id?: string;
  active_group_id?: string;
  start_time?: string;
  end_time?: string;
}

const ENDPOINT = "/api/active/visits";

export const GET = createProxyGetByIdHandler(ENDPOINT);
export const PUT = createProxyPutHandler<unknown, VisitUpdateRequest>(ENDPOINT);
export const DELETE = createProxyDeleteHandler(ENDPOINT);
