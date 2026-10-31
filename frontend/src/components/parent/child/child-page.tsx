"use client";

import { useEffect, useMemo, useState } from "react";
import {
  CalendarDotsIcon,
  IdentificationCardIcon,
  UsersIcon,
} from "@phosphor-icons/react/ssr";
import { useRouter, useSearchParams } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import { ButtonLink } from "~/components/ui/button";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
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
import { ChildConsentsSection } from "~/components/parent/child/child-consents-section";
import { CoursesSection } from "~/components/parent/child/courses-section";
import {
  ChildSwitcher,
  type ChildSwitcherItem,
} from "~/components/parent/child/child-switcher";
import {
  ChildMasterDataView,
  useChildMasterData,
} from "~/components/parent/child-master-data";
import GuardiansPanel from "~/components/parent/guardians-panel";
import { useApiLoadError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  UNKNOWN_CHILD_TODAY,
  fetchChildParentConferences,
  type Child,
  type ChildToday,
} from "~/lib/parent-api";
import { parentPath } from "~/lib/parent-url";
import { parseISODate } from "~/lib/date-helpers";
import type { ParentConferenceOffer } from "~/lib/parent-conferences";
import { requiresGuardianReason } from "~/lib/parent-request-reason";
import { SharingOptionsProvider } from "~/components/parent/sharing-options-context";
import {
  SharedParentFeaturesProvider,
  ChildRequestsProvider,
} from "~/components/parent/child-requests-context";
import {
  ParentPage,
  ParentPageHeader,
  ParentPageSkeleton,
} from "~/components/parent/parent-page";

import {
  useParentChildren,
  useParentToday,
} from "~/lib/hooks/use-parent-resources";

const EMPTY_CHILDREN: Child[] = [];
const EMPTY_TODAY: Record<string, ChildToday> = {};

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
  const backT = useTranslations("parentChildDetail");
  const childResource = useParentChildren();
  const children = childResource.data ?? EMPTY_CHILDREN;
  const statuses = useParentToday(childResource.data);
  const selectionToday = statuses.data ?? EMPTY_TODAY;
  const loading =
    (!childResource.data && !childResource.error) ||
    (!studentId && children.length > 1 && !statuses.data && !statuses.error);
  const failed = !!childResource.error;
  const { error: childError, mutate: reloadChildren } = childResource;
  const {
    error: loadError,
    show: showLoadError,
    clear: clearLoadError,
  } = useApiLoadError();
  useEffect(() => {
    if (!childError) {
      clearLoadError();
      return;
    }
    void showLoadError(childError, {
      object: t("errorObject"),
      retry: () => void reloadChildren(),
    });
  }, [childError, reloadChildren, clearLoadError, showLoadError, t]);

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

  if (failed) return <LoadErrorAlert error={loadError} />;

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

  // Kein Ladefehler: die Adresse nennt ein Kind, das nicht zu diesem Konto
  // gehört. Der Weg zurück führt zur Übersicht der eigenen Kinder.
  if (!active) {
    return (
      <ParentPage>
        <ParentPageHeader
          title={navT(children.length > 1 ? "childrenMultiple" : "childSingle")}
          backHref={parentPath("/parents/children")}
          backLabel={backT(children.length > 1 ? "back" : "backSingle")}
        />
        <p className="moto-content-surface rounded-2xl border p-5 text-sm leading-6 text-gray-600 shadow-sm backdrop-blur-md">
          {t("notFound")}
        </p>
      </ParentPage>
    );
  }

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
        backLabel={children.length > 1 ? backT("back") : undefined}
      />
      {/* key: ein Kindwechsel setzt alle Abschnitte zurueck, damit nie Daten
          des vorigen Kindes unter dem neuen Namen stehen. */}
      {/* Eine Freigabe-Liste pro Kind: alle Abschnitte teilen sich einen
          einzigen Abruf der möglichen Empfänger. Ebenso teilen sie laufende
          Abrufe der Schulschalter und des Wochenplans. */}
      <SharedParentFeaturesProvider linkedChildren={children}>
        <SharingOptionsProvider>
          <ChildRequestsProvider>
            <ChildSections
              key={active.student_id}
              child={active}
              today={
                selectionToday[active.student_id] ??
                (statuses.error ? UNKNOWN_CHILD_TODAY : null)
              }
            />
          </ChildRequestsProvider>
        </SharingOptionsProvider>
      </SharedParentFeaturesProvider>
    </ParentPage>
  );
}

