"use client";

import { useEffect, useState } from "react";
import { Plus, Trash2 } from "lucide-react";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { CustomSelect } from "~/components/ui/custom-select";
import { ISODatePicker } from "~/components/ui/date-picker";
import { Loading } from "~/components/ui/loading";
import { Modal } from "~/components/ui/modal";
import { createLogger } from "~/lib/logger";
import {
  staffStammdatenService,
  type StammdatenArbeitsvertrag,
  type StammdatenGender,
  type StammdatenKontakt,
  type StammdatenPerson,
  type StammdatenQualifikation,
} from "~/lib/staff-api";

// Section edit modals for the Stammdaten tab (#1423). One modal per section,
// mirroring the time-tracking edit pattern: full-section submit plus an
// optional Begründung that lands in the Änderungsprotokoll.

const logger = createLogger({ component: "StammdatenSectionModals" });

const genderOptions = [
  { value: "", label: "Keine Angabe" },
  { value: "female", label: "Weiblich" },
  { value: "male", label: "Männlich" },
  { value: "diverse", label: "Divers" },
] as const;

const employmentOptions = [
  { value: "", label: "Keine Angabe" },
  { value: "full_time", label: "Vollzeit" },
  { value: "part_time", label: "Teilzeit" },
  { value: "minijob", label: "Minijob" },
] as const;

function emptyToNull(value: string): string | null {
  const trimmed = value.trim();
  return trimmed === "" ? null : trimmed;
}

function NoteField({
  value,
  onChange,
}: {
  readonly value: string;
  readonly onChange: (next: string) => void;
}) {
  return (
    <Input
      controlSize="compact"
      label="Begründung (optional)"
      name="stammdaten-note"
      value={value}
      onChange={(e) => onChange(e.target.value)}
      placeholder="Erscheint im Änderungsprotokoll"
    />
  );
}

function ModalFooter({
  onClose,
  onSave,
  saving,
  disabled,
}: {
  readonly onClose: () => void;
  readonly onSave: () => void;
  readonly saving: boolean;
  readonly disabled?: boolean;
}) {
  return (
    <div className="flex justify-end gap-2">
      <Button type="button" variant="outline" size="md" onClick={onClose}>
        Abbrechen
      </Button>
      <Button
        type="button"
        variant="primary"
        size="md"
        onClick={onSave}
        disabled={saving || disabled}
      >
        {saving ? "Speichert…" : "Speichern"}
      </Button>
    </div>
  );
}

function SelectField({
  id,
  label,
  value,
  options,
  onChange,
}: {
  readonly id: string;
  readonly label: string;
  readonly value: string;
  readonly options: readonly { value: string; label: string }[];
  readonly onChange: (value: string) => void;
}) {
  return (
    <div>
      <label
        id={`${id}-label`}
        htmlFor={id}
        className="mb-2 block text-sm font-medium text-gray-700"
      >
        {label}
      </label>
      <CustomSelect
        id={id}
        labelId={`${id}-label`}
        value={value}
        options={options}
        onChange={onChange}
      />
    </div>
  );
}

function useSectionSave(onSaved: () => void) {
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const run = async (event: string, save: () => Promise<void>) => {
    setSaving(true);
    setError(null);
    try {
      await save();
      onSaved();
    } catch (err) {
      logger.error(event, {
        error: err instanceof Error ? err.message : String(err),
      });
      setError(err instanceof Error ? err.message : "Speichern fehlgeschlagen");
    } finally {
      setSaving(false);
    }
  };

  return { saving, error, run };
}

// --- Person -----------------------------------------------------------------

