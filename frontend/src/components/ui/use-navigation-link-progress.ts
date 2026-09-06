"use client";

import type NextLink from "next/link";
import {
  useContext,
  useRef,
  type ComponentProps,
  type MouseEvent,
} from "react";
import { NavigationProgressContext } from "~/components/ui/navigation-progress-store";

type NavigationEvent = Parameters<
  NonNullable<ComponentProps<typeof NextLink>["onNavigate"]>
>[0];

/** Kapselt die Meldung und den Abbruch eines `NavigationLink`-Wechsels. */
export function useNavigationLinkProgress(
  href: string | null,
  onClick: ComponentProps<typeof NextLink>["onClick"],
  onNavigate: ComponentProps<typeof NextLink>["onNavigate"],
) {
  const store = useContext(NavigationProgressContext);
  const pendingNavigationId = useRef<number | null>(null);

  const cancelPendingNavigation = () => {
    if (pendingNavigationId.current === null) return;
    store?.cancelNavigation(pendingNavigationId.current);
    pendingNavigationId.current = null;
  };

  return {
    onClick: (event: MouseEvent<HTMLAnchorElement>) => {
      pendingNavigationId.current = null;
      const target =
        href !== null && isClientSideLinkClick(event)
          ? navigationTarget(href)
          : null;
      if (target !== null && target !== currentUrl()) {
        pendingNavigationId.current =
          store?.startLinkNavigation(target) ?? null;
      }

      onClick?.(event);
      if (event.defaultPrevented) cancelPendingNavigation();
    },
    onNavigate: (event: NavigationEvent) => {
      let cancelled = false;
      onNavigate?.({
        preventDefault: () => {
          cancelled = true;
          event.preventDefault();
        },
      });
      if (cancelled) cancelPendingNavigation();
    },
  };
}

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
