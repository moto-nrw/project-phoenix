"use client";

import type { ExtendedStudent } from "~/lib/hooks/use-student-data";
import {
  PersonIcon,
  ChevronDownIcon,
  WarningIcon,
} from "~/components/students/student-detail-components";

interface PersonalInfoEditFormProps {
  editedStudent: ExtendedStudent;
  onStudentChange: (student: ExtendedStudent) => void;
  onSave: () => Promise<void>;
  onCancel: () => void;
}

export function PersonalInfoEditForm({
  editedStudent,
  onStudentChange,
  onSave,
  onCancel,
}: Readonly<PersonalInfoEditFormProps>) {
  const updateField = <K extends keyof ExtendedStudent>(
    field: K,
    value: ExtendedStudent[K],
  ) => {
    onStudentChange({ ...editedStudent, [field]: value });
  };

  return (
    <div className="rounded-2xl border border-gray-100 bg-white/50 p-4 backdrop-blur-sm sm:p-6">
      <div className="mb-4 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg bg-[#83CD2D]/10 text-[#83CD2D] sm:h-10 sm:w-10">
            <PersonIcon />
          </div>
          <h2 className="text-base font-semibold text-gray-900 sm:text-lg">
            Persönliche Informationen
          </h2>
        </div>
      </div>
      <div className="space-y-3">
        <TextInput
          id="student-first-name"
          label="Vorname"
          value={editedStudent.first_name ?? ""}
          onChange={(value) => updateField("first_name", value)}
        />
        <TextInput
          id="student-last-name"
          label="Nachname"
          value={editedStudent.second_name ?? ""}
          onChange={(value) => updateField("second_name", value)}
        />
        <TextInput
          id="student-school-class"
          label="Klasse"
          value={editedStudent.school_class}
          onChange={(value) => updateField("school_class", value)}
        />
        <DateInput
          id="student-birthday"
          label="Geburtsdatum"
          value={editedStudent.birthday}
          onChange={(value) => updateField("birthday", value)}
        />
        <SelectInput
          id="student-buskind"
          label="Buskind"
          value={editedStudent.buskind ? "true" : "false"}
          onChange={(value) => updateField("buskind", value === "true")}
          options={[
            { value: "false", label: "Nein" },
            { value: "true", label: "Ja" },
          ]}
        />
        <SelectInput
          id="student-pickup-status"
          label="Abholstatus"
          value={editedStudent.pickup_status ?? ""}
          onChange={(value) => updateField("pickup_status", value || undefined)}
          options={[
            { value: "", label: "Nicht gesetzt" },
            {
              value: "Geht alleine nach Hause",
              label: "Geht alleine nach Hause",
            },
            { value: "Wird abgeholt", label: "Wird abgeholt" },
          ]}
        />
        <SicknessToggle
          isSick={editedStudent.sick ?? false}
          onToggle={() => updateField("sick", !editedStudent.sick)}
        />
        <TextAreaInput
          id="student-health-info"
          label="Gesundheitsinformationen"
          value={editedStudent.health_info ?? ""}
          onChange={(value) => updateField("health_info", value)}
          placeholder="Allergien, Medikamente, wichtige medizinische Informationen"
          rows={3}
        />
        <TextAreaInput
          id="student-supervisor-notes"
          label="Betreuernotizen"
          value={editedStudent.supervisor_notes ?? ""}
          onChange={(value) => updateField("supervisor_notes", value)}
          placeholder="Notizen für Betreuer"
          rows={3}
        />
        <TextAreaInput
          id="student-extra-info"
          label="Elternnotizen"
          value={editedStudent.extra_info ?? ""}
          onChange={(value) => updateField("extra_info", value)}
          placeholder="Notizen der Eltern"
          rows={2}
        />
        <FormActions onSave={onSave} onCancel={onCancel} />
      </div>
    </div>
  );
}

// =============================================================================
// FORM INPUT COMPONENTS
// =============================================================================

interface TextInputProps {
  id: string;
  label: string;
  value: string;
  onChange: (value: string) => void;
}

function TextInput({ id, label, value, onChange }: Readonly<TextInputProps>) {
  return (
    <div>
      <label htmlFor={id} className="mb-1 block text-xs text-gray-500">
        {label}
      </label>
      <input
        id={id}
        type="text"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="w-full rounded-lg border border-gray-300 px-3 py-2.5 text-sm focus:ring-2 focus:ring-blue-500 focus:outline-none"
      />
    </div>
  );
}

interface DateInputProps {
  id: string;
  label: string;
  value?: string;
  onChange: (value: string) => void;
}

