"use client";

import { useRef, useState } from "react";
import { useParams } from "next/navigation";
import { RoleGuard } from "~/components/auth/role-guard";
import { Button, buttonClassName } from "~/components/ui/button";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { EditActions } from "~/components/ui/edit-actions";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { ConfirmationModal } from "~/components/ui/modal";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import { Textarea } from "~/components/ui/textarea";
import {
  WeekProtocolStatusBadge,
  weekProtocolBlockStatus,
  weekProtocolWeekLabel,
} from "~/components/week-protocols/week-protocol-status";
import { useApiErrorDisplay, useApiFormError } from "~/contexts/ToastContext";
import {
  berlinClockFromISO,
  formatBerlinDate,
  formatDate,
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

const WEEKDAYS = [
  "Sonntag",
  "Montag",
  "Dienstag",
  "Mittwoch",
  "Donnerstag",
  "Freitag",
  "Samstag",
];

function dayTitle(date: string): string {
  return `${WEEKDAYS[parseISODate(date).getDay()]}, ${formatDate(date).slice(0, 6)}`;
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
      title="Wochenprotokoll"
      back
      backHref={weekProtocolsPath(protocol?.groupId)}
      backLabel="Zurück zu den Wochenprotokollen"
      loading={isLoading && !data}
      error={loadError}
      stats={
        protocol ? (
          <TenantPageStats
            items={[
              { value: protocol.groupName, label: "" },
              { value: weekProtocolWeekLabel(protocol), label: "" },
            ]}
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
          <SectionCard title="Stand">
            <div className="flex flex-wrap items-center gap-3">
              <WeekProtocolStatusBadge protocol={protocol} />
              <p className="text-sm text-gray-700">{stateMessage(protocol)}</p>
            </div>
          </SectionCard>
          <FormErrorAlert message={formError.error} />
          {protocol.days.map((day) => (
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
            ) : (
              <p className="text-sm whitespace-pre-line text-gray-900">
                {protocol.weekNote || "–"}
              </p>
            )}
          </SectionCard>
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
  return (
    <SectionCard title={dayTitle(day.date)} description={dayDescription(day)}>
      <div className="space-y-4">
        {day.captured && <DayFacts day={day} />}
        {note === null ? (
          <DataGrid columns={1}>
            <DataField label="Ergänzung">
              <span className="whitespace-pre-line">{day.note || "–"}</span>
            </DataField>
          </DataGrid>
        ) : (
          <Textarea
            id={`note-${day.date}`}
            label="Ergänzung"
            // Fünf Felder heißen sichtbar gleich; der Tag macht sie für
            // Screenreader unterscheidbar.
            aria-label={`Ergänzung für ${dayTitle(day.date)}`}
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

function dayDescription(day: WeekProtocolDay): string | undefined {
  if (!day.captured) {
    return "Dieser Tag wurde nicht festgehalten.";
  }
  if (day.kind === "holiday") return "Feiertag";
  if (day.kind === "closing") return "Schließtag";
  return undefined;
}

function DayFacts({ day }: Readonly<{ day: WeekProtocolDay }>) {
  const careDay = day.kind === "care";
  return (
    <>
      <DataGrid columns={4}>
        <DataField label="Anwesend">{day.present}</DataField>
        {careDay && (
          <>
            <DataField label="Krank">{day.sick}</DataField>
            <DataField label="Entschuldigt">{day.excused}</DataField>
            {day.classTrip > 0 && (
              <DataField label="Klassenfahrt">{day.classTrip}</DataField>
            )}
            <DataField label="Fehlt ohne Abmeldung">{day.absent}</DataField>
          </>
        )}
      </DataGrid>
      <DataGrid columns={1}>
        {careDay && (
          <DataField label="Fehlende Kinder">
            {day.missing.length === 0 ? (
              "–"
            ) : (
              <ul className="space-y-1">
                {day.missing.map((child) => (
                  <li key={child.studentId}>
                    {child.name}: {child.label}
                  </li>
                ))}
              </ul>
            )}
          </DataField>
        )}
        <DataField label="Aktivitäten und Blöcke">
          {day.blocks.length === 0 ? (
            "–"
          ) : (
            <ul className="space-y-1">
              {day.blocks.map((block) => (
                <li key={`${block.startTime}-${block.title}`}>
                  {block.startTime} bis {block.endTime} {block.title}:{" "}
                  {weekProtocolBlockStatus(block)}
                </li>
              ))}
            </ul>
          )}
        </DataField>
        {day.notices.length > 0 && (
          <DataField label="Tagesinformationen">
            <ul className="space-y-1">
              {day.notices.map((notice) => (
                <li key={notice.title}>
                  <span className="font-medium">{notice.title}</span>
                  {notice.body ? `: ${notice.body}` : ""}
                </li>
              ))}
            </ul>
          </DataField>
        )}
      </DataGrid>
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
