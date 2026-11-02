"use client";

import type { PrefetchResource } from "./data-prefetch-intent";
import NextLink from "next/link";
import { forwardRef, type ComponentProps } from "react";
import { useNavigationLinkProgress } from "~/components/ui/use-navigation-link-progress";

import { useLinkIntentPrefetch } from "./use-link-intent-prefetch";
export { HOVER_PREFETCH_DELAY_MS } from "./use-link-intent-prefetch";

type NavigationLinkProps = Omit<
  ComponentProps<typeof NextLink>,
  "href" | "prefetch"
> & {
  readonly href: string;
  readonly resources?: readonly PrefetchResource[];
};

/**
 * `next/link` mit einer frühzeitigen Fortschrittsmeldung. Der Link markiert
 * seinen Wechsel vor dem Router-Dispatch und verwirft die Meldung sofort,
 * wenn ein eigener Handler oder `onNavigate` den Klick abbricht.
 *
 * Vorladen erst bei Absicht (#2976, #4987): Ein sichtbarer Link lädt nichts.
 * Bleibt die Maus kurz auf ihm, bekommt er den Fokus oder beginnt eine
 * Berührung, schaltet er `prefetch` auf `true`. Next lädt dann die volle
 * Zielseite samt ihrer JS-Module, nicht nur bis zur Ladehülle; der Klick
 * danach wartet nicht mehr auf den Code der Seite. Der Router hält diese
 * Daten nur im Speicher des Tabs; Logout und Schulwechsel laden die Seite
 * komplett neu und verwerfen ihn damit.
 *
 * In der Entwicklung lädt `next/link` selbst nichts vor, Links in einen neuen
 * Tab und fremde Ziele lässt diese Komponente aus.
 */
const NavigationLink = forwardRef<HTMLAnchorElement, NavigationLinkProps>(
  function NavigationLink(
    {
      href,
      target,
      onClick,
      onNavigate,
      onPointerEnter,
      onPointerLeave,
      onFocus,
      onTouchStart,
      onTouchMove,
      onTouchEnd,
      onTouchCancel,
      onBlur,
      resources,
      ...props
    },
    ref,
  ) {
    const navigationHandlers = useNavigationLinkProgress(
      href,
      onClick,
      onNavigate,
    );
    const intentProps = useLinkIntentPrefetch({
      href,
      target,
      onClick: navigationHandlers.onClick,
      onPointerEnter,
      onPointerLeave,
      onFocus,
      onTouchStart,
      onTouchMove,
      onTouchEnd,
      onTouchCancel,
      onBlur,
      resources,
    });

    return (
      <NextLink
        {...props}
        ref={ref}
        href={href}
        target={target}
        {...navigationHandlers}
        {...intentProps}
      />
    );
  },
);

export default NavigationLink;
