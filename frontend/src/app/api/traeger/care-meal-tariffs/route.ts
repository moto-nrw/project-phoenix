import { proxyGet, proxyPut } from "~/lib/traeger/route-wrapper.server";

// Mensa-Preise des Trägers (#4686): je Schule ab einem Monat je Essen oder
// als Pauschale je Kind und Monat.
export const GET = proxyGet("/traeger/care-meal-tariffs");

export const PUT = proxyPut("/traeger/care-meal-tariffs");
