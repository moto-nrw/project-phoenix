"use client";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { Pencil } from "lucide-react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { EditActions } from "~/components/ui/edit-actions";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { ConfirmationModal } from "~/components/ui/modal";
import { InsetPanel } from "~/components/ui/inset-panel";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { SectionLabel } from "~/components/ui/section-label";
import { StatusBadge } from "~/components/ui/status-badge";
import { Textarea } from "~/components/ui/textarea";
import { ToggleChip } from "~/components/ui/toggle-chip";
import {
  OfferingOccupancyLine,
  OfferingRowShell,
} from "~/components/enrollment/offering-row-shell";
import { formatDateTime } from "~/components/enrollment/enrollment-request-display";
import { CarePlanImpact } from "~/components/students/care-plan-impact";
import { useApiFormError, useApiLoadError } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { type CareOffering, listCareOfferings } from "~/lib/care-offering-api";
import {
  availableCareOfferings,
  careOfferingAvailabilityReason,
} from "~/lib/care-offering-availability";
import {
  type CareOfferingBookingStats,
  fetchCareOfferingBookingStats,
} from "~/lib/care-offering-booking-stats";
import { materializeCareSelection } from "~/lib/care-offering-selection";
import { announceChangeRequestsChanged } from "~/lib/change-requests-refresh";
import { changeHistoryConcept } from "~/lib/concepts/changeHistory";
import {
  type AdminOfferingAdjustment,
  type AdminRequestChild,
  type AdminRequestChildOffering,
  listAdminChildOfferingAdjustments,
  updateAdminChildOfferings,
} from "~/lib/enrollment-admin-api";
import { supportedGradeLevelMax } from "~/lib/grade-level";
import { createLogger } from "~/lib/logger";
import { describeOfferingAdjustmentChanges } from "~/lib/offering-adjustment-history";
import { mapRosterImpact, type RosterImpact } from "~/lib/roster-impact";
import { useCareOfferingsEnabled, useTenant } from "~/lib/tenant-context";
import { weekdayShortLabel, weekdayShortList } from "~/lib/weekdays";

const logger = createLogger({ component: "ChildOfferingAdjustment" });

/**
 * An offering the child's grade level rules out. Parents never see these;
 * admins must, or a configured restriction reads as a missing feature
 * (#2186). A booking the child already holds stays removable — Bestandsschutz
 * means the rule does not revoke it, not that it can never be corrected — but
 * a blocked offering can never be newly added here. The documented workaround
 * is to relax the rule in the Angebots-Katalog first.
 *
 * `automaticDays` are days derived from a trigger offering. The backend
 * re-derives them on every save regardless of the availability rule, so the
 * row must show them as HELD even though there is no manual tick behind them
 * — an empty checkbox next to a booking the backend keeps is a lie (#2186
 * review). They are also not removable here: they disappear only with their
 * trigger.
 */
