"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { Alert } from "~/components/ui/alert";
import { Skeleton } from "~/components/ui/skeleton";
import {
  PickupTimeModal,
  SickNoteModal,
  useChildCare,
} from "~/components/parent/child-care";
import { BookedCareSection } from "~/components/parent/child/booked-care-section";
import { ChildDayCard } from "~/components/parent/child/child-day-card";
import {
  ChildSwitcher,
  type ChildSwitcherItem,
} from "~/components/parent/child/child-switcher";
import { ChildMasterDataView } from "~/components/parent/child-master-data";
import GuardiansPanel from "~/components/parent/guardians-panel";
import { ParentSection } from "~/components/parent/shell/parent-section";
import { createLogger } from "~/lib/logger";
import {
  UNKNOWN_CHILD_TODAY,
  getChildToday,
  listMyChildren,
  type Child,
  type ChildToday,
} from "~/lib/parent-api";
import { parentPath } from "~/lib/parent-url";

const logger = createLogger({ component: "ChildPage" });

/**
 * Der Kinderbereich, kind-zentriert nach Entscheidung E9.
 *
 * Bei einem Kind gibt es keine Liste und keinen Umschalter, die Seite zeigt
 * direkt dieses Kind. Bei mehreren steht oben ein Umschalter.
 *
 * Vier Abschnitte in fester Reihenfolge: Heute, Gebuchte Betreuung, Daten,
 * Eltern und Abholberechtigte. Die frueheren Rubriken "Betreuungszeiten"
 * (#2302) und "AGs und Gruppen" (#2303) sind ersatzlos entfallen.
 */
export function ChildPage({
  studentId,
}: Readonly<{
  /** Aus der URL. Fehlt er, wird das erste Kind gezeigt. */
  studentId?: string;
}>) {
  const t = useTranslations("parentChild");
  const router = useRouter();
  const [children, setChildren] = useState<Child[]>([]);
  const [loading, setLoading] = useState(true);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    let active = true;
    listMyChildren()
      .then((list) => {
        if (!active) return;
        setChildren(list);
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
  }, []);

  const active = useMemo(() => {
    if (children.length === 0) return null;
    if (!studentId) return children[0] ?? null;
    return children.find((child) => child.student_id === studentId) ?? null;
  }, [children, studentId]);

  const switcherItems = useMemo<ChildSwitcherItem[]>(
    () =>
      children.map((child) => ({
        studentId: child.student_id,
        name: `${child.first_name} ${child.last_name}`,
      })),
    [children],
  );

  if (loading) {
    return (
      <div className="space-y-5">
        <Skeleton className="h-56 w-full rounded-2xl" />
        <Skeleton className="h-64 w-full rounded-2xl" />
      </div>
    );
  }

  if (failed) return <Alert type="error" message={t("loadError")} />;

  if (children.length === 0) {
    return (
      <p className="rounded-2xl border border-gray-200 bg-white p-5 text-[17px] text-gray-600 shadow-sm">
        {t("noChildren")}
      </p>
    );
  }

  if (!active) return <Alert type="error" message={t("notFound")} />;

  return (
    <div className="space-y-6">
      <h1 className="text-[28px] leading-tight font-bold text-balance text-gray-900">
        {`${active.first_name} ${active.last_name}`}
      </h1>
      <ChildSwitcher
        items={switcherItems}
        activeId={active.student_id}
        onSelect={(next) =>
          router.push(parentPath(`/parents/children/${next}`))
        }
      />
      {/* key: ein Kindwechsel setzt alle Abschnitte zurueck, damit nie Daten
          des vorigen Kindes unter dem neuen Namen stehen. */}
      <ChildSections key={active.student_id} child={active} />
    </div>
  );
}

function ChildSections({ child }: Readonly<{ child: Child }>) {
  const t = useTranslations("parentChild");
  const searchParams = useSearchParams();
  const router = useRouter();
  const care = useChildCare(child.student_id);
  const [today, setToday] = useState<ChildToday>(UNKNOWN_CHILD_TODAY);
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
  useEffect(() => {
    if (care.loading || !requestedAction) return;
    if (requestedAction === "sick" && care.features.sick_note_enabled) {
      setModal("sick");
    } else if (
      requestedAction === "pickup" &&
      (care.features.pickup_change_enabled ||
        care.careExceptions.some((entry) => entry.source === "guardian"))
    ) {
      setModal("pickup");
    }
    router.replace(parentPath(`/parents/children/${child.student_id}`));
  }, [requestedAction, care, router, child.student_id]);

  return (
    <>
      <ParentSection title={t("sections.today")} bare>
        <div className="space-y-3">
          <ChildDayCard
            child={{
              studentId: child.student_id,
              firstName: child.first_name,
              lastName: child.last_name,
              schoolClass: child.school_class,
            }}
            today={today}
            hideIdentity
            features={care.loading ? undefined : care.features}
            onSick={
              care.features.sick_note_enabled
                ? () => setModal("sick")
                : undefined
            }
            onPickup={
              care.features.pickup_change_enabled
                ? () => setModal("pickup")
                : undefined
            }
          />
          <p className="text-[17px] text-gray-700">
            <PlannedPickup care={care} />
          </p>
        </div>
      </ParentSection>

      <BookedCareSection studentId={child.student_id} />

      <ChildMasterDataView studentId={child.student_id} childName={fullName} />

      <GuardiansPanel
        studentId={child.student_id}
        canInvite={care.features.related_accounts_invite_enabled}
        canRemove={care.features.related_accounts_remove_enabled}
      />

      {modal === "sick" && (
        <SickNoteModal
          onClose={() => setModal(null)}
          onSubmit={care.reportSick}
          excusedRequiresApproval={care.features.excused_requires_approval}
        />
      )}
      {modal === "pickup" && (
        <PickupTimeModal
          careExceptions={care.careExceptions}
          careExceptionsLoaded={care.careExceptionsLoaded}
          pickupChangeEnabled={care.features.pickup_change_enabled}
          onClose={() => setModal(null)}
          onSubmit={care.saveCareException}
          onRemove={care.removeCareException}
        />
      )}
    </>
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
