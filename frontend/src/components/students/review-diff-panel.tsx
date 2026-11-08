import type { ReactNode } from "react";
import { InsetPanel } from "~/components/ui/inset-panel";
import { SectionLabel } from "~/components/ui/section-label";

/**
 * The calm "current → requested" panel shown inside an expanded review card,
 * matching the parent-side RequestDiffPanel surface (gray-50, a calm
 * SectionLabel as heading). Rows are passed as children so each queue keeps its own layout.
 */
export function ReviewDiffPanel({
  title,
  children,
}: Readonly<{ title: string; children: ReactNode }>) {
  return (
    <InsetPanel className="mt-3 space-y-2">
      <SectionLabel as="p">{title}</SectionLabel>
      {children}
    </InsetPanel>
  );
}
