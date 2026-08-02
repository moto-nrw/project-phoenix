"use client";

import { useState } from "react";
import Link from "next/link";
import { ChevronDown, Eye, EyeOff, Lock } from "lucide-react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Loading } from "~/components/ui/loading";
import { Modal } from "~/components/ui/modal";
import { StatusBadge } from "~/components/ui/status-badge";
import { formatDate, todayISO } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { employmentTypeLabels } from "~/lib/staff-helpers";
import { useSWRAuth } from "~/lib/swr";
import {
  staffPayrollNumberService,
  staffStammdatenService,
  type StaffFinancialPlain,
  type StaffStammdaten,
} from "~/lib/staff-api";
import {
  ArbeitsvertragEditModal,
  FinancialEditModal,
  KontaktEditModal,
  PersonEditModal,
  QualifikationenEditModal,
} from "./stammdaten-section-modals";

// Stammdaten tab (#1417 Tranche 2b + #1423): the master-data home of one
// staff member. Sections Person / Kontakt / Arbeitsvertrag / Qualifikationen
// ride on the directory permissions (users:read to see, users:update to
// edit), Abrechnung stays behind time_tracking:manage, and Bank & Steuer is
// staff:financial only — the card renders a lock hint without it, and every
// read of the stored values is access-logged server-side.

const logger = createLogger({ component: "StammdatenTab" });

const genderLabels: Record<string, string> = {
  female: "Weiblich",
  male: "Männlich",
  diverse: "Divers",
};

type SectionModal =
  | "person"
  | "kontakt"
  | "arbeitsvertrag"
  | "qualifikationen"
  | "payroll"
  | null;

// --- shared section chrome --------------------------------------------------

function SectionCard({
  kicker,
  title,
  description,
  action,
  children,
}: {
  readonly kicker: string;
  readonly title: string;
  readonly description?: string;
  readonly action?: React.ReactNode;
  readonly children: React.ReactNode;
}) {
  const [collapsed, setCollapsed] = useState(false);

  return (
    <section className="moto-content-surface overflow-hidden rounded-2xl border p-5 shadow-sm">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
            {kicker}
          </p>
          <h3 className="mt-1 text-base font-semibold text-gray-900">
            {title}
          </h3>
          {description && (
            <p className="mt-1 max-w-2xl text-xs leading-5 text-gray-500">
              {description}
            </p>
          )}
        </div>
        <div className="flex shrink-0 items-center gap-1">
          {action}
          <Button
            type="button"
            variant="ghost"
            size="icon"
            aria-label={
              collapsed ? `${title} ausklappen` : `${title} einklappen`
            }
            aria-expanded={!collapsed}
            onClick={() => setCollapsed((prev) => !prev)}
          >
            <ChevronDown
              className={`h-4 w-4 transition-transform ${collapsed ? "-rotate-90" : ""}`}
              aria-hidden="true"
            />
          </Button>
        </div>
      </div>
      {!collapsed && <div className="mt-4">{children}</div>}
    </section>
  );
}

function Field({
  label,
  value,
  mono,
  emptyText = "—",
}: {
  readonly label: string;
  readonly value: string | null | undefined;
  readonly mono?: boolean;
  readonly emptyText?: string;
}) {
  return (
    <div>
      <dt className="text-xs font-medium text-gray-500">{label}</dt>
      <dd
        className={`mt-0.5 text-sm ${value ? "text-gray-800" : "text-gray-400"} ${mono ? "tabular-nums" : ""}`}
      >
        {value ?? emptyText}
      </dd>
    </div>
  );
}

function FieldGrid({ children }: { readonly children: React.ReactNode }) {
  return (
    <dl className="grid grid-cols-1 gap-x-6 gap-y-3 sm:grid-cols-2">
      {children}
    </dl>
  );
}

function EditAction({
  visible,
  onClick,
}: {
  readonly visible: boolean;
  readonly onClick: () => void;
}) {
  if (!visible) return null;
  return (
    <Button type="button" variant="ghost" size="compact" onClick={onClick}>
      Bearbeiten
    </Button>
  );
}

// --- main tab ---------------------------------------------------------------

