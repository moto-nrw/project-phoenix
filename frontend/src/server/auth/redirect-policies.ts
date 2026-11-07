import "server-only";
import type { NextAuthConfig } from "next-auth";
import { createLogger } from "~/lib/logger";

// The Auth.js callbacks' log component; a jwt-claims import would close an
// import cycle through the portal table.
const authLogger = createLogger({ component: "NextAuthConfig" });

export type RedirectPolicy = NonNullable<
  NonNullable<NextAuthConfig["callbacks"]>["redirect"]
>;

/** The tenant cookie spans every school subdomain, so its redirects may too. */
export const parentDomainRedirect: RedirectPolicy = ({ url, baseUrl }) => {
  if (url.startsWith("/")) return `${baseUrl}${url}`;

  const urlObj = new URL(url);
  const baseObj = new URL(baseUrl);

  if (urlObj.origin === baseObj.origin) return url;

  if (
    urlObj.hostname.endsWith("localhost") &&
    baseObj.hostname.endsWith("localhost")
  ) {
    return url;
  }

  const getParentDomain = (hostname: string) => {
    const parts = hostname.split(".");
    return parts.length > 2 ? parts.slice(-2).join(".") : hostname;
  };
  if (getParentDomain(urlObj.hostname) === getParentDomain(baseObj.hostname)) {
    return url;
  }

  authLogger.warn("redirect_blocked", {
    url,
    baseUrl,
    reason: "origin mismatch",
  });
  return baseUrl;
};

/**
 * Allows a target when one hostname is a subdomain of the other
 * (operator.staging.moto-app.de is a child of staging.moto-app.de).
 */
export const subdomainRedirect: RedirectPolicy = ({ url, baseUrl }) => {
  if (url.startsWith("/")) return `${baseUrl}${url}`;

  const urlObj = new URL(url);
  const baseObj = new URL(baseUrl);

  if (urlObj.origin === baseObj.origin) return url;

  if (
    urlObj.hostname.endsWith("localhost") &&
    baseObj.hostname.endsWith("localhost")
  ) {
    return url;
  }

  if (
    urlObj.hostname.endsWith(`.${baseObj.hostname}`) ||
    baseObj.hostname.endsWith(`.${urlObj.hostname}`)
  ) {
    return url;
  }

  authLogger.warn("operator_redirect_blocked", {
    url,
    baseUrl,
    reason: "origin mismatch",
  });
  return baseUrl;
};

/**
 * Host-only sessions redirect only to their exact origin. Parsing relative
 * URLs before comparing origins also rejects protocol-relative paths such as
 * "//other-host.example".
 */
export const exactOriginRedirect: RedirectPolicy = ({ url, baseUrl }) => {
  try {
    const redirectUrl = url.startsWith("/")
      ? new URL(url, baseUrl)
      : new URL(url);
    const baseUrlObject = new URL(baseUrl);

    if (redirectUrl.origin === baseUrlObject.origin) return redirectUrl.href;
  } catch {
    // An invalid callback target is handled like any other foreign origin.
  }

  authLogger.warn("school_redirect_blocked", {
    url,
    baseUrl,
    reason: "origin mismatch",
  });
  return baseUrl;
};
