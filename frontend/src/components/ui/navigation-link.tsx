"use client";

import NextLink from "next/link";
import { forwardRef, useContext, type ComponentProps } from "react";
import { NavigationProgressContext } from "~/components/ui/navigation-progress";

type NavigationLinkProps = ComponentProps<typeof NextLink>;

/**
 * `next/link` mit einer frühzeitigen Fortschrittsmeldung. `onNavigate` läuft
 * nur für tatsächlich gestartete clientseitige Wechsel; ein eigener
 * `onClick`, der den Klick abbricht, wird daher nicht fälschlich gemeldet.
 */
const NavigationLink = forwardRef<HTMLAnchorElement, NavigationLinkProps>(
  function NavigationLink({ href, onNavigate, ...props }, ref) {
    const store = useContext(NavigationProgressContext);

    return (
      <NextLink
        {...props}
        ref={ref}
        href={href}
        onNavigate={(event) => {
          let cancelled = false;
          onNavigate?.({
            preventDefault: () => {
              cancelled = true;
              event.preventDefault();
            },
          });
          if (cancelled || typeof href !== "string") return;

          const target = navigationTarget(href);
          if (target !== null && target !== currentUrl()) {
            store?.startNavigation(target);
          }
        }}
      />
    );
  },
);

export default NavigationLink;

function navigationTarget(href: string): string | null {
  try {
    const target = new URL(href, window.location.href);
    if (target.origin !== window.location.origin) return null;
    return normalizedUrl(target.pathname, target.search);
  } catch {
    return null;
  }
}

function currentUrl() {
  return normalizedUrl(window.location.pathname, window.location.search);
}

function normalizedUrl(pathname: string, search: string) {
  const normalizedSearch = new URLSearchParams(search).toString();
  return normalizedSearch === "" ? pathname : `${pathname}?${normalizedSearch}`;
}
