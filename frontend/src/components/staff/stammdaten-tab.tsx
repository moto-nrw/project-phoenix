"use client";

import { useRef, useState } from "react";
import { Eye, EyeOff, Lock } from "lucide-react";
import { Alert } from "~/components/ui/alert";
import { Button, ButtonLink } from "~/components/ui/button";
import { EditActions } from "~/components/ui/edit-actions";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import {
  DataField,
  DataFieldSkeleton,
  DataGrid,
} from "~/components/ui/detail-modal-components";
import { StatusBadge } from "~/components/ui/status-badge";
import { SectionCard } from "~/components/ui/section-card";
import { useApiErrorDisplay, useApiFormError } from "~/contexts/ToastContext";
import { formatDate, todayISO } from "~/lib/date-helpers";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { createLogger } from "~/lib/logger";
import { employmentTypeLabels } from "~/lib/staff-helpers";
import { useSWRAuth } from "~/lib/swr";
import { useTenantAwarePath } from "~/lib/tenant-path";
import {
  staffPayrollNumberService,
  staffStammdatenService,
  type StaffFinancialPlain,
  type StaffStammdaten,
} from "~/lib/staff-api";
import {
  ArbeitsvertragFields,
  FinancialFields,
  KontaktFields,
  PersonFields,
  PersonnelNumberFields,
  QualifikationenFields,
  buildDraft,
  emptyToNull,
  personnelNumberValid,
  toArbeitsvertragPayload,
  toKontaktPayload,
  toPersonPayload,
  toQualifikationenPayload,
  weeklyHoursValid,
  type FinancialDraft,
  type StammdatenDraft,
} from "./stammdaten-section-forms";

// Stammdaten tab (#1417 Tranche 2b + #1423): the master-data home of one
// staff member. Sections Person / Kontakt / Arbeitsvertrag / Qualifikationen
// ride on the personnel permission staff:stammdaten (#2906 — deliberately not
// users:read or users:update, both of which the ordinary Betreuer role
// holds), Abrechnung stays behind time_tracking:manage, and Bank & Steuer is
// staff:financial only — the card renders a lock hint without it, and every
// read of the stored values is access-logged server-side.
//
// Bearbeitet wird am Objekt (Bauart 2): EIN Bearbeiten-Zustand für den
// ganzen Reiter, EIN Speichern unten, EINE Begründung. Die Gruppen bleiben
// dabei sichtbar. Gespeichert wird pro Gruppe hintereinander, weil das
// Backend je Gruppe einen eigenen Endpunkt hat; der Nutzer sieht einen
// Vorgang, Fehler stehen oben im Bearbeiten-Bereich und am Feld (#2511).

const logger = createLogger({ component: "StammdatenTab" });

const genderLabels: Record<string, string> = {
  female: "Weiblich",
  male: "Männlich",
  diverse: "Divers",
};

/** Ein Wert oder der Gedankenstrich für „nicht hinterlegt". */
function Value({
  value,
  emptyText = "–",
}: {
  readonly value: string | null | undefined;
  readonly emptyText?: string;
}) {
  if (value) return <>{value}</>;
  return <span className="font-normal text-gray-400">{emptyText}</span>;
}

