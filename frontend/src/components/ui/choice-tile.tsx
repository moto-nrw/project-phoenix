import type {
  ButtonHTMLAttributes,
  HTMLAttributes,
  LabelHTMLAttributes,
  ReactNode,
} from "react";

import { cn } from "~/lib/utils";

/**
 * ChoiceTile: the shared framed row or tile a person can pick.
 *
 * It is the control-shaped sibling of the card surfaces: a white gray-200
 * frame at rest, a tinted frame when `selected`, a muted one when `disabled`.
 * Three element shapes cover every use in the app:
 *
 * - `label` (default) wraps a native Checkbox / Radio so the whole tile is
 *   the hit area (`ui-kit/require-checkbox-label`).
 * - `button` is a toggle or a mode picker; pass `aria-pressed` yourself when
 *   the tile is a real toggle, and `onClick` for the action.
 * - `div` is a row whose selection state is driven by a control inside it.
 *
 * `tone` colours the selected state: gray for neutral picks, green for the
 * brand action, blue for informational choices. The tile does not own its
 * padding beyond the default row (`px-3 py-2.5`); denser or roomier tiles
 * override it through `className` (`p-3`, `p-4`), which `cn` merges.
 *
 * A hand-rolled `rounded-xl border … bg-white` with a selected branch is
 * exactly what `ui-kit/no-hand-rolled-surface` rejects; use this instead.
 */
export type ChoiceTileTone = "gray" | "green" | "blue";

const SELECTED_CLASS: Record<ChoiceTileTone, string> = {
  gray: "border-gray-300 bg-gray-50 text-gray-900",
  green: "border-moto-green/50 bg-moto-green/10 text-gray-900",
  blue: "border-moto-blue bg-moto-blue/10 text-gray-950",
};

const BASE_CLASS =
  "flex items-center gap-3 rounded-xl border px-3 py-2.5 text-left text-sm font-medium transition-colors";

const FOCUS_CLASS: Record<"label" | "button" | "div", string> = {
  label: "cursor-pointer focus-within:ring-2 focus-within:ring-gray-300",
  button:
    "focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none",
  div: "",
};

export function choiceTileClassName({
  as = "label",
  selected = false,
  disabled = false,
  tone = "gray",
  className,
}: Readonly<{
  as?: "label" | "button" | "div";
  selected?: boolean;
  disabled?: boolean;
  tone?: ChoiceTileTone;
  className?: string;
}>): string {
  let state: string;
  if (disabled) {
    state = "cursor-not-allowed border-gray-200 bg-gray-50 opacity-60";
  } else if (selected) {
    state = SELECTED_CLASS[tone];
  } else {
    state =
      "border-gray-200 bg-white text-gray-700 hover:border-gray-300 hover:bg-gray-50";
  }
  return cn(BASE_CLASS, disabled ? "" : FOCUS_CLASS[as], state, className);
}

type CommonProps = Readonly<{
  selected?: boolean;
  disabled?: boolean;
  tone?: ChoiceTileTone;
  className?: string;
  children?: ReactNode;
}>;

type LabelTileProps = CommonProps & { as?: "label" } & Omit<
    LabelHTMLAttributes<HTMLLabelElement>,
    keyof CommonProps
  >;

type ButtonTileProps = CommonProps & { as: "button" } & Omit<
    ButtonHTMLAttributes<HTMLButtonElement>,
    keyof CommonProps | "type"
  >;

type DivTileProps = CommonProps & { as: "div" } & Omit<
    HTMLAttributes<HTMLDivElement>,
    keyof CommonProps
  >;

export type ChoiceTileProps = LabelTileProps | ButtonTileProps | DivTileProps;

export function ChoiceTile(props: ChoiceTileProps) {
  if (props.as === "button") {
    const { as, selected, disabled, tone, className, children, ...rest } =
      props;
    return (
      <button
        type="button"
        disabled={disabled}
        className={choiceTileClassName({
          as,
          selected,
          disabled,
          tone,
          className,
        })}
        {...rest}
      >
        {children}
      </button>
    );
  }
  if (props.as === "div") {
    const { as, selected, disabled, tone, className, children, ...rest } =
      props;
    return (
      <div
        className={choiceTileClassName({
          as,
          selected,
          disabled,
          tone,
          className,
        })}
        {...rest}
      >
        {children}
      </div>
    );
  }
  const {
    as = "label",
    selected,
    disabled,
    tone,
    className,
    children,
    ...rest
  } = props;
  return (
    <label
      aria-disabled={disabled ? true : undefined}
      className={choiceTileClassName({
        as,
        selected,
        disabled,
        tone,
        className,
      })}
      {...rest}
    >
      {children}
    </label>
  );
}
