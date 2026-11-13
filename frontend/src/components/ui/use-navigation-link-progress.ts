"use client";

import type NextLink from "next/link";
import {
  useContext,
  useRef,
  type ComponentProps,
  type MouseEvent,
} from "react";
import { NavigationProgressContext } from "~/components/ui/navigation-progress-store";
import { currentUrl, navigationTarget } from "~/components/ui/navigation-url";

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
