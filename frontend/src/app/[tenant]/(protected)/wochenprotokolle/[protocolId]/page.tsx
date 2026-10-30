"use client";

import { useRef, useState, type ReactNode } from "react";
import { useParams } from "next/navigation";
import { RoleGuard } from "~/components/auth/role-guard";
import { Button, buttonClassName } from "~/components/ui/button";
import { EditActions } from "~/components/ui/edit-actions";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { ConfirmationModal } from "~/components/ui/modal";
import { SectionCard } from "~/components/ui/section-card";
import { StatCard } from "~/components/ui/stat-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { StatusColorBadge } from "~/components/ui/status-color-badge";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import { Textarea } from "~/components/ui/textarea";
import {
  WeekProtocolBlockBadge,
  WeekProtocolStatusBadge,
  weekProtocolWeekLabel,
} from "~/components/week-protocols/week-protocol-status";
import { useApiErrorDisplay, useApiFormError } from "~/contexts/ToastContext";
import { DAY_LOG_STATUS_COLORS } from "~/lib/day-log-api";
import {
  berlinClockFromISO,
  formatBerlinDate,
  formatDate,
  formatStatusDate,
  parseISODate,
} from "~/lib/date-helpers";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { createLogger } from "~/lib/logger";
import { useSWRAuth } from "~/lib/swr";
import {
  weekProtocolsPath,
  weekProtocolsService,
  type WeekProtocol,
  type WeekProtocolDay,
  type WeekProtocolNotes,
} from "~/lib/week-protocols-api";

// Wochenprotokoll (#4769): eine Woche einer Gruppe. Jeder Tag zeigt, was der
// Abend festgehalten hat. Im Entwurf ergänzt die Gruppenleitung die Tage und
// die Woche und schließt das Protokoll ab; danach ändert es sich nicht mehr
// und steht als PDF bereit.
//
// Bauart 2 (Objekt): Ergänzen ist der Bearbeiten-Zustand am Objekt, mit
// einem Speichern unten.

const logger = createLogger({ component: "WeekProtocolPage" });

const SHORT_WEEKDAYS = ["So", "Mo", "Di", "Mi", "Do", "Fr", "Sa"];

/** „Fr 30.10.“ für die Tageskacheln der Wochenübersicht. */
function shortDay(date: string): string {
  return `${SHORT_WEEKDAYS[parseISODate(date).getDay()]} ${formatDate(date).slice(0, 6)}`;
}

function stamp(iso: string): string {
  return `${formatBerlinDate(iso)}, ${berlinClockFromISO(iso)} Uhr`;
}

function notesOf(protocol: WeekProtocol): WeekProtocolNotes {
  return {
    weekNote: protocol.weekNote,
    days: Object.fromEntries(protocol.days.map((day) => [day.date, day.note])),
  };
}

/** Wer jetzt etwas tun kann, und bis wann. */
function stateMessage(protocol: WeekProtocol): string {
  switch (protocol.status) {
    case "collecting":
      return `Die Woche läuft noch. moto hält jeden Abend den Tag fest. Ab Freitagabend, ${formatDate(protocol.draftFrom)}, können Sie das Protokoll ergänzen.`;
    case "draft":
      return `Ergänzen Sie die Tage und die Woche und schließen Sie das Protokoll ab. Sonst schließt moto es am ${formatDate(protocol.autoCloseOn)} mit den bisherigen Ergänzungen ab.`;
    default:
      return protocol.closedAutomatically
        ? `moto hat das Protokoll am ${stamp(protocol.closedAt ?? "")} abgeschlossen. Es lässt sich nicht mehr ändern.`
        : `${protocol.closedByName} hat das Protokoll am ${stamp(protocol.closedAt ?? "")} abgeschlossen. Es lässt sich nicht mehr ändern.`;
  }
}

/** Was die Kachel eines Tages sagt: wie viele da waren, oder warum nichts. */
function dayTileValue(day: WeekProtocolDay): string {
  if (!day.captured) return "Nicht festgehalten";
  if (day.kind === "holiday") return "Feiertag";
  if (day.kind === "closing") return "Schließtag";
  const total = day.present + day.missing.length;
  if (total === 0) return "Keine Kinder";
  return `${day.present} von ${total} da`;
}

