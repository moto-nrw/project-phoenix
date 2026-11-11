import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { forwardedBody } from "~/lib/backend-proxy-response.server";

// The transparent public proxy preserves PDF bytes and disposition. No session.
export const GET = withPrivateDownloadNoStore(
  createPublicJsonProxy({
    method: "GET",
    onSuccess: (response) => {
      const headers = new Headers({
        "Cache-Control": "private, no-store",
        "X-Content-Type-Options": "nosniff",
      });
      for (const name of [
        "Content-Type",
        "Content-Disposition",
        "Content-Length",
      ]) {
        const value = response.headers.get(name);
        if (value) headers.set(name, value);
      }
      return new Response(forwardedBody(response), {
        status: response.status,
        headers,
      });
    },
    path: (_request, params) =>
      `/traeger/holiday-registrations/${requirePathSegmentParam(params, "token")}/documents/${requirePathSegmentParam(params, "documentId")}`,
  }),
);