export function StammdatenTab({
  staffId,
  canManagePayroll,
  canManagePayrollSettings,
  canEditSections = false,
  canViewFinancial = false,
}: {
  readonly staffId: string;
  readonly canManagePayroll: boolean;
  readonly canManagePayrollSettings: boolean;
  readonly canEditSections?: boolean;
  readonly canViewFinancial?: boolean;
}) {
  const [openModal, setOpenModal] = useState<SectionModal>(null);

  const {
    data: personnelNumber,
    error: payrollError,
    isLoading: payrollLoading,
    isValidating: payrollValidating,
    mutate: mutatePayroll,
  } = useSWRAuth(`staff-payroll-number-${staffId}`, () =>
    staffPayrollNumberService.get(staffId),
  );

  const {
    data: stammdaten,
    error: stammdatenError,
    isLoading: stammdatenLoading,
    isValidating: stammdatenValidating,
    mutate: mutateStammdaten,
  } = useSWRAuth(`staff-stammdaten-${staffId}`, () =>
    staffStammdatenService.get(staffId),
  );

  if (payrollLoading || stammdatenLoading) {
    return <Loading fullPage={false} />;
  }

  if (payrollError) {
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

  if (stammdatenError) {
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

  const closeAndRefresh = (mutator: () => void) => {
    setOpenModal(null);
    mutator();
  };

  const today = todayISO();

  return (
    <div className="space-y-5">
      {stammdaten && (
        <>
          <SectionCard
            kicker="Stammdaten"
            title="Person"
            action={
              <EditAction
                visible={canEditSections}
                onClick={() => setOpenModal("person")}
              />
            }
          >
            <FieldGrid>
              <Field label="Vorname" value={stammdaten.person.firstName} />
              <Field label="Nachname" value={stammdaten.person.lastName} />
              <Field
                label="Geburtsdatum"
                value={
                  stammdaten.person.birthday
                    ? formatDate(stammdaten.person.birthday)
                    : null
                }
              />
              <Field
                label="Geschlecht"
                value={
                  stammdaten.person.gender
                    ? (genderLabels[stammdaten.person.gender] ?? null)
                    : null
                }
              />
            </FieldGrid>
          </SectionCard>

          <SectionCard
            kicker="Stammdaten"
            title="Kontakt"
            action={
              <EditAction
                visible={canEditSections}
                onClick={() => setOpenModal("kontakt")}
              />
            }
          >
            <FieldGrid>
              <Field
                label="Adresse"
                value={formatAddress(stammdaten) ?? null}
              />
              <Field label="Telefon" value={stammdaten.kontakt.phone} />
              <Field label="E-Mail" value={stammdaten.kontakt.email} />
              <Field
                label="Notfallkontakt"
                value={formatEmergencyContact(stammdaten) ?? null}
              />
            </FieldGrid>
          </SectionCard>

          <SectionCard
            kicker="Stammdaten"
            title="Arbeitsvertrag"
            action={
              <EditAction
                visible={canEditSections}
                onClick={() => setOpenModal("arbeitsvertrag")}
              />
            }
          >
            <FieldGrid>
              <Field
                label="Eintrittsdatum"
                value={
                  stammdaten.arbeitsvertrag.entryDate
                    ? formatDate(stammdaten.arbeitsvertrag.entryDate)
                    : null
                }
              />
              <Field
                label="Beschäftigungstyp"
                value={
                  stammdaten.arbeitsvertrag.employmentType
                    ? (employmentTypeLabels[
                        stammdaten.arbeitsvertrag.employmentType
                      ] ?? stammdaten.arbeitsvertrag.employmentType)
                    : null
                }
              />
              <Field
                label="Befristet bis"
                value={
                  stammdaten.arbeitsvertrag.contractEndDate
                    ? formatDate(stammdaten.arbeitsvertrag.contractEndDate)
                    : "Unbefristet"
                }
              />
              <Field
                label="Probezeit bis"
                value={
                  stammdaten.arbeitsvertrag.probationEndDate
                    ? formatDate(stammdaten.arbeitsvertrag.probationEndDate)
                    : null
                }
              />
              <Field
                label="Wochenstunden lt. Vertrag"
                value={
                  stammdaten.arbeitsvertrag.weeklyHours != null
                    ? `${stammdaten.arbeitsvertrag.weeklyHours.toLocaleString("de-DE")} Std.`
                    : null
                }
                mono
              />
            </FieldGrid>
          </SectionCard>

          <SectionCard
            kicker="Stammdaten"
            title="Qualifikationen"
            description="Nachweise wie Erste-Hilfe-Kurs oder Schwimmschein, mit Ablaufdatum."
            action={
              <EditAction
                visible={canEditSections}
                onClick={() => setOpenModal("qualifikationen")}
              />
            }
          >
            {stammdaten.qualifikationen.length === 0 ? (
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
      <SectionCard
        kicker="Abrechnung"
        title="Personalnummer"
        description="Personalnummer aus dem Lohnsystem des Trägers. Ohne sie kann der spätere DATEV-Export diese Person keiner Abrechnung zuordnen."
        action={
          <EditAction
            visible={canManagePayroll}
            onClick={() => setOpenModal("payroll")}
          />
        }
      >
        <FieldGrid>
          <Field
            label="Personalnummer"
            value={personnelNumber ?? null}
            emptyText="Nicht gesetzt"
            mono
          />
        </FieldGrid>
        <p className="mt-4 text-xs text-gray-500">
          Lohnarten und DATEV-Mandantendaten werden zentral unter{" "}
          {canManagePayrollSettings ? (
            <Link href="/payroll" className="text-[#5080D8] hover:underline">
              Abrechnung
            </Link>
          ) : (
            "Abrechnung"
          )}{" "}
          gepflegt.
        </p>
      </SectionCard>

      {/* Bank & Steuer (#1423) */}
      {canViewFinancial ? (
        <FinancialSection staffId={staffId} />
      ) : (
        <section className="moto-content-surface rounded-2xl border p-5 shadow-sm">
          <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
            Vertraulich
          </p>
          <h3 className="mt-1 text-base font-semibold text-gray-900">
            Bank &amp; Steuer
          </h3>
          <div className="mt-3 flex items-start gap-2 rounded-xl bg-gray-50 p-3">
            <Lock
              className="mt-0.5 h-4 w-4 shrink-0 text-gray-400"
              aria-hidden="true"
            />
            <p className="text-sm text-gray-500">
              Nicht berechtigt: IBAN, Steuer-ID und SV-Nummer sind nur mit der
              Berechtigung „Bank- &amp; Steuerdaten“ sichtbar.
            </p>
          </div>
        </section>
      )}

      {/* Modals */}
      {openModal === "person" && stammdaten && (
        <PersonEditModal
          staffId={staffId}
          person={stammdaten.person}
          onClose={() => setOpenModal(null)}
          onSaved={() => closeAndRefresh(() => void mutateStammdaten())}
        />
      )}
      {openModal === "kontakt" && stammdaten && (
        <KontaktEditModal
          staffId={staffId}
          kontakt={stammdaten.kontakt}
          onClose={() => setOpenModal(null)}
          onSaved={() => closeAndRefresh(() => void mutateStammdaten())}
        />
      )}
      {openModal === "arbeitsvertrag" && stammdaten && (
        <ArbeitsvertragEditModal
          staffId={staffId}
          vertrag={stammdaten.arbeitsvertrag}
          onClose={() => setOpenModal(null)}
          onSaved={() => closeAndRefresh(() => void mutateStammdaten())}
        />
      )}
      {openModal === "qualifikationen" && stammdaten && (
        <QualifikationenEditModal
          staffId={staffId}
          qualifikationen={stammdaten.qualifikationen}
          onClose={() => setOpenModal(null)}
          onSaved={() => closeAndRefresh(() => void mutateStammdaten())}
        />
      )}
      {openModal === "payroll" && (
        <PersonnelNumberModal
          staffId={staffId}
          current={personnelNumber ?? null}
          onClose={() => setOpenModal(null)}
          onSaved={() => closeAndRefresh(() => void mutatePayroll())}
        />
      )}
    </div>
  );
}

// --- Bank & Steuer (#1423) --------------------------------------------------
//
// Its own component so the masked-data request only ever exists for holders
// of staff:financial — the section is simply not mounted otherwise.
function FinancialSection({ staffId }: { readonly staffId: string }) {
  const [editorOpen, setEditorOpen] = useState(false);
  const [revealed, setRevealed] = useState<StaffFinancialPlain | null>(null);
  const [revealing, setRevealing] = useState(false);
  const [revealError, setRevealError] = useState<string | null>(null);

  const { data: financial, mutate: mutateFinancial } = useSWRAuth(
    `staff-stammdaten-financial-${staffId}`,
    () => staffStammdatenService.getFinancial(staffId),
  );

  const toggleReveal = async () => {
    if (revealed) {
      setRevealed(null);
      return;
    }
    setRevealing(true);
    setRevealError(null);
    try {
      const plain = await staffStammdatenService.revealFinancial(staffId);
      setRevealed(plain);
    } catch (err) {
      logger.error("stammdaten_financial_reveal_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setRevealError(
        "Die Werte konnten nicht angezeigt werden. Bitte erneut versuchen.",
      );
    } finally {
      setRevealing(false);
    }
  };

  return (
    <>
      <SectionCard
        kicker="Vertraulich"
        title="Bank & Steuer"
        description="Jeder Abruf der gespeicherten Werte wird im Audit-Log protokolliert."
        action={
          <>
            <Button
              type="button"
              variant="ghost"
              size="compact"
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
            <EditAction visible onClick={() => setEditorOpen(true)} />
          </>
        }
      >
        <FieldGrid>
          <Field
            label="IBAN"
            value={revealed ? revealed.iban : (financial?.ibanMasked ?? null)}
            mono
          />
          <Field
            label="Steuer-ID"
            value={revealed ? revealed.taxId : (financial?.taxIdMasked ?? null)}
            mono
          />
          <Field
            label="SV-Nummer"
            value={
              revealed
                ? revealed.socialSecurityNumber
                : (financial?.socialSecurityNumberMasked ?? null)
            }
            mono
          />
        </FieldGrid>
        {revealError && (
          <p className="mt-2 text-sm text-[#FF3130]">{revealError}</p>
        )}
      </SectionCard>
      {editorOpen && (
        <FinancialEditModal
          staffId={staffId}
          onClose={() => setEditorOpen(false)}
          onSaved={() => {
            setEditorOpen(false);
            setRevealed(null);
            void mutateFinancial();
          }}
        />
      )}
    </>
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

// --- Personalnummer modal (#1417 Tranche 2b, unchanged behavior) ------------

function PersonnelNumberModal({
  staffId,
  current,
  onClose,
  onSaved,
}: {
  readonly staffId: string;
  readonly current: string | null;
  readonly onClose: () => void;
  readonly onSaved: () => void;
}) {
  const [value, setValue] = useState(current ?? "");
  const [note, setNote] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const trimmed = value.trim();
  const valid = trimmed === "" || /^\d{1,9}$/.test(trimmed);

  const handleSave = async () => {
    setSaving(true);
    setError(null);
    try {
      await staffPayrollNumberService.update(
        staffId,
        trimmed === "" ? null : trimmed,
        note.trim(),
      );
      onSaved();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Speichern fehlgeschlagen");
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal isOpen onClose={onClose} title="Personalnummer bearbeiten">
      <div className="space-y-4">
        <div>
          <label
            htmlFor="personnel-number"
            className="mb-1 block text-sm font-medium text-gray-700"
          >
            Personalnummer
          </label>
          <Input
            id="personnel-number"
            value={value}
            onChange={(e) => setValue(e.target.value)}
            placeholder="z. B. 1023"
            inputMode="numeric"
          />
          <p className="mt-1 text-xs text-gray-500">
            Nur Ziffern. Leer lassen, um die Nummer zu entfernen. Die Nummer
            muss der Personalnummer im Lohnsystem entsprechen und ist pro Schule
            eindeutig.
          </p>
          {!valid && (
            <p className="mt-1 text-xs text-[#FF3130]">
              Nur Ziffern, höchstens 9 Stellen.
            </p>
          )}
        </div>

        <div>
          <label
            htmlFor="personnel-number-note"
            className="mb-1 block text-sm font-medium text-gray-700"
          >
            Begründung (optional)
          </label>
          <Input
            id="personnel-number-note"
            value={note}
            onChange={(e) => setNote(e.target.value)}
            placeholder="Erscheint im Änderungsprotokoll"
          />
        </div>

        {error && <p className="text-sm text-[#FF3130]">{error}</p>}

        <div className="flex justify-end gap-2">
          <Button type="button" variant="outline" size="md" onClick={onClose}>
            Abbrechen
          </Button>
          <Button
            type="button"
            variant="primary"
            size="md"
            onClick={handleSave}
            disabled={saving || !valid || trimmed === (current ?? "").trim()}
          >
            {saving ? "Speichert…" : "Speichern"}
          </Button>
        </div>
      </div>
    </Modal>
  );
}
