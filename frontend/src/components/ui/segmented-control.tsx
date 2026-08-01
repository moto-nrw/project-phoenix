"use client";

// Segmented single-choice control: a small, fixed set of mutually exclusive
// options rendered as one control.
//
// Fills a real kit gap. `ui/Tabs` is Radix and panel-bound — correct when the
// choice swaps CONTENT, wrong when it is a form value (the Stempeluhr mode) or
// a view parameter that drives fetches rather than panels (Monat/Woche). Both
// of those were hand-rolled `<button>` clusters in the Zeiterfassung before.
//
// Semantics are toggle-buttons (`aria-pressed`), not radios: the options stay
// reachable as plain buttons for keyboard and assistive tech without inheriting
// radiogroup arrow-key navigation, which does not match a two- or three-option
// switcher sitting inline in a toolbar.
//
// Variants:
//   "joined" — segments share one bordered rounded-full container, the active
//              one is solid gray-900. Use for neutral view switchers.
//   "pills"  — separate tinted pills with a gap; the active pill takes its
//              item's brand tone. Use when each option carries its own meaning
//              (Vor Ort = green, Homeoffice = blue, Abwesend = red).
//
// Tones map to LOCATION_COLORS (lib/location-helper.ts) — never generic
// Tailwind hues.

// Not exported: callers reach it through the exported SegmentedControlItem,
// which is the type they actually annotate their items array with.
type SegmentedControlTone = "neutral" | "green" | "blue" | "red";

export interface SegmentedControlItem<T extends string> {
  readonly value: T;
  readonly label: string;
  /** Only honoured by the "pills" variant; "joined" is always neutral. */
  readonly tone?: SegmentedControlTone;
  readonly disabled?: boolean;
}

const ACTIVE_PILL: Record<SegmentedControlTone, string> = {
  neutral: "bg-gray-900 text-white ring-1 ring-gray-900",
  green: "bg-[#83CD2D]/10 text-[#70b525] ring-1 ring-[#83CD2D]/40",
  blue: "bg-[#5080D8]/10 text-[#5080D8] ring-1 ring-[#5080D8]/40",
  red: "bg-[#FF3130]/10 text-[#FF3130] ring-1 ring-[#FF3130]/40",
};

const INACTIVE_PILL = "bg-gray-100 text-gray-500 hover:bg-gray-200";

export function SegmentedControl<T extends string>({
  items,
  value,
  onChange,
  variant = "joined",
  fullWidth = false,
  ariaLabel,
  className = "",
}: {
  readonly items: readonly SegmentedControlItem<T>[];
  /** `null` renders every segment inactive — no silent preselection. */
  readonly value: T | null;
  readonly onChange: (next: T) => void;
  readonly variant?: "joined" | "pills";
  /** Stretch to the container and split the width evenly (modal tab bars). */
  readonly fullWidth?: boolean;
  readonly ariaLabel?: string;
  readonly className?: string;
}) {
  const base =
    "px-3 py-1.5 text-xs font-medium transition-all disabled:cursor-not-allowed disabled:opacity-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none";

  if (variant === "pills") {
    return (
      <div
        role="group"
        aria-label={ariaLabel}
        className={`flex flex-wrap justify-center gap-2 ${className}`}
      >
        {items.map((item) => {
          const active = item.value === value;
          return (
            <button
              key={item.value}
              type="button"
              aria-pressed={active}
              disabled={item.disabled}
              onClick={() => onChange(item.value)}
              className={`${base} rounded-full sm:px-4 ${
                active ? ACTIVE_PILL[item.tone ?? "neutral"] : INACTIVE_PILL
              }`}
            >
              {item.label}
            </button>
          );
        })}
      </div>
    );
  }

  // Full-width variant reads as a tab bar (light track, raised active
  // segment); the inline one is the compact bordered switcher.
  return (
    <div
      role="group"
      aria-label={ariaLabel}
      className={
        fullWidth
          ? `flex w-full gap-1 rounded-lg bg-gray-100 p-1 ${className}`
          : `inline-flex items-center overflow-hidden rounded-full border border-gray-200 bg-white ${className}`
      }
    >
      {items.map((item) => {
        const active = item.value === value;
        if (fullWidth) {
          return (
            <button
              key={item.value}
              type="button"
              aria-pressed={active}
              disabled={item.disabled}
              onClick={() => onChange(item.value)}
              className={`${base} flex-1 rounded-md text-sm ${
                active
                  ? "bg-white text-gray-900 shadow-sm"
                  : "text-gray-500 hover:text-gray-700"
              }`}
            >
              {item.label}
            </button>
          );
        }
        return (
          <button
            key={item.value}
            type="button"
            aria-pressed={active}
            disabled={item.disabled}
            onClick={() => onChange(item.value)}
            className={`${base} ${
              active
                ? "bg-gray-900 text-white"
                : "text-gray-500 hover:text-gray-700"
            }`}
          >
            {item.label}
          </button>
        );
      })}
    </div>
  );
}
