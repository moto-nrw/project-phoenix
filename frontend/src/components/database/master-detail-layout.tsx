"use client";

import { useEffect, useRef, useState, type ReactNode } from "react";
import { useIsMobile } from "~/components/ui/hooks/useIsMobile";
import { useModal } from "~/components/dashboard/modal-context";
import { cn } from "~/lib/utils";
import {
  Drawer,
  DrawerContent,
  DrawerHeader,
  DrawerTitle,
} from "~/components/ui/drawer";

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
  const containerRef = useRef<HTMLDivElement>(null);
  const [height, setHeight] = useState<string>("100dvh");

  useEffect(() => {
    const node = containerRef.current;
    if (!node) return;

    const measure = () => {
      const top = Math.max(0, Math.round(node.getBoundingClientRect().top));
      const next = `calc(100dvh - ${top + bottomOffset}px)`;
      setHeight((prev) => (prev === next ? prev : next));
    };

    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(document.documentElement);
    window.addEventListener("resize", measure);
    return () => {
      observer.disconnect();
      window.removeEventListener("resize", measure);
    };
  }, [bottomOffset]);

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
      style={{ height }}
      className={cn("flex w-full gap-4", className)}
    >
      <div
        className={cn(
          "moto-content-surface overflow-hidden rounded-2xl border shadow-sm",
          showDetail ? "shrink-0" : "flex-1",
        )}
        style={showDetail ? { width: listWidth } : undefined}
      >
        <div className="flex h-full flex-col">{list}</div>
      </div>
      {showDetail ? (
        <div className="moto-content-surface min-w-0 flex-1 overflow-hidden rounded-2xl border shadow-sm">
          <div className="flex h-full flex-col">{detail}</div>
        </div>
      ) : null}
    </div>
  );
}
