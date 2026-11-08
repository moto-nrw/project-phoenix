import { traegerPortal } from "~/server/auth/traeger";
import { makePortalHandlers } from "../portal-handlers.server";

/**
 * The Träger portal's route handlers ("moto träger", #4649), on the Träger
 * session only, so a Träger route never forwards a tenant, parent or school
 * token.
 */
export const traeger = makePortalHandlers(traegerPortal);

export const { proxyGet, proxyPost, proxyPut, proxyDelete, proxyUpload } =
  traeger;

/**
 * Träger exports filter by the page's query, so it travels whole. A POST
 * file request carries a letter's text and stays below 64 KiB.
 */
export const proxyFile: typeof traeger.proxyFile = (endpoint, options) =>
  traeger.proxyFile(endpoint, {
    forwardQuery: true,
    maxBodyBytes: 64 * 1024,
    ...options,
  });
