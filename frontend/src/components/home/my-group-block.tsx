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
import {
  isExpectedToday,
  useHomeGroup,
  type HomeMissingArrival,
  type HomePickup,
} from "~/lib/hooks/use-home-group";
import type { OgsLiveWireStudent } from "~/lib/ogs-group-live-api";
import { useTenantAwarePath } from "~/lib/tenant-path";

/** So viele Zeilen passen in eine Karte dieser Höhe ganz hinein. */
const MAX_ROWS = 3;

/**
 * Eine Zeile der Karte: ein Kind, das längst da sein sollte, eine Abholung,
 * die kommt, oder ein Kind, das heute fehlt.
 */
type GroupRow =
  | { readonly kind: "missing"; readonly arrival: HomeMissingArrival }
  | { readonly kind: "pickup"; readonly pickup: HomePickup }
  | { readonly kind: "away"; readonly student: OgsLiveWireStudent };

/**
 * Baustein „Meine Gruppe heute" (#2180): der Stand der eigenen Gruppe auf
 * einen Blick, für die Kraft am Tisch und am Handy.
 *
 * Die Reihenfolge ist die Dringlichkeit: zuerst, wer längst da sein sollte
 * und fehlt (die Frage des Vormittags), dann, was auf einen zukommt — die
 * nächsten Abholungen mit dem, was die Eltern dazu gesagt haben („Oma holt
 * ab", „Musikunterricht danach") —, zuletzt, wer heute fehlt und warum.
 * Anwesende werden nicht aufgezählt: die Abweichung ist die Nachricht, nicht
 * der Normalfall. Gearbeitet wird weiterhin auf „Meine Gruppen", dorthin
 * führt jede Zeile.
 */
export function MyGroupBlock() {
  const tenantPath = useTenantAwarePath();
  const now = useBerlinClock();
  const snapshot = useHomeGroup(true, now);
  const {
    group,
    present,
    total,
    elsewhere,
    away,
    missing,
    pickups,
    isLoading,
    error,
  } = snapshot;
  const missingIds = new Set(missing.map((entry) => entry.student.id));
  const rows: GroupRow[] = [
    ...missing.map((arrival): GroupRow => ({ kind: "missing", arrival })),
    ...pickups.map((pickup): GroupRow => ({ kind: "pickup", pickup })),
    ...away
      .filter((student) => !missingIds.has(student.id))
      .map((student): GroupRow => ({ kind: "away", student })),
  ];
  const { shown, hidden } = useHomeCardRows(rows, MAX_ROWS);
  const href = tenantPath("/ogs-groups");

  const stats = group
    ? [
        group.viaSubstitution ? `${group.name} (Vertretung)` : group.name,
        `${present} von ${total} da`,
        elsewhere > 0 ? `${elsewhere} außerhalb des Gruppenraums` : null,
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
            {rows.length === 0 ? (
              <EmptyState
                className="py-3"
                title="Alle da, keine Abholung mehr heute"
              />
            ) : (
              <>
                <ul className="space-y-2">
                  {shown.map((row) => {
                    switch (row.kind) {
                      case "missing":
                        return (
                          <MissingRow
                            key={`missing-${row.arrival.student.id}`}
                            arrival={row.arrival}
                            href={href}
                          />
                        );
                      case "pickup":
                        return (
                          <PickupRow
                            key={`pickup-${row.pickup.student.id}`}
                            pickup={row.pickup}
                            href={href}
                          />
                        );
                      case "away":
                        return (
                          <AwayRow
                            key={`away-${row.student.id}`}
                            student={row.student}
                            href={href}
                          />
                        );
                    }
                  })}
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

function StudentName({ student }: { readonly student: OgsLiveWireStudent }) {
  return (
    <span className="block truncate text-sm">
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
  );
}

const ROW_CLASS =
  "flex items-center justify-between gap-3 rounded-xl bg-gray-50/50 px-3 py-2 transition-colors hover:bg-gray-100/50";

/**
 * Ein Kind, das längst da sein sollte: die Zeile, die man am Vormittag
 * sucht. Getönt wie eine überfällige Erinnerung, damit sie aus der Liste
 * heraussticht; die Notiz der Eltern („Arzttermin, kommt danach") nimmt ihr
 * oft schon den Schrecken.
 */
function MissingRow({
  arrival,
  href,
}: {
  readonly arrival: HomeMissingArrival;
  readonly href: string;
}) {
  const { student, expected, note } = arrival;
  return (
    <li>
      <Link
        href={href}
        aria-label={`${student.first_name} ${student.last_name}, fehlt seit ${expected}: Gruppe öffnen`}
        className={`${ROW_CLASS} bg-moto-red/5 hover:bg-moto-red/10`}
      >
        <span className="min-w-0">
          <StudentName student={student} />
          {note && (
            <span className="block truncate text-xs text-gray-500">{note}</span>
          )}
        </span>
        <span className="flex shrink-0 flex-col items-end gap-0.5">
          <span className="text-sm font-semibold text-gray-900 tabular-nums">
            {expected}
          </span>
          <span className="text-moto-red-strong text-xs">Fehlt noch</span>
        </span>
      </Link>
    </li>
  );
}

/**
 * Eine Abholung: wer, wann, und was die Eltern dazu gesagt haben. Die Notiz
 * steht unter dem Namen, weil sie das ist, was man am Tisch wissen muss.
 */
function PickupRow({
  pickup,
  href,
}: {
  readonly pickup: HomePickup;
  readonly href: string;
}) {
  const { student, time, note, isException } = pickup;
  return (
    <li>
      <Link
        href={href}
        aria-label={`${student.first_name} ${student.last_name}, Abholung ${time}: Gruppe öffnen`}
        className={ROW_CLASS}
      >
        <span className="min-w-0">
          <StudentName student={student} />
          {note && (
            <span className="block truncate text-xs text-gray-500">{note}</span>
          )}
        </span>
        <span className="flex shrink-0 flex-col items-end gap-0.5">
          <span className="text-sm font-semibold text-gray-900 tabular-nums">
            {time}
          </span>
          {isException ? (
            <StatusBadge tone="orange" label="Ausnahme" />
          ) : (
            <span className="text-xs text-gray-500">Abholung</span>
          )}
        </span>
      </Link>
    </li>
  );
}

function AwayRow({
  student,
  href,
}: {
  readonly student: OgsLiveWireStudent;
  readonly href: string;
}) {
  return (
    <li>
      <Link
        href={href}
        aria-label={`${student.first_name} ${student.last_name}: Gruppe öffnen`}
        className={ROW_CLASS}
      >
        <span className="min-w-0">
          <StudentName student={student} />
          {student.arrival_notes && (
            <span className="block truncate text-xs text-gray-500">
              {student.arrival_notes}
            </span>
          )}
        </span>
        <AwayBadge student={student} />
      </Link>
    </li>
  );
}

/** Warum das Kind fehlt — dieselben Wörter wie auf „Meine Gruppen". */
function AwayBadge({ student }: { readonly student: OgsLiveWireStudent }) {
  if (student.sick) return <StatusBadge tone="red" label="Krank" />;
  // Noch nicht da, aber angekündigt: das ist kein Fehlen, das ist Warten.
  if (isExpectedToday(student)) {
    return <StatusBadge tone="gray" label={`Kommt ${student.arrival_time}`} />;
  }
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
