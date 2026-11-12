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
type ButtonExplanation =
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
    /**
     * Why the button cannot be used while `disabled` is true: what is
     * missing or has to happen first. The button then stays focusable
     * (`aria-disabled`), swallows click and submit, and shows the reason on
     * hover, keyboard focus and tap instead of `tooltip`. No effect while
     * `isLoading`, which keeps the native lock (#5608).
     */
    disabledReason?: string;
    isLoading?: boolean;
    loadingText?: string;
    variant?: ButtonVariant;
    size?: ButtonSize;
    touchTarget?: boolean;
  };

/**
 * A link has no native `disabled`: a blocked `ButtonLink` always names the
 * reason, otherwise it would be a greyed-out control nobody can explain.
 */
type ButtonLinkBlock =
  | { readonly disabled?: false; readonly disabledReason?: string }
  | {
      /** Blocks the navigation; the link stays focusable. */
      readonly disabled: boolean;
      /** Why the link cannot be used, see `Button`'s `disabledReason`. */
      readonly disabledReason: string;
    };

type ButtonLinkProps = Omit<
  React.ComponentProps<typeof Link>,
  "className" | "href"
> &
  ButtonExplanation &
  ButtonLinkBlock & {
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
  disabledReason,
  tooltip,
  noTooltip: _noTooltip,
  ...props
}: Readonly<ButtonProps>) {
  // Natives `disabled` feuert keine Zeigerereignisse, eine Blase mit dem
  // Grund wäre dann unerreichbar. Beim Laden bleibt es nativ (#5608).
  const blocked = Boolean(disabled && disabledReason && !isLoading);
  const button = (
    <button
      type="submit"
      data-icon-only={size === "icon" ? "" : undefined}
      className={buttonClassName({ variant, size, className, touchTarget })}
      {...props}
      {...(blocked && {
        "aria-disabled": true,
        // Verhindert auch das Absenden des Formulars, ebenso bei Enter im Feld.
        onClick: preventActivation,
      })}
      // Nach den Props: `disabled={false}` vom Aufrufer hebt die Ladesperre
      // nicht auf, sonst wäre ein zweites Absenden möglich (#5608).
      disabled={Boolean(isLoading) || (disabled && !blocked)}
    >
      {isLoading ? (loadingText ?? "Wird geladen …") : children}
    </button>
  );
  return withHint(button, blocked ? disabledReason : tooltip, blocked);
}

function preventActivation(event: React.MouseEvent) {
  event.preventDefault();
}

function withHint(
  control: React.ReactElement,
  text: string | undefined,
  blocked: boolean,
) {
  return text ? (
    <ActionHint text={text} showOnTap={blocked}>
      {control}
    </ActionHint>
  ) : (
    control
  );
}

/** A navigation link styled by the same shared button variants as actions. */
export function ButtonLink({
  variant = "primary",
  size = "base",
  className = "",
  resources,
  tooltip,
  noTooltip: _noTooltip,
  disabled,
  disabledReason,
  ...props
}: Readonly<ButtonLinkProps>) {
  const blocked = Boolean(disabled);
  const intentProps = useLinkIntentPrefetch({
    ...props,
    // Ein gesperrter Link lädt sein Ziel nicht vor und navigiert nicht.
    ...(blocked && { prefetch: false, onClick: preventActivation }),
    resources,
  });
  const link = (
    <Link
      // Kennzeichen für die Bedienhöhe der Kopfkarte (`TenantPage`): sie
      // erfasst Knöpfe über `button`, Links in Button-Optik über dieses
      // Attribut (#5531).
      className={buttonClassName({ variant, size, className })}
      {...props}
      // Auch der Mittelklick öffnet das Ziel nicht in einem neuen Tab.
      {...(blocked && {
        "aria-disabled": true,
        onAuxClick: preventActivation,
      })}
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
  return withHint(link, blocked ? disabledReason : tooltip, blocked);
}