function BlockedOfferingRow({
  offering,
  gradeLevel,
  gradeLevelMax,
  booked,
  automaticDays,
  stats,
  onRemove,
}: Readonly<{
  offering: CareOffering;
  gradeLevel: number | null | undefined;
  gradeLevelMax: number | null;
  booked: boolean;
  automaticDays: readonly string[];
  stats: CareOfferingBookingStats | undefined;
  onRemove: () => void;
}>) {
  const reason = careOfferingAvailabilityReason(
    offering,
    gradeLevel,
    gradeLevelMax ?? undefined,
  );
  const inputId = `blocked-offering-${offering.id}`;
  const heldAutomatically = automaticDays.length > 0;
  return (
    <OfferingRowShell state="blocked">
      {/* A real <label> is load-bearing, not decoration: the kit Checkbox
          renders an sr-only input behind a pointer-events-none visual, so
          without the label wrapping it a mouse click on the box does nothing
          (#2186 review). */}
      <label
        htmlFor={inputId}
        className={`flex items-start gap-3 ${booked ? "cursor-pointer" : "cursor-not-allowed"}`}
      >
        <Checkbox
          id={inputId}
          className="mt-0.5"
          checked={booked || heldAutomatically}
          disabled={!booked}
          onChange={onRemove}
          aria-label={`${offering.name} bleibt gebucht`}
        />
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-sm font-medium text-gray-500">
              {offering.name}
            </span>
            {booked ? (
              <StatusBadge
                tone="orange"
                label="bereits gebucht"
                title="Diese Buchung bleibt bestehen. Sie kann hier entfernt, aber nicht erneut hinzugefügt werden."
              />
            ) : null}
            {heldAutomatically ? (
              <StatusBadge
                tone="blue"
                label={`automatisch mitgebucht: ${weekdayShortList(automaticDays)}`}
                title="Diese Tage werden aus dem auslösenden Angebot abgeleitet und bleiben bestehen, solange dieses gebucht ist. Sie können hier nicht einzeln entfernt werden."
              />
            ) : null}
          </div>
          {reason ? (
            <span className="mt-1 block text-xs text-gray-500">{reason}</span>
          ) : null}
          <OfferingOccupancyLine stats={stats} />
        </div>
      </label>
    </OfferingRowShell>
  );
}

