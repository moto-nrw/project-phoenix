"use client";

import { Plus, Trash2 } from "lucide-react";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { CustomSelect } from "~/components/ui/custom-select";
import { ISODateInput, ISODatePicker } from "~/components/ui/date-picker";
import type {
  StaffStammdaten,
  StammdatenArbeitsvertrag,
  StammdatenGender,
  StammdatenKontakt,
  StammdatenPerson,
  StammdatenQualifikation,
} from "~/lib/staff-api";

// Feldgruppen der Stammdaten (#1423). Sie sind reine Eingabeflächen ohne
// eigenen Speichern-Knopf: der Reiter hat EINEN Bearbeiten-Zustand mit EINEM
// Speichern (Bauart 2, Regeln 3 und 4). Vorher trug jede Gruppe ihr eigenes
// Modal — fünf Dialoge für einen Datensatz.

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

export function emptyToNull(value: string): string | null {
  const trimmed = value.trim();
  return trimmed === "" ? null : trimmed;
}

export interface QualificationRow {
  name: string;
  acquiredOn: string;
  expiresOn: string;
}

export interface FinancialDraft {
  iban: string;
  taxId: string;
  socialSecurityNumber: string;
}

/** Alle Felder des Reiters als Zeichenketten — ein Entwurf, ein Speichern. */
export interface StammdatenDraft {
  firstName: string;
  lastName: string;
  birthday: string;
  gender: string;
  street: string;
  postalCode: string;
  city: string;
  phone: string;
  email: string;
  emergencyName: string;
  emergencyPhone: string;
  entryDate: string;
  contractEnd: string;
  probationEnd: string;
  weeklyHours: string;
  employmentType: string;
  qualifikationen: QualificationRow[];
  personnelNumber: string;
  /** `null`, solange die Klartextwerte nicht angefordert wurden. */
  financial: FinancialDraft | null;
}

export function buildDraft(
  stammdaten: StaffStammdaten | undefined,
  personnelNumber: string | null | undefined,
): StammdatenDraft {
  const person: Partial<StammdatenPerson> = stammdaten?.person ?? {};
  const kontakt: Partial<StammdatenKontakt> = stammdaten?.kontakt ?? {};
  const vertrag: Partial<StammdatenArbeitsvertrag> =
    stammdaten?.arbeitsvertrag ?? {};
  return {
    firstName: person.firstName ?? "",
    lastName: person.lastName ?? "",
    birthday: person.birthday ?? "",
    gender: person.gender ?? "",
    street: kontakt.addressStreet ?? "",
    postalCode: kontakt.addressPostalCode ?? "",
    city: kontakt.addressCity ?? "",
    phone: kontakt.phone ?? "",
    email: kontakt.email ?? "",
    emergencyName: kontakt.emergencyContactName ?? "",
    emergencyPhone: kontakt.emergencyContactPhone ?? "",
    entryDate: vertrag.entryDate ?? "",
    contractEnd: vertrag.contractEndDate ?? "",
    probationEnd: vertrag.probationEndDate ?? "",
    weeklyHours: vertrag.weeklyHours != null ? String(vertrag.weeklyHours) : "",
    employmentType: vertrag.employmentType ?? "",
    qualifikationen: (stammdaten?.qualifikationen ?? []).map((q) => ({
      name: q.name,
      acquiredOn: q.acquiredOn ?? "",
      expiresOn: q.expiresOn ?? "",
    })),
    personnelNumber: personnelNumber ?? "",
    financial: null,
  };
}

/** Deutsches Dezimalkomma („29,5") zählt wie der Punkt. */
export function parseWeeklyHours(value: string): number | null {
  return value.trim() === "" ? null : Number(value.trim().replace(",", "."));
}

export function weeklyHoursValid(value: string): boolean {
  const hours = parseWeeklyHours(value);
  return hours === null || (!Number.isNaN(hours) && hours >= 0 && hours <= 80);
}

export function personnelNumberValid(value: string): boolean {
  const trimmed = value.trim();
  return trimmed === "" || /^\d{1,9}$/.test(trimmed);
}

export function toPersonPayload(draft: StammdatenDraft) {
  return {
    firstName: draft.firstName.trim(),
    lastName: draft.lastName.trim(),
    birthday: emptyToNull(draft.birthday),
    gender: (draft.gender || null) as StammdatenGender | null,
  };
}

export function toKontaktPayload(draft: StammdatenDraft) {
  return {
    addressStreet: emptyToNull(draft.street),
    addressPostalCode: emptyToNull(draft.postalCode),
    addressCity: emptyToNull(draft.city),
    phone: emptyToNull(draft.phone),
    email: emptyToNull(draft.email),
    emergencyContactName: emptyToNull(draft.emergencyName),
    emergencyContactPhone: emptyToNull(draft.emergencyPhone),
  };
}

