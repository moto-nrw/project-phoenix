// The list fields of the OGS-Anmeldung (enrollment-form.tsx): phone numbers
// (`phone_list`) and further contacts (`contact_list`) a school asks for in
// its form template.

import { Plus, Trash2 } from "lucide-react";

import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { CustomSelect } from "~/components/ui/custom-select";
import { RequiredMark } from "~/components/ui/field-label";
import { Input } from "~/components/ui/input";
import { Radio } from "~/components/ui/radio";
import {
  copyStableObjectKey,
  getStableObjectKey,
} from "~/lib/stable-object-key";

import {
  asStringMap,
  type ContactEntryValue,
  type CustomFieldInputProps,
  type PhoneEntry,
} from "./enrollment-form-shared";

function asPhoneArray(v: unknown): PhoneEntry[] {
  if (!Array.isArray(v)) return [];
  return v.filter((row): row is PhoneEntry => {
    if (!row || typeof row !== "object") return false;
    const r = row as Record<string, unknown>;
    return typeof r.phone_number === "string";
  });
}

export function phoneListValueMissing(value: unknown): boolean {
  if (!Array.isArray(value) || value.length === 0) return true;
  return value.some((row) => {
    if (!row || typeof row !== "object") return true;
    const phoneNumber = (row as Partial<PhoneEntry>).phone_number;
    return typeof phoneNumber !== "string" || phoneNumber.trim() === "";
  });
}

export function contactListValueMissing(value: unknown): boolean {
  if (!Array.isArray(value) || value.length === 0) return true;
  return value.some((row) => {
    if (!row || typeof row !== "object") return true;
    const contact = row as ContactEntryValue;
    const hasName =
      typeof contact.first_name === "string" &&
      contact.first_name.trim() !== "" &&
      typeof contact.last_name === "string" &&
      contact.last_name.trim() !== "";
    const hasEmail = (contact.email ?? "").trim() !== "";
    const hasPhone =
      Array.isArray(contact.phone_numbers) &&
      contact.phone_numbers.some((phone) => phone.phone_number.trim() !== "");
    return !hasName || (!hasEmail && !hasPhone);
  });
}

