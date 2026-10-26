"use client";

import NextLink from "next/link";
import {
  forwardRef,
  useEffect,
  useRef,
  useState,
  type ComponentProps,
  type FocusEvent,
  type PointerEvent,
} from "react";
import { useNavigationLinkProgress } from "~/components/ui/use-navigation-link-progress";

type NavigationLinkProps = Omit<
  ComponentProps<typeof NextLink>,
  "href" | "prefetch"
> & {
  readonly href: string;
};

/**
 * So lange muss die Maus auf dem Link bleiben, bevor er vorlädt. Wer mit der
 * Maus nur über die Seitenleiste fährt, löst damit nichts aus (#4987).
 */
export const HOVER_PREFETCH_DELAY_MS = 80;

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
      ...props
    },
    ref,
  ) {
    const navigationHandlers = useNavigationLinkProgress(
      href,
      onClick,
      onNavigate,
    );
    const [intent, setIntent] = useState({ href, target, prefetch: false });
    const sameDestination = intent.href === href && intent.target === target;
    const prefetch = sameDestination && intent.prefetch;
    // Reset before rendering next/link so a reused link never inherits intent.
    if (!sameDestination) setIntent({ href, target, prefetch: false });
    const hoverTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
    const canPrefetch = target !== "_blank" && isInternalPath(href);

    const cancelHover = () => {
      if (hoverTimer.current === null) return;
      clearTimeout(hoverTimer.current);
      hoverTimer.current = null;
    };
    useEffect(() => cancelHover, [href, target]);

    const startPrefetch = () => {
      cancelHover();
      if (canPrefetch) setIntent({ href, target, prefetch: true });
    };

    return (
      <NextLink
        {...props}
        ref={ref}
        href={href}
        target={target}
        prefetch={prefetch}
        onPointerEnter={(event: PointerEvent<HTMLAnchorElement>) => {
          onPointerEnter?.(event);
          if (prefetch || !canPrefetch) return;
          if (event.pointerType === "mouse") {
            cancelHover();
            hoverTimer.current = setTimeout(
              startPrefetch,
              HOVER_PREFETCH_DELAY_MS,
            );
          } else {
            // Touch und Stift: der Finger liegt schon auf dem Link.
            startPrefetch();
          }
        }}
        onPointerLeave={(event: PointerEvent<HTMLAnchorElement>) => {
          onPointerLeave?.(event);
          cancelHover();
        }}
        onFocus={(event: FocusEvent<HTMLAnchorElement>) => {
          onFocus?.(event);
          startPrefetch();
        }}
        {...navigationHandlers}
      />
    );
  },
);

function isInternalPath(href: string): boolean {
  return href.startsWith("/") && !href.startsWith("//");
}

export default NavigationLink;
