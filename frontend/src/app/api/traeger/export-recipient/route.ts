import { proxyGet, proxyPost } from "~/lib/traeger/route-wrapper.server";

// Empfänger der Finanz-Exporte des Trägers (#6824, ADR 0218; #6911, ADR
// 0221): alle Empfänger ohne Passwort, wer welches Format bekommt und die
// letzten Übergaben; einen Empfänger anlegen. Nur mit „Finanz-Export“.
const RECIPIENTS = "/traeger/export-recipient/";

export const GET = proxyGet(RECIPIENTS);
export const POST = proxyPost(RECIPIENTS);