function WeekProtocolContent() {
  const params = useParams();
  const protocolId = params.protocolId as string;
  const { data, error, isLoading, mutate } = useSWRAuth<WeekProtocol>(
    `week-protocol-${protocolId}`,
    () => weekProtocolsService.get(protocolId),
  );
  const loadError = useSwrLoadError(error, "das Wochenprotokoll", mutate);
  const [notes, setNotes] = useState<WeekProtocolNotes | null>(null);
  const [saving, setSaving] = useState(false);
  const [confirmClose, setConfirmClose] = useState(false);
  const [closing, setClosing] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formError = useApiFormError(formRef);
  const actionError = useApiErrorDisplay();

  const protocol = data ?? null;
  const editing = notes !== null && protocol?.status === "draft";

  const save = async () => {
    if (!notes) return;
    setSaving(true);
    formError.clear();
    try {
      const saved = await weekProtocolsService.saveNotes(protocolId, notes);
      await mutate(saved, { revalidate: false });
      setNotes(null);
    } catch (err) {
      logger.error("week_protocol_notes_failed", {
        protocol_id: protocolId,
        error: err instanceof Error ? err.message : String(err),
      });
      await formError.show(err, { object: "die Ergänzungen" });
    } finally {
      setSaving(false);
    }
  };

  const close = async () => {
    setClosing(true);
    try {
      const closed = await weekProtocolsService.close(protocolId);
      await mutate(closed, { revalidate: false });
      setConfirmClose(false);
    } catch (err) {
      logger.error("week_protocol_close_failed", {
        protocol_id: protocolId,
        error: err instanceof Error ? err.message : String(err),
      });
      setConfirmClose(false);
      await actionError.show(err, { object: "das Wochenprotokoll" });
    } finally {
      setClosing(false);
    }
  };

  return (
    <TenantPage
      title={protocol?.groupName ?? "Wochenprotokoll"}
      back
      backHref={weekProtocolsPath(protocol?.groupId)}
      backLabel="Zurück zu den Wochenprotokollen"
      loading={isLoading && !data}
      error={loadError}
      stats={
        protocol ? (
          <TenantPageStats
            items={[{ value: weekProtocolWeekLabel(protocol), label: "" }]}
          />
        ) : undefined
      }
      actions={
        protocol ? (
          <HeadActions
            protocol={protocol}
            editing={editing}
            onEdit={() => setNotes(notesOf(protocol))}
            onClose={() => setConfirmClose(true)}
          />
        ) : undefined
      }
      overlays={
        <ConfirmationModal
          isOpen={confirmClose}
          onClose={() => setConfirmClose(false)}
          onConfirm={() => void close()}
          title="Wochenprotokoll abschließen?"
          confirmText="Abschließen"
          isConfirmLoading={closing}
          loadingText="Wird abgeschlossen…"
        >
          <p className="text-sm text-gray-700">
            Danach lässt sich das Protokoll nicht mehr ändern. Es steht dann als
            PDF bereit.
          </p>
        </ConfirmationModal>
      }
    >
      {protocol && (
        <form
          ref={formRef}
          className="space-y-6"
          onSubmit={(event) => {
            event.preventDefault();
            void save();
          }}
        >
          <SectionCard
            title="Diese Woche"
            titleBadge={<WeekProtocolStatusBadge protocol={protocol} />}
            description={stateMessage(protocol)}
          >
            <ul className="grid grid-cols-2 gap-2 sm:grid-cols-5">
              {protocol.days.map((day) => (
                <li key={day.date}>
                  <StatCard
                    variant="tile"
                    label={shortDay(day.date)}
                    value={dayTileValue(day)}
                    tone={day.captured ? undefined : "gray"}
                  />
                </li>
              ))}
            </ul>
          </SectionCard>
          <FormErrorAlert message={formError.error} />
          <SectionCard
            title="Ergänzung für die Woche"
            description="Was in dieser Woche für die Gruppe wichtig war."
          >
            {editing ? (
              <Textarea
                id="week-note"
                aria-label="Ergänzung für die Woche"
                rows={4}
                maxLength={4000}
                value={notes?.weekNote ?? ""}
                disabled={saving}
                onChange={(event) =>
                  setNotes((current) =>
                    current
                      ? { ...current, weekNote: event.target.value }
                      : current,
                  )
                }
              />
            ) : protocol.weekNote ? (
              <p className="text-sm whitespace-pre-line text-gray-900">
                {protocol.weekNote}
              </p>
            ) : (
              <p className="text-sm text-gray-500">Noch keine Ergänzung.</p>
            )}
          </SectionCard>
          {protocol.days
            // Ein Tag ohne Abend und ohne Ergänzung steht nur in der
            // Übersicht; beim Ergänzen bekommt jeder Tag sein Feld.
            .filter((day) => editing || day.captured || day.note)
            .map((day) => (
              <DayCard
                key={day.date}
                day={day}
                note={editing ? (notes?.days[day.date] ?? "") : null}
                onNoteChange={(value) =>
                  setNotes((current) =>
                    current
                      ? {
                          ...current,
                          days: { ...current.days, [day.date]: value },
                        }
                      : current,
                  )
                }
                disabled={saving}
              />
            ))}
          {editing && (
            <EditActions
              onCancel={() => {
                formError.clear();
                setNotes(null);
              }}
              saving={saving}
              saveType="submit"
            />
          )}
        </form>
      )}
    </TenantPage>
  );
}

