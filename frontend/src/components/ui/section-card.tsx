"use client";

import type { ReactNode } from "react";
import { ChevronDown } from "lucide-react";
import { useState } from "react";
import { Button } from "~/components/ui/button";

/** Collapsible content surface for detail sections with an optional header action. */
export function SectionCard({
  kicker,
  title,
  description,
  action,
  children,
}: Readonly<{
  kicker: string;
  title: string;
  description?: string;
  action?: ReactNode;
  children: ReactNode;
}>) {
  const [collapsed, setCollapsed] = useState(false);

  return (
    <section className="moto-content-surface overflow-hidden rounded-2xl border p-5 shadow-sm">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
            {kicker}
          </p>
          <h3 className="mt-1 text-base font-semibold text-gray-900">
            {title}
          </h3>
          {description && (
            <p className="mt-1 max-w-2xl text-xs leading-5 text-gray-500">
              {description}
            </p>
          )}
        </div>
        <div className="flex shrink-0 items-center gap-1">
          {action}
          <Button
            type="button"
            variant="ghost"
            size="icon"
            aria-label={
              collapsed ? `${title} ausklappen` : `${title} einklappen`
            }
            aria-expanded={!collapsed}
            onClick={() => setCollapsed((prev) => !prev)}
          >
            <ChevronDown
              className={`h-4 w-4 transition-transform ${collapsed ? "-rotate-90" : ""}`}
              aria-hidden="true"
            />
          </Button>
        </div>
      </div>
      {!collapsed && <div className="mt-4">{children}</div>}
    </section>
  );
}
