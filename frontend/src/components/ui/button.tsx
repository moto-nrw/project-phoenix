"use client";

import { ActionHint } from "./action-hint";
import {
  buttonStyles,
  type ButtonSize,
  type ButtonVariant,
} from "./button-styles";
import { iconOnlyMarker } from "./control-geometry";
import NavigationLink, { type NavigationLinkProps } from "./navigation-link";
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
    isLoading?: boolean;
    loadingText?: string;
    variant?: ButtonVariant;
    size?: ButtonSize;
    touchTarget?: boolean;
  };

type ButtonLinkProps = Omit<NavigationLinkProps, "className"> &
  ButtonExplanation & {
    readonly ref?: React.Ref<HTMLAnchorElement>;
    readonly variant?: ButtonVariant;
    readonly size?: ButtonSize;
    readonly className?: string;
  };

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
      {...iconOnlyMarker(size === "icon")}
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

/**
 * Ein `NavigationLink` in Button-Optik: Vorladen und Fortschritt wie jeder
 * interne Link des Kits (#6570).
 */
export function ButtonLink({
  variant = "primary",
  size = "base",
  className = "",
  tooltip,
  noTooltip: _noTooltip,
  ...linkProps
}: Readonly<ButtonLinkProps>) {
  const link = (
    <NavigationLink
      {...linkProps}
      className={buttonClassName({ variant, size, className })}
      // Kennzeichen für die Bedienhöhe der Kopfkarte (`TenantPage`): sie
      // erfasst Knöpfe über `button`, Links in Button-Optik über dieses
      // Attribut (#5531).
      data-button-link=""
      {...iconOnlyMarker(size === "icon")}
    />
  );
  return tooltip ? <ActionHint text={tooltip}>{link}</ActionHint> : link;
}
