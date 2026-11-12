// The one status pill (#1629, #5584). Tones map to the brand hexes from
// LOCATION_COLORS with matching tinted backgrounds and darkened label colors.
// Use this whenever a status is one of a fixed set of semantic outcomes; for
// data-driven colors (a raw hex from LOCATION_COLORS) use StatusColorBadge,
// which renders the same pill through StatusPillBase below.
//
// No leading dot: it only repeated what tint and label already say and made
// rows of pills busier without adding meaning (#2476).
import { isValidElement, type ComponentType, type ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import { WarningIcon } from "@phosphor-icons/react/ssr";

import { MOTO_COLOR_PALETTE } from "~/lib/location-helper";

export type StatusBadgeTone =
  | "blue"
  | "green"
  | "warning"
  | "orange"
  | "red"
  | "gray"
  | "purple"
  | "cyan"
  | "outline";

/** `xs` is for dense grids (plan cells, calendar rows): 12 px, tight padding. */
export type StatusBadgeSize = "md" | "xs";

const TONES: Record<
  Exclude<StatusBadgeTone, "outline">,
  { bg: string; text: string }
> = {
  blue: {
    bg: MOTO_COLOR_PALETTE.blue.soft,
    text: MOTO_COLOR_PALETTE.blue.strong,
  },
  green: {
    bg: MOTO_COLOR_PALETTE.green.soft,
    text: MOTO_COLOR_PALETTE.green.strong,
  },
  // LOCATION_COLORS.WARNING: "needs attention" (open, missing, pending).
  // Yellow tint plus a warning icon, so it never reads as the orange
  // Schulhof pill next to it (#5617).
  warning: {
    bg: MOTO_COLOR_PALETTE.amber.pill,
    text: MOTO_COLOR_PALETTE.amber.pillText,
  },
  // LOCATION_COLORS.SCHOOLYARD hue. Not for "needs attention": that is
  // `warning`. Only labels without a warning meaning (Feiertag, Demo).
  orange: {
    bg: MOTO_COLOR_PALETTE.orange.soft,
    text: MOTO_COLOR_PALETTE.orange.strong,
  },
  red: {
    bg: MOTO_COLOR_PALETTE.red.soft,
    text: MOTO_COLOR_PALETTE.red.strong,
  },
  // Muss mit LOCATION_BADGE_TONES[neutral.base] uebereinstimmen, sonst
  // driften der graue StatusBadge und der graue StatusColorBadge auseinander.
  gray: {
    bg: MOTO_COLOR_PALETTE.neutral.soft,
    text: "#4B5563",
  },
  // LOCATION_COLORS.EXCUSED: „Entschuldigt“ keeps its own colour (#5576).
  purple: {
    bg: MOTO_COLOR_PALETTE.purple.soft,
    text: MOTO_COLOR_PALETTE.purple.strong,
  },
  // LOCATION_COLORS.CLASS_TRIP: „Klassenfahrt“ keeps its own colour (#5576).
  cyan: {
    bg: MOTO_COLOR_PALETTE.cyan.soft,
    text: MOTO_COLOR_PALETTE.cyan.strong,
  },
};

// Quiet state without a colour of its own (Entwurf): white with a gray frame.
const OUTLINE_CLASS = "border border-gray-300 bg-white text-gray-700";

function sizeClass(size: StatusBadgeSize, compact: boolean) {
  if (size === "xs") return "px-1.5 py-px";
  return compact ? "px-2 py-0.5" : "px-2.5 py-1";
}

type IconComponent =
  LucideIcon | ComponentType<{ className?: string; "aria-hidden"?: "true" }>;

// The warning tone's own icon (#5617). Filled, like the Alert warning icon: at
// 12 px an outlined triangle smears into a blob on 1x screens, its "!" gone.
function WarningFillIcon(props: {
  className?: string;
  "aria-hidden"?: "true";
}) {
  return <WarningIcon weight="fill" {...props} />;
}

function isIconComponent(icon: unknown): icon is IconComponent {
  // Lucide icons are forwardRef objects (`render`), plain components are
  // functions. Elements, arrays and text are nodes, not components.
  return (
    typeof icon === "function" ||
    (typeof icon === "object" &&
      icon !== null &&
      !isValidElement(icon) &&
      "render" in icon)
  );
}

function renderIcon(icon: IconComponent | ReactNode) {
  if (icon === null || icon === undefined || typeof icon === "boolean") {
    return null;
  }
  if (isIconComponent(icon)) {
    const Icon = icon;
    return <Icon className="h-3 w-3 shrink-0" aria-hidden="true" />;
  }
  return (
    <span className="inline-flex shrink-0" aria-hidden="true">
      {icon}
    </span>
  );
}

/** `data-*` hooks land on the pill element itself. */
export type StatusPillDataAttributes = {
  readonly [key: `data-${string}`]: string | undefined;
};

interface StatusPillCommonProps extends StatusPillDataAttributes {
  readonly label: string;
  /** Layout classes for the rendered pill, for example `w-full` in a grid. */
  readonly className?: string;
  /** Hides a shortened visual label when a caller supplies its full text elsewhere. */
  readonly ariaHidden?: boolean;
  /**
   * Native tooltip on the badge itself. Use when the short label needs the
   * full context on hover (e.g. "Feiertag" → the holiday's name). Set it here
   * rather than on a wrapper so the tooltip belongs to the element that
   * carries the text.
   */
  readonly title?: string;
  /**
   * Optional state icon: a Lucide component or a rendered icon element
   * (Phosphor, concept icon). Use with a distinct label, never as the only
   * cue; it is hidden from screen readers.
   */
  readonly icon?: IconComponent | ReactNode;
  /** Extra screen-reader context for a short visible label. */
  readonly accessibleLabel?: string;
  /** Matches compact informational labels that sit inside dense cards. */
  readonly compact?: boolean;
  /** `xs` for dense grids; wins over `compact`. */
  readonly size?: StatusBadgeSize;
  /**
   * Shortens a long label (a body's name) to the width of its container
   * with an ellipsis; the full label stays in the tooltip.
   */
  readonly truncate?: boolean;
  /** Lets a long label break onto a second line instead of overflowing. */
  readonly wrap?: boolean;
}

/**
 * Shared shape of every kit pill. Only StatusBadge, StatusColorBadge and
 * DataTableStatusBadge render it; everything else picks one of those.
 */
export function StatusPillBase({
  label,
  className,
  ariaHidden,
  title,
  icon,
  accessibleLabel,
  compact = false,
  size = "md",
  backgroundColor,
  textColor,
  outline = false,
  truncate = false,
  wrap = false,
  ...dataAttributes
}: StatusPillCommonProps & {
  readonly backgroundColor?: string;
  readonly textColor?: string;
  readonly outline?: boolean;
}) {
  const iconNode = renderIcon(icon);
  return (
    <span
      aria-hidden={ariaHidden}
      title={title ?? (truncate ? label : undefined)}
      className={`inline-flex items-center rounded-full text-xs font-medium ${
        wrap ? "max-w-full min-w-0 whitespace-normal" : "whitespace-nowrap"
      } ${
        iconNode ? "gap-1" : ""
      } ${truncate ? "max-w-full min-w-0" : ""} ${sizeClass(size, compact)} ${outline ? OUTLINE_CLASS : ""} ${className ?? ""}`}
      style={outline ? undefined : { backgroundColor, color: textColor }}
      {...dataAttributes}
    >
      {iconNode}
      {accessibleLabel ? (
        <span className="sr-only">{accessibleLabel}</span>
      ) : null}
      {truncate ? <span className="truncate">{label}</span> : label}
    </span>
  );
}

export function StatusBadge({
  tone,
  ...props
}: StatusPillCommonProps & { readonly tone: StatusBadgeTone }) {
  if (tone === "outline") return <StatusPillBase {...props} outline />;
  const styles = TONES[tone];
  return (
    <StatusPillBase
      {...props}
      icon={props.icon ?? (tone === "warning" ? WarningFillIcon : undefined)}
      backgroundColor={styles.bg}
      textColor={styles.text}
    />
  );
}
