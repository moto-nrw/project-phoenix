"use client";

import {
  buttonStyles,
  type ButtonSize,
  type ButtonVariant,
} from "./button-styles";
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

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  ref?: React.Ref<HTMLButtonElement>;
  isLoading?: boolean;
  loadingText?: string;
  variant?: ButtonVariant;
  size?: ButtonSize;
  touchTarget?: boolean;
}

interface ButtonLinkProps extends Omit<NavigationLinkProps, "className"> {
  readonly ref?: React.Ref<HTMLAnchorElement>;
  readonly variant?: ButtonVariant;
  readonly size?: ButtonSize;
  readonly className?: string;
}

export function Button({
  children,
  isLoading,
  loadingText,
  variant = "primary",
  size = "base",
  className = "",
  touchTarget = false,
  disabled,
  ...props
}: Readonly<ButtonProps>) {
  return (
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
}

/**
 * Ein `NavigationLink` in Button-Optik: Vorladen und Fortschritt wie jeder
 * interne Link des Kits (#6570).
 */
export function ButtonLink({
  variant = "primary",
  size = "base",
  className = "",
  ...linkProps
}: Readonly<ButtonLinkProps>) {
  return (
    <NavigationLink
      {...linkProps}
      className={buttonClassName({ variant, size, className })}
      // Kennzeichen für die Bedienhöhe der Kopfkarte (`TenantPage`): sie
      // erfasst Knöpfe über `button`, Links in Button-Optik über dieses
      // Attribut (#5531).
      data-button-link=""
      data-icon-only={size === "icon" ? "" : undefined}
    />
  );
}
