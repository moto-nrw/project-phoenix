"use client";

import { useEffect, useRef, useState } from "react";
import { Download, MoreVertical } from "lucide-react";

import { ExportRangeForm } from "~/components/ui/export-range-form";

type View = "closed" | "menu" | "form";

export function StaffExportButton({
  staffId,
  yearStart,
}: {
  readonly staffId: string;
  readonly yearStart: Date;
}) {
  const [view, setView] = useState<View>("closed");
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (view === "closed") return;
    function handleClickOutside(event: MouseEvent) {
      if (
        containerRef.current &&
        !containerRef.current.contains(event.target as Node)
      ) {
        setView("closed");
      }
    }
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [view]);

  return (
    <div className="relative" ref={containerRef}>
      <button
        type="button"
        onClick={() => setView((v) => (v === "closed" ? "menu" : "closed"))}
        aria-label="Menü öffnen"
        aria-haspopup="menu"
        aria-expanded={view !== "closed"}
        className="rounded-lg p-1.5 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600"
      >
        <MoreVertical className="h-5 w-5" />
      </button>

      {view === "menu" && (
        <div
          role="menu"
          className="absolute top-full right-0 z-50 mt-1 w-40 overflow-hidden rounded-xl border border-gray-100 bg-white py-1 shadow-lg"
        >
          <button
            type="button"
            onClick={() => setView("form")}
            className="flex w-full items-center gap-2 px-4 py-2 text-left text-sm text-gray-700 transition-colors hover:bg-gray-50"
          >
            <Download className="h-4 w-4" />
            Exportieren
          </button>
        </div>
      )}

      {view === "form" && (
        <div className="fixed inset-x-4 top-20 z-50 max-h-[calc(100vh-6rem)] overflow-y-auto rounded-xl border border-gray-100 bg-white shadow-lg sm:absolute sm:inset-auto sm:top-full sm:right-0 sm:mt-1 sm:max-h-none sm:overflow-visible">
          <ExportRangeForm
            exportUrl={`/api/staff/${staffId}/time-tracking/export`}
            anchor={yearStart}
            onDone={() => setView("closed")}
          />
        </div>
      )}
    </div>
  );
}
