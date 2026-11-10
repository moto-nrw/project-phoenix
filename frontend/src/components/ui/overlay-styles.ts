/** Visual policy shared by every UI-kit surface that obscures page content. */

/** Blur applied to the page behind an overlay. */
export const OVERLAY_BACKDROP_CLASS = "backdrop-blur-sm";

/**
 * Tint of the backdrop: one color and one opacity for Modal, FormModal,
 * Drawer and SlideOver, so stacked or adjacent overlays dim the page in the
 * same gray instead of three different ones (#2932).
 */
export const OVERLAY_BACKDROP_TINT_CLASS = "bg-black/40";

/** Fully transparent start state for the backdrop enter/exit animation. */
export const OVERLAY_BACKDROP_TINT_HIDDEN_CLASS = "bg-black/0";

/**
 * Panel look of the centered dialog (`Modal`, and through it `FormModal`,
 * `ConfirmationModal`, `ChoiceModal`, `ConfirmDeleteModal`): soft white
 * gradient, glass blur and a deep shadow with a hint of brand blue.
 */
export const OVERLAY_PANEL_STYLE = {
  background:
    "linear-gradient(135deg, rgba(255,255,255,0.95) 0%, rgba(248,250,252,0.98) 100%)",
  backdropFilter: "blur(20px)",
  boxShadow:
    "0 25px 50px -12px rgba(0, 0, 0, 0.25), 0 8px 16px -8px rgba(80, 128, 216, 0.15)",
  animationFillMode: "both",
} as const;

/**
 * Stacking order of everything that floats above the page, lowest first.
 * Every kit overlay takes its level from here instead of writing a number;
 * a new level goes here with the reason it sits where it does.
 *
 * Literal class strings so Tailwind finds them. `Z_INDEX` carries the same
 * levels for inline `style.zIndex` (portaled menus, positioned popovers).
 */
/**
 * Klebt am unteren Rand einer Seite. Unter `lg` liegt dort die untere
 * Navigation, die Leiste steht dann darüber.
 */
export const STICKY_ABOVE_BOTTOM_NAV =
  "sticky bottom-[calc(6rem+env(safe-area-inset-bottom))] lg:bottom-4";

export const Z_LAYER = {
  /** Mobile bottom navigation. */
  nav: "z-30",
  /** Sticky portal header. */
  header: "z-40",
  /** Floating action buttons, level with the header. */
  fab: "z-40",
  /** Backdrop of a side panel (`SlideOver`), below its panel. */
  sheetBackdrop: "z-40",
  /**
   * Click-catcher of the filter panel: above nav and FABs, so an outside tap
   * closes the panel instead of firing a FAB, below the panel itself.
   */
  panelBackdrop: "z-[45]",
  /** Drawer, slide-over and filter panels, the full-page loading veil. */
  sheet: "z-50",
  /** In-flow dropdown menus anchored below their trigger. */
  dropdown: "z-50",
  /** Centered dialogs (`Modal` and everything built on it). */
  modal: "z-[9999]",
  /** Guided-tour coach mark: above open dialogs, below popovers. */
  coach: "z-[10000]",
  /** Calendars and anchored popovers, also when opened from a dialog. */
  popover: "z-[10001]",
} as const;

/** `Z_LAYER` levels as numbers for inline `style.zIndex`. */
export const Z_INDEX = {
  /** Portaled select menus: above dialogs, below calendars. */
  menu: 10000,
  /** Anchored popover panels and calendars. */
  popover: 10001,
  /**
   * Menus opened from inside a popover or calendar (overflow menu in an
   * `AnchoredPopover`, month/year select in the calendar).
   */
  popoverMenu: 10002,
} as const;
