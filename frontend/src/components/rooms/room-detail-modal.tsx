// components/rooms/room-detail-modal.tsx
//
// Responsive wrapper that opens the room-detail view from /rooms without
// navigating away from the card grid (#1374). Right-side slide-over on
// desktop, vaul Drawer (bottom) on mobile , keeps the rooms grid
// visible behind the panel so the user retains context while drilling
// into a room (review feedback, #1323).
//
// Driven by URL state (?room={id}) on the rooms page so deep links work
// and the back button closes the overlay.

"use client";

import { useCallback, useEffect, useState } from "react";
import { X } from "lucide-react";
import { useIsMobile } from "~/components/ui/hooks/useIsMobile";
import { useScrollLock } from "~/components/ui/hooks/useScrollLock";
import {
  Drawer,
  DrawerClose,
  DrawerContent,
  DrawerHeader,
  DrawerTitle,
} from "~/components/ui/drawer";
import {
  SlideOver,
  SlideOverClose,
  SlideOverContent,
  SlideOverHeader,
  SlideOverTitle,
} from "~/components/ui/slide-over";
import { useModal } from "~/components/dashboard/modal-context";
import { RoomDetailLoader } from "./room-detail-content";
import { TransitStudentsSection } from "./transit-students-section";

export const TRANSIT_ROOM_ID = "__transit__";

interface RoomDetailModalProps {
  readonly roomId: string | null;
  readonly onClose: () => void;
}

function TransitDetailContent({
  headerAction,
  onSelectionActiveChange,
}: {
  readonly headerAction: React.ReactNode;
  readonly onSelectionActiveChange?: (active: boolean) => void;
}) {
  return (
    <div>
      <div className="sticky top-0 z-20 flex items-center gap-3 border-b border-gray-200/70 bg-gray-50/95 px-5 pt-5 pb-4 backdrop-blur supports-[backdrop-filter]:bg-gray-50/85 sm:px-6">
        <div className="min-w-0 flex-1">
          <h1
            tabIndex={-1}
            className="min-w-0 truncate text-2xl leading-tight font-bold text-gray-900 md:text-3xl"
          >
            Unterwegs
          </h1>
          <p className="mt-1 truncate text-xs font-medium text-gray-500">
            Kinder ohne Raumzuweisung
          </p>
        </div>
        <div className="ml-auto flex-shrink-0">{headerAction}</div>
      </div>
      <div className="px-5 pt-5 sm:px-6">
        <TransitStudentsSection
          onSelectionActiveChange={onSelectionActiveChange}
        />
      </div>
    </div>
  );
}

export function RoomDetailModal({ roomId, onClose }: RoomDetailModalProps) {
  const isMobile = useIsMobile();
  const { isModalOpen } = useModal();
  const open = roomId !== null;
  const [hasActiveSelection, setHasActiveSelection] = useState(false);

  useScrollLock(open);

  useEffect(() => {
    setHasActiveSelection(false);
  }, [roomId]);

  const shouldBlockDismiss = isModalOpen || hasActiveSelection;
  const guardOutside = useCallback(
    (event: Event) => {
      if (shouldBlockDismiss) event.preventDefault();
    },
    [shouldBlockDismiss],
  );
  const guardEscape = useCallback(
    (event: KeyboardEvent) => {
      if (shouldBlockDismiss) event.preventDefault();
    },
    [shouldBlockDismiss],
  );
  const closeButtonClass =
    "inline-flex h-9 w-9 items-center justify-center rounded-full text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-900 focus:outline-none focus-visible:ring-2 focus-visible:ring-gray-300";

  if (!isMobile) {
    return (
      <SlideOver
        open={open}
        onOpenChange={(next) => {
          if (!next) onClose();
        }}
      >
        <SlideOverContent
          id="room-detail-panel"
          widthClass="sm:w-[520px]"
          className="overscroll-contain bg-gray-50"
          aria-describedby={undefined}
          onInteractOutside={guardOutside}
          onEscapeKeyDown={guardEscape}
        >
          <SlideOverHeader className="sr-only">
            <SlideOverTitle>Raumdetails</SlideOverTitle>
          </SlideOverHeader>
          <div
            data-modal-content="true"
            className="min-h-0 flex-1 scrollbar-thin scrollbar-thumb-gray-300 scrollbar-track-transparent overflow-auto pb-6"
          >
            {roomId === TRANSIT_ROOM_ID ? (
              <TransitDetailContent
                onSelectionActiveChange={setHasActiveSelection}
                headerAction={
                  <SlideOverClose
                    aria-label="Raumdetails schließen"
                    className={closeButtonClass}
                  >
                    <X className="h-5 w-5" aria-hidden="true" />
                  </SlideOverClose>
                }
              />
            ) : roomId ? (
              <RoomDetailLoader
                roomId={roomId}
                headerAction={
                  <SlideOverClose
                    aria-label="Raumdetails schließen"
                    className={closeButtonClass}
                  >
                    <X className="h-5 w-5" aria-hidden="true" />
                  </SlideOverClose>
                }
                onSelectionActiveChange={setHasActiveSelection}
              />
            ) : null}
          </div>
        </SlideOverContent>
      </SlideOver>
    );
  }

  return (
    <Drawer
      open={open}
      onOpenChange={(next) => {
        if (!next) onClose();
      }}
      direction={isMobile ? "bottom" : "right"}
      // shouldScaleBackground only makes sense for the bottom sheet ,
      // on a right-side panel the background scaling looks broken
      // because the page slides on the wrong axis.
      shouldScaleBackground={isMobile}
    >
      <DrawerContent
        id="room-detail-panel"
        className={isMobile ? "max-h-[90vh] overscroll-contain" : undefined}
        aria-describedby={undefined}
        onInteractOutside={guardOutside}
        onEscapeKeyDown={guardEscape}
      >
        <DrawerHeader className="sr-only">
          <DrawerTitle>Raumdetails</DrawerTitle>
        </DrawerHeader>
        <div
          data-modal-content="true"
          className="min-h-0 flex-1 scrollbar-thin scrollbar-thumb-gray-300 scrollbar-track-transparent overflow-auto pb-6"
        >
          {/* min-h-[60vh] keeps the mobile bottom sheet from collapsing
              when the loader / error fallback returns a tiny payload.
              No-op on desktop , the side panel is already h-full. */}
          <div className="min-h-[60vh]">
            {roomId === TRANSIT_ROOM_ID ? (
              <TransitDetailContent
                onSelectionActiveChange={setHasActiveSelection}
                headerAction={
                  <DrawerClose
                    aria-label="Raumdetails schließen"
                    className={closeButtonClass}
                  >
                    <X className="h-5 w-5" aria-hidden="true" />
                  </DrawerClose>
                }
              />
            ) : roomId ? (
              <RoomDetailLoader
                roomId={roomId}
                headerAction={
                  <DrawerClose
                    aria-label="Raumdetails schließen"
                    className={closeButtonClass}
                  >
                    <X className="h-5 w-5" aria-hidden="true" />
                  </DrawerClose>
                }
                onSelectionActiveChange={setHasActiveSelection}
              />
            ) : null}
          </div>
        </div>
      </DrawerContent>
    </Drawer>
  );
}
