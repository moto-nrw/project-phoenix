"use client";

import { roomKeys } from "~/lib/swr/rooms-keys";

import {
  Suspense,
  useCallback,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import {
  useParams,
  usePathname,
  useRouter,
  useSearchParams,
} from "next/navigation";
import { useSession } from "next-auth/react";
import { FileDown, Trash2 } from "lucide-react";
import { ButtonLink } from "~/components/ui/button";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import type { OverflowMenuItem } from "~/components/ui/page-header/OverflowMenu";
import { TenantPage, type TenantPageTab } from "~/components/ui/tenant-page";
import { RoomStatusBadge } from "~/components/rooms/room-status-badge";
import {
  RoomDetailContent,
  RoomDetailSkeleton,
  useRoomDetail,
} from "~/components/rooms/room-detail-content";
import { RoomStammdatenTab } from "~/components/rooms/room-stammdaten-tab";
import {
  parseRoomPlanView,
  useRoomPlanHref,
  type RoomPlanView,
} from "~/components/rooms/room-plan-navigation";
import {
  RoomPlanControls,
  RoomPlanTab,
} from "~/components/rooms/room-plan-lazy";
import { roomsConfig } from "~/components/database/configs/rooms.config";
import {
  useApiErrorDisplay,
  useApiFormError,
  useToast,
} from "~/contexts/ToastContext";
import { roomService } from "~/lib/api";
import { canReadSupervisionData, hasPermission } from "~/lib/auth-utils";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { useOptionalSupervision } from "~/lib/supervision-context";
import { createCrudService } from "~/lib/database/service-factory";
import { isValidISODate } from "~/lib/date-helpers";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { createLogger } from "~/lib/logger";
import { exportPlan } from "~/lib/plan-export-api";
import { NO_PLAN_FILTER } from "~/lib/plan-filter";
import { roomPlanCareDay } from "~/lib/room-plan";
import { MOTO_CONCEPTS } from "~/lib/moto-concepts";
import { formatFloor, isSystemRoom, type Room } from "~/lib/room-helpers";
import { useTenantMutate, useTenantMutateMatching } from "~/lib/swr";
import { ROOM_DERIVED_CACHE_KEY_FRAGMENTS } from "~/lib/swr/room-derived-caches";
import { usePresenceMode, useWeekendFollowsFriday } from "~/lib/tenant-context";
import { resolveDetailReferrer, useTenantAwarePath } from "~/lib/tenant-path";
import { useTenantRouter } from "~/lib/tenant-router";
import { RoomDetailLoadingPage } from "./page-skeleton";

const logger = createLogger({ component: "RoomDetailPage" });

type RoomTab = "uebersicht" | "plan" | "stammdaten";

/** Parameter des Reiters „Plan“ (#4768); andere Reiter tragen sie nicht. */
const PLAN_TAB_PARAMS = ["d", "view"] as const;

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
 * Kind da ist (binärer Modus), gibt es keine Belegung und damit keinen
 * Reiter „Übersicht".
 *
 * „Plan" (#4768) zeigt, wann der Raum laut Betreuungsplan belegt und wann er
 * frei ist, für alle, die den Betreuungsplan lesen dürfen, auch im binären
 * Modus. URL: `tab=plan`, `d` (Tag) und `view` (`tag` | `woche`).
 */
function RoomDetailPageContent() {
  const params = useParams();
  const pathname = usePathname();
  const navRouter = useRouter();
  const searchParams = useSearchParams();
  const router = useTenantRouter();
  const tenantPath = useTenantAwarePath();
  const roomId = params.id as string;
  const referrer = resolveDetailReferrer(searchParams.get("from"), "/rooms", [
    "/rooms",
    "/database/rooms",
  ]);
  const backLabel = backLabelFor(referrer);
  const { data: session } = useSession();
  const presenceMode = usePresenceMode();
  const weekendOpen = useWeekendFollowsFriday();
  const { success: toastSuccess } = useToast();
  // Stammdaten speichern: Fehler im Formular, Feldfehler am Feld (#2517).
  const saveErrors = useApiFormError();
  // Löschen: der Fehler bleibt im offenen Löschdialog.
  const deleteErrors = useApiFormError();
  const { clear: clearDeleteErrors, show: showDeleteError } = deleteErrors;
  const retryDeleteRef = useRef<() => void>(() => undefined);
  const tenantMutate = useTenantMutate();
  const refreshRoomFamily = useTenantMutateMatching([roomKeys.family]);
  const refreshRoomConsumers = useTenantMutateMatching(
    ROOM_DERIVED_CACHE_KEY_FRAGMENTS,
  );
  const service = useMemo(() => createCrudService(roomsConfig), []);

  const { room, history, loading, error, historyDisabled, historyError } =
    useRoomDetail(roomId);

  // Spiegel der Backend-Gates auf PUT/DELETE /api/rooms/{id}.
  const canUpdate = hasPermission(session, "rooms:update");
  const canDelete = hasPermission(session, "rooms:delete");
  // Plan des Raums (#4768): dasselbe Recht wie die Leseansicht des
  // Betreuungsplans; der Druck braucht dazu die Namen (wie dort).
  const canReadPlan = hasPermission(session, "schedules:read");
  const canManageSchedules = hasPermission(session, "schedules:manage");
  const canExportPlan = canReadPlan && hasPermission(session, "users:read");
  // Im binären Modus führt die Schule keine Belegung; die Raumseite ist dann
  // der reine Datensatz.
  const hasOccupancy = presenceMode !== "binary";
  // Ein offener Raum (#3065) hat seine gemeinsame Aufsichtsansicht. Sie ist
  // seit #4372 nur noch über „Räume“ erreichbar, also über diese Seite.
  // Dieselbe Regel wie der Zugang zur Aufsichtsseite.
  const { overviewEnabled } = useOptionalSupervision();
  const canOpenSupervision = canReadSupervisionData(session) || overviewEnabled;

  const tabItems: TenantPageTab[] = [
    ...(hasOccupancy ? [{ value: "uebersicht", label: "Übersicht" }] : []),
    ...(canReadPlan ? [{ value: "plan", label: "Plan" }] : []),
    ...(canUpdate ? [{ value: "stammdaten", label: "Stammdaten" }] : []),
  ];
  const requestedTab = searchParams.get("tab");
  const defaultTab: RoomTab =
    requestedTab === "stammdaten" && canUpdate
      ? "stammdaten"
      : requestedTab === "plan" && canReadPlan
        ? "plan"
        : hasOccupancy
          ? "uebersicht"
          : canUpdate || !canReadPlan
            ? "stammdaten"
            : "plan";
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
      if (nextTab !== "plan") {
        for (const key of PLAN_TAB_PARAMS) query.delete(key);
      }
      const queryString = query.toString();
      navRouter.replace(queryString ? `${pathname}?${queryString}` : pathname, {
        scroll: false,
      });
    },
    [navRouter, pathname, searchParams],
  );

  // Zeitraum des Reiters „Plan“: ein ungültiges `d` fällt still auf heute.
  const todayISO = useBerlinToday();
  const rawPlanDay = searchParams.get("d");
  const planAnchorISO =
    rawPlanDay !== null && isValidISODate(rawPlanDay) ? rawPlanDay : todayISO;
  // Der Reiter zeigt Wochenend-Deeplinks ohne Wochenendbetreuung ab Montag.
  // Der Druck muss dieselbe angezeigte Woche verwenden.
  const planExportDayISO = roomPlanCareDay(
    planAnchorISO,
    weekendOpen,
    todayISO,
  );
  const planView = parseRoomPlanView(searchParams.get("view"));
  const planHref = useRoomPlanHref(
    roomId,
    planView,
    planAnchorISO,
    canManageSchedules,
  );
  const setPlanParams = useCallback(
    (next: { view?: RoomPlanView; d?: string }) => {
      const query = new URLSearchParams(searchParams.toString());
      query.set("tab", "plan");
      if (next.d) query.set("d", next.d);
      if (next.view) query.set("view", next.view);
      navRouter.replace(`${pathname}?${query.toString()}`, { scroll: false });
    },
    [navRouter, pathname, searchParams],
  );
  // „Raumplan als PDF“ (#4768): der Aushang der angezeigten Woche, mit dem
  // Raumfilter des Betreuungsplan-Exports. Bewusst ohne den Druckdialog des
  // Betreuungsplans: dessen Kalender wöge mehr als die ganze Raumseite.
  const { show: showExportError } = useApiErrorDisplay();
  const [exporting, setExporting] = useState(false);
  const exportRoomPlan = useCallback(async () => {
    setExporting(true);
    try {
      await exportPlan(
        "betreuungsplan",
        {
          from: planExportDayISO,
          to: planExportDayISO,
          template: "offerings",
          variant: "aushang",
          filter: { ...NO_PLAN_FILTER, roomId },
        },
        "pdf",
        "download",
      );
    } catch (exportError) {
      void showExportError(exportError, {
        object: "das Erstellen des Raumplans",
        retry: () => void exportRoomPlan(),
      });
    } finally {
      setExporting(false);
    }
  }, [planExportDayISO, roomId, showExportError]);

  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const [deleting, setDeleting] = useState(false);

  useSetBreadcrumb({ roomName: room?.name, referrerPage: referrer });

  const refreshRoomLists = useCallback(
    () =>
      Promise.all([
        tenantMutate(roomKeys.detailKey(roomId)),
        refreshRoomFamily(),
      ]),
    [roomId, tenantMutate, refreshRoomFamily],
  );

  const handleSaveRoom = useCallback(
    async (data: Partial<Room>) => {
      if (!room) return;
      try {
        const payload = roomsConfig.form.transformBeforeSubmit
          ? roomsConfig.form.transformBeforeSubmit(data)
          : data;
        await service.update(room.id, payload);
        const savedName = data.name?.trim() || room.name;
        toastSuccess(`Der Raum „${savedName}“ ist gespeichert.`);
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
    clearDeleteErrors();
    try {
      await roomService.deleteRoom(room.id);
    } catch (deleteError) {
      logger.warn("failed to delete room", {
        room_id: room.id,
        error:
          deleteError instanceof Error
            ? deleteError.message
            : String(deleteError),
      });
      void showDeleteError(deleteError, {
        object: "das Löschen des Raums",
        retry: () => retryDeleteRef.current(),
      });
      setDeleting(false);
      return;
    }
    try {
      toastSuccess(`Der Raum „${room.name}“ ist gelöscht.`);
      await refreshRoomFamily();
      setShowDeleteModal(false);
      router.push(referrer);
    } finally {
      setDeleting(false);
    }
  }, [
    clearDeleteErrors,
    showDeleteError,
    referrer,
    room,
    router,
    refreshRoomFamily,
    toastSuccess,
  ]);
  useLayoutEffect(() => {
    retryDeleteRef.current = () => void handleDeleteRoom();
  });

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

  const showsPlan = activeTab === "plan" && canReadPlan;
  const menuItems: OverflowMenuItem[] = [
    ...(showsPlan && canExportPlan
      ? [
          {
            label: exporting ? "Raumplan wird erstellt…" : "Raumplan als PDF",
            icon: <FileDown className="size-4" aria-hidden />,
            onClick: () => {
              if (!exporting) void exportRoomPlan();
            },
          },
        ]
      : []),
    ...(canDelete && !isSystemRoom(room)
      ? [
          {
            label: "Löschen",
            icon: <Trash2 className="size-4" aria-hidden />,
            destructive: true,
            onClick: () => setShowDeleteModal(true),
          },
        ]
      : []),
  ];

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
          {hasOccupancy && room.isOpenRoom && canOpenSupervision ? (
            <ButtonLink
              variant="outline"
              size="md"
              href={tenantPath(`/active-supervisions?room=${room.id}`)}
            >
              Zur Aufsicht
            </ButtonLink>
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
      searchSlot={
        showsPlan ? (
          <RoomPlanControls
            view={planView}
            anchorISO={planAnchorISO}
            onChange={setPlanParams}
            planHref={planHref}
            planLinkLabel={
              canManageSchedules
                ? "Im Betreuungsplan bearbeiten"
                : "Im Betreuungsplan ansehen"
            }
          />
        ) : undefined
      }
      overlays={
        <>
          <ConfirmDeleteModal
            isOpen={showDeleteModal}
            onClose={() => {
              clearDeleteErrors();
              setShowDeleteModal(false);
            }}
            onConfirm={() => void handleDeleteRoom()}
            title="Raum löschen?"
            description={
              <>
                Möchten Sie den Raum{" "}
                <span className="font-medium">{room.name}</span> wirklich
                löschen? Diese Aktion kann nicht rückgängig gemacht werden.
              </>
            }
            gate={{ mode: "twoStep" }}
            loading={deleting}
            error={deleteErrors.error}
          />
        </>
      }
    >
      {/* Ohne Übersicht und Stammdaten bleiben die Rauminformationen stehen;
          ein Plan steht dann darunter, ohne Reiterleiste. */}
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
      {showsPlan ? (
        <RoomPlanTab
          roomId={room.id}
          view={planView}
          anchorISO={planAnchorISO}
        />
      ) : null}
      {activeTab === "stammdaten" && canUpdate ? (
        <RoomStammdatenTab
          room={room}
          showOccupancy={hasOccupancy}
          onSave={handleSaveRoom}
          errorPath={saveErrors}
        />
      ) : null}
    </TenantPage>
  );
}
