import type React from "react";
import { SectionHeader } from "~/components/ui/concept-section-header";

/**
 * Mobile-optimized info card component
 * Displays a card with an icon, title, and content
 */
export function InfoCard({
  title,
  children,
  icon,
}: Readonly<{
  title: string;
  children: React.ReactNode;
  icon: React.ReactNode;
}>) {
  return (
    <div className="moto-content-surface flex h-full flex-col rounded-2xl border p-4 shadow-sm backdrop-blur sm:p-6">
      <SectionHeader className="mb-4" title={title} icon={icon} />
      {/* flex-1 lets a child opt into bottom-pinning via mt-auto (e.g. an action
          row that should align across cards of different content length). */}
      <div className="flex flex-1 flex-col space-y-3">{children}</div>
    </div>
  );
}

/**
 * Simplified info item component
 * Displays a label-value pair within an InfoCard
 */
export function InfoItem({
  label,
  value,
  icon,
}: Readonly<{
  label: string;
  value: string | React.ReactNode;
  icon?: React.ReactNode;
}>) {
  return (
    <div className="flex items-start gap-3">
      {icon && <div className="mt-0.5 flex-shrink-0 text-gray-400">{icon}</div>}
      <div className="min-w-0 flex-1">
        <p className="mb-1 text-xs text-gray-500">{label}</p>
        <div className="text-sm font-medium text-gray-900">{value}</div>
      </div>
    </div>
  );
}
