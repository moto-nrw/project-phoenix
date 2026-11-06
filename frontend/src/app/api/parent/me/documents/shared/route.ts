import { proxyGet } from "~/lib/parent/route-wrapper.server";

interface BackendParentSharedDocument {
  id: string;
  organization_id: string;
  traeger_name: string;
  title: string;
  file_name: string;
  content_type: string;
  size_bytes: number;
  released_at: string;
}

/**
 * Proxy GET /api/parent/me/documents/shared → backend
 * /parent/me/documents/shared (#4692). The documents a Träger released for
 * the parents of the account's children; the account comes from the parent
 * JWT server-side, never from query or body.
 */
export const GET = proxyGet<BackendParentSharedDocument[]>(
  "/parent/me/documents/shared",
);
