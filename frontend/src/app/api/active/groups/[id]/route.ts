// app/api/active/groups/[id]/route.ts
import {
  createProxyDeleteHandler,
  createProxyGetByIdHandler,
  createProxyPutHandler,
} from "~/lib/route-wrapper";

/**
 * Type definition for group update request
 */
interface GroupUpdateRequest {
  name?: string;
  description?: string;
  room_id?: string;
}

const ENDPOINT = "/api/active/groups";

export const GET = createProxyGetByIdHandler(ENDPOINT);
export const PUT = createProxyPutHandler<unknown, GroupUpdateRequest>(ENDPOINT);
export const DELETE = createProxyDeleteHandler(ENDPOINT);
