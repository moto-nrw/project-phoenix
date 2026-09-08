"use client";

import { ChevronRight } from "lucide-react";

import { Alert } from "~/components/ui/alert";
import { EmptyState } from "~/components/ui/empty-state";
import Link from "~/components/ui/navigation-link";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import {
  HOME_CARD_BODY,
  HomeCardIcon,
} from "~/components/home/home-block-content";
import {
  HomeMoreRow,
  useBerlinClock,
  useHomeCardRows,
} from "~/components/home/home-card-rows";
import { useHomeGroup } from "~/lib/hooks/use-home-group";
import type { OgsLiveWireStudent } from "~/lib/ogs-group-live-api";
import { useTenantAwarePath } from "~/lib/tenant-path";

/** So viele fehlende Kinder passen namentlich in eine Karte dieser Höhe. */
const MAX_ROWS = 3;

/**
 * Baustein „Meine Gruppe heute" (#2180): der Stand der eigenen Gruppe auf
 * einen Blick — wie viele da sind, wann die nächste Abholung ist, und wer
 * heute fehlt. Anwesende werden bewusst nicht aufgezählt: die Abweichung ist
 * die Nachricht, nicht der Normalfall. Gearbeitet wird weiterhin auf „Meine
 * Gruppen", dorthin führt jede Zeile.
 */
export function MyGroupBlock() {
  const tenantPath = useTenantAwarePath();
  const now = useBerlinClock();
  const snapshot = useHomeGroup(true, now);
  const { group, present, total, away, nextPickup, isLoading, error } =
    snapshot;
  const { shown, hidden } = useHomeCardRows(away, MAX_ROWS);
  const href = tenantPath("/ogs-groups");

  const stats = group
    ? [
        group.viaSubstitution ? `${group.name} (Vertretung)` : group.name,
        `${present} von ${total} da`,
        nextPickup ? `nächste Abholung ${nextPickup}` : null,
      ]
        .filter(Boolean)
        .join(" · ")
    : null;

  return (
    <SectionCard
      title="Meine Gruppe heute"
      leading={<HomeCardIcon concept="groups" />}
      className="flex h-full flex-col"
      bodyClassName={HOME_CARD_BODY}
      actions={
        <Link
          href={href}
          aria-label="Meine Gruppe heute: zur Gruppe"
          className="flex items-center gap-1 text-sm font-medium text-gray-600 transition-colors hover:text-gray-900"
        >
          Zur Gruppe
          <ChevronRight className="h-4 w-4" aria-hidden="true" />
        </Link>
      }
    >
      {(() => {
        if (error) {
          return (
            <Alert
              type="error"
              message="Ihre Gruppe konnte nicht geladen werden. Bitte die Seite neu laden."
            />
          );
        }
        if (isLoading && !group) {
          return (
            <div className="space-y-2" aria-hidden="true">
              <div className="h-4 w-3/5 animate-pulse rounded bg-gray-200"></div>
              {[1, 2].map((i) => (
                <div
                  key={i}
                  className="flex items-center justify-between gap-3 rounded-xl bg-gray-50/50 px-3 py-2"
                >
                  <div className="h-4 w-24 animate-pulse rounded bg-gray-200"></div>
                  <div className="h-4 w-16 animate-pulse rounded bg-gray-200"></div>
                </div>
              ))}
            </div>
          );
        }
        if (!group) {
          return (
            <EmptyState
              className="py-4"
              title="Ihnen ist heute keine Gruppe zugeteilt"
              description="Sobald Sie eine Gruppe betreuen, steht ihr Stand hier."
            />
          );
        }
        return (
          <>
            {stats && (
              <p className="mb-2 text-sm text-gray-600 sm:truncate">{stats}</p>
            )}
            {away.length === 0 ? (
              <EmptyState
                className="py-3"
                title="Alle Kinder Ihrer Gruppe sind da"
              />
            ) : (
              <>
                <ul className="space-y-2">
                  {shown.map((student) => (
                    <li key={student.id}>
                      <Link
                        href={href}
                        aria-label={`${student.first_name} ${student.last_name}: Gruppe öffnen`}
                        className="flex items-center justify-between gap-3 rounded-xl bg-gray-50/50 px-3 py-2 transition-colors hover:bg-gray-100/50"
                      >
                        <span className="min-w-0 truncate text-sm">
                          <span className="font-medium text-gray-900">
                            {student.first_name} {student.last_name}
                          </span>
                          {student.school_class && (
                            <span className="text-gray-500">
                              {" · "}
                              {student.school_class}
                            </span>
                          )}
                        </span>
                        <AwayBadge student={student} />
                      </Link>
                    </li>
                  ))}
                </ul>
                <HomeMoreRow hidden={hidden} href={href} label="Kinder" />
              </>
            )}
          </>
        );
      })()}
    </SectionCard>
  );
}

/** Warum das Kind fehlt — dieselben Wörter wie auf „Meine Gruppen". */
function AwayBadge({ student }: { readonly student: OgsLiveWireStudent }) {
  if (student.sick) return <StatusBadge tone="red" label="Krank" />;
  if (student.class_trip)
    return <StatusBadge tone="blue" label="Klassenfahrt" />;
  if (student.excused) return <StatusBadge tone="gray" label="Entschuldigt" />;
  if (student.day_planning_status === "not_coming_today") {
    return (
      <StatusBadge
        tone="gray"
        label={student.day_planning_label ?? "Kommt heute nicht"}
      />
    );
  }
  return <StatusBadge tone="gray" label="Zuhause" />;
}
