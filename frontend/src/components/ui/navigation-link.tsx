"use client";

import NextLink from "next/link";
import { forwardRef, type ComponentProps } from "react";
import { useNavigationLinkProgress } from "~/components/ui/use-navigation-link-progress";

type NavigationLinkProps = Omit<ComponentProps<typeof NextLink>, "href"> & {
  readonly href: string;
};
/**
 * `next/link` mit einer frühzeitigen Fortschrittsmeldung. Der Link markiert
 * seinen Wechsel vor dem Router-Dispatch und verwirft die Meldung sofort,
 * wenn ein eigener Handler oder `onNavigate` den Klick abbricht.
 */
const NavigationLink = forwardRef<HTMLAnchorElement, NavigationLinkProps>(
  function NavigationLink({ href, onClick, onNavigate, ...props }, ref) {
    const navigationHandlers = useNavigationLinkProgress(
      href,
      onClick,
      onNavigate,
    );

    return (
      <NextLink {...props} ref={ref} href={href} {...navigationHandlers} />
    );
  },
);

export default NavigationLink;
