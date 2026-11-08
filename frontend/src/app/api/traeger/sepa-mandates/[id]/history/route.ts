import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Der Verlauf eines SEPA-Mandats (#4695): was wann geschah, ohne Werte.
export const GET = proxyGet(
  (params) =>
    `/traeger/sepa-mandates/${requirePathSegmentParam(params)}/history`,
);
