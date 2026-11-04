"use client";

import { useEffect } from "react";

/**
 * Sends a path-mode page (<domain>/<school>/...) to the same page on the
 * school's subdomain (#5130). It runs in the browser because only the
 * browser knows the fragment: display links carry their token there
 * (`/display#<token>`), and a server redirect inside a streamed response
 * would drop it. Nothing of the school loads before.
 */
export function PathModeRedirect({ href }: { readonly href: string }) {
  useEffect(() => {
    window.location.replace(`${href}${window.location.hash}`);
  }, [href]);
  return null;
}
