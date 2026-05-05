// components/rooms/room-detail-modal.tsx
//
// Responsive wrapper that opens the room-detail view from /rooms without
// navigating away from the card grid (#1374). Right-side slide-over on
// desktop, vaul Drawer (bottom) on mobile — keeps the rooms grid
// visible behind the panel so the user retains context while drilling
// into a room (review feedback, #1323).
//
// Driven by URL state (?room={id}) on the rooms page so deep links work
// and the back button closes the overlay.

"use client";

import { X } from "lucide-react";
import { useIsMobile } from "~/hooks/useIsMobile";
import {
  Drawer,
  DrawerClose,
  DrawerContent,
  DrawerHeader,
  DrawerTitle,
} from "~/components/ui/drawer";
import { useModal } from "~/components/dashboard/modal-context";
import { RoomDetailLoader } from "./room-detail-content";

interface RoomDetailModalProps {
  readonly roomId: string | null;
  readonly onClose: () => void;
}

export function RoomDetailModal({ roomId, onClose }: RoomDetailModalProps) {
  const isMobile = useIsMobile();
  const { isModalOpen } = useModal();
  const open = roomId !== null;

  // Nested Modals (e.g. a future edit/confirm dialog) portal to
  // document.body, so Vaul's outside-click / escape handlers would
  // otherwise dismiss the drawer when the user interacts with the
  // modal. Block the dismiss while a child modal is open — same guard
  // MasterDetailLayout uses.
  const guardOutside = (event: Event) => {
    if (isModalOpen) event.preventDefault();
  };
  const guardEscape = (event: KeyboardEvent) => {
    if (isModalOpen) event.preventDefault();
  };

  return (
    <Drawer
      open={open}
      onOpenChange={(next) => {
        if (!next) onClose();
      }}
      direction={isMobile ? "bottom" : "right"}
      // shouldScaleBackground only makes sense for the bottom sheet —
      // on a right-side panel the background scaling looks broken
      // because the page slides on the wrong axis.
      shouldScaleBackground={isMobile}
    >
      <DrawerContent
        className={isMobile ? "max-h-[90vh]" : undefined}
        aria-describedby={undefined}
        onInteractOutside={guardOutside}
        onEscapeKeyDown={guardEscape}
      >
        <DrawerHeader className="sr-only">
          <DrawerTitle>Raumdetails</DrawerTitle>
        </DrawerHeader>
        <div className="min-h-0 flex-1 overflow-auto px-4 pt-4 pb-6 sm:px-6 sm:pt-6">
          {roomId ? (
            <RoomDetailLoader
              roomId={roomId}
              // Inline X close button — flows into the room header row
              // alongside the room name + status badge so the slide-over
              // header is one balanced line. Desktop only: the bottom
              // sheet dismisses via its drag handle. The button itself is
              // a Vaul DrawerClose, so vaul handles dismissal — no manual
              // onClick wiring needed.
              headerAction={
                !isMobile ? (
                  <DrawerClose
                    aria-label="Raumdetails schließen"
                    className="inline-flex h-9 w-9 items-center justify-center rounded-full text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-900 focus:ring-2 focus:ring-blue-500/40 focus:outline-none"
                  >
                    <X className="h-5 w-5" aria-hidden="true" />
                  </DrawerClose>
                ) : undefined
              }
            />
          ) : null}
        </div>
      </DrawerContent>
    </Drawer>
  );
}
