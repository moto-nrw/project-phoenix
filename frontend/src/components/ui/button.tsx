"use client";

import type { PrefetchResource } from "./data-prefetch-intent";
import {
  buttonStyles,
  type ButtonSize,
  type ButtonVariant,
} from "./button-styles";
import Link from "next/link";
import { useLinkIntentPrefetch } from "./use-link-intent-prefetch";
import { cn } from "~/lib/utils";

export function buttonClassName({
  variant,
  size,
  className,
  touchTarget = false,
}: Readonly<{
  variant: ButtonVariant;
  size: ButtonSize;
  className: string;
  touchTarget?: boolean;
}>) {
  return cn(buttonStyles({ variant, size, className, touchTarget }));
}

// Die kurze Brücke vermeidet, dass ButtonLink den Portal-Anbieter lädt.
declare global {
  interface Window {
    m?: (href: string) => void;
  }
}

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  ref?: React.Ref<HTMLButtonElement>;
  isLoading?: boolean;
  loadingText?: string;
  variant?: ButtonVariant;
  size?: ButtonSize;
  touchTarget?: boolean;
}

interface ButtonLinkProps extends Omit<
  React.ComponentProps<typeof Link>,
  "className" | "href"
> {
  readonly href: string;
  readonly resources?: readonly PrefetchResource[];
  readonly variant?: ButtonVariant;
  readonly size?: ButtonSize;
  readonly className?: string;
}
type ButtonLinkNavigationEvent = Parameters<
  NonNullable<ButtonLinkProps["onNavigate"]>
>[0];

export function Button({
  children,
  isLoading,
  loadingText,
  variant = "primary",
  size = "base",
  className = "",
  touchTarget = false,
  ...props
}: Readonly<ButtonProps>) {
  return (
    <button
      type="submit"
      disabled={isLoading}
      data-icon-only={size === "icon" ? "" : undefined}
      className={buttonClassName({ variant, size, className, touchTarget })}
      {...props}
    >
      {isLoading ? (loadingText ?? "Loading...") : children}
    </button>
  );
}

/** A navigation link styled by the same shared button variants as actions. */
export function ButtonLink({
  variant = "primary",
  size = "base",
  className = "",
  resources,
  ...props
}: Readonly<ButtonLinkProps>) {
  const intentProps = useLinkIntentPrefetch({ ...props, resources });
  return (
    <Link
      // Kennzeichen für die Bedienhöhe der Kopfkarte (`TenantPage`): sie
      // erfasst Knöpfe über `button`, Links in Button-Optik über dieses
      // Attribut (#5531).
      className={buttonClassName({ variant, size, className })}
      {...props}
      data-button-link=""
      data-icon-only={size === "icon" ? "" : undefined}
      {...intentProps}
      onNavigate={(event: ButtonLinkNavigationEvent) => {
        let cancelled: boolean | undefined;
        props.onNavigate?.({
          preventDefault() {
            cancelled = true;
            event.preventDefault();
          },
        });
        if (!cancelled) window.m?.(props.href);
      }}
    />
  );
}
