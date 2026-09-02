"use client";

import { useEffect, useRef, useState } from "react";
import { Download } from "lucide-react";

import { ExportRangeForm } from "~/components/ui/export-range-form";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";

export function StaffExportButton({
  staffId,
  yearStart,
}: {
  readonly staffId: string;
  readonly yearStart: Date;
}) {
  // The kebab trigger and its menu are ui/OverflowMenu (portal-rendered,
  // outside-click and Escape handled there). Only the range form stays local:
  // it is a form popover, not a menu entry.
  const [formOpen, setFormOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!formOpen) return;
    function handleClickOutside(event: MouseEvent) {
      if (
        containerRef.current &&
        !containerRef.current.contains(event.target as Node)
      ) {
        setFormOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [formOpen]);

  return (
    <div className="relative" ref={containerRef}>
      <OverflowMenu
        ariaLabel="Menü öffnen"
        onOpen={() => setFormOpen(false)}
        items={[
          {
            label: "Exportieren",
            icon: <Download className="h-4 w-4" />,
            onClick: () => setFormOpen(true),
          },
        ]}
      />

      {formOpen && (
        <div className="moto-popover-surface fixed inset-x-4 top-20 z-50 max-h-[calc(100vh-6rem)] overflow-y-auto rounded-xl border sm:absolute sm:inset-auto sm:top-full sm:right-0 sm:mt-1 sm:max-h-none sm:overflow-visible">
          <ExportRangeForm
            exportUrl={`/api/staff/${staffId}/time-tracking/export`}
            anchor={yearStart}
            onDone={() => setFormOpen(false)}
          />
        </div>
      )}
    </div>
  );
}
