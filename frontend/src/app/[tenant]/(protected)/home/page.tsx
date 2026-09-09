"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { SlidersHorizontal } from "lucide-react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";

import { HomeBlockContent } from "~/components/home/home-block-content";
import { HomeBoard } from "~/components/home/home-board";
import { NowStrip } from "~/components/home/now-strip";
import { PhaseExpiryWarnings } from "~/components/enrollment/phase-expiry-warnings";
import { Button } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { TenantPage } from "~/components/ui/tenant-page";
import { hasEffectiveAdminScope } from "~/lib/auth-utils";
import { fetchBirthdayOverviewClient } from "~/lib/birthdays-api";
import type { BirthdayOverview } from "~/lib/birthdays-api";
import { fetchDashboardAnalyticsClient } from "~/lib/dashboard-api";
import type { DashboardAnalytics } from "~/lib/dashboard-helpers";
import { formatStatusDate } from "~/lib/date-helpers";
import { getTimeBasedGreeting } from "~/lib/greeting";
import {
  appendPlacement,
  defaultSpanFor,
  movePlacementBy,
  placePlacement,
  placementWithSpan,
  sortedPlacements,
  resolveHomeLayout,
  withoutPlacement,
  type HomeBlockKey,
  type HomeBlockPlacement,
  type HomeBlockSpan,
  type HomeMoveDirection,
  type HomeMoveTarget,
} from "~/lib/home-blocks";
import { useHomeBlockAccess } from "~/lib/hooks/use-home-block-access";
import { useReminders } from "~/lib/hooks/use-reminders";
import { useHomeLayout } from "~/lib/hooks/use-home-layout";
import { createLogger } from "~/lib/logger";
import { useSWRAuth } from "~/lib/swr/hooks";
import {
  useNFCEnabled,
  useOpenCareGroupMode,
  usePresenceMode,
  useTenantSafe,
  useTenantSlugSafe,
  useTimetableEnabled,
} from "~/lib/tenant-context";
import { useTenantAwarePath } from "~/lib/tenant-path";
import { useTenantRouter } from "~/lib/tenant-router";
import { UserContextProvider } from "~/lib/usercontext-context";
import { DashboardSkeleton } from "./page-skeleton";

const logger = createLogger({ component: "HomePage" });

// Alles, was aus den Betriebszahlen (/analytics) lebt. Steht keiner dieser
// Bausteine auf der Fläche, wird die Abfrage gar nicht erst gestellt (#2875).
const ANALYTICS_BLOCKS: readonly HomeBlockKey[] = [
  "tile.students_present",
  "tile.students_in_rooms",
  "tile.students_in_transit",
  "tile.students_on_playground",
  "tile.students_sick",
  "tile.students_excused",
  "tile.students_home",
  "tile.active_activities",
  "tile.capacity_utilization",
  "section.recent_activity",
  "section.current_activities",
  "section.active_groups",
  // Die Kräfte in Aufsicht kommen aus derselben Antwort.
  "section.staff_today",
];