export function ChildOfferingAdjustment({
  child,
  onSaved,
  phaseId,
  requestId,
}: Readonly<{
  child: AdminRequestChild;
  requestId: string;
  phaseId: string;
  onSaved: () => void;
}>) {
  const careOfferingsEnabled = useCareOfferingsEnabled();
  // The tenant's configured ceiling, so a blocked-offering reason names only
  // grades this school actually has (#2186 review).
  const { tenant } = useTenant();
  const gradeLevelMax = supportedGradeLevelMax(tenant?.gradeLevelMax);
  // Der Bearbeiten-Zustand der Fläche (Bauart 2 Regel 3, #3119): die
  // Auswahl erscheint an Ort und Stelle, kein Modal.
  const [editing, setEditing] = useState(false);
  const [catalog, setCatalog] = useState<CareOffering[]>([]);
  // Offerings the child's grade level rules out. Kept separate from `catalog`
  // so payload building and the auto-add preview keep operating on exactly
  // the selectable set, while the UI can still SHOW the blocked ones with a
  // reason instead of leaving a silent gap in the list (#2186).
  const [blockedCatalog, setBlockedCatalog] = useState<CareOffering[]>([]);
  const [rawCatalog, setRawCatalog] = useState<CareOffering[]>([]);
  const [bookingStats, setBookingStats] = useState<
    Record<string, CareOfferingBookingStats>
  >({});
  const [history, setHistory] = useState<AdminOfferingAdjustment[]>([]);
  const [selected, setSelected] = useState<Set<string>>(() =>
    initialManualOfferingIDs(child.offerings),
  );
  const [days, setDays] = useState<Record<string, string[]>>(() =>
    initialManualOfferingDays(child.offerings),
  );
  const [reason, setReason] = useState("");
  const [loading, setLoading] = useState(false);
  const [catalogLoaded, setCatalogLoaded] = useState(false);
  const [saving, setSaving] = useState(false);
  // Fehler stehen oben im Bearbeiten-Bereich (Bauart 2 Regel 5).
  const errors = useApiFormError();
  const catalogError = useApiLoadError();
  const historyError = useApiLoadError();
  const showHistoryError = historyError.show;
  const clearHistoryError = historyError.clear;
  // „Wiederholen“ ruft die aktuelle Fassung auf, nicht die vom Fehler.
  const latestSaveRef = useRef<() => Promise<void>>(async () => undefined);
  const latestOpenEditorRef = useRef<() => Promise<void>>(
    async () => undefined,
  );
  const latestHistoryRef = useRef<() => Promise<void>>(async () => undefined);
  const [withdrawalConfirmationOpen, setWithdrawalConfirmationOpen] =
    useState(false);
  const [pendingWithdrawalInput, setPendingWithdrawalInput] = useState<{
    reason: string;
    offerings: Array<{ offering_id: string; selected_days?: string[] }>;
  } | null>(null);
  const [withdrawalCreated, setWithdrawalCreated] = useState(false);
  // Was die letzte Korrektur im Betreuungsplan bewirkt hat (#5102).
  const [rosterImpact, setRosterImpact] = useState<RosterImpact | null>(null);
  useEffect(() => {
    if (!careOfferingsEnabled) setEditing(false);
  }, [careOfferingsEnabled]);

  const loadHistory = useCallback(async () => {
    clearHistoryError();
    try {
      const rows = await listAdminChildOfferingAdjustments(requestId, child.id);
      setHistory(rows);
    } catch (err) {
      logger.warn("offering_adjustment_history_load_failed", {
        error: err instanceof Error ? err.message : String(err),
        child_id: child.id,
      });
      // Ohne Hinweis läse sich die fehlende Liste als "nie geändert".
      await showHistoryError(err, {
        object: "die Änderungshistorie",
        retry: () => void latestHistoryRef.current(),
      });
    }
  }, [child.id, requestId, clearHistoryError, showHistoryError]);
  useLayoutEffect(() => {
    latestHistoryRef.current = loadHistory;
  });

  useEffect(() => {
    void loadHistory();
  }, [loadHistory]);

  const resetEditorSelection = (offerings: CareOffering[]) => {
    const available = availableCareOfferings(
      offerings,
      child.target_grade_level,
    );
    const availableIDs = new Set(available.map((offering) => offering.id));
    // Bestandsschutz: a booking the child already holds survives a rule that
    // was tightened after the fact. Dropping it here would silently delete
    // the booking on the next save — the exact failure mode #2186 reports,
    // in the flow meant to fix it. adjustmentPayloadOfferings carries such
    // ids through via the child's existing offerings.
    const nextSelected = new Set(initialManualOfferingIDs(child.offerings));
    for (const offering of available) {
      if (offering.is_active && offering.is_required) {
        nextSelected.add(offering.id);
      }
    }
    setCatalog(available);
    setBlockedCatalog(
      offerings.filter((offering) => !availableIDs.has(offering.id)),
    );
    setSelected(nextSelected);
  };

  const openEditor = async () => {
    if (!careOfferingsEnabled) return;
    setEditing(true);
    setRosterImpact(null);
    errors.clear();
    catalogError.clear();
    setDays(initialManualOfferingDays(child.offerings));
    // Occupancy is advisory: without it a full offering only announces itself
    // as an error after the whole correction is submitted (#2186). Loaded
    // beside the catalog rather than awaited with it — a slow or failing
    // stats call must never keep the editor in its loading state.
    void fetchCareOfferingBookingStats(phaseId)
      .then(setBookingStats)
      .catch((statsErr: unknown) => {
        setBookingStats({});
        logger.warn("offering_adjustment_booking_stats_failed", {
          error:
            statsErr instanceof Error ? statsErr.message : String(statsErr),
        });
      });
    if (catalogLoaded) {
      resetEditorSelection(rawCatalog);
      return;
    }
    setLoading(true);
    try {
      const offerings = await listCareOfferings(phaseId);
      setRawCatalog(offerings);
      resetEditorSelection(offerings);
      setCatalogLoaded(true);
    } catch (err) {
      logger.warn("offering_adjustment_catalog_load_failed", {
        error: err instanceof Error ? err.message : String(err),
        child_id: child.id,
      });
      setCatalog([]);
      setBlockedCatalog([]);
      setRawCatalog([]);
      setCatalogLoaded(false);
      await catalogError.show(err, {
        object: "die Angebotsliste",
        retry: () => void latestOpenEditorRef.current(),
      });
    } finally {
      setLoading(false);
    }
  };
  useLayoutEffect(() => {
    latestOpenEditorRef.current = openEditor;
  });

  const preview = useMemo(() => {
    const { automaticDays } = materializeCareSelection(
      catalog,
      { offeringIds: selected, offeringDays: days },
      child.target_grade_level || undefined,
    );
    return {
      automaticDays: Object.fromEntries(
        catalog.map((offering) => [
          offering.id,
          offering.available_days.filter((day) =>
            automaticDays[offering.id]?.has(day),
          ),
        ]),
      ),
    };
  }, [catalog, child.target_grade_level, days, selected]);

  // Blocked offerings are excluded from `catalog`, so the client-side preview
  // above never covers them. Their derived days therefore have to come from
  // what is on file — otherwise a booking the backend keeps re-deriving reads
  // as unbooked in the editor (#2186 review).
  const automaticDaysOnFile = useMemo(
    () => automaticOfferingDays(child.offerings),
    [child.offerings],
  );

  const handleToggle = (offering: CareOffering) => {
    if (offering.is_required) return;
    const next = new Set(selected);
    if (next.has(offering.id)) {
      next.delete(offering.id);
    } else {
      next.add(offering.id);
      if (
        offering.days_of_week_mode === "parent_choice" &&
        (days[offering.id]?.length ?? 0) === 0
      ) {
        setDays((current) => ({
          ...current,
          [offering.id]: [...offering.available_days],
        }));
      }
    }
    setSelected(next);
  };

  const handleDayToggle = (offering: CareOffering, day: string) => {
    setDays((prev) => {
      const current = new Set(prev[offering.id] ?? []);
      if (current.has(day)) current.delete(day);
      else current.add(day);
      const ordered = offering.available_days.filter((d) => current.has(d));
      return { ...prev, [offering.id]: ordered };
    });
  };

  const removeAllCareDays = () => {
    const careOfferingIDs = new Set(
      rawCatalog
        .filter((offering) => offering.counts_as_care)
        .map((offering) => offering.id),
    );
    setSelected(
      new Set([...selected].filter((id) => !careOfferingIDs.has(id))),
    );
  };

  const hasSelectedCareDays = rawCatalog.some(
    (offering) => offering.counts_as_care && selected.has(offering.id),
  );

  const finishSave = async () => {
    setWithdrawalConfirmationOpen(false);
    setPendingWithdrawalInput(null);
    setEditing(false);
    setReason("");
    await loadHistory();
    onSaved();
  };

  const saveAdjustment = async (
    input: {
      reason: string;
      offerings: Array<{ offering_id: string; selected_days?: string[] }>;
    },
    completeWithdrawalConfirmed: boolean,
  ) => {
    const updated = await updateAdminChildOfferings(requestId, child.id, {
      ...input,
      completeWithdrawalConfirmed,
    });
    setRosterImpact(mapRosterImpact(updated?.roster_impact));
    setWithdrawalCreated(completeWithdrawalConfirmed);
    if (completeWithdrawalConfirmed) {
      announceChangeRequestsChanged();
    }
    await finishSave();
  };

  const handleSave = async () => {
    const trimmedReason = reason.trim();
    if (trimmedReason === "") {
      errors.invalid("Bitte tragen Sie eine Begründung ein.", {
        reason: "Die Begründung fehlt.",
      });
      return;
    }
    if (!catalogLoaded) return;
    setSaving(true);
    errors.clear();
    const input = {
      reason: trimmedReason,
      offerings: adjustmentPayloadOfferings(
        catalog,
        selected,
        days,
        child.offerings,
      ),
    };
    try {
      await saveAdjustment(input, false);
    } catch (err) {
      if (
        err instanceof ApiError &&
        err.code === "enrollment.complete_withdrawal_confirmation_required"
      ) {
        setPendingWithdrawalInput(input);
        setWithdrawalConfirmationOpen(true);
      } else {
        logger.warn("offering_adjustment_save_failed", {
          error: err instanceof Error ? err.message : String(err),
          child_id: child.id,
        });
        await errors.show(err, {
          object: "die Angebotsänderung",
          retry: () => void latestSaveRef.current(),
        });
      }
    } finally {
      setSaving(false);
    }
  };
  useLayoutEffect(() => {
    latestSaveRef.current = handleSave;
  });

  const confirmCompleteWithdrawal = async () => {
    if (!pendingWithdrawalInput) return;
    setSaving(true);
    errors.clear();
    try {
      await saveAdjustment(pendingWithdrawalInput, true);
    } catch (err) {
      setWithdrawalConfirmationOpen(false);
      logger.warn("offering_adjustment_withdrawal_failed", {
        error: err instanceof Error ? err.message : String(err),
        child_id: child.id,
      });
      // Der Dialog ist zu, der Entwurf steht noch im Bearbeiten-Bereich:
      // „Wiederholen“ dort fragt wieder nach.
      await errors.show(err, {
        object: "die Angebotsänderung",
        retry: () => void latestSaveRef.current(),
      });
    } finally {
      setSaving(false);
    }
  };

  const cancelEditing = () => {
    setEditing(false);
    errors.clear();
    catalogError.clear();
  };

  if (!careOfferingsEnabled && history.length === 0 && !historyError.error)
    return null;

  return (
    <InsetPanel framed>
      {withdrawalCreated ? (
        <Alert
          type="warning"
          message="Abmeldung noch abschließen. Die Betreuungstage sind entfernt. Eine berechtigte Person muss noch das Ende der Betreuung eintragen."
        />
      ) : null}
      {careOfferingsEnabled ? (
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <SectionLabel as="h4">Nachbearbeitung</SectionLabel>
            <p className="mt-1 text-sm text-gray-600">
              {child.offerings_unavailable
                ? "Die gebuchten Angebote konnten nicht geladen werden. Bitte die Seite neu laden, eine Korrektur würde sonst auf einem unbekannten Stand aufsetzen."
                : editing
                  ? "Auswahl ändern und mit Begründung speichern. Automatisch verknüpfte Angebote werden beim Speichern neu berechnet."
                  : "Angebote können für dieses bestätigte Kind korrigiert werden."}
            </p>
          </div>
          {/* Correcting on top of an unknown selection would replace the
              family's real bookings with whatever the empty editor holds
              (#2185), so the entry point disappears until the data is back. */}
          {child.offerings_unavailable || editing ? null : (
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={() => void openEditor()}
              className="inline-flex items-center justify-center gap-2"
            >
              <Pencil className="h-4 w-4" aria-hidden />
              Bearbeiten
            </Button>
          )}
        </div>
      ) : null}

      {/* Der Bearbeiten-Zustand an Ort und Stelle (Bauart 2 Regel 3 und 4,
          #3119): Auswahl, Begründung und ein `EditActions` unten. Die
          Abmelde-Rückfrage ist der einzige Dialog und liegt über dieser
          Fläche; Auswahl und Begründung bleiben darunter erhalten. */}
      {careOfferingsEnabled && editing ? (
        <div className="mt-3 space-y-4 border-t border-gray-100 pt-3">
          <FormErrorAlert message={errors.error} />
          <LoadErrorAlert error={catalogError.error} />
          {loading ? (
            <p className="text-sm text-gray-500">Angebote werden geladen…</p>
          ) : (
            <div className="space-y-2">
              {catalog.map((offering) => {
                const checked = selected.has(offering.id);
                const autoDays = preview.automaticDays[offering.id] ?? [];
                const inputId = `offering-adjustment-${child.id}-${offering.id}`;
                return (
                  <OfferingRowShell
                    key={offering.id}
                    state={checked ? "selected" : "available"}
                  >
                    {/* Wie in BlockedOfferingRow: das Kit-Checkbox-Feld ist
                        sr-only, erst das <label> mit htmlFor macht den Klick
                        auf den Kasten wirksam. */}
                    <label
                      htmlFor={inputId}
                      className={`flex items-start gap-3 ${
                        offering.is_required
                          ? "cursor-default"
                          : "cursor-pointer"
                      }`}
                    >
                      <Checkbox
                        id={inputId}
                        className="mt-0.5"
                        checked={checked}
                        disabled={offering.is_required}
                        onChange={() => handleToggle(offering)}
                      />
                      <span className="min-w-0 flex-1">
                        <span className="flex flex-wrap items-center gap-2">
                          <span className="text-sm font-medium text-gray-900">
                            {offering.name}
                          </span>
                          {!offering.is_active ? (
                            <StatusBadge tone="gray" label="Inaktiv" />
                          ) : null}
                          {offering.is_required ? (
                            <StatusBadge tone="blue" label="Pflichtangebot" />
                          ) : null}
                          {autoDays.length > 0 ? (
                            <StatusBadge
                              tone="blue"
                              label={`automatisch mitgebucht: ${weekdayShortList(autoDays)}`}
                            />
                          ) : null}
                        </span>
                        {offering.description ? (
                          <span className="mt-1 block text-xs text-gray-500">
                            {offering.description}
                          </span>
                        ) : null}
                        <OfferingOccupancyLine
                          stats={bookingStats[offering.id]}
                        />
                      </span>
                    </label>

                    {checked &&
                    offering.days_of_week_mode === "parent_choice" ? (
                      <div className="mt-3 flex flex-wrap gap-2 pl-8">
                        {offering.available_days.map((day) => (
                          <ToggleChip
                            key={day}
                            pressed={(days[offering.id] ?? []).includes(day)}
                            onPressedChange={() =>
                              handleDayToggle(offering, day)
                            }
                          >
                            {weekdayShortLabel(day)}
                          </ToggleChip>
                        ))}
                      </div>
                    ) : null}
                  </OfferingRowShell>
                );
              })}
              {blockedCatalog.length > 0 ? (
                <div className="space-y-2 pt-1">
                  <SectionLabel as="p">
                    Für dieses Kind nicht wählbar
                  </SectionLabel>
                  {blockedCatalog.map((offering) => (
                    <BlockedOfferingRow
                      key={offering.id}
                      offering={offering}
                      gradeLevel={child.target_grade_level}
                      gradeLevelMax={gradeLevelMax}
                      booked={selected.has(offering.id)}
                      automaticDays={automaticDaysOnFile[offering.id] ?? []}
                      stats={bookingStats[offering.id]}
                      onRemove={() =>
                        setSelected((prev) => {
                          const next = new Set(prev);
                          next.delete(offering.id);
                          return next;
                        })
                      }
                    />
                  ))}
                </div>
              ) : null}
              {hasSelectedCareDays ? (
                <Button
                  type="button"
                  variant="outline_danger"
                  onClick={removeAllCareDays}
                >
                  Alle Betreuungstage entfernen
                </Button>
              ) : null}
            </div>
          )}

          <Textarea
            name="reason"
            error={errors.fieldError("reason")}
            label="Begründung"
            value={reason}
            onChange={(event) => setReason(event.target.value)}
            rows={3}
            autoComplete="off"
            placeholder="z. B. Randstunde nach Rücksprache mit der Schule ergänzt"
          />
          <EditActions
            onCancel={cancelEditing}
            onSave={() => void handleSave()}
            saving={saving}
            disabled={loading || !catalogLoaded}
          />
        </div>
      ) : null}

      {rosterImpact && !editing ? (
        <div className="mt-3">
          <CarePlanImpact
            impact={rosterImpact}
            studentId={child.created_student_id}
          />
        </div>
      ) : null}

      {historyError.error ? (
        <LoadErrorAlert
          error={historyError.error}
          className={careOfferingsEnabled ? "mt-3" : undefined}
        />
      ) : null}
      {history.length > 0 ? (
        <div
          className={
            careOfferingsEnabled ? "mt-3 border-t border-gray-100 pt-3" : ""
          }
        >
          <SectionLabel as="p" className="flex items-center gap-2">
            <MotoConceptIcon concept={changeHistoryConcept} size={14} />
            Änderungshistorie
          </SectionLabel>
          <ul className="mt-2 space-y-2">
            {history.slice(0, 5).map((entry) => (
              <li key={entry.id} className="text-xs leading-5 text-gray-600">
                <span className="font-medium text-gray-900">
                  {formatDateTime(entry.changed_at)}
                </span>{" "}
                von{" "}
                <span className="font-medium text-gray-900">
                  {entry.actor_name_snapshot ??
                    entry.actor_email_snapshot ??
                    `Account ${entry.actor_account_id}`}
                </span>
                : {entry.reason}
                <ul className="mt-1 space-y-1">
                  {describeOfferingAdjustmentChanges(
                    entry.before,
                    entry.after,
                  ).map((line) => (
                    <li
                      key={`${line.days ?? ""}|${line.text}`}
                      className="flex items-baseline gap-1.5 text-gray-500"
                    >
                      {line.days ? (
                        <span className="shrink-0">
                          <StatusBadge compact tone="gray" label={line.days} />
                        </span>
                      ) : null}
                      <span className="min-w-0">{line.text}</span>
                    </li>
                  ))}
                </ul>
              </li>
            ))}
          </ul>
        </div>
      ) : null}

      <ConfirmationModal
        isOpen={withdrawalConfirmationOpen}
        onClose={() => setWithdrawalConfirmationOpen(false)}
        onConfirm={() => void confirmCompleteWithdrawal()}
        title="Alle Betreuungstage entfernen?"
        confirmText="Änderung speichern"
        cancelText="Zurück"
        isConfirmLoading={saving}
        isDismissDisabled={saving}
      >
        <p>
          Danach ist für {child.first_name} kein Betreuungstag mehr gebucht. Die
          Abmeldung muss anschließend abgeschlossen werden.
        </p>
      </ConfirmationModal>
    </InsetPanel>
  );
}