export function toArbeitsvertragPayload(draft: StammdatenDraft) {
  const hours = parseWeeklyHours(draft.weeklyHours);
  return {
    entryDate: emptyToNull(draft.entryDate),
    contractEndDate: emptyToNull(draft.contractEnd),
    probationEndDate: emptyToNull(draft.probationEnd),
    weeklyHours: hours === null ? null : Number(hours.toFixed(2)),
    employmentType: draft.employmentType || null,
  };
}

export function toQualifikationenPayload(
  draft: StammdatenDraft,
): StammdatenQualifikation[] {
  return draft.qualifikationen.map((row) => ({
    id: null,
    name: row.name.trim(),
    acquiredOn: emptyToNull(row.acquiredOn),
    expiresOn: emptyToNull(row.expiresOn),
  }));
}

type Patch = (patch: Partial<StammdatenDraft>) => void;

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
        onChange={onChange}
        options={[...options]}
      />
    </div>
  );
}

export function PersonFields({
  draft,
  onChange,
  berlinToday,
  onBirthdayValidityChange,
}: {
  readonly draft: StammdatenDraft;
  readonly onChange: Patch;
  readonly berlinToday: string;
  readonly onBirthdayValidityChange: (valid: boolean) => void;
}) {
  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
      <Input
        controlSize="compact"
        label="Vorname"
        name="stammdaten-first-name"
        value={draft.firstName}
        onChange={(e) => onChange({ firstName: e.target.value })}
      />
      <Input
        controlSize="compact"
        label="Nachname"
        name="stammdaten-last-name"
        value={draft.lastName}
        onChange={(e) => onChange({ lastName: e.target.value })}
      />
      <ISODateInput
        label="Geburtsdatum"
        id="stammdaten-birthday"
        value={draft.birthday}
        onChange={(value) => onChange({ birthday: value })}
        onValidityChange={onBirthdayValidityChange}
        max={berlinToday}
        maxDateError="Das Geburtsdatum darf nicht in der Zukunft liegen."
      />
      <SelectField
        id="stammdaten-gender"
        label="Geschlecht"
        value={draft.gender}
        options={genderOptions}
        onChange={(value) => onChange({ gender: value })}
      />
    </div>
  );
}

export function KontaktFields({
  draft,
  onChange,
}: {
  readonly draft: StammdatenDraft;
  readonly onChange: Patch;
}) {
  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
      <div className="sm:col-span-2">
        <Input
          controlSize="compact"
          label="Straße und Hausnummer"
          name="stammdaten-street"
          value={draft.street}
          onChange={(e) => onChange({ street: e.target.value })}
        />
      </div>
      <Input
        controlSize="compact"
        label="PLZ"
        name="stammdaten-postal-code"
        value={draft.postalCode}
        onChange={(e) => onChange({ postalCode: e.target.value })}
        inputMode="numeric"
      />
      <Input
        controlSize="compact"
        label="Ort"
        name="stammdaten-city"
        value={draft.city}
        onChange={(e) => onChange({ city: e.target.value })}
      />
      <Input
        controlSize="compact"
        label="Telefon"
        name="stammdaten-phone"
        value={draft.phone}
        onChange={(e) => onChange({ phone: e.target.value })}
        inputMode="tel"
      />
      <Input
        controlSize="compact"
        label="E-Mail"
        name="stammdaten-email"
        value={draft.email}
        onChange={(e) => onChange({ email: e.target.value })}
        inputMode="email"
      />
      <Input
        controlSize="compact"
        label="Notfallkontakt (Name)"
        name="stammdaten-emergency-name"
        value={draft.emergencyName}
        onChange={(e) => onChange({ emergencyName: e.target.value })}
      />
      <Input
        controlSize="compact"
        label="Notfallkontakt (Telefon)"
        name="stammdaten-emergency-phone"
        value={draft.emergencyPhone}
        onChange={(e) => onChange({ emergencyPhone: e.target.value })}
        inputMode="tel"
      />
    </div>
  );
}