export function PhoneListInput({
  field,
  inputName,
  value,
  onChange,
  error,
  tr,
}: CustomFieldInputProps) {
  const errorId = `enrollment-${inputName}-error`;
  const phones = asPhoneArray(value);
  const phoneTypeLabels = asStringMap(tr.raw("phoneTypes"));
  const update = (next: PhoneEntry[]) => onChange(next);
  const setRow = (idx: number, patch: Partial<PhoneEntry>) =>
    update(
      phones.map((phone, i) =>
        i === idx ? copyStableObjectKey(phone, { ...phone, ...patch }) : phone,
      ),
    );
  return (
    <fieldset
      className={`rounded-2xl border bg-gray-50/70 p-4 ${error ? "border-moto-red" : "border-gray-200"}`}
      aria-invalid={error ? "true" : undefined}
      aria-describedby={error ? errorId : undefined}
    >
      <legend className="px-1 text-sm font-semibold text-gray-900">
        {field.label}
        {field.required && <RequiredMark />}
      </legend>
      {field.help_text && (
        <p className="mt-1 text-xs leading-5 text-gray-500">
          {field.help_text}
        </p>
      )}
      {phones.length === 0 && (
        <p className="mt-2 text-sm text-gray-500">
          {tr("structured.nonePhone")}
        </p>
      )}
      <ul className="space-y-2">
        {phones.map((p, idx) => (
          <li
            key={getStableObjectKey(p, "phone")}
            className="moto-content-surface grid items-end gap-3 rounded-xl border p-3 shadow-sm md:grid-cols-[minmax(0,1fr)_180px_auto_auto]"
          >
            <Input
              id={`${inputName}-${idx}-number`}
              label={tr("structured.number")}
              type="tel"
              controlSize="compact"
              value={p.phone_number}
              onChange={(e) => setRow(idx, { phone_number: e.target.value })}
            />
            <label className="block" htmlFor={`${inputName}-${idx}-type`}>
              <span className="text-xs font-medium text-gray-700">
                {tr("structured.type")}
              </span>
              <CustomSelect
                id={`${inputName}-${idx}-type`}
                value={p.phone_type}
                onChange={(value) =>
                  setRow(idx, {
                    phone_type: value as PhoneEntry["phone_type"],
                  })
                }
                ariaLabel={tr("structured.type")}
                className="mt-1 border-gray-200 bg-white"
                options={(
                  [
                    "mobile",
                    "home",
                    "work",
                    "other",
                  ] as PhoneEntry["phone_type"][]
                ).map((key) => ({
                  value: key,
                  label: phoneTypeLabels[key] ?? key,
                }))}
              />
            </label>
            <StructuredToggle
              checked={Boolean(p.is_primary)}
              onChange={() =>
                update(
                  phones.map((row, i) =>
                    copyStableObjectKey(row, {
                      ...row,
                      is_primary: i === idx,
                    }),
                  ),
                )
              }
              label={tr("structured.primary")}
              name={`${inputName}-primary`}
              type="radio"
            />
            <Button
              type="button"
              variant="outline_danger"
              size="icon"
              onClick={() => update(phones.filter((_, i) => i !== idx))}
              aria-label={tr("structured.removeNumber")}
            >
              <Trash2 className="h-4 w-4" aria-hidden="true" />
            </Button>
          </li>
        ))}
      </ul>
      <Button
        type="button"
        variant="surface"
        size="md"
        onClick={() =>
          update([
            ...phones,
            { phone_number: "", phone_type: "mobile" } satisfies PhoneEntry,
          ])
        }
        className="mt-3 gap-2"
      >
        <Plus className="h-4 w-4" aria-hidden="true" />
        {tr("structured.addPhone")}
      </Button>
      <FieldErrorHint id={errorId} message={error} />
    </fieldset>
  );
}

function asContactArray(v: unknown): ContactEntryValue[] {
  if (!Array.isArray(v)) return [];
  return v.filter((row): row is ContactEntryValue => {
    if (!row || typeof row !== "object") return false;
    const r = row as Record<string, unknown>;
    return typeof r.first_name === "string" && typeof r.last_name === "string";
  });
}

function blankContact(): ContactEntryValue {
  return {
    first_name: "",
    last_name: "",
    relationship_type: "",
    phone_numbers: [],
    can_pickup: false,
    is_emergency_contact: false,
  };
}