export function PersonEditModal({
  staffId,
  person,
  onClose,
  onSaved,
}: {
  readonly staffId: string;
  readonly person: StammdatenPerson;
  readonly onClose: () => void;
  readonly onSaved: () => void;
}) {
  const [firstName, setFirstName] = useState(person.firstName);
  const [lastName, setLastName] = useState(person.lastName);
  const [birthday, setBirthday] = useState(person.birthday ?? "");
  const [gender, setGender] = useState<string>(person.gender ?? "");
  const [note, setNote] = useState("");
  const { saving, error, run } = useSectionSave(onSaved);

  const valid = firstName.trim() !== "" && lastName.trim() !== "";

  return (
    <Modal isOpen onClose={onClose} title="Person bearbeiten">
      <div className="space-y-4">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Input
            controlSize="compact"
            label="Vorname"
            name="stammdaten-first-name"
            value={firstName}
            onChange={(e) => setFirstName(e.target.value)}
          />
          <Input
            controlSize="compact"
            label="Nachname"
            name="stammdaten-last-name"
            value={lastName}
            onChange={(e) => setLastName(e.target.value)}
          />
          <ISODatePicker
            label="Geburtsdatum"
            id="stammdaten-birthday"
            value={birthday}
            onChange={setBirthday}
          />
          <SelectField
            id="stammdaten-gender"
            label="Geschlecht"
            value={gender}
            options={genderOptions}
            onChange={setGender}
          />
        </div>
        <NoteField value={note} onChange={setNote} />
        {error && <p className="text-sm text-[#FF3130]">{error}</p>}
        <ModalFooter
          onClose={onClose}
          saving={saving}
          disabled={!valid}
          onSave={() =>
            void run("stammdaten_person_save_failed", () =>
              staffStammdatenService.updatePerson(
                staffId,
                {
                  firstName: firstName.trim(),
                  lastName: lastName.trim(),
                  birthday: emptyToNull(birthday),
                  gender: (gender || null) as StammdatenGender | null,
                },
                note.trim(),
              ),
            )
          }
        />
      </div>
    </Modal>
  );
}

// --- Kontakt ----------------------------------------------------------------

export function KontaktEditModal({
  staffId,
  kontakt,
  onClose,
  onSaved,
}: {
  readonly staffId: string;
  readonly kontakt: StammdatenKontakt;
  readonly onClose: () => void;
  readonly onSaved: () => void;
}) {
  const [street, setStreet] = useState(kontakt.addressStreet ?? "");
  const [postalCode, setPostalCode] = useState(kontakt.addressPostalCode ?? "");
  const [city, setCity] = useState(kontakt.addressCity ?? "");
  const [phone, setPhone] = useState(kontakt.phone ?? "");
  const [email, setEmail] = useState(kontakt.email ?? "");
  const [emergencyName, setEmergencyName] = useState(
    kontakt.emergencyContactName ?? "",
  );
  const [emergencyPhone, setEmergencyPhone] = useState(
    kontakt.emergencyContactPhone ?? "",
  );
  const [note, setNote] = useState("");
  const { saving, error, run } = useSectionSave(onSaved);

  return (
    <Modal isOpen onClose={onClose} title="Kontakt bearbeiten">
      <div className="space-y-4">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div className="sm:col-span-2">
            <Input
              controlSize="compact"
              label="Straße und Hausnummer"
              name="stammdaten-street"
              value={street}
              onChange={(e) => setStreet(e.target.value)}
            />
          </div>
          <Input
            controlSize="compact"
            label="PLZ"
            name="stammdaten-postal-code"
            value={postalCode}
            onChange={(e) => setPostalCode(e.target.value)}
            inputMode="numeric"
          />
          <Input
            controlSize="compact"
            label="Ort"
            name="stammdaten-city"
            value={city}
            onChange={(e) => setCity(e.target.value)}
          />
          <Input
            controlSize="compact"
            label="Telefon"
            name="stammdaten-phone"
            value={phone}
            onChange={(e) => setPhone(e.target.value)}
            inputMode="tel"
          />
          <Input
            controlSize="compact"
            label="E-Mail"
            name="stammdaten-email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            inputMode="email"
          />
          <Input
            controlSize="compact"
            label="Notfallkontakt (Name)"
            name="stammdaten-emergency-name"
            value={emergencyName}
            onChange={(e) => setEmergencyName(e.target.value)}
          />
          <Input
            controlSize="compact"
            label="Notfallkontakt (Telefon)"
            name="stammdaten-emergency-phone"
            value={emergencyPhone}
            onChange={(e) => setEmergencyPhone(e.target.value)}
            inputMode="tel"
          />
        </div>
        <NoteField value={note} onChange={setNote} />
        {error && <p className="text-sm text-[#FF3130]">{error}</p>}
        <ModalFooter
          onClose={onClose}
          saving={saving}
          onSave={() =>
            void run("stammdaten_kontakt_save_failed", () =>
              staffStammdatenService.updateKontakt(
                staffId,
                {
                  addressStreet: emptyToNull(street),
                  addressPostalCode: emptyToNull(postalCode),
                  addressCity: emptyToNull(city),
                  phone: emptyToNull(phone),
                  email: emptyToNull(email),
                  emergencyContactName: emptyToNull(emergencyName),
                  emergencyContactPhone: emptyToNull(emergencyPhone),
                },
                note.trim(),
              ),
            )
          }
        />
      </div>
    </Modal>
  );
}

