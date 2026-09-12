"use client";

import { Suspense, useCallback, useMemo, useState } from "react";
import {
  useParams,
  usePathname,
  useRouter,
  useSearchParams,
} from "next/navigation";
import { useSession } from "next-auth/react";
import { Trash2 } from "lucide-react";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import type { OverflowMenuItem } from "~/components/ui/page-header/OverflowMenu";
import { TenantPage, type TenantPageTab } from "~/components/ui/tenant-page";
import { RoomStatusBadge } from "~/components/rooms/room-status-badge";
import {
  roomDetailKey,
  RoomDetailContent,
  RoomDetailSkeleton,
  useRoomDetail,
} from "~/components/rooms/room-detail-content";
import { RoomStammdatenTab } from "~/components/rooms/room-stammdaten-tab";
import { roomsConfig } from "~/components/database/configs/rooms.config";
import { useToast } from "~/contexts/ToastContext";
import { hasPermission } from "~/lib/auth-utils";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { createCrudService } from "~/lib/database/service-factory";
import { createLogger } from "~/lib/logger";
import { MOTO_CONCEPTS } from "~/lib/moto-concepts";
import { formatFloor, isSystemRoom, type Room } from "~/lib/room-helpers";
import { useTenantMutate, useTenantMutateMatching } from "~/lib/swr";
import {
  ROOM_DERIVED_CACHE_KEY_FRAGMENTS,
  ROOM_LIST_CACHE_KEYS,
} from "~/lib/swr/room-derived-caches";
import { usePresenceMode } from "~/lib/tenant-context";
import { resolveDetailReferrer } from "~/lib/tenant-path";
import { useTenantRouter } from "~/lib/tenant-router";
import { getDbOperationMessage } from "~/lib/use-notification";
import { RoomDetailLoadingPage } from "./page-skeleton";

const logger = createLogger({ component: "RoomDetailPage" });

type RoomTab = "uebersicht" | "stammdaten";

/** Rückweg-Beschriftung: die Sammlung, aus der man kam (`?from=`). */
function backLabelFor(referrer: string): string {
  return referrer.startsWith("/database/rooms")
    ? "Zurück zu den Räumen der Datenverwaltung"
    : "Zurück zu den Räumen";
}

export default function RoomDetailPage() {
  return (
    <Suspense fallback={<RoomDetailLoadingPage />}>
      <RoomDetailPageContent />
    </Suspense>
  );
}

/**
 * Die Raumseite: die einzige Objektansicht eines Raums (BAUARTEN-SPEC
 * Bauart 2, #3115). Bis dahin gab es drei Zustände: ein Pane in der
 * Datenverwaltung, ein Slide-over auf /rooms und diese Route als tote
 * Weiterleitung. Jetzt führen Übersicht und Register beide hierher.
 *
 * „Übersicht" ist der laufende Betrieb (Belegung, Kinder im Raum,
 * Historie); „Stammdaten" ist der Datensatz (Name, Gebäude, Farbe, offener
 * Raum) für alle, die Räume bearbeiten dürfen. Erfasst die Schule nur, ob ein
 * Kind da ist (binärer Modus), gibt es keine Belegung und damit nur den
 * Reiter „Stammdaten".
 */