export function ArbeitsvertragFields({
  draft,
  onChange,
}: {
  readonly draft: StammdatenDraft;
  readonly onChange: Patch;
}) {
  const hoursValid = weeklyHoursValid(draft.weeklyHours);
  return (
    <div className="space-y-4">
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <ISODatePicker
          label="Eintrittsdatum"
          id="stammdaten-entry-date"
          value={draft.entryDate}
          onChange={(value) => onChange({ entryDate: value })}
        />
        <SelectField
          id="stammdaten-employment"
          label="Beschäftigungstyp"
          value={draft.employmentType}
          options={employmentOptions}
          onChange={(value) => onChange({ employmentType: value })}
        />
        <ISODatePicker
          label="Befristet bis"
          id="stammdaten-contract-end"
          value={draft.contractEnd}
          onChange={(value) => onChange({ contractEnd: value })}
        />
        <ISODatePicker
          label="Probezeit bis"
          id="stammdaten-probation-end"
          value={draft.probationEnd}
          onChange={(value) => onChange({ probationEnd: value })}
        />
        <Input
          controlSize="compact"
          label="Wochenstunden lt. Vertrag"
          name="stammdaten-weekly-hours"
          value={draft.weeklyHours}
          onChange={(e) => onChange({ weeklyHours: e.target.value })}
          inputMode="decimal"
          placeholder="z. B. 29,5"
          error={hoursValid ? undefined : "Bitte einen Wert zwischen 0 und 80."}
        />
      </div>
      <p className="text-xs text-gray-500">
        Ein leeres „Befristet bis“ bedeutet unbefristet. Die Wochenstunden
        dokumentieren den Vertrag; das Arbeitszeitmodell für die Soll-Berechnung
        wird im Reiter Arbeitszeitmodell gepflegt.
      </p>
    </div>
  );
}

export function QualifikationenFields({
  draft,
  onChange,
}: {
  readonly draft: StammdatenDraft;
  readonly onChange: Patch;
}) {
  const rows = draft.qualifikationen;
  const setRow = (index: number, patch: Partial<QualificationRow>) => {
    onChange({
      qualifikationen: rows.map((row, i) =>
        i === index ? { ...row, ...patch } : row,
      ),
    });
  };

  return (
    <div className="space-y-4">
      {rows.length === 0 && (
        <p className="text-sm text-gray-500">
          Keine Qualifikationen hinterlegt.
        </p>
      )}
      <div className="space-y-3">
        {rows.map((row, index) => (
          // Die Zeilen sind positionsgebunden (die Liste wird komplett
          // ersetzt); der Index ist die Identität.
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
                    onChange({
                      qualifikationen: rows.filter((_, i) => i !== index),
                    })
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
          onChange({
            qualifikationen: [
              ...rows,
              { name: "", acquiredOn: "", expiresOn: "" },
            ],
          })
        }
      >
        <Plus className="mr-1 h-4 w-4" aria-hidden="true" />
        Qualifikation hinzufügen
      </Button>
    </div>
  );
}

export function PersonnelNumberFields({
  draft,
  onChange,
}: {
  readonly draft: StammdatenDraft;
  readonly onChange: Patch;
}) {
  const valid = personnelNumberValid(draft.personnelNumber);
  return (
    <div>
      <Input
        id="personnel-number"
        label="Personalnummer"
        controlSize="compact"
        value={draft.personnelNumber}
        onChange={(e) => onChange({ personnelNumber: e.target.value })}
        placeholder="z. B. 1023"
        inputMode="numeric"
        error={valid ? undefined : "Nur Ziffern, höchstens 9 Stellen."}
      />
      <p className="mt-1 text-xs text-gray-500">
        Nur Ziffern. Leer lassen, um die Nummer zu entfernen. Die Nummer muss
        der Personalnummer im Lohnsystem entsprechen und ist pro Schule
        eindeutig.
      </p>
    </div>
  );
}

export function FinancialFields({
  values,
  onChange,
}: {
  readonly values: FinancialDraft;
  readonly onChange: (patch: Partial<FinancialDraft>) => void;
}) {
  return (
    <div className="space-y-4">
      <Input
        controlSize="compact"
        label="IBAN"
        name="stammdaten-iban"
        value={values.iban}
        onChange={(e) => onChange({ iban: e.target.value })}
        placeholder="DE00 0000 0000 0000 0000 00"
        autoComplete="off"
      />
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <Input
          controlSize="compact"
          label="Steuer-ID"
          name="stammdaten-tax-id"
          value={values.taxId}
          onChange={(e) => onChange({ taxId: e.target.value })}
          placeholder="11 Ziffern"
          inputMode="numeric"
          autoComplete="off"
        />
        <Input
          controlSize="compact"
          label="SV-Nummer"
          name="stammdaten-sv-number"
          value={values.socialSecurityNumber}
          onChange={(e) => onChange({ socialSecurityNumber: e.target.value })}
          placeholder="z. B. 65170839J003"
          autoComplete="off"
        />
      </div>
      <p className="text-xs text-gray-500">
        Der Abruf und jede Änderung dieser Felder werden im Audit-Log
        protokolliert. Leere Felder löschen den gespeicherten Wert.
      </p>
    </div>
  );
}
