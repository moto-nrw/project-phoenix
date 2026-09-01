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