export function StammdatenTab({
  staffId,
  canManagePayroll,
  canManagePayrollSettings,
  canViewSections = false,
  canEditSections = false,
  canViewFinancial = false,
}: {
  readonly staffId: string;
  readonly canManagePayroll: boolean;
  readonly canManagePayrollSettings: boolean;
  readonly canViewSections?: boolean;
  readonly canEditSections?: boolean;
  readonly canViewFinancial?: boolean;
}) {
  const tenantPath = useTenantAwarePath();
  const berlinToday = useBerlinToday();

  const {
    data: personnelNumber,
    error: payrollError,
    isLoading: payrollLoading,
    isValidating: payrollValidating,
    mutate: mutatePayroll,
  } = useSWRAuth(
    canManagePayroll ? `staff-payroll-number-${staffId}` : null,
    () => staffPayrollNumberService.get(staffId),
  );

  const {
    data: stammdaten,
    error: stammdatenError,
    isValidating: stammdatenValidating,
    mutate: mutateStammdaten,
  } = useSWRAuth(canViewSections ? `staff-stammdaten-${staffId}` : null, () =>
    staffStammdatenService.get(staffId),
  );

  // Die maskierten Werte werden nur mit staff:financial angefragt: ohne die
  // Berechtigung bleibt der Schlüssel null und es geht keine Anfrage raus.
  const {
    data: financial,
    error: financialError,
    mutate: mutateFinancial,
  } = useSWRAuth(
    canViewFinancial ? `staff-stammdaten-financial-${staffId}` : null,
    () => staffStammdatenService.getFinancial(staffId),
  );

  // Anzeigen-Zustand der Bankdaten (Lesemodus).
  const [revealed, setRevealed] = useState<StaffFinancialPlain | null>(null);
  const [revealing, setRevealing] = useState(false);

  // Bearbeiten-Zustand des ganzen Reiters.
  const [draft, setDraft] = useState<StammdatenDraft | null>(null);
  const [baseline, setBaseline] = useState<StammdatenDraft | null>(null);
  const [note, setNote] = useState("");
  const [saving, setSaving] = useState(false);
  const [birthdayValid, setBirthdayValid] = useState(true);
  const [financialLoading, setFinancialLoading] = useState(false);
  // Fehler beim Speichern und beim Laden zum Ändern stehen im Alert oben im
  // Bearbeiten-Bereich, Feldfehler am Feld. Das Anzeigen im Lesemodus ist
  // keine Formularaktion und meldet als Toast (#2511).
  const editAreaRef = useRef<HTMLDivElement>(null);
  const formErrors = useApiFormError(editAreaRef);
  const actionErrors = useApiErrorDisplay();
  // Stand je Abschnitt, der in diesem Bearbeiten-Vorgang schon gespeichert
  // ist: ein erneutes Speichern schickt ihn nicht noch einmal (sonst doppelte
  // Einträge im Änderungsprotokoll).
  const savedSectionsRef = useRef(new Map<string, string>());
  // „Wiederholen“ im Alert speichert den aktuellen Entwurf, nicht den vom
  // Zeitpunkt des Fehlers.
  const latestSaveRef = useRef<() => Promise<void>>(async () => undefined);

  if (canManagePayroll && payrollError) {
    return (
      <div className="space-y-2">
        <Alert
          type="error"
          message="Die Personalnummer konnte nicht geladen werden."
        />
        <Button
          type="button"
          size="compact"
          variant="outline"
          isLoading={payrollValidating}
          loadingText="Wird geladen..."
          onClick={() => void mutatePayroll()}
        >
          Erneut laden
        </Button>
      </div>
    );
  }

  if (canViewSections && stammdatenError) {
    return (
      <div className="space-y-2">
        <Alert
          type="error"
          message="Die Stammdaten konnten nicht geladen werden."
        />
        <Button
          type="button"
          size="compact"
          variant="outline"
          isLoading={stammdatenValidating}
          loadingText="Wird geladen..."
          onClick={() => void mutateStammdaten()}
        >
          Erneut laden
        </Button>
      </div>
    );
  }

  const today = todayISO();
  const editing = draft !== null;
  const sectionsEditable = canEditSections && Boolean(stammdaten);
  const payrollEditable = canManagePayroll && !payrollLoading;
  const financialEditable = canViewFinancial && !financialError;
  const canStartEditing =
    sectionsEditable || payrollEditable || financialEditable;

  const patchDraft = (patch: Partial<StammdatenDraft>) =>
    setDraft((current) => (current ? { ...current, ...patch } : current));

  const patchFinancial = (patch: Partial<FinancialDraft>) =>
    setDraft((current) =>
      current?.financial
        ? { ...current, financial: { ...current.financial, ...patch } }
        : current,
    );

  const startEditing = () => {
    const next = buildDraft(stammdaten, personnelNumber);
    setDraft(next);
    setBaseline(next);
    setNote("");
    formErrors.clear();
    savedSectionsRef.current.clear();
    setBirthdayValid(true);
  };

  const cancelEditing = () => {
    setDraft(null);
    setBaseline(null);
    setNote("");
    formErrors.clear();
    savedSectionsRef.current.clear();
  };

  // Die Klartextwerte der Bankdaten werden erst auf ausdrückliche Anforderung
  // geladen — jeder Abruf landet im Audit-Log, deshalb löst nicht schon der
  // Wechsel in den Bearbeiten-Zustand ihn aus.
  const loadFinancialForEditing = async () => {
    setFinancialLoading(true);
    formErrors.clear();
    try {
      const plain = await staffStammdatenService.revealFinancial(staffId);
      setDraft((current) =>
        current
          ? {
              ...current,
              financial: {
                iban: plain.iban ?? "",
                taxId: plain.taxId ?? "",
                socialSecurityNumber: plain.socialSecurityNumber ?? "",
              },
            }
          : current,
      );
      setBaseline((current) =>
        current
          ? {
              ...current,
              financial: {
                iban: plain.iban ?? "",
                taxId: plain.taxId ?? "",
                socialSecurityNumber: plain.socialSecurityNumber ?? "",
              },
            }
          : current,
      );
    } catch (err) {
      logger.error("stammdaten_financial_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await formErrors.show(err, {
        object: "die Anzeige der Bank- und Steuerdaten",
        retry: () => void loadFinancialForEditing(),
      });
    } finally {
      setFinancialLoading(false);
    }
  };

  const toggleReveal = async () => {
    if (revealed) {
      setRevealed(null);
      return;
    }
    setRevealing(true);
    try {
      const plain = await staffStammdatenService.revealFinancial(staffId);
      setRevealed(plain);
    } catch (err) {
      logger.error("stammdaten_financial_reveal_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await actionErrors.show(err, {
        object: "die Anzeige der Bank- und Steuerdaten",
        retry: () => void toggleReveal(),
      });
    } finally {
      setRevealing(false);
    }
  };

  const changed = (
    section: string,
    pick: (value: StammdatenDraft) => unknown,
  ) =>
    draft !== null &&
    baseline !== null &&
    JSON.stringify(pick(draft)) !==
      (savedSectionsRef.current.get(section) ?? JSON.stringify(pick(baseline)));

  const draftValid =
    draft === null ||
    ((!sectionsEditable ||
      (draft.firstName.trim() !== "" &&
        draft.lastName.trim() !== "" &&
        birthdayValid &&
        weeklyHoursValid(draft.weeklyHours) &&
        draft.qualifikationen.every((row) => row.name.trim() !== ""))) &&
      (!payrollEditable || personnelNumberValid(draft.personnelNumber)));

  const handleSave = async () => {
    if (!draft) return;
    const reason = note.trim();
    // Der Rückgabewert der einzelnen Dienste ist hier ohne Belang — gespeichert
    // wird der Reiter als Ganzes, und die Fläche lädt danach ohnehin neu.
    const steps: {
      label: string;
      /** Was die Meldung nennt, mit Artikel (ADR 0006). */
      object: string;
      event: string;
      /** Der gesendete Stand, gemerkt nach Erfolg. */
      saved: string;
      run: () => Promise<unknown>;
    }[] = [];
    const section = (label: string) => `die Änderung an „${label}“`;

    if (sectionsEditable) {
      if (changed("Person", (d) => toPersonPayload(d))) {
        steps.push({
          label: "Person",
          object: section("Person"),
          saved: JSON.stringify(((d) => toPersonPayload(d))(draft)),
          event: "stammdaten_person_save_failed",
          run: () =>
            staffStammdatenService.updatePerson(
              staffId,
              toPersonPayload(draft),
              reason,
            ),
        });
      }
      if (changed("Kontakt", (d) => toKontaktPayload(d))) {
        steps.push({
          label: "Kontakt",
          object: section("Kontakt"),
          saved: JSON.stringify(((d) => toKontaktPayload(d))(draft)),
          event: "stammdaten_kontakt_save_failed",
          run: () =>
            staffStammdatenService.updateKontakt(
              staffId,
              toKontaktPayload(draft),
              reason,
            ),
        });
      }
      if (changed("Arbeitsvertrag", (d) => toArbeitsvertragPayload(d))) {
        steps.push({
          label: "Arbeitsvertrag",
          object: section("Arbeitsvertrag"),
          saved: JSON.stringify(((d) => toArbeitsvertragPayload(d))(draft)),
          event: "stammdaten_arbeitsvertrag_save_failed",
          run: () =>
            staffStammdatenService.updateArbeitsvertrag(
              staffId,
              toArbeitsvertragPayload(draft),
              reason,
            ),
        });
      }
      if (changed("Qualifikationen", (d) => toQualifikationenPayload(d))) {
        steps.push({
          label: "Qualifikationen",
          object: section("Qualifikationen"),
          saved: JSON.stringify(((d) => toQualifikationenPayload(d))(draft)),
          event: "stammdaten_qualifikationen_save_failed",
          run: () =>
            staffStammdatenService.updateQualifikationen(
              staffId,
              toQualifikationenPayload(draft),
              reason,
            ),
        });
      }
    }

    if (
      payrollEditable &&
      changed("Personalnummer", (d) => d.personnelNumber.trim())
    ) {
      steps.push({
        label: "Personalnummer",
        object: "die Personalnummer",
        saved: JSON.stringify(draft.personnelNumber.trim()),
        event: "stammdaten_payroll_save_failed",
        run: () =>
          staffPayrollNumberService.update(
            staffId,
            emptyToNull(draft.personnelNumber),
            reason,
          ),
      });
    }

    if (
      financialEditable &&
      draft.financial &&
      changed("Bank & Steuer", (d) => d.financial)
    ) {
      const values = draft.financial;
      steps.push({
        label: "Bank & Steuer",
        object: section("Bank & Steuer"),
        saved: JSON.stringify(values),
        event: "stammdaten_financial_save_failed",
        run: () =>
          staffStammdatenService.updateFinancial(
            staffId,
            {
              iban: emptyToNull(values.iban),
              taxId: emptyToNull(values.taxId),
              socialSecurityNumber: emptyToNull(values.socialSecurityNumber),
            },
            reason,
          ),
      });
    }

    if (steps.length === 0) {
      cancelEditing();
      return;
    }

    setSaving(true);
    formErrors.clear();
    // Gespeichert wird Abschnitt für Abschnitt. Scheitert einer, hält der
    // Vorgang dort an: die Meldung nennt genau diesen Abschnitt, die übrigen
    // Änderungen bleiben im offenen Entwurf, und nichts scheitert still.
    let failure: { object: string; error: unknown } | null = null;
    for (const step of steps) {
      try {
        await step.run();
        savedSectionsRef.current.set(step.label, step.saved);
      } catch (err) {
        logger.error(step.event, {
          error: err instanceof Error ? err.message : String(err),
        });
        failure = { object: step.object, error: err };
        break;
      }
    }
    setSaving(false);

    void mutateStammdaten();
    void mutatePayroll();
    void mutateFinancial();
    setRevealed(null);

    if (failure) {
      await formErrors.show(failure.error, {
        object: failure.object,
        retry: () => void latestSaveRef.current(),
      });
      return;
    }
    cancelEditing();
  };
  latestSaveRef.current = handleSave;

  return (
    <div ref={editAreaRef} className="space-y-5">
      {canStartEditing && !editing && (
        <div className="flex justify-end">
          <Button
            type="button"
            variant="outline"
            size="md"
            className="bg-white"
            onClick={startEditing}
          >
            Bearbeiten
          </Button>
        </div>
      )}

      {editing && <FormErrorAlert message={formErrors.error} />}

      {canViewSections && (
        <>
          <SectionCard collapsible title="Person">
            {editing && draft && sectionsEditable ? (
              <PersonFields
                draft={draft}
                onChange={patchDraft}
                berlinToday={berlinToday}
                onBirthdayValidityChange={setBirthdayValid}
                fieldError={formErrors.fieldError}
              />
            ) : (
              <DataGrid>
                {stammdaten ? (
                  <>
                    <DataField label="Vorname">
                      <Value value={stammdaten.person.firstName} />
                    </DataField>
                    <DataField label="Nachname">
                      <Value value={stammdaten.person.lastName} />
                    </DataField>
                    <DataField label="Geburtsdatum">
                      <Value
                        value={
                          stammdaten.person.birthday
                            ? formatDate(stammdaten.person.birthday)
                            : null
                        }
                      />
                    </DataField>
                    <DataField label="Geschlecht">
                      <Value
                        value={
                          stammdaten.person.gender
                            ? (genderLabels[stammdaten.person.gender] ?? null)
                            : null
                        }
                      />
                    </DataField>
                  </>
                ) : (
                  <>
                    <DataFieldSkeleton />
                    <DataFieldSkeleton />
                    <DataFieldSkeleton />
                    <DataFieldSkeleton />
                  </>
                )}
              </DataGrid>
            )}
          </SectionCard>

          <SectionCard collapsible title="Kontakt">
            {editing && draft && sectionsEditable ? (
              <KontaktFields draft={draft} onChange={patchDraft} />
            ) : (
              <DataGrid>
                {stammdaten ? (
                  <>
                    <DataField label="Adresse">
                      <Value value={formatAddress(stammdaten) ?? null} />
                    </DataField>
                    <DataField label="Telefon">
                      <Value value={stammdaten.kontakt.phone} />
                    </DataField>
                    <DataField label="E-Mail">
                      <Value value={stammdaten.kontakt.email} />
                    </DataField>
                    <DataField label="Notfallkontakt">
                      <Value
                        value={formatEmergencyContact(stammdaten) ?? null}
                      />
                    </DataField>
                  </>
                ) : (
                  <>
                    <DataFieldSkeleton />
                    <DataFieldSkeleton />
                    <DataFieldSkeleton />
                    <DataFieldSkeleton />
                  </>
                )}
              </DataGrid>
            )}
          </SectionCard>

          <SectionCard collapsible title="Arbeitsvertrag">
            {editing && draft && sectionsEditable ? (
              <ArbeitsvertragFields draft={draft} onChange={patchDraft} />
            ) : (
              <DataGrid>
                {stammdaten ? (
                  <>
                    <DataField label="Eintrittsdatum">
                      <Value
                        value={
                          stammdaten.arbeitsvertrag.entryDate
                            ? formatDate(stammdaten.arbeitsvertrag.entryDate)
                            : null
                        }
                      />
                    </DataField>
                    <DataField label="Beschäftigungstyp">
                      <Value
                        value={
                          stammdaten.arbeitsvertrag.employmentType
                            ? (employmentTypeLabels[
                                stammdaten.arbeitsvertrag.employmentType
                              ] ?? stammdaten.arbeitsvertrag.employmentType)
                            : null
                        }
                      />
                    </DataField>
                    <DataField label="Befristet bis">
                      <Value
                        value={
                          stammdaten.arbeitsvertrag.contractEndDate
                            ? formatDate(
                                stammdaten.arbeitsvertrag.contractEndDate,
                              )
                            : "Unbefristet"
                        }
                      />
                    </DataField>
                    <DataField label="Probezeit bis">
                      <Value
                        value={
                          stammdaten.arbeitsvertrag.probationEndDate
                            ? formatDate(
                                stammdaten.arbeitsvertrag.probationEndDate,
                              )
                            : null
                        }
                      />
                    </DataField>
                    <DataField label="Wochenstunden lt. Vertrag" mono>
                      <Value
                        value={
                          stammdaten.arbeitsvertrag.weeklyHours != null
                            ? `${stammdaten.arbeitsvertrag.weeklyHours.toLocaleString("de-DE")} Std.`
                            : null
                        }
                      />
                    </DataField>
                  </>
                ) : (
                  <>
                    <DataFieldSkeleton />
                    <DataFieldSkeleton />
                    <DataFieldSkeleton />
                    <DataFieldSkeleton />
                    <DataFieldSkeleton />
                  </>
                )}
              </DataGrid>
            )}
          </SectionCard>

          <SectionCard
            collapsible
            title="Qualifikationen"
            description="Nachweise wie Erste-Hilfe-Kurs oder Schwimmschein, mit Ablaufdatum."
          >
            {editing && draft && sectionsEditable ? (
              <QualifikationenFields draft={draft} onChange={patchDraft} />
            ) : !stammdaten ? (
              <DataGrid>
                <DataFieldSkeleton />
                <DataFieldSkeleton />
              </DataGrid>
            ) : stammdaten.qualifikationen.length === 0 ? (
              <p className="text-sm text-gray-400">
                Keine Qualifikationen hinterlegt.
              </p>
            ) : (
              <ul className="divide-y divide-gray-100">
                {stammdaten.qualifikationen.map((q, index) => {
                  const expired = q.expiresOn !== null && q.expiresOn < today;
                  const meta = [
                    q.acquiredOn
                      ? `erworben ${formatDate(q.acquiredOn)}`
                      : null,
                    q.expiresOn
                      ? `gültig bis ${formatDate(q.expiresOn)}`
                      : null,
                  ]
                    .filter(Boolean)
                    .join(" · ");
                  return (
                    <li
                      key={q.id ?? `${q.name}-${index}`}
                      className="flex flex-wrap items-center justify-between gap-2 py-2.5 first:pt-0 last:pb-0"
                    >
                      <div className="min-w-0">
                        <p className="text-sm font-medium text-gray-800">
                          {q.name}
                        </p>
                        {meta && (
                          <p className="mt-0.5 text-xs text-gray-500">{meta}</p>
                        )}
                      </div>
                      {expired && <StatusBadge label="Abgelaufen" tone="red" />}
                    </li>
                  );
                })}
              </ul>
            )}
          </SectionCard>
        </>
      )}

      {/* Abrechnung (#1417 Tranche 2b) */}
      {canManagePayroll ? (
        <SectionCard
          collapsible
          title="Personalnummer"
          description="Personalnummer aus dem Lohnsystem des Trägers. Ohne sie kann der spätere DATEV-Export diese Person keiner Abrechnung zuordnen. Lohnarten und DATEV-Mandantendaten werden zentral unter „Abrechnung“ gepflegt."
          action={
            canManagePayrollSettings && !editing ? (
              <ButtonLink
                href={tenantPath("/payroll")}
                variant="ghost"
                size="compact"
              >
                Abrechnung
              </ButtonLink>
            ) : null
          }
        >
          {editing && draft && payrollEditable ? (
            <PersonnelNumberFields draft={draft} onChange={patchDraft} />
          ) : (
            <DataGrid>
              {payrollLoading ? (
                <DataFieldSkeleton />
              ) : (
                <DataField label="Personalnummer" mono>
                  <Value
                    value={personnelNumber ?? null}
                    emptyText="Nicht gesetzt"
                  />
                </DataField>
              )}
            </DataGrid>
          )}
        </SectionCard>
      ) : null}

      {/* Bank & Steuer (#1423) */}
      {canViewFinancial ? (
        <SectionCard
          collapsible
          title="Bank & Steuer"
          description="Jeder Abruf der gespeicherten Werte wird im Audit-Log protokolliert."
          action={
            !editing && !financialError ? (
              <Button
                type="button"
                variant="outline"
                size="compact"
                className="bg-white"
                onClick={() => void toggleReveal()}
                disabled={revealing}
              >
                {revealed ? (
                  <>
                    <EyeOff className="mr-1 h-4 w-4" aria-hidden="true" />
                    Verbergen
                  </>
                ) : (
                  <>
                    <Eye className="mr-1 h-4 w-4" aria-hidden="true" />
                    Anzeigen
                  </>
                )}
              </Button>
            ) : null
          }
        >
          {financialError ? (
            <Alert
              type="error"
              message="Die Bank- und Steuerdaten konnten nicht geladen werden."
              action={
                <Button
                  type="button"
                  variant="outline"
                  size="compact"
                  onClick={() => void mutateFinancial()}
                >
                  Erneut laden
                </Button>
              }
            />
          ) : editing && draft ? (
            draft.financial ? (
              <FinancialFields
                values={draft.financial}
                onChange={patchFinancial}
              />
            ) : (
              <div className="space-y-3">
                <p className="text-sm text-gray-600">
                  Zum Ändern müssen die gespeicherten Werte geladen werden. Der
                  Abruf wird im Audit-Log protokolliert.
                </p>
                <Button
                  type="button"
                  variant="outline"
                  size="md"
                  isLoading={financialLoading}
                  loadingText="Wird geladen…"
                  onClick={() => void loadFinancialForEditing()}
                >
                  Werte zum Ändern laden
                </Button>
              </div>
            )
          ) : (
            <DataGrid>
              <DataField label="IBAN" mono>
                <Value
                  value={
                    revealed ? revealed.iban : (financial?.ibanMasked ?? null)
                  }
                />
              </DataField>
              <DataField label="Steuer-ID" mono>
                <Value
                  value={
                    revealed ? revealed.taxId : (financial?.taxIdMasked ?? null)
                  }
                />
              </DataField>
              <DataField label="SV-Nummer" mono>
                <Value
                  value={
                    revealed
                      ? revealed.socialSecurityNumber
                      : (financial?.socialSecurityNumberMasked ?? null)
                  }
                />
              </DataField>
            </DataGrid>
          )}
        </SectionCard>
      ) : (
        <SectionCard title="Bank & Steuer">
          <div className="flex items-start gap-2 rounded-xl bg-gray-50 p-3">
            <Lock
              className="mt-0.5 h-4 w-4 shrink-0 text-gray-400"
              aria-hidden="true"
            />
            <p className="text-sm text-gray-500">
              Nicht berechtigt: IBAN, Steuer-ID und SV-Nummer sind nur mit der
              Berechtigung „Bank- &amp; Steuerdaten“ sichtbar.
            </p>
          </div>
        </SectionCard>
      )}

      {editing && draft && (
        <SectionCard>
          <div className="space-y-4">
            <Input
              controlSize="compact"
              label="Begründung (optional)"
              name="stammdaten-note"
              value={note}
              onChange={(e) => setNote(e.target.value)}
              placeholder="Erscheint im Änderungsprotokoll"
            />
            <EditActions
              onCancel={cancelEditing}
              onSave={() => void handleSave()}
              saving={saving}
              disabled={!draftValid}
            />
          </div>
        </SectionCard>
      )}
    </div>
  );
}

function formatAddress(stammdaten: StaffStammdaten): string | undefined {
  const { addressStreet, addressPostalCode, addressCity } = stammdaten.kontakt;
  const cityLine = [addressPostalCode, addressCity].filter(Boolean).join(" ");
  const parts = [addressStreet, cityLine].filter(Boolean);
  return parts.length > 0 ? parts.join(", ") : undefined;
}

function formatEmergencyContact(
  stammdaten: StaffStammdaten,
): string | undefined {
  const { emergencyContactName, emergencyContactPhone } = stammdaten.kontakt;
  const parts = [emergencyContactName, emergencyContactPhone].filter(Boolean);
  return parts.length > 0 ? parts.join(" · ") : undefined;
}
