"use client";

/**
 * Sammel-Vertretung (#2284): eine Person für MEHRERE Tage in einem Schritt
 * abwesend melden und optional durch eine Ersatzperson vertreten lassen — der
 * Mehrtages-Bruder des SubstitutionSlideOver.
 *
 * Ablauf: abwesende Person + Zeitraum (von–bis) wählen → das Modal lädt die
 * Termine der Person im Zeitraum (GET /instances?from&to, clientseitig
 * gefiltert) und zeigt sie nach Tagen gruppiert. Einzelne Tage können
 * abgewählt werden; die Auswahl bleibt TAGESWEIT, weil Abwesenheit und
 * Vertretung im Backend tagesweite Semantik haben (#1840) — es gibt hier
 * bewusst keine Termin-Granularität unterhalb eines Tages.
 *
 * Der Save ist EIN atomarer Request über das gemeinsame Vertretungsmodul:
 * entweder landen alle gewählten Tage oder keiner. Ein Fehler nennt den
 * betroffenen Tag, damit er abgewählt werden kann.
 */

import { useEffect, useMemo, useState } from "react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { CustomSelect } from "~/components/ui/custom-select";
import { ISODatePicker } from "~/components/ui/date-picker";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import { useToast } from "~/contexts/ToastContext";
import { formatDate, parseISODate } from "~/lib/date-helpers";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { createLogger } from "~/lib/logger";
import { useSWRAuth, useTenantMutateMatching } from "~/lib/swr";
import { substitutionService } from "~/lib/substitution-api";
import { timetableService } from "~/lib/timetable-api";
import { getGermanWeekdayShort } from "~/lib/timetable-helpers";
import type { EnrichedInstance } from "~/lib/timetable-types";

const logger = createLogger({ component: "BulkSubstitutionModal" });

/** Obergrenze der GEWÄHLTEN Tage; spiegelt MaxBulkSubstitutionDates im Backend. */
const MAX_SELECTED_DATES = 31;

/**
 * Obergrenze des Vorschau-Zeitraums in inklusiven Kalendertagen; spiegelt
 * maxInstanceListRangeDays des Range-Reads. Der Zeitraum darf LÄNGER sein als
 * das Save-Limit: 31 planbare Termintage können sich über mehr als 31
 * Kalendertage verteilen (Wochenenden, Ferien). Begrenzt wird die Auswahl,
 * nicht der Kalender.
 */
const MAX_RANGE_DAYS = 56;

interface StaffOption {
  id: string;
  name: string;
}

interface BulkSubstitutionModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly staffOptions: StaffOption[];
  readonly staffLoadError: boolean;
  /** Cache-Refresh nach einem committeten Save (Woche + Lücken). */
  readonly onSaved: () => Promise<void> | void;
}

/** Tage (inklusive) zwischen zwei ISO-Daten; 0 bei gleichem Tag. */
function rangeDays(fromISO: string, toISO: string): number {
  const from = parseISODate(fromISO);
  const to = parseISODate(toISO);
  return Math.round((to.getTime() - from.getTime()) / 86_400_000);
}

interface DayGroup {
  date: string;
  instances: EnrichedInstance[];
  /** Jede Zuordnung der Person an diesem Tag ist bereits abwesend gemeldet. */
  fullyAbsent: boolean;
}