function RoomDetailPageContent() {
  const params = useParams();
  const pathname = usePathname();
  const navRouter = useRouter();
  const searchParams = useSearchParams();
  const router = useTenantRouter();
  const roomId = params.id as string;
  const referrer = resolveDetailReferrer(searchParams.get("from"), "/rooms", [
    "/rooms",
    "/database/rooms",
  ]);
  const backLabel = backLabelFor(referrer);
  const { data: session } = useSession();
  const presenceMode = usePresenceMode();
  const { success: toastSuccess, error: toastError } = useToast();
  const tenantMutate = useTenantMutate();
  const refreshRoomConsumers = useTenantMutateMatching(
    ROOM_DERIVED_CACHE_KEY_FRAGMENTS,
  );
  const service = useMemo(() => createCrudService(roomsConfig), []);

  const { room, history, loading, error, historyDisabled, historyError } =
    useRoomDetail(roomId);

  // Spiegel der Backend-Gates auf PUT/DELETE /api/rooms/{id}.
  const canUpdate = hasPermission(session, "rooms:update");
  const canDelete = hasPermission(session, "rooms:delete");
  // Im binären Modus führt die Schule keine Belegung; die Raumseite ist dann
  // der reine Datensatz.
  const hasOccupancy = presenceMode !== "binary";

  const tabItems: TenantPageTab[] = [
    ...(hasOccupancy ? [{ value: "uebersicht", label: "Übersicht" }] : []),
    ...(canUpdate ? [{ value: "stammdaten", label: "Stammdaten" }] : []),
  ];
  const requestedTab = searchParams.get("tab");
  const defaultTab: RoomTab =
    requestedTab === "stammdaten" && canUpdate
      ? "stammdaten"
      : hasOccupancy
        ? "uebersicht"
        : "stammdaten";
  const [selectedTab, setSelectedTab] = useState<string | null>(null);
  const activeTab =
    selectedTab && tabItems.some((tab) => tab.value === selectedTab)
      ? selectedTab
      : defaultTab;

  const handleTabChange = useCallback(
    (nextTab: string) => {
      setSelectedTab(nextTab);
      const query = new URLSearchParams(searchParams.toString());
      if (nextTab === "uebersicht") {
        query.delete("tab");
      } else {
        query.set("tab", nextTab);
      }
      const queryString = query.toString();
      navRouter.replace(queryString ? `${pathname}?${queryString}` : pathname, {
        scroll: false,
      });
    },
    [navRouter, pathname, searchParams],
  );

  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const [deleting, setDeleting] = useState(false);

  useSetBreadcrumb({ roomName: room?.name, referrerPage: referrer });

  const refreshRoomLists = useCallback(
    () =>
      Promise.all([
        tenantMutate(roomDetailKey(roomId)),
        ...ROOM_LIST_CACHE_KEYS.map((key) => tenantMutate(key)),
      ]),
    [roomId, tenantMutate],
  );

  const handleSaveRoom = useCallback(
    async (data: Partial<Room>) => {
      if (!room) return;
      try {
        const payload = roomsConfig.form.transformBeforeSubmit
          ? roomsConfig.form.transformBeforeSubmit(data)
          : data;
        await service.update(room.id, payload);
        toastSuccess(
          getDbOperationMessage("update", roomsConfig.name.singular, room.name),
        );
        await Promise.all([
          refreshRoomLists(),
          // Refetch every consumer that holds room-stamped data so badges
          // pick up the new color without a manual reload.
          refreshRoomConsumers(),
        ]);
      } catch (updateError) {
        logger.error("failed to update room", {
          room_id: room.id,
          error:
            updateError instanceof Error
              ? updateError.message
              : String(updateError),
        });
        throw updateError;
      }
    },
    [refreshRoomConsumers, refreshRoomLists, room, service, toastSuccess],
  );

  const handleDeleteRoom = useCallback(async () => {
    if (!room) return;
    setDeleting(true);
    try {
      const deleteError = await service.delete(room.id);
      if (deleteError) {
        toastError(deleteError);
        return;
      }
      toastSuccess(
        getDbOperationMessage("delete", roomsConfig.name.singular, room.name),
      );
      await Promise.all(ROOM_LIST_CACHE_KEYS.map((key) => tenantMutate(key)));
      setShowDeleteModal(false);
      router.push(referrer);
    } finally {
      setDeleting(false);
    }
  }, [referrer, room, router, service, tenantMutate, toastError, toastSuccess]);

  if (loading && !room) {
    return <RoomDetailLoadingPage referrer={referrer} backLabel={backLabel} />;
  }

  if (error || !room) {
    return (
      <TenantPage
        title="Raum"
        back
        backHref={referrer}
        backLabel={backLabel}
        error={error ?? "Raum nicht gefunden"}
      />
    );
  }

  const menuItems: OverflowMenuItem[] =
    canDelete && !isSystemRoom(room)
      ? [
          {
            label: "Löschen",
            icon: <Trash2 className="size-4" aria-hidden />,
            destructive: true,
            onClick: () => setShowDeleteModal(true),
          },
        ]
      : [];

  // Statuszeile: Ort und Art des Raums, wie sie die Sammlung auch zeigt.
  const statusLine = [
    room.building && room.floor !== undefined
      ? `${room.building} · ${formatFloor(room.floor)}`
      : (room.building ??
        (room.floor === undefined ? null : formatFloor(room.floor))),
    room.category ?? null,
    room.capacity !== undefined ? `${room.capacity} Plätze` : null,
  ]
    .filter(Boolean)
    .join(" · ");

  return (
    <TenantPage
      title={room.name}
      stats={statusLine || undefined}
      back
      backHref={referrer}
      backLabel={backLabel}
      leading={
        <MotoDuotoneIcon
          icon={MOTO_CONCEPTS.rooms.icon}
          tone={MOTO_CONCEPTS.rooms.tone}
          size={40}
        />
      }
      actions={
        <>
          {hasOccupancy ? (
            <RoomStatusBadge isOccupied={room.isOccupied} />
          ) : null}
          {menuItems.length > 0 ? (
            <OverflowMenu ariaLabel="Weitere Aktionen" items={menuItems} />
          ) : null}
        </>
      }
      tabs={
        tabItems.length > 1
          ? {
              value: activeTab,
              onChange: handleTabChange,
              items: tabItems,
              label: "Bereiche des Raums",
            }
          : undefined
      }
      overlays={
        <ConfirmDeleteModal
          isOpen={showDeleteModal}
          onClose={() => setShowDeleteModal(false)}
          onConfirm={() => void handleDeleteRoom()}
          title="Raum löschen?"
          description={
            <>
              Möchten Sie den Raum{" "}
              <span className="font-medium">{room.name}</span> wirklich löschen?
              Diese Aktion kann nicht rückgängig gemacht werden.
            </>
          }
          gate={{ mode: "twoStep" }}
          loading={deleting}
          error=""
        />
      }
    >
      {!hasOccupancy && !canUpdate ? (
        <RoomDetailContent
          room={room}
          history={history}
          historyDisabled={historyDisabled}
          historyError={historyError}
          showOccupancy={false}
        />
      ) : null}
      {activeTab === "uebersicht" && hasOccupancy ? (
        loading ? (
          <RoomDetailSkeleton />
        ) : (
          <RoomDetailContent
            room={room}
            history={history}
            historyDisabled={historyDisabled}
            historyError={historyError}
          />
        )
      ) : null}
      {activeTab === "stammdaten" && canUpdate ? (
        <RoomStammdatenTab room={room} onSave={handleSaveRoom} />
      ) : null}
    </TenantPage>
  );
}