function HeadActions({
  protocol,
  editing,
  onEdit,
  onClose,
}: Readonly<{
  protocol: WeekProtocol;
  editing: boolean;
  onEdit: () => void;
  onClose: () => void;
}>) {
  if (protocol.status === "closed") {
    // A download, not a page: a plain anchor in the kit's button look.
    return (
      <a
        href={weekProtocolsService.pdfUrl(protocol.id)}
        className={buttonClassName({
          variant: "outline",
          size: "md",
          className: "",
        })}
      >
        PDF herunterladen
      </a>
    );
  }
  if (protocol.status !== "draft" || editing) return null;
  return (
    <div className="flex flex-wrap gap-2">
      <Button type="button" variant="outline" size="md" onClick={onEdit}>
        Ergänzen
      </Button>
      <Button type="button" variant="primary" size="md" onClick={onClose}>
        Abschließen
      </Button>
    </div>
  );
}

function DayCard({
  day,
  note,
  onNoteChange,
  disabled,
}: Readonly<{
  day: WeekProtocolDay;
  /** The note being edited, or null when the day is only shown. */
  note: string | null;
  onNoteChange: (value: string) => void;
  disabled: boolean;
}>) {
  const title = formatStatusDate(day.date);
  return (
    <SectionCard
      id={`tag-${day.date}`}
      title={title}
      titleBadge={dayBadge(day)}
      description={daySummary(day)}
    >
      <div className="space-y-6">
        {day.captured && <DayFacts day={day} />}
        {note === null ? (
          day.note ? (
            <DaySection title="Ergänzung">
              <p className="rounded-xl bg-gray-50 px-3 py-2 text-sm whitespace-pre-line text-gray-900">
                {day.note}
              </p>
            </DaySection>
          ) : null
        ) : (
          <Textarea
            id={`note-${day.date}`}
            label="Ergänzung"
            // Fünf Felder heißen sichtbar gleich; der Tag macht sie für
            // Screenreader unterscheidbar.
            aria-label={`Ergänzung für ${title}`}
            rows={3}
            maxLength={2000}
            value={note}
            disabled={disabled}
            onChange={(event) => onNoteChange(event.target.value)}
          />
        )}
      </div>
    </SectionCard>
  );
}

function dayBadge(day: WeekProtocolDay) {
  if (!day.captured)
    return <StatusBadge label="Nicht festgehalten" tone="gray" />;
  if (day.kind === "holiday")
    return <StatusBadge label="Feiertag" tone="blue" />;
  if (day.kind === "closing")
    return <StatusBadge label="Schließtag" tone="blue" />;
  return undefined;
}

/** „4 von 13 Kindern da“, nur an Betreuungstagen. */
function daySummary(day: WeekProtocolDay): string | undefined {
  if (!day.captured) {
    return "An diesem Abend hat moto nichts festgehalten.";
  }
  if (day.kind !== "care") return undefined;
  const total = day.present + day.missing.length;
  if (total === 0) return "Die Gruppe hatte an diesem Tag keine Kinder.";
  return `${day.present} von ${total} ${total === 1 ? "Kind" : "Kindern"} da`;
}

function DaySection({
  title,
  count,
  children,
}: Readonly<{ title: string; count?: number; children: ReactNode }>) {
  return (
    <section>
      <h3 className="mb-1 text-xs font-semibold tracking-wide text-gray-500 uppercase">
        {title}
        {count !== undefined && ` (${count})`}
      </h3>
      {children}
    </section>
  );
}

