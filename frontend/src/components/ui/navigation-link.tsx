"use client";

import NextLink, { type LinkProps } from "next/link";
import {
  forwardRef,
  useContext,
  useRef,
  type ComponentProps,
  type MouseEvent,
} from "react";
import { NavigationProgressContext } from "~/components/ui/navigation-progress";

type NavigationLinkProps = ComponentProps<typeof NextLink>;
type NavigationEvent = Parameters<NonNullable<LinkProps["onNavigate"]>>[0];

/**
 * `next/link` mit einer frühzeitigen Fortschrittsmeldung. Der Link markiert
 * seinen Wechsel vor dem Router-Dispatch und verwirft die Meldung sofort,
 * wenn ein eigener Handler oder `onNavigate` den Klick abbricht.
 */
const NavigationLink = forwardRef<HTMLAnchorElement, NavigationLinkProps>(
  function NavigationLink({ href, onClick, onNavigate, ...props }, ref) {
    const store = useContext(NavigationProgressContext);
    const pendingNavigationId = useRef<number | null>(null);

    const cancelPendingNavigation = () => {
      if (pendingNavigationId.current === null) return;
      store?.cancelNavigation(pendingNavigationId.current);
      pendingNavigationId.current = null;
    };

    return (
      <NextLink
        {...props}
        ref={ref}
        href={href}
        onClick={(event: MouseEvent<HTMLAnchorElement>) => {
          pendingNavigationId.current = null;
          const target =
            typeof href === "string" && isClientSideLinkClick(event)
              ? navigationTarget(href)
              : null;
          if (target !== null && target !== currentUrl()) {
            pendingNavigationId.current =
              store?.startLinkNavigation(target) ?? null;
          }

          onClick?.(event);
          if (event.defaultPrevented) cancelPendingNavigation();
        }}
        onNavigate={(event: NavigationEvent) => {
          let cancelled = false;
          onNavigate?.({
            preventDefault: () => {
              cancelled = true;
              event.preventDefault();
            },
          });
          if (cancelled) cancelPendingNavigation();
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

function isClientSideLinkClick(event: MouseEvent<HTMLAnchorElement>) {
  if (
    event.defaultPrevented ||
    event.button !== 0 ||
    event.metaKey ||
    event.ctrlKey ||
    event.shiftKey ||
    event.altKey
  ) {
    return false;
  }

  const link = event.currentTarget;
  return (
    (link.target === "" || link.target === "_self") &&
    !link.hasAttribute("download")
  );
}

function normalizedUrl(pathname: string, search: string) {
  const normalizedSearch = new URLSearchParams(search).toString();
  return normalizedSearch === "" ? pathname : `${pathname}?${normalizedSearch}`;
}
