import { unwrapEnvelope } from "~/lib/backend-client.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import {
  proxyGet,
  proxyPut,
  traeger,
} from "~/lib/traeger/route-wrapper.server";

// Ein Empfänger der Finanz-Exporte (#6911, ADR 0221): lesen mit seinen
// Übergaben, speichern und entfernen.
const recipientPath = (params: Record<string, unknown>) =>
  `/traeger/export-recipient/${requirePathSegmentParam(params)}`;

export const GET = proxyGet(recipientPath);
export const PUT = proxyPut(recipientPath);
// Removing answers the page without the recipient, so the list shows it
// without a second read; proxyDelete would drop that body and answer 204.
export const DELETE = traeger.del(async (request, token, params) =>
  unwrapEnvelope(
    await traeger.api("DELETE", recipientPath(params), token, undefined, {
      request,
    }),
  ),
);