export function BulkSubstitutionModal({
  isOpen,
  onClose,
  staffOptions,
  staffLoadError,
  onSaved,
}: BulkSubstitutionModalProps) {
  const toast = useToast();
  // Berliner Kalendertag, nicht Browser-Lokalzeit: das Backend validiert gegen
  // timezone.TodayDate() (Berlin); ein Browser in einer anderen Zeitzone wäre
  // um Mitternacht sonst einen Tag daneben. Der Hook rollt um Berliner
  // Mitternacht weiter, das Modal bleibt über die Seite dauerhaft gemountet.
  const today = useBerlinToday();

  const [absentStaffId, setAbsentStaffId] = useState("");
  const [fromISO, setFromISO] = useState(today);
  const [toISO, setToISO] = useState(today);
  const [substituteStaffId, setSubstituteStaffId] = useState("");
  const [reason, setReason] = useState("");
  // Abgewählte Tage statt gewählter: neue Tage aus einer Zeitraumänderung sind
  // damit automatisch ausgewählt, ohne die bestehende Abwahl zu verlieren.
  const [deselected, setDeselected] = useState<ReadonlySet<string>>(new Set());
  const [saving, setSaving] = useState(false);

  // Nach dem Tagesübergang wären Von/Bis-Werte von gestern ungültige
  // Vergangenheit; auf den neuen Berliner "heute"-Anker nachziehen.
  useEffect(() => {
    setFromISO((prev) => (prev !== "" && prev < today ? today : prev));
    setToISO((prev) => (prev !== "" && prev < today ? today : prev));
  }, [today]);

  const rangeValid =
    fromISO !== "" &&
    toISO !== "" &&
    fromISO <= toISO &&
    rangeDays(fromISO, toISO) + 1 <= MAX_RANGE_DAYS;
  const rangeTooLong =
    fromISO !== "" &&
    toISO !== "" &&
    rangeDays(fromISO, toISO) + 1 > MAX_RANGE_DAYS;

  // Termine des Zeitraums laden, sobald Person und gültiger Zeitraum stehen.
  // Der Key trägt beide Daten; die Personenfilterung ist clientseitig, damit
  // ein Personenwechsel keinen neuen Fetch braucht. Das "timetable-"-Präfix
  // hängt die Vorschau in das SSE-Invalidierungsnetz (use-global-sse
  // revalidiert bei staffing_deviation_changed alle timetable-Keys) — sonst
  // bekäme sie fremde Planänderungen nie mit.
  const swrKey =
    isOpen && absentStaffId !== "" && rangeValid
      ? `timetable-sammel-vertretung-${fromISO}-${toISO}`
      : null;
  const {
    data: rangeData,
    isLoading: rangeLoading,
    error: rangeError,
  } = useSWRAuth(swrKey, () => timetableService.getWeek(fromISO, toISO));

  // Nach einem committeten Save tragen alle Vorschau-Caches den Stand VOR dem
  // Save; sie werden geleert statt revalidiert (siehe handleSave).
  const clearPreviewCache = useTenantMutateMatching([
    "timetable-sammel-vertretung-",
  ]);

  const dayGroups: DayGroup[] = useMemo(() => {
    if (!rangeData || absentStaffId === "") return [];
    const byDate = new Map<string, EnrichedInstance[]>();
    for (const inst of rangeData.instances) {
      // Vergangene Tage und abgeschlossene/abgesagte Blöcke sind historischer
      // Bestand — das Backend würde sie ohnehin ablehnen bzw. überspringen.
      if (inst.date < today) continue;
      if (inst.status !== "planned" && inst.status !== "active") continue;
      if (!inst.staff.some((row) => row.staffId === absentStaffId)) continue;
      const list = byDate.get(inst.date) ?? [];
      list.push(inst);
      byDate.set(inst.date, list);
    }
    return [...byDate.entries()]
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([date, instances]) => ({
        date,
        instances: instances.sort((a, b) =>
          a.startTime.localeCompare(b.startTime),
        ),
        fullyAbsent: instances.every((inst) =>
          inst.staff.every(
            (row) => row.staffId !== absentStaffId || row.isAbsent,
          ),
        ),
      }));
  }, [rangeData, absentStaffId, today]);

  const selectedDates = useMemo(
    () => dayGroups.filter((g) => !deselected.has(g.date)).map((g) => g.date),
    [dayGroups, deselected],
  );
  const selectedInstanceCount = useMemo(
    () =>
      dayGroups
        .filter((g) => !deselected.has(g.date))
        .reduce((sum, g) => sum + g.instances.length, 0),
    [dayGroups, deselected],
  );

  const absentOptions = useMemo(
    () => staffOptions.map((s) => ({ value: s.id, label: s.name })),
    [staffOptions],
  );
  const substituteOptions = useMemo(
    () => [
      { value: "", label: "Keine — nur abwesend melden" },
      ...staffOptions
        .filter((s) => s.id !== absentStaffId)
        .map((s) => ({ value: s.id, label: s.name })),
    ],
    [staffOptions, absentStaffId],
  );

  const toggleDay = (date: string) => {
    setDeselected((prev) => {
      const next = new Set(prev);
      if (next.has(date)) next.delete(date);
      else next.add(date);
      return next;
    });
  };

  const resetAndClose = () => {
    setAbsentStaffId("");
    setFromISO(today);
    setToISO(today);
    setSubstituteStaffId("");
    setReason("");
    setDeselected(new Set());
    onClose();
  };

  // Das Backend nimmt höchstens 31 Daten pro Save: gezählt werden die
  // GEWÄHLTEN Tage, nicht die Kalenderspanne.
  const tooManyDates = selectedDates.length > MAX_SELECTED_DATES;

  const canSave =
    !saving &&
    absentStaffId !== "" &&
    rangeValid &&
    // Nur mit erfolgreich geladener Vorschau des AKTUELLEN Zeitraums: die
    // globale keepPreviousData-Konfiguration hält beim Zeitraumwechsel die
    // Tage des alten Zeitraums in data — ein schneller Save würde sonst Tage
    // außerhalb des neu gewählten Zeitraums senden.
    !rangeLoading &&
    !rangeError &&
    selectedDates.length > 0 &&
    !tooManyDates;

  const handleSave = async () => {
    if (!canSave) return;
    setSaving(true);
    try {
      const result = await substitutionService.applyBulkSubstitution({
        absentStaffId,
        substituteStaffId: substituteStaffId || undefined,
        dates: selectedDates,
        reason: reason.trim() || undefined,
      });
      toast.success(
        substituteStaffId
          ? `Vertretung eingetragen: ${result.totalAffected} Termin(e) an ${result.days.length} Tag(en)`
          : `Abwesenheit eingetragen: ${result.totalAffected} Termin(e) an ${result.days.length} Tag(en)`,
      );
      if (result.warningCount > 0) {
        toast.error(
          `${result.warningCount} mögliche Zeitüberschneidung(en) prüfen.`,
        );
      }
      // Alle Vorschau-Caches leeren: sie tragen den Stand VOR dem Save, und
      // mit keepPreviousData würde ein erneutes Öffnen desselben Zeitraums
      // die veraltete Auswahl sofort wieder speicherbar anzeigen. Leeren
      // statt revalidieren, damit der nächste Mount frisch lädt (isLoading)
      // und der Save-Gate bis dahin greift. Ein Fehler hier darf den bereits
      // committeten Save nicht als Fehler melden.
      await clearPreviewCache({ clear: true }).catch((err: unknown) => {
        logger.warn("bulk_preview_cache_clear_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      });
      await onSaved();
      resetAndClose();
    } catch (err) {
      // Der Save ist alles-oder-nichts: bei einem Fehler bleibt das Formular
      // mit allen Eingaben offen, die Meldung nennt den betroffenen Tag.
      logger.error("bulk_substitution_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      toast.error(
        err instanceof Error ? err.message : "Speichern fehlgeschlagen",
      );
    } finally {
      setSaving(false);
    }
  };

  const absentName =
    staffOptions.find((s) => s.id === absentStaffId)?.name ?? "";

  return (
    <FormModal
      isOpen={isOpen}
      onClose={resetAndClose}
      // Der Save ist EIN atomarer Request: ein während des Speicherns
      // geschlossenes Modal sähe abgebrochen aus, während der Server noch
      // committet; alle Schließwege bleiben bis zur Antwort blockiert.
      closeDisabled={saving}
      title="Sammel-Vertretung"
      size="lg"
      footer={
        <div className="flex justify-end gap-2">
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={resetAndClose}
            disabled={saving}
          >
            Abbrechen
          </Button>
          <Button
            type="button"
            variant="primary"
            size="md"
            onClick={handleSave}
            disabled={!canSave}
          >
            {saving
              ? "Speichert…"
              : selectedDates.length > 0
                ? `Für ${selectedDates.length} Tag(e) speichern`
                : "Speichern"}
          </Button>
        </div>
      }
    >
      <div className="space-y-4">
        <p className="text-sm leading-relaxed text-gray-600">
          Trägt eine Abwesenheit — und optional eine Ersatzperson — für alle
          Termine einer Person im gewählten Zeitraum in einem Schritt ein.
          Gespeichert wird alles zusammen oder nichts.
        </p>

        {staffLoadError && (
          <Alert
            type="error"
            message="Personalliste konnte nicht geladen werden. Bitte die Seite neu laden."
          />
        )}

        <div>
          <span className="mb-1 block text-sm font-medium text-gray-700">
            Abwesende Person
          </span>
          <CustomSelect
            value={absentStaffId}
            options={absentOptions}
            ariaLabel="Abwesende Person"
            placeholder="Person wählen…"
            onChange={(value) => {
              setAbsentStaffId(value);
              if (value === substituteStaffId) setSubstituteStaffId("");
              setDeselected(new Set());
            }}
          />
        </div>

        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <ISODatePicker
            label="Von"
            id="bulk-substitution-from"
            value={fromISO}
            min={today}
            required
            hideClearButton
            onChange={(value) => {
              setFromISO(value);
              if (value && toISO && value > toISO) setToISO(value);
              setDeselected(new Set());
            }}
          />
          <ISODatePicker
            label="Bis"
            id="bulk-substitution-to"
            value={toISO}
            min={fromISO || today}
            required
            hideClearButton
            onChange={(value) => {
              setToISO(value);
              setDeselected(new Set());
            }}
          />
        </div>
        {rangeTooLong && (
          <Alert
            type="error"
            message={`Der Zeitraum darf höchstens ${MAX_RANGE_DAYS} Tage umfassen.`}
          />
        )}

        {/* Vorschau der betroffenen Tage — Auswahl ist tagesweit. */}
        {absentStaffId !== "" && rangeValid && (
          <div>
            <span className="mb-1 block text-sm font-medium text-gray-700">
              Betroffene Termine
              {absentName ? ` von ${absentName}` : ""}
            </span>
            <div className="mb-2">
              <Alert
                type="info"
                announce="off"
                message="Alle noch offenen Termine. Die Änderung gilt an jedem ausgewählten Tag. Sie gilt für alle Termine dieser Person."
              />
            </div>
            {rangeError ? (
              <Alert
                type="error"
                message="Termine konnten nicht geladen werden. Bitte erneut versuchen."
              />
            ) : rangeLoading ? (
              <p className="py-2 text-sm text-gray-500">
                Termine werden geladen…
              </p>
            ) : dayGroups.length === 0 ? (
              <p className="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-sm text-gray-500">
                Im gewählten Zeitraum gibt es keine planbaren Termine dieser
                Person.
              </p>
            ) : (
              <ul className="max-h-64 space-y-2 overflow-y-auto pr-1">
                {dayGroups.map((group) => {
                  const checked = !deselected.has(group.date);
                  return (
                    <li key={group.date}>
                      <ChoiceTile
                        htmlFor={`bulk-day-${group.date}`}
                        className="items-start p-3 shadow-sm"
                      >
                        <span className="mt-0.5 inline-flex">
                          <Checkbox
                            id={`bulk-day-${group.date}`}
                            checked={checked}
                            onChange={() => toggleDay(group.date)}
                            aria-label={`${formatDate(group.date)} auswählen`}
                          />
                        </span>
                        <span className="min-w-0 flex-1">
                          <span className="flex items-center gap-2 text-sm font-medium text-gray-900">
                            {getGermanWeekdayShort(parseISODate(group.date))}
                            {", "}
                            {formatDate(group.date)}
                            {group.fullyAbsent && (
                              <span className="rounded-full bg-gray-100 px-2 py-0.5 text-[11px] font-medium text-gray-500">
                                bereits abwesend gemeldet
                              </span>
                            )}
                          </span>
                          <span className="mt-0.5 block text-xs text-gray-500">
                            {group.instances
                              .map((inst) => `${inst.startTime} ${inst.title}`)
                              .join(" · ")}
                          </span>
                        </span>
                      </ChoiceTile>
                    </li>
                  );
                })}
              </ul>
            )}
            {tooManyDates && (
              <div className="mt-2">
                <Alert
                  type="error"
                  message={`Höchstens ${MAX_SELECTED_DATES} Tage pro Speichern. Bitte einzelne Tage abwählen.`}
                />
              </div>
            )}
          </div>
        )}

        <div>
          <span className="mb-1 block text-sm font-medium text-gray-700">
            Ersatzperson
          </span>
          <CustomSelect
            value={substituteStaffId}
            options={substituteOptions}
            ariaLabel="Ersatzperson"
            placeholder="Keine — nur abwesend melden"
            disabled={staffLoadError}
            onChange={setSubstituteStaffId}
          />
          <p className="mt-1 text-[11px] text-gray-400">
            Ohne Ersatzperson werden die Termine nur als abwesend markiert.
          </p>
        </div>

        <Input
          id="bulk-substitution-reason"
          label="Grund (optional)"
          value={reason}
          maxLength={500}
          placeholder="z. B. Krankheit"
          onChange={(e) => setReason(e.target.value)}
        />

        {canSave && (
          <p className="text-xs text-gray-500">
            {selectedInstanceCount} Termin(e) an {selectedDates.length} Tag(en)
            werden {substituteStaffId ? "vertreten" : "als abwesend markiert"}.
          </p>
        )}
      </div>
    </FormModal>
  );
}
