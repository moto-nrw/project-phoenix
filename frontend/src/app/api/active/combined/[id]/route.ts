// app/api/active/combined/[id]/route.ts
import {
  createProxyDeleteHandler,
  createProxyGetByIdHandler,
  createProxyPutHandler,
} from "~/lib/route-wrapper";

/**
 * Type definition for combined group update request
 */
interface CombinedGroupUpdateRequest {
  name?: string;
  description?: string;
  room_id?: string;
}

const ENDPOINT = "/api/active/combined";

export const GET = createProxyGetByIdHandler(ENDPOINT);
export const PUT = createProxyPutHandler<unknown, CombinedGroupUpdateRequest>(
  ENDPOINT,
);
export const DELETE = createProxyDeleteHandler(ENDPOINT);