function DateInput({ id, label, value, onChange }: Readonly<DateInputProps>) {
  const displayValue = value ? value.split("T")[0] : "";

  return (
    <div>
      <label htmlFor={id} className="mb-1 block text-xs text-gray-500">
        {label}
      </label>
      <input
        id={id}
        type="date"
        value={displayValue}
        onChange={(e) => onChange(e.target.value)}
        className="w-full rounded-lg border border-gray-300 px-3 py-2.5 text-sm focus:ring-2 focus:ring-blue-500 focus:outline-none"
      />
    </div>
  );
}

interface SelectOption {
  value: string;
  label: string;
}

interface SelectInputProps {
  id: string;
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: SelectOption[];
}

function SelectInput({
  id,
  label,
  value,
  onChange,
  options,
}: Readonly<SelectInputProps>) {
  return (
    <div>
      <label htmlFor={id} className="mb-1 block text-xs text-gray-500">
        {label}
      </label>
      <div className="relative">
        <select
          id={id}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          className="w-full appearance-none rounded-lg border border-gray-300 bg-white px-3 py-2.5 pr-10 text-sm focus:ring-2 focus:ring-blue-500 focus:outline-none"
        >
          {options.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </select>
        <ChevronDownIcon className="pointer-events-none absolute top-1/2 right-3 h-4 w-4 -translate-y-1/2 text-gray-400" />
      </div>
    </div>
  );
}

interface TextAreaInputProps {
  id: string;
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  rows?: number;
}

function TextAreaInput({
  id,
  label,
  value,
  onChange,
  placeholder,
  rows = 3,
}: Readonly<TextAreaInputProps>) {
  return (
    <div>
      <label htmlFor={id} className="mb-1 block text-xs text-gray-500">
        {label}
      </label>
      <textarea
        id={id}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="min-h-[80px] w-full rounded-lg border border-gray-300 px-3 py-2.5 text-sm focus:ring-2 focus:ring-blue-500 focus:outline-none"
        rows={rows}
        placeholder={placeholder}
      />
    </div>
  );
}

// =============================================================================
// SICKNESS TOGGLE
// =============================================================================

interface SicknessToggleProps {
  isSick: boolean;
  onToggle: () => void;
}

function SicknessToggle({ isSick, onToggle }: Readonly<SicknessToggleProps>) {
  const containerClass = isSick
    ? "border-pink-200 bg-pink-50"
    : "border-gray-200 bg-gray-50";

  const iconBgClass = isSick
    ? "bg-pink-100 text-pink-600"
    : "bg-gray-200 text-gray-500";

  const textClass = isSick ? "text-pink-900" : "text-gray-700";

  const toggleClass = isSick ? "bg-pink-500" : "bg-gray-300";

  const knobTransform = isSick ? "translate-x-6" : "translate-x-1";

  return (
    <div
      className={`rounded-lg border p-4 transition-colors ${containerClass}`}
    >
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div
            className={`flex h-9 w-9 items-center justify-center rounded-lg ${iconBgClass}`}
          >
            <WarningIcon />
          </div>
          <div>
            <p className={`text-sm font-medium ${textClass}`}>
              Kind krankmelden
            </p>
            <p className="text-xs text-gray-500">
              Wird beim nächsten Check-in zurückgesetzt
            </p>
          </div>
        </div>
        <button
          type="button"
          role="switch"
          aria-checked={isSick}
          onClick={onToggle}
          className={`relative inline-flex h-7 w-12 flex-shrink-0 items-center rounded-full transition-colors duration-200 focus:outline-none focus-visible:ring-2 focus-visible:ring-pink-500 focus-visible:ring-offset-2 ${toggleClass}`}
        >
          <span
            className={`inline-block h-5 w-5 transform rounded-full bg-white shadow-sm transition-transform duration-200 ${knobTransform}`}
          />
        </button>
      </div>
    </div>
  );
}

// =============================================================================
// FORM ACTIONS
// =============================================================================

interface FormActionsProps {
  onSave: () => Promise<void>;
  onCancel: () => void;
}

function FormActions({ onSave, onCancel }: Readonly<FormActionsProps>) {
  return (
    <div className="flex flex-col-reverse gap-2 pt-4 sm:flex-row sm:justify-end">
      <button
        type="button"
        onClick={onCancel}
        className="inline-flex items-center justify-center rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 transition-all duration-200 hover:bg-gray-50 hover:shadow-lg active:scale-[0.99] sm:hover:scale-[1.01]"
      >
        Abbrechen
      </button>
      <button
        type="button"
        onClick={onSave}
        className="inline-flex items-center justify-center rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-all duration-200 hover:bg-gray-700 hover:shadow-lg active:scale-[0.99] sm:hover:scale-[1.01]"
      >
        Speichern
      </button>
    </div>
  );
}