// Die Wörter der Zähler der Tagesauswertung: Kachel und Kinderzeile sagen
// dasselbe. Das gespeicherte Label bleibt Rückfall für einen fremden Grund.
const MISSING_REASONS: Record<string, { label: string; color: string }> = {
  sick: { label: "Krank", color: DAY_LOG_STATUS_COLORS.sick },
  excused: { label: "Entschuldigt", color: DAY_LOG_STATUS_COLORS.excused },
  class_trip: {
    label: "Klassenfahrt",
    color: DAY_LOG_STATUS_COLORS.class_trip,
  },
  absent: { label: "Unentschuldigt", color: DAY_LOG_STATUS_COLORS.absent },
};

function DayFacts({ day }: Readonly<{ day: WeekProtocolDay }>) {
  // Ohne Kinder sagt der Satz unter dem Titel alles; vier Nullen und eine
  // leere Liste wiederholten ihn nur.
  const careDay = day.kind === "care" && day.present + day.missing.length > 0;
  return (
    <>
      {careDay && (
        <div
          className={`grid grid-cols-2 gap-2 ${day.classTrip > 0 ? "sm:grid-cols-5" : "sm:grid-cols-4"}`}
        >
          <StatCard variant="tile" label="Anwesend" value={day.present} />
          <StatCard variant="tile" label="Krank" value={day.sick} />
          <StatCard variant="tile" label="Entschuldigt" value={day.excused} />
          {day.classTrip > 0 && (
            <StatCard
              variant="tile"
              label="Klassenfahrt"
              value={day.classTrip}
            />
          )}
          <StatCard
            variant="tile"
            label="Unentschuldigt"
            value={day.absent}
            tone={day.absent > 0 ? "red" : undefined}
          />
        </div>
      )}
      <div className="grid gap-6 lg:grid-cols-2">
        {careDay && (
          <DaySection title="Fehlende Kinder" count={day.missing.length}>
            {day.missing.length === 0 ? (
              <p className="py-2 text-sm text-gray-500">
                Alle Kinder waren da.
              </p>
            ) : (
              <ul className="divide-y divide-gray-100">
                {day.missing.map((child) => (
                  <li
                    key={child.studentId}
                    className="flex items-center justify-between gap-3 py-2"
                  >
                    <span className="min-w-0 truncate text-sm text-gray-900">
                      {child.name}
                    </span>
                    <StatusColorBadge
                      label={
                        MISSING_REASONS[child.reason]?.label ?? child.label
                      }
                      color={
                        MISSING_REASONS[child.reason]?.color ??
                        DAY_LOG_STATUS_COLORS.absent
                      }
                    />
                  </li>
                ))}
              </ul>
            )}
          </DaySection>
        )}
        <DaySection title="Blöcke" count={day.blocks.length}>
          {day.blocks.length === 0 ? (
            <p className="py-2 text-sm text-gray-500">
              Keine Blöcke an diesem Tag.
            </p>
          ) : (
            <ul className="divide-y divide-gray-100">
              {day.blocks.map((block) => (
                <li
                  key={`${block.startTime}-${block.title}`}
                  className="flex items-start justify-between gap-3 py-2"
                >
                  <div className="min-w-0">
                    <p className="truncate text-sm text-gray-900">
                      {block.title}
                    </p>
                    <p className="text-xs text-gray-500 tabular-nums">
                      {block.startTime} bis {block.endTime} Uhr
                      {block.status === "cancelled" && block.cancelReason
                        ? ` · ${block.cancelReason}`
                        : ""}
                    </p>
                  </div>
                  <WeekProtocolBlockBadge block={block} />
                </li>
              ))}
            </ul>
          )}
        </DaySection>
      </div>
      {day.notices.length > 0 && (
        <DaySection title="Tagesinformationen" count={day.notices.length}>
          <ul className="divide-y divide-gray-100">
            {day.notices.map((notice) => (
              <li key={notice.title} className="py-2">
                <p className="text-sm font-medium text-gray-900">
                  {notice.title}
                </p>
                {notice.body && (
                  <p className="mt-0.5 text-sm text-gray-600">{notice.body}</p>
                )}
              </li>
            ))}
          </ul>
        </DaySection>
      )}
    </>
  );
}

export default function WeekProtocolPage() {
  return (
    <RoleGuard
      variant="staffOrAdmin"
      permission={["groups:update", "groups:manage"]}
      fallback={<TenantPage title="Wochenprotokoll" loading />}
    >
      <WeekProtocolContent />
    </RoleGuard>
  );
}
