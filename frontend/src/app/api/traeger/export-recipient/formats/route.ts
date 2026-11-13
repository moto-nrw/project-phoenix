import { proxyPut } from "~/lib/traeger/route-wrapper.server";

// Welcher Empfänger welches Exportformat bekommt (#6911, ADR 0222).
export const PUT = proxyPut("/traeger/export-recipient/formats");