// --- Arbeitsvertrag ---------------------------------------------------------

export function ArbeitsvertragEditModal({
  staffId,
  vertrag,
  onClose,
  onSaved,
}: {
  readonly staffId: string;
  readonly vertrag: StammdatenArbeitsvertrag;
  readonly onClose: () => void;
  readonly onSaved: () => void;
}) {
  const [entryDate, setEntryDate] = useState(vertrag.entryDate ?? "");
  const [contractEnd, setContractEnd] = useState(vertrag.contractEndDate ?? "");
  const [probationEnd, setProbationEnd] = useState(
    vertrag.probationEndDate ?? "",
  );
  const [weeklyHours, setWeeklyHours] = useState(
    vertrag.weeklyHours != null ? String(vertrag.weeklyHours) : "",
  );
  const [employmentType, setEmploymentType] = useState(
    vertrag.employmentType ?? "",
  );
  const [note, setNote] = useState("");
  const { saving, error, run } = useSectionSave(onSaved);

  // German decimal comma ("29,5") must parse like the dot form.
  const hours =
    weeklyHours.trim() === ""
      ? null
      : Number(weeklyHours.trim().replace(",", "."));
  const hoursValid =
    hours === null || (!Number.isNaN(hours) && hours >= 0 && hours <= 80);

  return (
    <Modal isOpen onClose={onClose} title="Arbeitsvertrag bearbeiten">
      <div className="space-y-4">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <ISODatePicker
            label="Eintrittsdatum"
            id="stammdaten-entry-date"
            value={entryDate}
            onChange={setEntryDate}
          />
          <SelectField
            id="stammdaten-employment"
            label="Beschäftigungstyp"
            value={employmentType}
            options={employmentOptions}
            onChange={setEmploymentType}
          />
          <ISODatePicker
            label="Befristet bis"
            id="stammdaten-contract-end"
            value={contractEnd}
            onChange={setContractEnd}
          />
          <ISODatePicker
            label="Probezeit bis"
            id="stammdaten-probation-end"
            value={probationEnd}
            onChange={setProbationEnd}
          />
          <Input
            controlSize="compact"
            label="Wochenstunden lt. Vertrag"
            name="stammdaten-weekly-hours"
            value={weeklyHours}
            onChange={(e) => setWeeklyHours(e.target.value)}
            inputMode="decimal"
            placeholder="z. B. 29,5"
            error={
              hoursValid ? undefined : "Bitte einen Wert zwischen 0 und 80."
            }
          />
        </div>
        <p className="text-xs text-gray-500">
          Ein leeres „Befristet bis“ bedeutet unbefristet. Die Wochenstunden
          dokumentieren den Vertrag; das Arbeitszeitmodell für die
          Soll-Berechnung wird im Tab Arbeitszeitmodell gepflegt.
        </p>
        <NoteField value={note} onChange={setNote} />
        {error && <p className="text-sm text-[#FF3130]">{error}</p>}
        <ModalFooter
          onClose={onClose}
          saving={saving}
          disabled={!hoursValid}
          onSave={() =>
            void run("stammdaten_arbeitsvertrag_save_failed", () =>
              staffStammdatenService.updateArbeitsvertrag(
                staffId,
                {
                  entryDate: emptyToNull(entryDate),
                  contractEndDate: emptyToNull(contractEnd),
                  probationEndDate: emptyToNull(probationEnd),
                  weeklyHours: hours === null ? null : Number(hours.toFixed(2)),
                  employmentType: employmentType || null,
                },
                note.trim(),
              ),
            )
          }
        />
      </div>
    </Modal>
  );
}