function initialManualOfferingIDs(
  offerings?: AdminRequestChildOffering[],
): Set<string> {
  const ids = new Set<string>();
  for (const offering of offerings ?? []) {
    const automaticOnly =
      (offering.automatic_selected_days?.length ?? 0) > 0 &&
      (offering.manual_selected_days?.length ?? 0) === 0;
    if (!automaticOnly) ids.add(offering.offering_id);
  }
  return ids;
}

/**
 * The days each booking on file holds automatically, i.e. derived from a
 * trigger offering rather than ticked by anyone. Keyed by offering id, absent
 * when a booking has none.
 */
function automaticOfferingDays(
  offerings?: AdminRequestChildOffering[],
): Record<string, string[]> {
  const out: Record<string, string[]> = {};
  for (const offering of offerings ?? []) {
    const automatic = offering.automatic_selected_days ?? [];
    if (automatic.length > 0) out[offering.offering_id] = [...automatic];
  }
  return out;
}

function initialManualOfferingDays(
  offerings?: AdminRequestChildOffering[],
): Record<string, string[]> {
  const out: Record<string, string[]> = {};
  for (const offering of offerings ?? []) {
    const manual =
      (offering.manual_selected_days?.length ?? 0) > 0
        ? offering.manual_selected_days
        : offering.automatic_selected_days?.length
          ? []
          : offering.selected_days;
    if (manual && manual.length > 0) out[offering.offering_id] = [...manual];
  }
  return out;
}

function adjustmentPayloadOfferings(
  catalog: CareOffering[],
  selected: Set<string>,
  days: Record<string, string[]>,
  existingOfferings?: AdminRequestChildOffering[],
): Array<{ offering_id: string; selected_days?: string[] }> {
  const payload = catalog
    .filter((offering) => selected.has(offering.id))
    .map((offering) => ({
      offering_id: offering.id,
      selected_days:
        offering.days_of_week_mode === "parent_choice"
          ? (days[offering.id] ?? [])
          : undefined,
    }));
  const catalogIDs = new Set(catalog.map((offering) => offering.id));
  for (const offering of existingOfferings ?? []) {
    if (
      catalogIDs.has(offering.offering_id) ||
      !selected.has(offering.offering_id)
    ) {
      continue;
    }
    payload.push({
      offering_id: offering.offering_id,
      selected_days:
        offering.days_of_week_mode === "parent_choice"
          ? (days[offering.offering_id] ?? offering.selected_days ?? [])
          : undefined,
    });
  }
  return payload;
}
