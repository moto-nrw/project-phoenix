"use client";

import type { PrefetchResource } from "./data-prefetch-intent";
import Link from "next/link";
import { useLinkIntentPrefetch } from "./use-link-intent-prefetch";
import { cn } from "~/lib/utils";

// Die kurze Brücke vermeidet, dass ButtonLink den Portal-Anbieter lädt.
declare global {
  interface Window {
    m?: (href: string) => void;
  }
}

type ButtonVariant =
  | "primary"
  | "secondary"
  | "outline"
  | "outline_danger"
  | "danger"
  | "warning"
  | "success"
  | "surface"
  | "ghost"
  | "ghost_danger"
  | "ghost_tone"
  | "link"
  | "fab"
  | "dashed";
type ButtonSize =
  "sm" | "md" | "field" | "base" | "lg" | "xl" | "compact" | "icon" | "card";

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  ref?: React.Ref<HTMLButtonElement>;
  isLoading?: boolean;
  loadingText?: string;
  variant?: ButtonVariant;
  size?: ButtonSize;
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

export function buttonClassName({
  variant,
  size,
  className,
}: Readonly<{
  variant: ButtonVariant;
  size: ButtonSize;
  className: string;
}>) {
  const textSizes: Record<ButtonSize, string> = {
    sm: "text-sm",
    md: "text-sm",
    field: "text-sm",
    base: "text-base",
    lg: "text-lg",
    xl: "text-xl",
    compact: "text-xs",
    icon: "text-xs",
    card: "text-sm",
  };
  const sizeStyles: Record<ButtonSize, string> = {
    sm: "rounded-lg px-5 py-3",
    md: "rounded-lg px-4 py-2",
    // 40 px wie `Input controlSize="compact"`: Auth-Formulare und Zeilen, in
    // denen der Knopf neben einem 40-px-Feld steht.
    field: "h-10 rounded-lg px-4",
    base: "rounded-lg px-5 py-3",
    lg: "rounded-lg px-5 py-3",
    xl: "rounded-lg px-5 py-3",
    compact: "h-8 gap-1.5 rounded-md px-2.5 disabled:cursor-not-allowed",
    // `aspect-square` statt `w-8`: erzwingt die Kopfkarte ihre Bedienhöhe
    // (40/44 px), folgt die Breite — sonst wird der quadratische Knopf oval.
    icon: "aspect-square h-8 rounded-md pointer-coarse:min-w-11 disabled:cursor-not-allowed",
    card: "w-full rounded-2xl p-4",
  };
  const variantStyles: Record<ButtonVariant, string> = {
    primary:
      "bg-gray-900 text-white shadow-md hover:bg-gray-700 hover:shadow-lg",
    secondary:
      "bg-gray-200 text-gray-800 shadow-md hover:bg-gray-300 hover:shadow-md",
    outline:
      "bg-transparent text-gray-700 ring-1 ring-gray-300 shadow-md hover:bg-gray-50 hover:ring-gray-400",
    outline_danger:
      "bg-moto-red-soft text-moto-red-strong ring-moto-red/30 shadow-md ring-1 hover:bg-moto-red/20 hover:ring-moto-red/50",
    danger:
      "bg-moto-red text-white shadow-md hover:bg-moto-red-strong hover:shadow-lg",
    // Bestätigung mit Nebenwirkung, die kein Löschen ist (Schließtag trotzdem
    // planen, Einzelanpassungen verwerfen): Orange statt Rot. Dunkler Text:
    // Weiß auf moto-orange erreicht nur 2,4:1 und verfehlt AA.
    warning:
      "bg-moto-orange text-gray-950 shadow-md hover:bg-moto-orange-hover hover:shadow-lg",
    // Weiß auf Grün (#5141) braucht die dunklen Füllstufen: moto-green selbst
    // trägt Weiß nur mit 1,96:1. fill 4,84:1, fill-hover 6,0:1, fill-active 7,0:1.
    success:
      "bg-moto-green-fill text-white shadow-md hover:bg-moto-green-fill-hover hover:shadow-lg active:bg-moto-green-fill-active",
    surface:
      "bg-white text-gray-700 ring-1 ring-gray-200 shadow-sm hover:bg-gray-50 hover:ring-gray-300 active:bg-gray-100",
    ghost:
      "bg-transparent text-gray-600 hover:bg-gray-100 hover:text-gray-900 active:bg-gray-200",
    // Leiser roter Knopf: Entfernen in einer Zeile oder einem Formular, wo
    // ein gefüllter roter Knopf zu laut wäre.
    ghost_danger:
      "bg-transparent text-moto-red-strong hover:bg-moto-red-soft active:bg-moto-red/20",
    // Leiser Knopf auf einer farbigen Meldung (Toast, Fehlerkasten): nimmt die
    // Textfarbe der Fläche an, rot auf Fehler, grün auf Erfolg.
    ghost_tone:
      "bg-transparent text-current hover:bg-black/5 active:bg-black/10",
    // Textknopf („Passwort vergessen?“): kein Polster, kein Schatten, aber
    // derselbe sichtbare Fokusring wie jeder Kit-Knopf.
    link: "h-auto rounded-sm bg-transparent px-0 py-0 text-gray-600 underline-offset-2 shadow-none hover:text-gray-900 hover:underline",
    // Schwebender Anlegen-Knopf: rund, 56 px, grün wie `success`. Bringt sein
    // Maß selbst mit; die Größe des Aufrufs spielt keine Rolle.
    fab: "size-14 rounded-full p-0 bg-moto-green-fill text-white shadow-lg hover:bg-moto-green-fill-hover hover:shadow-xl active:bg-moto-green-fill-active",
    // Gestrichelte Hinzufügen-Fläche („Weitere Nummer hinzufügen“).
    dashed:
      "border border-dashed border-gray-300 bg-gray-50 text-gray-600 hover:border-gray-400 hover:bg-gray-100 hover:text-gray-700",
  };
  // Fingerregel (#5531): Bei Bedienung per Finger ist jeder Kit-Knopf
  // mindestens 44 px hoch (Apple HIG 44 pt), `icon` auch 44 px breit. Mit Maus
  // bleibt es bei den Kit-Maßen. Kein Knopf braucht dafür ein eigenes
  // `min-h-11`. Kein `transform` im Übergang: das Kit skaliert nicht.
  const baseStyles =
    "inline-flex items-center justify-center font-medium transition-[background-color,box-shadow,color,opacity] duration-200 pointer-coarse:min-h-11 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:opacity-50";

  return cn(
    baseStyles,
    sizeStyles[size],
    variantStyles[variant],
    textSizes[size],
    className,
  );
}

export function Button({
  children,
  isLoading,
  loadingText,
  variant = "primary",
  size = "base",
  className = "",
  ...props
}: Readonly<ButtonProps>) {
  return (
    <button
      type="submit"
      disabled={isLoading}
      data-icon-only={size === "icon" ? "" : undefined}
      className={buttonClassName({ variant, size, className })}
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
