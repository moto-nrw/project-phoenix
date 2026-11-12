import {
  proxyDelete,
  proxyGet,
  proxyPut,
} from "~/lib/traeger/route-wrapper.server";

// Empfänger der Finanz-Exporte des Trägers (#6824, ADR 0218): lesen ohne
// Passwort, speichern und entfernen, nur mit dem Recht „Finanz-Export“.
export const GET = proxyGet("/traeger/export-recipient/");
export const PUT = proxyPut("/traeger/export-recipient/");
export const DELETE = proxyDelete("/traeger/export-recipient/");
