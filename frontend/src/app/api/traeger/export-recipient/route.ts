import { unwrapEnvelope } from "~/lib/backend-client.server";
import {
  proxyGet,
  proxyPut,
  traeger,
} from "~/lib/traeger/route-wrapper.server";

// Empfänger der Finanz-Exporte des Trägers (#6824, ADR 0218): lesen ohne
// Passwort, speichern und entfernen, nur mit dem Recht „Finanz-Export“.
const RECIPIENT = "/traeger/export-recipient/";

export const GET = proxyGet(RECIPIENT);
export const PUT = proxyPut(RECIPIENT);
// Removing answers the emptied recipient, so the page shows it without a
// second read; proxyDelete would drop that body and answer 204.
export const DELETE = traeger.del(async (request, token) =>
  unwrapEnvelope(
    await traeger.api("DELETE", RECIPIENT, token, undefined, { request }),
  ),
);
