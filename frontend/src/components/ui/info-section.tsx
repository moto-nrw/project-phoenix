import type { ReactNode } from "react";

import { InsetPanel } from "~/components/ui/inset-panel";

interface InfoSectionProps {
  readonly title: string;
  /** Optional leading glyph; a titled group without one renders text only. */
  readonly icon?: ReactNode;
  readonly children: ReactNode;
}

/**
 * Container component for information sections in detail modals.
 * Neutral surface (no accent-tinted backgrounds); the icon supplies its own
 * color (typically a MotoDuotoneIcon/MotoConceptIcon).
 */
export function InfoSection({ title, icon, children }: InfoSectionProps) {
  return (
    <InsetPanel framed padding="md">
      <h3 className="mb-2 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-3 md:text-sm">
        {icon ? (
          <span className="h-3.5 w-3.5 md:h-4 md:w-4">{icon}</span>
        ) : null}
        {title}
      </h3>
      {children}
    </InsetPanel>
  );
}
