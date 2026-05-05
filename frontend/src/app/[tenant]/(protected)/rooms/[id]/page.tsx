"use client";

import { useParams, useSearchParams } from "next/navigation";
import { useTenantRouter } from "~/lib/tenant-router";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { Loading } from "~/components/ui/loading";
import { BackButton } from "~/components/ui/back-button";
import { BinaryModeGuard } from "~/components/tenant/binary-mode-guard";
import {
  RoomDetailContent,
  useRoomDetail,
} from "~/components/rooms/room-detail-content";

// /rooms/[id] is kept as a deep-link / fallback target. The card grid
// at /rooms now opens a responsive modal (#1374) instead of navigating
// here, but bookmarks and direct URL entry still land on this page
// shell that adds the breadcrumb + mobile back button around the shared
// RoomDetailContent body.
export default function RoomDetailPage() {
  return (
    <BinaryModeGuard>
      <RoomDetailPageContent />
    </BinaryModeGuard>
  );
}

function RoomDetailPageContent() {
  const router = useTenantRouter();
  const params = useParams();
  const searchParams = useSearchParams();
  const roomId = params.id as string;
  const referrer = searchParams.get("from") ?? "/rooms";

  const { room, history, loading, error } = useRoomDetail(roomId);

  useSetBreadcrumb({
    roomName: room?.name,
  });

  if (loading) {
    return <Loading message="Laden..." fullPage={false} />;
  }

  if (error || !room) {
    return (
      <div className="flex min-h-[80vh] flex-col items-center justify-center">
        <div className="mb-4 rounded-lg border border-red-200 bg-red-50 p-4 text-red-800">
          {error ?? "Raum nicht gefunden"}
        </div>
        <button
          onClick={() => router.push(referrer)}
          className="mt-4 rounded bg-blue-100 px-4 py-2 text-blue-800 transition-colors hover:bg-blue-200"
        >
          Zurück
        </button>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-7xl">
      <BackButton referrer={referrer} />
      <RoomDetailContent room={room} history={history} />
    </div>
  );
}
