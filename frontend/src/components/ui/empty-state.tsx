import type { ReactNode } from "react";

// Centered empty-state block: optional muted icon, title, description, and an
// optional action slot. Extracted from the operator provisioning pages and the
// tenant list pages, which all hand-rolled this same column (#1629). Pass the
// icon as a ReactNode (an inline SVG or a lucide icon sized h-12 w-12) so
// existing screens keep their exact artwork.
export function EmptyState({
  icon,
  title,
  description,
  action,
  variant = "default",
  className = "",
}: {
  readonly icon?: ReactNode;
  readonly title: string;
  readonly description?: string;
  readonly action?: ReactNode;
  readonly variant?: "default" | "compact";
  readonly className?: string;
}) {
  if (variant === "compact") {
    return (
      <div className={`flex items-start gap-3 py-2 text-left ${className}`}>
        {icon != null ? (
          <span className="mt-0.5 shrink-0 text-gray-400" aria-hidden="true">
            {icon}
          </span>
        ) : null}
        <div className="min-w-0 flex-1">
          <p className="text-sm font-semibold text-gray-900">{title}</p>
          {description != null && description !== "" ? (
            <p className="mt-0.5 text-sm leading-6 text-gray-500">
              {description}
            </p>
          ) : null}
          {action != null ? <div className="mt-3">{action}</div> : null}
        </div>
      </div>
    );
  }

  return (
    <div
      className={`flex flex-col items-center gap-3 py-12 text-center ${className}`}
    >
      {icon != null ? (
        <span className="text-gray-400" aria-hidden="true">
          {icon}
        </span>
      ) : null}
      <p className="text-lg font-medium text-gray-900">{title}</p>
      {description != null && description !== "" ? (
        <p className="max-w-md text-sm leading-relaxed text-gray-500">
          {description}
        </p>
      ) : null}
      {action != null ? <div className="mt-2">{action}</div> : null}
    </div>
  );
}
