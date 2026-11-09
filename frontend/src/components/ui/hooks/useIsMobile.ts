import { BELOW_MD, useMediaQuery } from "~/lib/hooks/use-media-query";

/**
 * Below Tailwind's `md` (48rem): the exact counterpart of `md:` classes,
 * also at fractional widths and with a larger browser font. `false` until the
 * first effect, so server and client render the desktop variant first.
 */
export function useIsMobile(): boolean {
  return useMediaQuery(BELOW_MD);
}