function ChildSections({
  child,
  today,
}: Readonly<{ child: Child; today: ChildToday | null }>) {
  const searchParams = useSearchParams();
  const router = useRouter();
  const t = useTranslations("parentChild");
  const care = useChildCare(child.student_id);
  const {
    error: careLoadError,
    show: showCareLoadError,
    clear: clearCareLoadError,
  } = useApiLoadError();
  const careFailure = care.loadError;
  const refreshCare = care.refresh;
  // Fehlt eine Liste oder die Freigaben der Schule, fehlen auch Knöpfe. Der
  // Kasten sagt, warum, und lädt auf Wunsch neu.
  useEffect(() => {
    if (!careFailure) {
      clearCareLoadError();
      return;
    }
    void showCareLoadError(careFailure, {
      object: t("errorObjectCare"),
      retry: refreshCare,
    });
  }, [careFailure, clearCareLoadError, refreshCare, showCareLoadError, t]);
  const [modal, setModal] = useState<null | "sick" | "pickup">(null);
  const fullName = `${child.first_name} ${child.last_name}`;

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
  const canReportAbsence =
    care.features.sick_note_enabled ||
    (care.features.excused_note_enabled ?? care.features.sick_note_enabled);
  const canManageExistingPickup =
    (hasGuardianPickup || hasPendingPickupRequest) &&
    care.features.pickup_manage_allowed === true;
  useEffect(() => {
    if (care.loading || !requestedAction) return;
    if (requestedAction === "sick" && canReportAbsence) {
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
    canReportAbsence,
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
      <LoadErrorAlert error={careLoadError} />
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
        onSick={canReportAbsence ? () => setModal("sick") : undefined}
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
        canEditAbsenceRequests={
          care.features.absence_request_edit_enabled === true
        }
        reasonRequired={requiresGuardianReason(care.features)}
        onCareRefresh={care.refresh}
      />

      {modal === "sick" && canReportAbsence && (
        <SickNoteModal
          studentId={child.student_id}
          onClose={() => setModal(null)}
          onSubmit={care.reportSick}
          sickEnabled={care.features.sick_note_enabled}
          excusedEnabled={
            care.features.excused_note_enabled ??
            care.features.sick_note_enabled
          }
          sickRequiresApproval={care.features.sick_requires_approval}
          reasonRequired={requiresGuardianReason(care.features)}
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
          departureModeEnabled={
            care.features.pickup_change_mode_enabled === true
          }
          childFirstName={child.first_name}
          today={today ?? undefined}
          onClose={() => setModal(null)}
          onSubmit={care.saveCareException}
          onRemove={care.removeCareException}
          reasonRequired={requiresGuardianReason(care.features)}
          cutoffTime={care.features.pickup_change_cutoff_time}
          todayClosed={care.features.pickup_change_today_closed}
          onCutoffPassed={care.refresh}
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
  canEditAbsenceRequests,
  reasonRequired,
  onCareRefresh,
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
  canEditAbsenceRequests: boolean;
  reasonRequired: boolean;
  onCareRefresh: () => void;
}>) {
  const t = useTranslations("parentChild");
  const conferenceT = useTranslations("parentConference");
  const locale = useLocale();
  const [activeArea, setActiveArea] = useState<ChildArea>("betreuung");
  // Ein Reiter lädt seine Daten erst beim ersten Öffnen. Danach bleibt er
  // gemountet und nur verborgen, damit Eingaben und geladene Listen beim
  // Hin- und Herwechseln erhalten bleiben.
  const [openedAreas, setOpenedAreas] = useState<ReadonlySet<ChildArea>>(
    () => new Set(["betreuung"]),
  );
  const masterData = useChildMasterData(child.student_id);
  // Elternsprechtag (#4801): nur sichtbar, wenn die Klasse einen hat. Ein
  // Ladefehler bleibt still, denn der Eintrag ist nur ein Weg zur Seite des
  // Sprechtags, die ihren Fehler selbst zeigt.
  const [conferences, setConferences] = useState<ParentConferenceOffer[]>([]);
  useEffect(() => {
    let active = true;
    fetchChildParentConferences(child.student_id)
      .then((loaded) => {
        if (active) setConferences(loaded);
      })
      .catch((err: unknown) => {
        logger.warn("parent_child_conferences_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      });
    return () => {
      active = false;
    };
  }, [child.student_id]);
  const booking = conferences.find((offer) => offer.booking);

  return (
    <>
      {conferences.length > 0 ? (
        <ParentSection
          title={conferenceT("entryTitle")}
          description={
            booking?.booking
              ? conferenceT("entryBooked", {
                  date: parseISODate(booking.date).toLocaleDateString(locale, {
                    weekday: "long",
                    day: "numeric",
                    month: "long",
                  }),
                  time: conferenceT("timeRange", {
                    from: booking.booking.start_time,
                    to: booking.booking.end_time,
                  }),
                })
              : conferenceT("entryDescription", { child: child.first_name })
          }
          concept="calendar"
          actions={
            <ButtonLink
              href={parentPath(
                `/parents/children/${encodeURIComponent(child.student_id)}/elternsprechtag`,
              )}
              variant="outline"
              size="md"
            >
              {booking
                ? conferenceT("entryActionBooked")
                : conferenceT("entryAction")}
            </ButtonLink>
          }
        />
      ) : null}
      <Tabs
        value={activeArea}
        onValueChange={(value) => {
          const area = value as ChildArea;
          setActiveArea(area);
          setOpenedAreas((opened) =>
            opened.has(area) ? opened : new Set(opened).add(area),
          );
        }}
        className="space-y-5"
      >
        <div className="pb-1">
          <TabsList
            aria-label={t("areas.label")}
            data-parent-tour="child-area-tabs"
            className="flex h-auto w-full gap-1 rounded-xl bg-gray-100 p-1"
          >
            <AreaTab
              value="betreuung"
              label={t("areas.care.title")}
              shortLabel={t("areas.care.shortTitle")}
              icon={CalendarDotsIcon}
              tourId="child-care-tab"
            />
            <AreaTab
              value="angaben"
              label={t("areas.data.title")}
              shortLabel={t("areas.data.shortTitle")}
              icon={IdentificationCardIcon}
              tourId="child-data-tab"
            />
            <AreaTab
              value="kontakte"
              label={t("areas.contacts.title")}
              shortLabel={t("areas.contacts.shortTitle")}
              icon={UsersIcon}
              tourId="child-contacts-tab"
            />
          </TabsList>
        </div>

        <TabsContent
          forceMount
          value="betreuung"
          data-parent-tour="child-care"
          className="mt-0 space-y-5 data-[state=inactive]:hidden"
        >
          {(sickDays.length > 0 || excusedRequests.length > 0) && (
            <ParentSection title={t("care.absenceTitle")} concept="sick">
              <SickStatusSummary
                studentId={child.student_id}
                sickDays={sickDays}
                excusedRequests={excusedRequests}
                canEdit={canEditAbsenceRequests}
                reasonRequired={reasonRequired}
                onEdited={onCareRefresh}
              />
            </ParentSection>
          )}
          <BookedCareSection
            studentId={child.student_id}
            childFirstName={child.first_name}
            careEnded={careEnded}
            enrolledUntil={child.enrolled_until}
            reasonRequired={reasonRequired}
          />
          {/* Kurse (#3075): eigener Abschnitt, damit eine Kursanfrage nicht
              zwischen Mittagessen und Betreuungszeiten verschwindet. Schaltet
              die Schule sie aus, rendert der Abschnitt nichts. */}
          <CoursesSection studentId={child.student_id} careEnded={careEnded} />
          <ChildMasterDataView
            studentId={child.student_id}
            childName={childName}
            area="departure"
            masterData={masterData}
          />
        </TabsContent>

        <TabsContent
          forceMount
          value="angaben"
          data-parent-tour="child-data"
          className="mt-0 space-y-5 data-[state=inactive]:hidden"
        >
          {openedAreas.has("angaben") && (
            <>
              <ChildMasterDataView
                studentId={child.student_id}
                childName={childName}
                area="details"
                masterData={masterData}
              />
              <ChildConsentsSection studentId={child.student_id} />
            </>
          )}
        </TabsContent>

        <TabsContent
          forceMount
          value="kontakte"
          data-parent-tour="child-contacts"
          className="mt-0 data-[state=inactive]:hidden"
        >
          {openedAreas.has("kontakte") && (
            <GuardiansPanel
              studentId={child.student_id}
              canInvite={canInvite}
              canRemove={canRemove}
              canAddContact={canAddContact}
              canManagePickup={canManagePickup}
            />
          )}
        </TabsContent>
      </Tabs>
    </>
  );
}

function AreaTab({
  value,
  label,
  shortLabel,
  icon: Icon,
  tourId,
}: Readonly<{
  value: ChildArea;
  label: string;
  shortLabel: string;
  icon: PhosphorIcon;
  tourId?: string;
}>) {
  return (
    <TabsTrigger
      value={value}
      aria-label={label}
      data-parent-tour={tourId}
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
