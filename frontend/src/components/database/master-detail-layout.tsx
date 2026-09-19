"use client";

import type { ReactNode } from "react";
import { useIsMobile } from "~/components/ui/hooks/useIsMobile";
import { useModal } from "~/components/dashboard/modal-context";
import { cn } from "~/lib/utils";
import {
  Drawer,
  DrawerContent,
  DrawerHeader,
  DrawerTitle,
} from "~/components/ui/drawer";
import { useFillHeight } from "./use-fill-height";

/**
 * Liste links, Objekt rechts. Zulässig nur, wo das Pane die EINZIGE Ansicht
 * des Objekttyps ist und die Auswahl in der Adresse steht (BAUARTEN-SPEC
 * Bauart 1 Regel 2, #3115). Hat der Typ eine Objektroute, ist die Sammlung
 * einspaltig (`DatabaseListLayout`) und jede Zeile ein Link dorthin; die
 * Ratsche `bauart/one-detail-per-type` hält die Liste der erlaubten Flächen.
 */
interface MasterDetailLayoutProps {
  list: ReactNode;
  detail: ReactNode;
  selectedId: string | null;
  onDeselect: () => void;
  listWidth?: number;
  mobileDrawerTitle?: string;
  className?: string;
  /** Breathing room below the split view (e.g. to clear surrounding padding). */
  bottomOffset?: number;
  /**
   * How the desktop layout behaves when nothing is selected.
   * - `"placeholder"` (default): list keeps `listWidth`, detail renders the passed node (typically an empty state).
   * - `"expand"`: list takes the full width and the detail pane is not rendered.
   */
  unselectedBehavior?: "placeholder" | "expand";
}

export function MasterDetailLayout({
  list,
  detail,
  selectedId,
  onDeselect,
  listWidth = 440,
  mobileDrawerTitle = "Details",
  className,
  bottomOffset = 32,
  unselectedBehavior = "placeholder",
}: MasterDetailLayoutProps) {
  const isMobile = useIsMobile();
  const { isModalOpen } = useModal();
  const { ref: containerRef, height } =
    useFillHeight<HTMLDivElement>(bottomOffset);

  if (isMobile) {
    return (
      <div
        ref={containerRef}
        style={{ height }}
        className={cn("flex w-full flex-col", className)}
      >
        <div className="moto-content-surface min-h-0 flex-1 overflow-hidden rounded-2xl border shadow-sm">
          <div className="h-full overflow-auto">{list}</div>
        </div>
        <Drawer
          open={selectedId !== null}
          onOpenChange={(open) => {
            if (!open) onDeselect();
          }}
        >
          <DrawerContent
            className="max-h-[90vh] bg-white"
            aria-describedby={undefined}
            // Modals (Edit/Confirmation) portal to document.body and therefore
            // live outside the drawer's DOM. Without these guards Vaul's
            // DismissableLayer treats every tap inside an open modal as an
            // outside-click and closes the drawer — which unmounts the modal
            // before the user can interact with it. See issue #1358.
            onInteractOutside={(event: Event) => {
              if (isModalOpen) event.preventDefault();
            }}
            onEscapeKeyDown={(event: KeyboardEvent) => {
              if (isModalOpen) event.preventDefault();
            }}
          >
            <DrawerHeader className="sr-only">
              <DrawerTitle>{mobileDrawerTitle}</DrawerTitle>
            </DrawerHeader>
            <div className="min-h-0 flex-1 overflow-auto pb-6">{detail}</div>
          </DrawerContent>
        </Drawer>
      </div>
    );
  }

  const showDetail =
    unselectedBehavior === "placeholder" || selectedId !== null;

  return (
    <div
      ref={containerRef}
      // Die Richtung steht im style, nicht in einer Klasse: die Regel „der
      // Rumpf füllt die Höhe" in globals.css macht jede Hülle auf dem Weg zur
      // letzten Kartenfläche zur Flex-SPALTE, und weil sie ungeschichtet ist,
      // schlägt sie jede Tailwind-Utility. Ohne diese Zeile lagen Liste und
      // Objektansicht übereinander statt nebeneinander.
      style={{ height, flexDirection: "row" }}
      className={cn("flex w-full gap-4", className)}
    >
      <div
        className={cn(
          "moto-content-surface overflow-hidden rounded-2xl border shadow-sm",
          showDetail ? "shrink-0" : "flex-1",
        )}
        style={
          showDetail
            ? { width: listWidth, flex: "0 0 auto" }
            : { flex: "1 1 0%" }
        }
      >
        <div className="flex h-full flex-col">{list}</div>
      </div>
      {showDetail ? (
        // `flex` ebenfalls im style: die Wachstumsregel aus globals.css gibt
        // der letzten Kartenfläche `flex: 1 0 auto`, damit sie in einer
        // Spalte bis zur Unterkante wächst. In dieser Zeile heißt dasselbe
        // „nicht schrumpfen" — die Objektansicht lief dadurch über den
        // rechten Rand hinaus.
        <div
          className="moto-content-surface min-w-0 flex-1 overflow-hidden rounded-2xl border shadow-sm"
          style={{ flex: "1 1 0%" }}
        >
          <div className="flex h-full flex-col">{detail}</div>
        </div>
      ) : null}
    </div>
  );
}
