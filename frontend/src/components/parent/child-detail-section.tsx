import type React from "react";
import type { LucideIcon } from "lucide-react";
import { SectionCard } from "~/components/ui/section-card";

/**
 * Shared section wrapper for the parent child-detail views (master data +
 * care schedule). Lives in its own module so both views can import it without
 * creating a circular dependency between `child-master-data` and
 * `child-care-schedule`.
 *
 * Thin adapter over the kit `SectionCard` — the parents portal used to carry
 * its own card markup here, which is exactly the drift the kit exists to stop.
 */
export function Section({
  title,
  hint,
  icon,
  actions,
  children,
}: Readonly<{
  title: string;
  hint: string;
  icon?: LucideIcon;
  actions?: React.ReactNode;
  children: React.ReactNode;
}>) {
  return (
    <SectionCard
      title={title}
      description={hint}
      icon={icon}
      actions={actions}
      bodyClassName="mt-4 space-y-4"
    >
      {children}
    </SectionCard>
  );
}
