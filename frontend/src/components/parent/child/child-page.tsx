"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  CalendarDotsIcon,
  IdentificationCardIcon,
  UsersThreeIcon,
} from "@phosphor-icons/react/ssr";
import { useRouter, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { Alert } from "~/components/ui/alert";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "~/components/ui/tabs";
import {
  PickupTimeModal,
  SickNoteModal,
  SickStatusSummary,
  useChildCare,
} from "~/components/parent/child-care";
import { BookedCareSection } from "~/components/parent/child/booked-care-section";
import { ParentSection } from "~/components/parent/shell/parent-section";
import { ChildDayCard } from "~/components/parent/child/child-day-card";
import {
  ChildSwitcher,
  type ChildSwitcherItem,
} from "~/components/parent/child/child-switcher";
import { ChildMasterDataView } from "~/components/parent/child-master-data";
import GuardiansPanel from "~/components/parent/guardians-panel";
import { createLogger } from "~/lib/logger";
import {
  UNKNOWN_CHILD_TODAY,
  getChildToday,
  listMyChildren,
  type Child,
  type ChildToday,
} from "~/lib/parent-api";
import { parentPath } from "~/lib/parent-url";
import {
  ParentPage,
  ParentPageHeader,
  ParentPageSkeleton,
} from "~/components/parent/parent-page";

const logger = createLogger({ component: "ChildPage" });

type ChildArea = "betreuung" | "angaben" | "kontakte";
type PhosphorIcon = typeof CalendarDotsIcon;

/**
 * Der Kinderbereich, kind-zentriert nach Entscheidung E9.
 *
 * Bei einem Kind zeigt die Seite direkt das Profil. Bei mehreren beginnt sie
 * mit einer Auswahlseite und zeigt danach nur das gewaehlte Kinderprofil.
 *
 * Der Tagesstatus bleibt immer sichtbar. Drei lokale Reiter gliedern die
 * weiteren Inhalte, ohne eine neue Seite oder Navigationsebene zu oeffnen.
 *
 * Die frueheren Rubriken "Betreuungszeiten" (#2302) und "AGs und Gruppen"
 * (#2303) sind ersatzlos entfallen.
 */
export function ChildPage({
  studentId,
}: Readonly<{
  /** Aus der URL. Fehlt er, folgt je nach Anzahl Profil oder Auswahl. */
  studentId?: string;
}>) {
  const t = useTranslations("parentChild");
  const navT = useTranslations("parentNav");
  const [children, setChildren] = useState<Child[]>([]);
  const [selectionToday, setSelectionToday] = useState<
    Readonly<Record<string, ChildToday>>
  >({});
  const [loading, setLoading] = useState(true);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    let active = true;
    setLoading(true);
    listMyChildren()
      .then(async (list) => {
        const todayEntries =
          !studentId && list.length > 1
            ? await Promise.all(
                list.map(
                  async (child) =>
                    [
                      child.student_id,
                      await getChildToday(child.student_id).catch(
                        () => UNKNOWN_CHILD_TODAY,
                      ),
                    ] as const,
                ),
              )
            : [];
        if (!active) return;
        setChildren(list);
        setSelectionToday(Object.fromEntries(todayEntries));
        setFailed(false);
      })
      .catch((err: unknown) => {
        if (!active) return;
        logger.warn("parent_child_page_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        setFailed(true);
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [studentId]);

  const active = useMemo(() => {
    if (children.length === 0) return null;
    if (!studentId) return children[0] ?? null;
    return children.find((child) => child.student_id === studentId) ?? null;
  }, [children, studentId]);

  const switcherItems = useMemo<ChildSwitcherItem[]>(
    () =>
      children.map((child) => ({
        studentId: child.student_id,
        firstName: child.first_name,
        lastName: child.last_name,
        schoolClass: child.school_class,
        today: selectionToday[child.student_id] ?? UNKNOWN_CHILD_TODAY,
      })),
    [children, selectionToday],
  );

  if (loading) {
    return <ParentPageSkeleton rows={2} />;
  }

  if (failed) return <Alert type="error" message={t("loadError")} />;

  if (children.length === 0) {
    return (
      <ParentPage>
        <ParentPageHeader title={t("sections.today")} />
        <p className="moto-content-surface rounded-2xl border p-5 text-sm leading-6 text-gray-600 shadow-sm backdrop-blur-md">
          {t("noChildren")}
        </p>
      </ParentPage>
    );
  }

  if (!studentId && children.length > 1) {
    return (
      <ParentPage>
        <ParentPageHeader
          kicker={t("overviewKicker")}
          title={navT("childrenMultiple")}
          description={t("overviewDescription")}
        />
        <ChildSwitcher items={switcherItems} />
      </ParentPage>
    );
  }

  if (!active) return <Alert type="error" message={t("notFound")} />;

  const fullName = `${active.first_name} ${active.last_name}`;
  const childContext = [active.school_class, active.school_name]
    .filter(Boolean)
    .join(" · ");

  return (
    <ParentPage>
      <ParentPageHeader
        title={fullName}
        description={childContext}
        media={
          <span
            data-testid="child-profile-icon"
            className="grid size-11 shrink-0 place-items-center rounded-xl bg-gray-50"
          >
            <MotoConceptIcon concept="children" size={28} aria-hidden="true" />
          </span>
        }
        backHref={
          children.length > 1 ? parentPath("/parents/children") : undefined
        }
        backLabel={children.length > 1 ? navT("childrenMultiple") : undefined}
      />
      {/* key: ein Kindwechsel setzt alle Abschnitte zurueck, damit nie Daten
          des vorigen Kindes unter dem neuen Namen stehen. */}
      <ChildSections key={active.student_id} child={active} />
    </ParentPage>
  );
}

function ChildSections({ child }: Readonly<{ child: Child }>) {
  const searchParams = useSearchParams();
  const router = useRouter();
  const t = useTranslations("parentChild");
  const care = useChildCare(child.student_id);
  const [today, setToday] = useState<ChildToday | null>(null);
  const [modal, setModal] = useState<null | "sick" | "pickup">(null);
  const fullName = `${child.first_name} ${child.last_name}`;

  const loadToday = useCallback(() => {
    getChildToday(child.student_id)
      .then(setToday)
      .catch(() => setToday(UNKNOWN_CHILD_TODAY));
  }, [child.student_id]);

  useEffect(() => {
    loadToday();
    const refresh = () => loadToday();
    window.addEventListener("parent-child-status-refresh", refresh);
    window.addEventListener("focus", refresh);
    return () => {
      window.removeEventListener("parent-child-status-refresh", refresh);
      window.removeEventListener("focus", refresh);
    };
  }, [loadToday]);

  // Die Startseite verlinkt ihre Aktionen als ?action=sick / ?action=pickup.
  // Die Kinderseite fuehrt den Dialog, deshalb wird er hier geoeffnet und der
  // Parameter danach aus der Adresse genommen.
  const requestedAction = searchParams.get("action");
  const hasGuardianPickup = care.careExceptions.some(
    (entry) => entry.pickup_source === "guardian",
  );
  // Ein noch offener Abholantrag haelt den Dialog ebenso offen wie eine
  // bestehende eigene Ausnahme: er hat noch keine Ausnahme geschrieben (die
  // entsteht erst mit der Freigabe), und der Dialog ist der einzige Ort, an
  // dem er zurueckgezogen werden kann. Schaltet die OGS das Aendern nach dem
  // Absenden ab, saesse der Antrag sonst fest, obwohl das Backend das
  // Zuruecknehmen weiter erlaubt (es haengt nur an pickup_manage, nicht am
  // Schulschalter) — dieselbe Regel wie beim Wochenplan-Antrag.
  const hasPendingPickupRequest = care.pickupChangeRequests.some(
    (request) => request.status === "pending",
  );
  const canManageExistingPickup =
    (hasGuardianPickup || hasPendingPickupRequest) &&
    care.features.pickup_manage_allowed === true;
  useEffect(() => {
    if (care.loading || !requestedAction) return;
    if (requestedAction === "sick" && care.features.sick_note_enabled) {
      setModal("sick");
    } else if (
      requestedAction === "pickup" &&
      (care.features.pickup_change_enabled || canManageExistingPickup)
    ) {
      setModal("pickup");
    }
    router.replace(parentPath(`/parents/children/${child.student_id}`));
  }, [
    requestedAction,
    care,
    canManageExistingPickup,
    router,
    child.student_id,
  ]);

  // Betreuung beendet (#2487): das Backend schaltet dafür jede Schreib-Fähigkeit
  // ab, die Karten und Reiter bauen ihre Knöpfe aus genau diesen Flags. Ohne
  // diesen einen Satz bliebe ein Profil zurück, dem still alle Aktionen fehlen
  // — und niemand wüsste warum.
  const careEnded = care.features.care_ended === true;

  return (
    <>
      {careEnded && !care.loading ? (
        <div className="rounded-2xl border border-gray-200 bg-gray-50 p-4">
          <p className="text-sm font-semibold text-gray-900">
            {t("careEnded.title")}
          </p>
          <p className="mt-1 text-sm text-gray-600">{t("careEnded.body")}</p>
        </div>
      ) : null}
      <ChildDayCard
        child={{
          studentId: child.student_id,
          firstName: child.first_name,
          lastName: child.last_name,
          schoolClass: child.school_class,
        }}
        today={today ?? UNKNOWN_CHILD_TODAY}
        loading={today === null || care.loading}
        hideIdentity
        features={
          care.loading
            ? undefined
            : {
                ...care.features,
                pickup_change_enabled:
                  care.features.pickup_change_enabled ||
                  canManageExistingPickup,
              }
        }
        onSick={
          care.features.sick_note_enabled ? () => setModal("sick") : undefined
        }
        onPickup={
          care.features.pickup_change_enabled || canManageExistingPickup
            ? () => setModal("pickup")
            : undefined
        }
        statusAside={<PickupSummary care={care} />}
      />
      <ChildAreaTabs
        child={child}
        childName={fullName}
        canInvite={care.features.related_accounts_invite_enabled}
        canRemove={care.features.related_accounts_remove_enabled}
        canAddContact={care.features.guardian_contact_manage_allowed}
        canManagePickup={care.features.pickup_manage_allowed === true}
        careEnded={careEnded}
        sickDays={care.sickDays}
        excusedRequests={care.excusedRequests}
        onWithdrawExcused={care.withdrawExcused}
      />

      {modal === "sick" && (
        <SickNoteModal
          studentId={child.student_id}
          onClose={() => setModal(null)}
          onSubmit={care.reportSick}
          sickRequiresApproval={care.features.sick_requires_approval}
          excusedRequiresApproval={care.features.excused_requires_approval}
        />
      )}
      {modal === "pickup" && (
        <PickupTimeModal
          studentId={child.student_id}
          careExceptions={care.careExceptions}
          pickupChangeRequests={care.pickupChangeRequests}
          careExceptionsLoaded={care.careExceptionsLoaded}
          pickupChangeRequestsLoaded={care.pickupChangeRequestsLoaded}
          pickupChangeEnabled={care.features.pickup_change_enabled}
          childFirstName={child.first_name}
          today={today ?? undefined}
          onClose={() => setModal(null)}
          onSubmit={care.saveCareException}
          onRemove={care.removeCareException}
        />
      )}
    </>
  );
}

function ChildAreaTabs({
  child,
  childName,
  canInvite,
  canRemove,
  canAddContact,
  canManagePickup,
  careEnded,
  sickDays,
  excusedRequests,
  onWithdrawExcused,
}: Readonly<{
  child: Child;
  childName: string;
  canInvite: boolean;
  canRemove: boolean;
  canAddContact: boolean;
  canManagePickup: boolean;
  careEnded: boolean;
  sickDays: import("~/lib/parent-api").StatusDay[];
  excusedRequests: import("~/lib/parent-api").ExcusedRequest[];
  onWithdrawExcused: (requestId: string) => Promise<void>;
}>) {
  const t = useTranslations("parentChild");
  const [activeArea, setActiveArea] = useState<ChildArea>("betreuung");

  return (
    <Tabs
      value={activeArea}
      onValueChange={(value) => setActiveArea(value as ChildArea)}
      className="space-y-5"
    >
      <div className="pb-1">
        <TabsList
          aria-label={t("areas.label")}
          className="flex h-auto w-full gap-1 rounded-xl bg-gray-100 p-1"
        >
          <AreaTab
            value="betreuung"
            label={t("areas.care.title")}
            shortLabel={t("areas.care.shortTitle")}
            icon={CalendarDotsIcon}
          />
          <AreaTab
            value="angaben"
            label={t("areas.data.title")}
            shortLabel={t("areas.data.shortTitle")}
            icon={IdentificationCardIcon}
          />
          <AreaTab
            value="kontakte"
            label={t("areas.contacts.title")}
            shortLabel={t("areas.contacts.shortTitle")}
            icon={UsersThreeIcon}
          />
        </TabsList>
      </div>

      <TabsContent
        forceMount
        value="betreuung"
        className="mt-0 space-y-5 data-[state=inactive]:hidden"
      >
        {(sickDays.length > 0 || excusedRequests.length > 0) && (
          <ParentSection title={t("care.absenceTitle")} concept="sick">
            <SickStatusSummary
              studentId={child.student_id}
              sickDays={sickDays}
              excusedRequests={excusedRequests}
              onWithdraw={onWithdrawExcused}
            />
          </ParentSection>
        )}
        <BookedCareSection
          studentId={child.student_id}
          childFirstName={child.first_name}
          careEnded={careEnded}
          enrolledUntil={child.enrolled_until}
        />
      </TabsContent>

      <TabsContent
        forceMount
        value="angaben"
        className="mt-0 space-y-5 data-[state=inactive]:hidden"
      >
        <ChildMasterDataView
          studentId={child.student_id}
          childName={childName}
          area="details"
        />
      </TabsContent>

      <TabsContent
        forceMount
        value="kontakte"
        className="mt-0 data-[state=inactive]:hidden"
      >
        <GuardiansPanel
          studentId={child.student_id}
          canInvite={canInvite}
          canRemove={canRemove}
          canAddContact={canAddContact}
          canManagePickup={canManagePickup}
        />
      </TabsContent>
    </Tabs>
  );
}

function AreaTab({
  value,
  label,
  shortLabel,
  icon: Icon,
}: Readonly<{
  value: ChildArea;
  label: string;
  shortLabel: string;
  icon: PhosphorIcon;
}>) {
  return (
    <TabsTrigger
      value={value}
      aria-label={label}
      className="min-h-16 min-w-0 flex-1 flex-col gap-1 rounded-lg px-1.5 py-2 text-base text-gray-600 shadow-none hover:bg-white/60 hover:text-gray-900 data-[state=active]:bg-white data-[state=active]:font-semibold data-[state=active]:text-gray-900 data-[state=active]:shadow-sm sm:min-h-12 sm:flex-row sm:gap-2 sm:px-4"
    >
      <Icon
        size={20}
        weight="regular"
        className="shrink-0"
        aria-hidden="true"
      />
      <span>{shortLabel}</span>
    </TabsTrigger>
  );
}

function PickupSummary({
  care,
}: Readonly<{ care: ReturnType<typeof useChildCare> }>) {
  const t = useTranslations("parentChild");
  return (
    <div className="flex min-w-0 items-start gap-3">
      <MotoConceptIcon
        concept="pickup"
        size={28}
        className="mt-0.5"
        aria-hidden="true"
      />
      <div className="min-w-0">
        <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
          {t("today.pickupLabel")}
        </p>
        <p className="mt-1 text-sm leading-6 font-medium text-gray-900">
          <PlannedPickup care={care} />
        </p>
      </div>
    </div>
  );
}

/** Die geplante Abholung im Klartext, nie als erfundener Wert (#1725). */
function PlannedPickup({
  care,
}: Readonly<{ care: ReturnType<typeof useChildCare> }>) {
  const t = useTranslations("parentChild");
  const pickup = care.todayPickup;
  switch (pickup.kind) {
    case "time":
      return (
        <>
          {pickup.changed
            ? t("today.pickupChanged", { time: pickup.time })
            : t("today.pickup", { time: pickup.time })}
        </>
      );
    case "absent":
      return <>{t("today.pickupAbsent")}</>;
    case "none":
      return <>{t("today.pickupNone")}</>;
    default:
      return <>{t("today.pickupUnknown")}</>;
  }
}