// --- Qualifikationen --------------------------------------------------------

interface QualificationRow {
  name: string;
  acquiredOn: string;
  expiresOn: string;
}

export function QualifikationenEditModal({
  staffId,
  qualifikationen,
  onClose,
  onSaved,
}: {
  readonly staffId: string;
  readonly qualifikationen: readonly StammdatenQualifikation[];
  readonly onClose: () => void;
  readonly onSaved: () => void;
}) {
  const [rows, setRows] = useState<QualificationRow[]>(
    qualifikationen.map((q) => ({
      name: q.name,
      acquiredOn: q.acquiredOn ?? "",
      expiresOn: q.expiresOn ?? "",
    })),
  );
  const [note, setNote] = useState("");
  const { saving, error, run } = useSectionSave(onSaved);

  const setRow = (index: number, patch: Partial<QualificationRow>) => {
    setRows((prev) =>
      prev.map((row, i) => (i === index ? { ...row, ...patch } : row)),
    );
  };

  const valid = rows.every((row) => row.name.trim() !== "");

  return (
    <Modal isOpen onClose={onClose} title="Qualifikationen bearbeiten">
      <div className="space-y-4">
        {rows.length === 0 && (
          <p className="text-sm text-gray-500">
            Keine Qualifikationen hinterlegt.
          </p>
        )}
        <div className="space-y-3">
          {rows.map((row, index) => (
            // Rows are positional (full-list replace); index is the identity.
            // eslint-disable-next-line react/no-array-index-key
            <div key={index} className="rounded-xl bg-gray-50 p-3">
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-[1fr_auto]">
                <Input
                  controlSize="compact"
                  label="Bezeichnung"
                  name={`stammdaten-qualification-${index}`}
                  value={row.name}
                  onChange={(e) => setRow(index, { name: e.target.value })}
                  placeholder="z. B. Erste-Hilfe-Kurs"
                />
                <div className="flex items-end justify-end">
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    aria-label="Qualifikation entfernen"
                    onClick={() =>
                      setRows((prev) => prev.filter((_, i) => i !== index))
                    }
                  >
                    <Trash2 className="h-4 w-4" aria-hidden="true" />
                  </Button>
                </div>
                <div className="grid grid-cols-1 gap-3 sm:col-span-2 sm:grid-cols-2">
                  <ISODatePicker
                    label="Erworben am"
                    id={`stammdaten-acquired-${index}`}
                    value={row.acquiredOn}
                    onChange={(value) => setRow(index, { acquiredOn: value })}
                  />
                  <ISODatePicker
                    label="Gültig bis"
                    id={`stammdaten-expires-${index}`}
                    value={row.expiresOn}
                    onChange={(value) => setRow(index, { expiresOn: value })}
                  />
                </div>
              </div>
            </div>
          ))}
        </div>
        <Button
          type="button"
          variant="ghost"
          size="compact"
          onClick={() =>
            setRows((prev) => [
              ...prev,
              { name: "", acquiredOn: "", expiresOn: "" },
            ])
          }
        >
          <Plus className="mr-1 h-4 w-4" aria-hidden="true" />
          Qualifikation hinzufügen
        </Button>
        <NoteField value={note} onChange={setNote} />
        {error && <p className="text-sm text-[#FF3130]">{error}</p>}
        <ModalFooter
          onClose={onClose}
          saving={saving}
          disabled={!valid}
          onSave={() =>
            void run("stammdaten_qualifikationen_save_failed", () =>
              staffStammdatenService.updateQualifikationen(
                staffId,
                rows.map((row) => ({
                  id: null,
                  name: row.name.trim(),
                  acquiredOn: emptyToNull(row.acquiredOn),
                  expiresOn: emptyToNull(row.expiresOn),
                })),
                note.trim(),
              ),
            )
          }
        />
      </div>
    </Modal>
  );
}

