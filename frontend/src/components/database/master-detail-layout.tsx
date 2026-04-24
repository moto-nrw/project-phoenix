"use client";

import { useEffect, useRef, useState, type ReactNode } from "react";
import { useIsMobile } from "~/hooks/useIsMobile";
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
}: MasterDetailLayoutProps) {
  const isMobile = useIsMobile();
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
        <div className="min-h-0 flex-1 overflow-auto">{list}</div>
        <Drawer
          open={selectedId !== null}
          onOpenChange={(open) => {
            if (!open) onDeselect();
          }}
        >
          <DrawerContent className="max-h-[90vh]" aria-describedby={undefined}>
            <DrawerHeader className="sr-only">
              <DrawerTitle>{mobileDrawerTitle}</DrawerTitle>
            </DrawerHeader>
            <div className="min-h-0 flex-1 overflow-auto pb-6">{detail}</div>
          </DrawerContent>
        </Drawer>
      </div>
    );
  }

  return (
    <div
      ref={containerRef}
      style={{ height }}
      className={cn("flex w-full gap-4", className)}
    >
      <div
        className="shrink-0 overflow-hidden rounded-xl border border-gray-200 bg-white"
        style={{ width: listWidth }}
      >
        <div className="flex h-full flex-col">{list}</div>
      </div>
      <div className="min-w-0 flex-1 overflow-hidden rounded-xl border border-gray-200 bg-white">
        <div className="flex h-full flex-col">{detail}</div>
      </div>
    </div>
  );
}