function HomeContent() {
  const router = useTenantRouter();
  const tenantPath = useTenantAwarePath();
  const nfcEnabled = useNFCEnabled();
  const openCareGroupMode = useOpenCareGroupMode();
  const presenceMode = usePresenceMode();
  const timetableEnabled = useTimetableEnabled();
  const tenantSlug = useTenantSlugSafe();
  // Nachrichten und Team-Chat sind Schulschalter aus dem Tenant-Resolve,
  // wie in der Seitenleiste.
  const tenant = useTenantSafe()?.tenant;
  const messagingEnabled = tenant?.messagingEnabled === true;
  const staffMessagingEnabled = tenant?.staffMessagingEnabled === true;
  const access = useHomeBlockAccess();
  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      router.replace("/");
    },
  });

  const {
    state: homeLayout,
    save: saveHomeLayout,
    reset: resetHomeLayout,
  } = useHomeLayout();

  const [birthdaysEnabled, setBirthdaysEnabled] = useState(true);
  // Erinnerungen sind pro Schule abschaltbar und stehen standardmäßig aus
  // (#1457). Die Abfrage läuft ohnehin für die Glocke in der Kopfzeile und
  // teilt sich deren SWR-Schlüssel; hier zählt nur, ob die Schule eine Art
  // eingeschaltet hat. Optimistisch an, solange die Antwort aussteht — sonst
  // flackerte der Baustein bei jedem Seitenaufruf herein.
  const remindersEnabled = useReminders().data?.enabled ?? true;
  // Der Entwurf im Anpassen-Modus. `null` heisst: gerade wird nicht angepasst.
  const [draft, setDraft] = useState<HomeBlockPlacement[] | null>(null);
  const [removedInDraft, setRemovedInDraft] = useState<HomeBlockKey[]>([]);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  const blockContext = useMemo(
    () => ({
      detailed: presenceMode !== "binary",
      openCareGroupMode,
      nfcEnabled,
      birthdaysEnabled,
      timetableEnabled,
      remindersEnabled,
      messagingEnabled,
      staffMessagingEnabled,
      access,
    }),
    [
      presenceMode,
      openCareGroupMode,
      nfcEnabled,
      birthdaysEnabled,
      timetableEnabled,
      remindersEnabled,
      messagingEnabled,
      staffMessagingEnabled,
      access,
    ],
  );

  // Was gespeichert ist, ergibt zusammen mit Standardansicht und Schulvorgabe
  // die Fläche. Im Anpassen-Modus zeigt stattdessen der Entwurf.
  const saved = useMemo(
    () =>
      resolveHomeLayout(
        blockContext,
        homeLayout.blocks,
        homeLayout.overrides,
        homeLayout.policies,
      ),
    [
      blockContext,
      homeLayout.blocks,
      homeLayout.overrides,
      homeLayout.policies,
    ],
  );

  const placements = draft ?? saved.placements;
  const placedKeys = useMemo(
    () => new Set(placements.map((placement) => placement.key)),
    [placements],
  );
  const addable = saved.available.filter(
    (block) =>
      !placedKeys.has(block.key) &&
      (homeLayout.policies[block.key] ?? "optional") !== "disabled",
  );
  const requiredKeys = useMemo(
    () =>
      new Set<HomeBlockKey>(
        Object.entries(homeLayout.policies)
          .filter(([, policy]) => policy === "required")
          .map(([key]) => key as HomeBlockKey),
      ),
    [homeLayout.policies],
  );

  const wantsBirthdayData = placedKeys.has("section.birthdays");
  // Das Analytics-Endpunkt verlangt groups:read. „Personal heute" bleibt
  // ohne dieses Recht sichtbar, erhält dann aber nur seine eigene Quelle.
  const needsAnalytics =
    access.has("groups:read") &&
    ANALYTICS_BLOCKS.some((key) => placedKeys.has(key));

  // Geburtstage leben auf ihrem eigenen Schlüssel: sie ändern sich einmal am
  // Tag, während die Betriebszahlen bei jedem Check-in über SSE neu geladen
  // werden (#1542). Ein Fehler hier darf die Startseite nie mitnehmen.
  const { data: birthdays, isLoading: birthdaysLoading } =
    useSWRAuth<BirthdayOverview>(
      wantsBirthdayData ? "birthday-overview" : null,
      fetchBirthdayOverviewClient,
      { refreshInterval: 30 * 60 * 1000 },
    );

  useEffect(() => {
    setBirthdaysEnabled(true);
  }, [tenantSlug, session?.user?.id]);

  useEffect(() => {
    setBirthdaysEnabled(birthdays?.enabled ?? true);
  }, [birthdays?.enabled]);

  const {
    data: dashboardData,
    isLoading,
    error: swrError,
  } = useSWRAuth<DashboardAnalytics>(
    needsAnalytics ? "dashboard-analytics" : null,
    fetchDashboardAnalyticsClient,
    { refreshInterval: 5 * 60 * 1000 },
  );

  if (swrError) {
    logger.error("dashboard_fetch_failed", {
      error: swrError instanceof Error ? swrError.message : String(swrError),
    });
  }

  const error = swrError ? "Fehler beim Laden der Dashboard-Daten" : null;

  const startEditing = useCallback(() => {
    setDraft([...saved.placements]);
    setRemovedInDraft([]);
    setSaveError(null);
  }, [saved.placements]);

  const cancelEditing = useCallback(() => {
    setDraft(null);
    setRemovedInDraft([]);
    setSaveError(null);
  }, []);

  // Jede Änderung am Entwurf läuft durch dieselben Regeln (`home-blocks.ts`):
  // eine Kachel liegt in ihrer Zelle, was sie überdeckt, rückt nach unten,
  // und Lücken bleiben, wie die Person sie gelassen hat.
  //
  // Beim Ziehen rechnet jeder Zug von der Anordnung beim Anfassen aus
  // (`base`), nicht vom letzten Zwischenstand: nur so kehren Kacheln, die
  // unterwegs ausgewichen sind, zurück, sobald die gezogene weiterzieht.
  const move = useCallback(
    (
      key: HomeBlockKey,
      target: HomeMoveTarget,
      base: readonly HomeBlockPlacement[],
    ) => {
      setDraft((current) => {
        if (!current) return current;
        const next = placePlacement(base, key, target);
        return next === base ? current : next;
      });
    },
    [],
  );

  const moveBy = useCallback(
    (key: HomeBlockKey, direction: HomeMoveDirection) => {
      setDraft((current) =>
        current ? movePlacementBy(current, key, direction) : current,
      );
    },
    [],
  );

  const changeSpan = useCallback((key: HomeBlockKey, span: HomeBlockSpan) => {
    setDraft((current) =>
      current ? placementWithSpan(current, key, span) : current,
    );
  }, []);

  const removeBlock = useCallback((key: HomeBlockKey) => {
    setDraft((current) => (current ? withoutPlacement(current, key) : null));
    setRemovedInDraft((current) =>
      current.includes(key) ? current : [...current, key],
    );
  }, []);

  const addBlock = useCallback((key: HomeBlockKey) => {
    setDraft((current) => {
      if (!current || current.some((placement) => placement.key === key)) {
        return current;
      }
      // Neu hinzugefügt heisst hinten: die vorhandene Anordnung soll sich
      // durch ein Hinzufügen nicht verschieben.
      return appendPlacement(current, key, defaultSpanFor(key));
    });
    setRemovedInDraft((current) => current.filter((entry) => entry !== key));
  }, []);

  const save = useCallback(async () => {
    if (!draft) return;
    setSaving(true);
    setSaveError(null);
    try {
      // Gespeichert wird die Anordnung UND was bewusst entfernt wurde. Ohne
      // das zweite käme ein entfernter Baustein aus der Standardansicht beim
      // nächsten Aufruf zurück.
      const overrides = { ...homeLayout.overrides };
      for (const key of removedInDraft) overrides[key] = false;
      for (const placement of draft) delete overrides[placement.key];
      // In Lesereihenfolge, damit dieselbe Anordnung immer gleich gespeichert
      // wird, egal in welcher Reihenfolge die Kacheln gezogen wurden.
      await saveHomeLayout(overrides, sortedPlacements(draft));
      setDraft(null);
      setRemovedInDraft([]);
    } catch (err: unknown) {
      logger.error("home_layout_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setSaveError(
        "Die Startseite konnte nicht gespeichert werden. Bitte erneut versuchen.",
      );
    } finally {
      setSaving(false);
    }
  }, [draft, homeLayout.overrides, removedInDraft, saveHomeLayout]);

  const restoreDefault = useCallback(async () => {
    setSaving(true);
    setSaveError(null);
    try {
      await resetHomeLayout();
      setDraft(null);
      setRemovedInDraft([]);
    } catch (err: unknown) {
      logger.error("home_layout_reset_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setSaveError(
        "Die Startseite konnte nicht zurückgesetzt werden. Bitte erneut versuchen.",
      );
    } finally {
      setSaving(false);
    }
  }, [resetHomeLayout]);

  if (
    status === "authenticated" &&
    session &&
    (session.error === "RefreshTokenExpired" || !session.user?.token)
  ) {
    logger.info("invalid session, redirecting to login");
    redirect(tenantPath("/"));
  }

  const editing = draft !== null;
  const firstName = session?.user?.name?.trim().split(/\s+/)[0];
  const greeting = getTimeBasedGreeting();
  const canReadPhaseExpiryWarnings = hasEffectiveAdminScope(session);
  // Nur das Datum: die Zahlen des Tages trägt die Jetzt-Zone darunter, und
  // die Kennzahlen stehen als Kacheln auf der Fläche. Dieselbe Zahl dreimal
  // auf einem Bildschirm sagt nicht mehr als einmal.
  const headerStats = formatStatusDate();

  const blockData = {
    analytics: dashboardData,
    analyticsLoading: isLoading,
    birthdays,
    birthdaysLoading,
    tenantPath,
  };

  return (
    <TenantPage
      title={
        editing
          ? "Startseite anpassen"
          : firstName
            ? `${greeting}, ${firstName}`
            : greeting
      }
      prominent
      statsLoading={isLoading}
      stats={
        editing
          ? "Karte ziehen, um sie zu verschieben. Anklicken, um Breite zu ändern oder sie zu entfernen."
          : headerStats
      }
      error={
        saveError
          ? { message: saveError, keepContent: true }
          : error
            ? { message: error, keepContent: dashboardData !== undefined }
            : null
      }
      actions={
        editing ? (
          <>
            <Button
              type="button"
              variant="outline"
              size="md"
              disabled={saving}
              onClick={cancelEditing}
            >
              Abbrechen
            </Button>
            <Button
              type="button"
              variant="primary"
              size="md"
              disabled={saving}
              onClick={save}
            >
              Fertig
            </Button>
          </>
        ) : (
          <Button
            type="button"
            variant="outline"
            size="md"
            className="gap-2"
            onClick={startEditing}
          >
            <SlidersHorizontal className="h-4 w-4" aria-hidden="true" />
            Anpassen
          </Button>
        )
      }
    >
      {canReadPhaseExpiryWarnings && !editing ? <PhaseExpiryWarnings /> : null}

      {/* Die Jetzt-Zone steht fest über dem Brett und ist kein Baustein: die
          Frage „was steht jetzt an" beantwortet die Startseite immer, egal
          was jemand darunter angeordnet oder entfernt hat. Im Anpassen-Modus
          tritt sie zurück, dort geht es um die Anordnung. */}
      {!editing ? <NowStrip access={access} context={blockContext} /> : null}

      {placements.length === 0 && !editing ? (
        <EmptyState
          title="Ihre Startseite ist leer"
          description={
            saved.available.length === 0
              ? "Für Ihre Berechtigungen gibt es hier noch keine Bausteine. Ihre Leitung kann Ihnen weitere Bereiche freischalten."
              : "Sie haben alle Bausteine entfernt."
          }
          action={
            saved.available.length > 0 ? (
              <Button
                type="button"
                variant="primary"
                size="md"
                onClick={startEditing}
              >
                Bausteine hinzufügen
              </Button>
            ) : undefined
          }
        />
      ) : (
        <HomeBoard
          placements={placements}
          addable={addable}
          requiredKeys={requiredKeys}
          editing={editing}
          onMove={move}
          onMoveBy={moveBy}
          onSpanChange={changeSpan}
          onRemove={removeBlock}
          onAdd={addBlock}
          onRestoreDefault={restoreDefault}
          restoring={saving}
        >
          {(placement) => (
            <HomeBlockContent blockKey={placement.key} data={blockData} />
          )}
        </HomeBoard>
      )}
    </TenantPage>
  );
}

/**
 * Die Startseite der App (#2180) — für jede Rolle dieselbe Route, aber nicht
 * dieselbe Fläche.
 *
 * Wer nichts anpasst, sieht die Standardansicht seiner Rolle. Wer anpasst,
 * stellt sich seine Fläche selbst zusammen — aus dem, was die eigenen Rechte,
 * der Betriebsmodus der Schule und die Vorgabe der Leitung freigeben.
 */
export default function HomePage() {
  const { status } = useSession();

  // Vor der Sitzung ist nicht entscheidbar, welche Bausteine gelten. Ohne
  // diesen Zwischenschritt blitzte eine leere Startseite auf, bevor die Rechte
  // da sind.
  if (status === "loading") {
    return <DashboardSkeleton />;
  }

  return (
    <UserContextProvider>
      <HomeContent />
    </UserContextProvider>
  );
}