// --- Bank & Steuer ----------------------------------------------------------

export function FinancialEditModal({
  staffId,
  onClose,
  onSaved,
}: {
  readonly staffId: string;
  readonly onClose: () => void;
  readonly onSaved: () => void;
}) {
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [iban, setIban] = useState("");
  const [taxId, setTaxId] = useState("");
  const [socialSecurityNumber, setSocialSecurityNumber] = useState("");
  const [note, setNote] = useState("");
  const { saving, error, run } = useSectionSave(onSaved);

  // Editing needs the plaintext values; the reveal is access-logged
  // server-side, so opening this modal leaves the same trace as the
  // "Anzeigen" toggle.
  useEffect(() => {
    let cancelled = false;
    staffStammdatenService
      .revealFinancial(staffId)
      .then((plain) => {
        if (cancelled) return;
        setIban(plain.iban ?? "");
        setTaxId(plain.taxId ?? "");
        setSocialSecurityNumber(plain.socialSecurityNumber ?? "");
        setLoading(false);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        logger.error("stammdaten_financial_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        setLoadError("Die Bank- und Steuerdaten konnten nicht geladen werden.");
        setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [staffId]);

  return (
    <Modal isOpen onClose={onClose} title="Bank & Steuer bearbeiten">
      {loading ? (
        <Loading fullPage={false} />
      ) : loadError ? (
        <p className="text-sm text-[#FF3130]">{loadError}</p>
      ) : (
        <div className="space-y-4">
          <Input
            controlSize="compact"
            label="IBAN"
            name="stammdaten-iban"
            value={iban}
            onChange={(e) => setIban(e.target.value)}
            placeholder="DE00 0000 0000 0000 0000 00"
            autoComplete="off"
          />
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <Input
              controlSize="compact"
              label="Steuer-ID"
              name="stammdaten-tax-id"
              value={taxId}
              onChange={(e) => setTaxId(e.target.value)}
              placeholder="11 Ziffern"
              inputMode="numeric"
              autoComplete="off"
            />
            <Input
              controlSize="compact"
              label="SV-Nummer"
              name="stammdaten-sv-number"
              value={socialSecurityNumber}
              onChange={(e) => setSocialSecurityNumber(e.target.value)}
              placeholder="z. B. 65170839J003"
              autoComplete="off"
            />
          </div>
          <p className="text-xs text-gray-500">
            Der Abruf und jede Änderung dieser Felder werden im Audit-Log
            protokolliert. Leere Felder löschen den gespeicherten Wert.
          </p>
          <NoteField value={note} onChange={setNote} />
          {error && <p className="text-sm text-[#FF3130]">{error}</p>}
          <ModalFooter
            onClose={onClose}
            saving={saving}
            onSave={() =>
              void run("stammdaten_financial_save_failed", () =>
                staffStammdatenService.updateFinancial(
                  staffId,
                  {
                    iban: emptyToNull(iban),
                    taxId: emptyToNull(taxId),
                    socialSecurityNumber: emptyToNull(socialSecurityNumber),
                  },
                  note.trim(),
                ),
              )
            }
          />
        </div>
      )}
    </Modal>
  );
}
