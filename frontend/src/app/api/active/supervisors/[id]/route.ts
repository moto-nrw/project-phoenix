// app/api/active/supervisors/[id]/route.ts
import {
  createProxyDeleteHandler,
  createProxyGetByIdHandler,
  createProxyPutHandler,
} from "~/lib/route-wrapper";

/**
 * Type definition for supervisor update request
 */
interface SupervisorUpdateRequest {
  staff_id?: string;
  active_group_id?: string;
}

const ENDPOINT = "/api/active/supervisors";

export const GET = createProxyGetByIdHandler(ENDPOINT);
export const PUT = createProxyPutHandler<unknown, SupervisorUpdateRequest>(
  ENDPOINT,
);
export const DELETE = createProxyDeleteHandler(ENDPOINT);
