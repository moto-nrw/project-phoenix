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
 * `ui-kit/button-needs-tooltip` requires one of them outside its exemptions.
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

type ButtonLinkProps = Omit<NavigationLinkProps, "className"> &
  ButtonExplanation &
  ButtonLinkBlock & {
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
      {...iconOnlyMarker(size === "icon")}
      className={buttonClassName({ variant, size, className, touchTarget })}
      {...(blocked ? withoutActivation(props) : props)}
      {...(blocked && BLOCKED_ACTIVATION_PROPS)}
      {...(blocked && { "aria-disabled": true })}
      // Nach den Props: `disabled={false}` vom Aufrufer hebt die Ladesperre
      // nicht auf, sonst wäre ein zweites Absenden möglich (#5608).
      disabled={Boolean(isLoading) || (disabled && !blocked)}
    >
      {isLoading ? (loadingText ?? "Wird geladen …") : children}
    </button>
  );
  return withHint(button, blocked ? disabledReason : tooltip, blocked);
}

function blockActivation(event: React.SyntheticEvent) {
  if (!isBlockedActivation(event)) return;
  event.preventDefault();
  event.stopPropagation();
}

function isBlockedActivation(event: Readonly<{ type: string; key?: string }>) {
  if (!event.type.startsWith("key")) return true;
  return event.key === "Enter" || event.key === " " || event.key === "Spacebar";
}

// Ein nativ gesperrter Knopf ruft keinen dieser Handler auf; ein gesperrter
// mit Grund bleibt fokussierbar und muss sie deshalb selbst weglassen.
const ACTIVATION_HANDLERS = [
  "onClick",
  "onDoubleClick",
  "onAuxClick",
  "onMouseDown",
  "onMouseUp",
  "onPointerDown",
  "onPointerUp",
  "onKeyDown",
  "onKeyUp",
  "onKeyPress",
  "onTouchStart",
  "onTouchEnd",
  "onClickCapture",
  "onDoubleClickCapture",
  "onAuxClickCapture",
  "onMouseDownCapture",
  "onMouseUpCapture",
  "onPointerDownCapture",
  "onPointerUpCapture",
  "onKeyDownCapture",
  "onKeyUpCapture",
  "onKeyPressCapture",
  "onTouchStartCapture",
  "onTouchEndCapture",
] as const;

// `aria-disabled` hat keine native Ereignissperre. Deshalb werden alle
// Aktivierungsereignisse in der Bubbling-Phase selbst abgebrochen: Der Grund
// bleibt erreichbar, aber weder der Aufrufer noch ein klickbarer Vorfahr
// erhält in dieser Phase die Aktion. Capture-Handler des Aufrufers werden
// oben entfernt.
const BLOCKED_ACTIVATION_PROPS = Object.fromEntries(
  ACTIVATION_HANDLERS.filter((name) => !name.endsWith("Capture")).map(
    (name) => [name, blockActivation],
  ),
);

function withoutActivation<T extends object>(props: T) {
  return Object.fromEntries(
    Object.entries(props).filter(
      ([key]) => !(ACTIVATION_HANDLERS as readonly string[]).includes(key),
    ),
  ) as Omit<T, (typeof ACTIVATION_HANDLERS)[number]>;
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
  disabled,
  disabledReason,
  ...linkProps
}: Readonly<ButtonLinkProps>) {
  if (disabled) {
    return withHint(
      <BlockedLink
        {...linkProps}
        className={buttonClassName({ variant, size, className })}
        iconOnly={size === "icon"}
      />,
      disabledReason,
      true,
    );
  }
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
  return withHint(link, tooltip, false);
}

/**
 * A blocked `ButtonLink` renders without `href`: neither a click nor the
 * context menu („In neuem Tab öffnen“) nor a long press reaches the target.
 * `role="link"` and `tabIndex` keep it a focusable link for every reader.
 */
function BlockedLink({
  children,
  className,
  iconOnly,
  href: _href,
  as: _as,
  replace: _replace,
  scroll: _scroll,
  shallow: _shallow,
  passHref: _passHref,
  prefetch: _prefetch,
  locale: _locale,
  legacyBehavior: _legacyBehavior,
  onNavigate: _onNavigate,
  transitionTypes: _transitionTypes,
  resources: _resources,
  ...props
}: Readonly<
  Omit<ButtonLinkProps, "variant" | "size" | "disabled" | "disabledReason"> & {
    readonly className: string;
    readonly iconOnly: boolean;
  }
>) {
  return (
    <a
      {...withoutActivation(props)}
      {...BLOCKED_ACTIVATION_PROPS}
      role="link"
      tabIndex={0}
      aria-disabled="true"
      className={className}
      data-button-link=""
      {...iconOnlyMarker(iconOnly)}
    >
      {children}
    </a>
  );
}