export function ContactListInput({
  field,
  inputName,
  value,
  onChange,
  error,
  tr,
}: CustomFieldInputProps) {
  const errorId = `enrollment-${inputName}-error`;
  const contacts = asContactArray(value);
  const update = (next: ContactEntryValue[]) => onChange(next);
  const setRow = (idx: number, patch: Partial<ContactEntryValue>) =>
    update(
      contacts.map((contact, i) =>
        i === idx
          ? copyStableObjectKey(contact, { ...contact, ...patch })
          : contact,
      ),
    );
  return (
    <fieldset
      className={`rounded-2xl border p-4 shadow-sm ${error ? "border-moto-red" : "moto-content-surface"}`}
      aria-invalid={error ? "true" : undefined}
      aria-describedby={error ? errorId : undefined}
    >
      <legend className="px-1 text-sm font-semibold text-gray-900">
        {field.label}
        {field.required && <RequiredMark />}
      </legend>
      {field.help_text && (
        <p className="mt-1 text-xs leading-5 text-gray-500">
          {field.help_text}
        </p>
      )}
      {contacts.length === 0 && (
        <p className="mt-2 text-sm text-gray-500">
          {tr("structured.noneContact")}
        </p>
      )}
      <ul className="space-y-3">
        {contacts.map((c, idx) => (
          <li
            key={getStableObjectKey(c, "contact")}
            className="rounded-2xl border border-gray-200 bg-gray-50/70 p-4"
          >
            <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
              <Input
                id={`${inputName}-${idx}-first-name`}
                label={tr("fields.firstNamePlain")}
                controlSize="compact"
                value={c.first_name}
                onChange={(e) => setRow(idx, { first_name: e.target.value })}
              />
              <Input
                id={`${inputName}-${idx}-last-name`}
                label={tr("fields.lastNamePlain")}
                controlSize="compact"
                value={c.last_name}
                onChange={(e) => setRow(idx, { last_name: e.target.value })}
              />
              <Input
                id={`${inputName}-${idx}-relationship`}
                label={tr("structured.relationship")}
                controlSize="compact"
                value={c.relationship_type ?? ""}
                onChange={(e) =>
                  setRow(idx, { relationship_type: e.target.value })
                }
              />
              <Input
                id={`${inputName}-${idx}-email`}
                label={tr("structured.optionalEmail")}
                type="email"
                controlSize="compact"
                value={c.email ?? ""}
                onChange={(e) => setRow(idx, { email: e.target.value })}
              />
            </div>

            <div className="mt-3">
              <PhoneListInput
                inputName={`${inputName}_${idx}_phones`}
                field={{
                  ...field,
                  key: `${field.key}_${idx}_phones`,
                  label: tr("structured.phoneNumbers"),
                  type: "phone_list",
                  required: false,
                }}
                value={c.phone_numbers ?? []}
                onChange={(v) =>
                  setRow(idx, {
                    phone_numbers: Array.isArray(v) ? (v as PhoneEntry[]) : [],
                  })
                }
                tr={tr}
              />
            </div>

            <div className="mt-3 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
              <div className="grid gap-2 sm:grid-cols-2">
                <StructuredToggle
                  checked={Boolean(c.can_pickup)}
                  onChange={(checked) => setRow(idx, { can_pickup: checked })}
                  label={tr("structured.pickup")}
                />
                <StructuredToggle
                  checked={Boolean(c.is_emergency_contact)}
                  onChange={(checked) =>
                    setRow(idx, { is_emergency_contact: checked })
                  }
                  label={tr("structured.emergency")}
                />
              </div>
              <Button
                type="button"
                variant="outline_danger"
                size="md"
                onClick={() => update(contacts.filter((_, i) => i !== idx))}
                className="gap-2"
              >
                <Trash2 className="h-4 w-4" aria-hidden="true" />
                {tr("structured.removeContact")}
              </Button>
            </div>
          </li>
        ))}
      </ul>
      <Button
        type="button"
        variant="surface"
        size="md"
        onClick={() => update([...contacts, blankContact()])}
        className="mt-3 gap-2"
      >
        <Plus className="h-4 w-4" aria-hidden="true" />
        {tr("structured.addContact")}
      </Button>
      <FieldErrorHint id={errorId} message={error} />
    </fieldset>
  );
}

// One yes/no flag of a list entry (Hauptnummer, Abholberechtigt,
// Notfallkontakt) as a tinted tile around the kit control.
function StructuredToggle({
  checked,
  label,
  name,
  onChange,
  type = "checkbox",
}: Readonly<{
  checked: boolean;
  label: string;
  name?: string;
  onChange: (checked: boolean) => void;
  type?: "checkbox" | "radio";
}>) {
  return (
    <ChoiceTile selected={checked} tone="green" className="h-9 py-0">
      {type === "radio" ? (
        <Radio
          name={name}
          checked={checked}
          onChange={(event) => onChange(event.target.checked)}
        />
      ) : (
        <Checkbox
          name={name}
          checked={checked}
          onChange={(event) => onChange(event.target.checked)}
        />
      )}
      {label}
    </ChoiceTile>
  );
}
