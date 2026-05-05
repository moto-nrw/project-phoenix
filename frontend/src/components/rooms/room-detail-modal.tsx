// components/rooms/room-detail-modal.tsx
//
// Responsive wrapper that opens the room-detail view from /rooms without
// navigating away from the card grid (#1374). Centered Modal on desktop,
// vaul Drawer (bottom) on mobile — same desktop/mobile split that
// MasterDetailLayout already uses for the database master-detail pages.
//
// Driven by URL state (?room={id}) on the rooms page so deep links work
// and the back button closes the overlay.

"use client";

import { useIsMobile } from "~/hooks/useIsMobile";
import { Modal } from "~/components/ui/modal";
import {
  Drawer,
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

  if (isMobile) {
    return (
      <Drawer
        open={open}
        onOpenChange={(next) => {
          if (!next) onClose();
        }}
      >
        <DrawerContent
          className="max-h-[90vh]"
          aria-describedby={undefined}
          // Mirror the MasterDetailLayout guards: nested Modals (e.g. a
          // future edit/confirm dialog) portal to document.body, so Vaul's
          // outside-click handler would otherwise dismiss the drawer when
          // the user taps inside the modal.
          onInteractOutside={(event) => {
            if (isModalOpen) event.preventDefault();
          }}
          onEscapeKeyDown={(event) => {
            if (isModalOpen) event.preventDefault();
          }}
        >
          <DrawerHeader className="sr-only">
            <DrawerTitle>Raumdetails</DrawerTitle>
          </DrawerHeader>
          <div className="min-h-0 flex-1 overflow-auto px-4 pt-2 pb-6">
            {roomId ? <RoomDetailLoader roomId={roomId} /> : null}
          </div>
        </DrawerContent>
      </Drawer>
    );
  }

  return (
    <Modal
      isOpen={open}
      onClose={onClose}
      // Non-empty title lifts the close button into its own header bar
      // so it can't collide with the room name + status badge in the
      // body of RoomDetailContent.
      title="Raumdetails"
      widthClass="mx-4 w-[calc(100%-2rem)] max-w-4xl"
    >
      {roomId ? <RoomDetailLoader roomId={roomId} /> : null}
    </Modal>
  );
}
