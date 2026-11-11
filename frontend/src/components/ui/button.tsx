"use client";

import { ActionHint } from "./action-hint";
import type { PrefetchResource } from "./data-prefetch-intent";
import {
  buttonStyles,
  type ButtonSize,
  type ButtonVariant,
} from "./button-styles";
import Link from "next/link.js";
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

/**
 * What a click does (#6643). Either `tooltip` or `noTooltip`, never both;
 * `ui-kit/button-needs-tooltip` requires one of them in its paths.
 */
export type ButtonExplanation =
  | {
      /**
       * One sentence on what a click does, shown on mouse hover and keyboard
       * focus (`ActionHint`). Replaces a `title` attribute.
       */
      readonly tooltip?: string;
      readonly noTooltip?: never;
    }
  | {
      readonly tooltip?: never;
      /**
       * Why the word on the button already says everything („Abbrechen“ in a
       * dialog). Renders nothing; it makes the decision visible in code.
       * The lint rule rejects an empty reason.
       */
      readonly noTooltip: string;
    };

type ButtonProps = React.ButtonHTMLAttributes<HTMLButtonElement> &
  ButtonExplanation & {
    ref?: React.Ref<HTMLButtonElement>;
    isLoading?: boolean;
    loadingText?: string;
    variant?: ButtonVariant;
    size?: ButtonSize;
    touchTarget?: boolean;
  };

type ButtonLinkProps = Omit<
  React.ComponentProps<typeof Link>,
  "className" | "href"
> &
  ButtonExplanation & {
    readonly href: string;
    readonly resources?: readonly PrefetchResource[];
    readonly variant?: ButtonVariant;
    readonly size?: ButtonSize;
    readonly className?: string;
  };
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
  disabled,
  tooltip,
  noTooltip: _noTooltip,
  ...props
}: Readonly<ButtonProps>) {
  const button = (
    <button
      type="submit"
      data-icon-only={size === "icon" ? "" : undefined}
      className={buttonClassName({ variant, size, className, touchTarget })}
      {...props}
      // Nach den Props: `disabled={false}` vom Aufrufer hebt die Ladesperre
      // nicht auf, sonst wäre ein zweites Absenden möglich (#5608).
      disabled={Boolean(isLoading) || disabled}
    >
      {isLoading ? (loadingText ?? "Wird geladen …") : children}
    </button>
  );
  return tooltip ? <ActionHint text={tooltip}>{button}</ActionHint> : button;
}

/** A navigation link styled by the same shared button variants as actions. */
export function ButtonLink({
  variant = "primary",
  size = "base",
  className = "",
  resources,
  tooltip,
  noTooltip: _noTooltip,
  ...props
}: Readonly<ButtonLinkProps>) {
  const intentProps = useLinkIntentPrefetch({ ...props, resources });
  const link = (
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
  return tooltip ? <ActionHint text={tooltip}>{link}</ActionHint> : link;
}
