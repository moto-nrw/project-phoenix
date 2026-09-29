import { createTenantJsonProxy } from "~/lib/backend-proxy-route.server";

const readPath = (
  _request: unknown,
  params: Record<string, string | string[] | undefined>,
) =>
  `/api/enrollment/admin/requests/${encodeURIComponent(String(params.id))}/read`;

/** Eine Anmeldung für die Person als gelesen markieren (#3778). */
export const PUT = createTenantJsonProxy({
  method: "PUT",
  path: readPath,
  body: "none",
  unauthorizedError: "Unauthenticated",
});

/** Eine Anmeldung für die Person wieder als ungelesen markieren (#3778). */
export const DELETE = createTenantJsonProxy({
  method: "DELETE",
  path: readPath,
  body: "none",
  unauthorizedError: "Unauthenticated",
});
