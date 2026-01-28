"use client";

import { useState, useEffect } from "react";
import { FormModal } from "~/components/ui/form-modal";
import { useToast } from "~/contexts/ToastContext";
import type { ExtendedStudent } from "~/lib/hooks/use-student-data";
import { ChevronDownIcon, WarningIcon } from "./student-detail-components";

interface PersonalInfoFormModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly student: ExtendedStudent;
  readonly onSave: (student: ExtendedStudent) => Promise<void>;
}

export function PersonalInfoFormModal({
  isOpen,
  onClose,
  student,
  onSave,
}: PersonalInfoFormModalProps) {
  const toast = useToast();
  const [editedStudent, setEditedStudent] = useState<ExtendedStudent>(student);
  const [isSaving, setIsSaving] = useState(false);

  // Reset form when modal opens with new student data
  useEffect(() => {
    if (isOpen) {
      setEditedStudent(student);
    }
  }, [isOpen, student]);

  const updateField = <K extends keyof ExtendedStudent>(
    field: K,
    value: ExtendedStudent[K],
  ) => {
    setEditedStudent((prev) => ({ ...prev, [field]: value }));
  };

  const handleSave = async () => {
    setIsSaving(true);
    try {
      await onSave(editedStudent);
      onClose();
    } catch (err) {
      console.error("Failed to save personal information:", err);
      toast.error("Fehler beim Speichern der persönlichen Informationen");
    } finally {
      setIsSaving(false);
    }
  };

  const handleCancel = () => {
    setEditedStudent(student);
    onClose();
  };

  return (
    <FormModal
      isOpen={isOpen}
      onClose={handleCancel}
      title="Persönliche Infos"
      size="lg"
      mobilePosition="center"
      footer={
        <>
          <button
            type="button"
            onClick={handleCancel}
            disabled={isSaving}
            className="inline-flex items-center justify-center rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 transition-all duration-200 hover:bg-gray-50 disabled:opacity-50"
          >
            Abbrechen
          </button>
          <button
            type="button"
            onClick={handleSave}
            disabled={isSaving}
            className="inline-flex items-center justify-center rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-all duration-200 hover:bg-gray-700 disabled:opacity-50"
          >
            {isSaving ? "Wird gespeichert..." : "Speichern"}
          </button>
        </>
      }
    >
      <div className="space-y-4">
        <TextInput
          id="modal-student-first-name"
          label="Vorname"
          value={editedStudent.first_name ?? ""}
          onChange={(value) => updateField("first_name", value)}
        />
        <TextInput
          id="modal-student-last-name"
          label="Nachname"
          value={editedStudent.second_name ?? ""}
          onChange={(value) => updateField("second_name", value)}
        />
        <TextInput
          id="modal-student-school-class"
          label="Klasse"
          value={editedStudent.school_class}
          onChange={(value) => updateField("school_class", value)}
        />
        <DateInput
          id="modal-student-birthday"
          label="Geburtsdatum"
          value={editedStudent.birthday}
          onChange={(value) => updateField("birthday", value)}
        />
        <SelectInput
          id="modal-student-buskind"
          label="Buskind"
          value={editedStudent.buskind ? "true" : "false"}
          onChange={(value) => updateField("buskind", value === "true")}
          options={[
            { value: "false", label: "Nein" },
            { value: "true", label: "Ja" },
          ]}
        />
        <SelectInput
          id="modal-student-pickup-status"
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
          id="modal-student-health-info"
          label="Gesundheitsinformationen"
          value={editedStudent.health_info ?? ""}
          onChange={(value) => updateField("health_info", value)}
          placeholder="Allergien, Medikamente, wichtige medizinische Informationen"
          rows={3}
        />
        <TextAreaInput
          id="modal-student-supervisor-notes"
          label="Betreuernotizen"
          value={editedStudent.supervisor_notes ?? ""}
          onChange={(value) => updateField("supervisor_notes", value)}
          placeholder="Notizen für Betreuer"
          rows={3}
        />
        <TextAreaInput
          id="modal-student-extra-info"
          label="Elternnotizen"
          value={editedStudent.extra_info ?? ""}
          onChange={(value) => updateField("extra_info", value)}
          placeholder="Notizen der Eltern"
          rows={2}
        />
      </div>
    </FormModal>
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
    ? "border-amber-200 bg-amber-50"
    : "border-gray-200 bg-gray-50";

  const iconBgClass = isSick
    ? "bg-amber-100 text-amber-600"
    : "bg-gray-200 text-gray-500";

  const textClass = isSick ? "text-amber-900" : "text-gray-700";

  const toggleClass = isSick ? "bg-amber-500" : "bg-gray-300";

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
          className={`relative inline-flex h-7 w-12 flex-shrink-0 items-center rounded-full transition-colors duration-200 focus:outline-none focus-visible:ring-2 focus-visible:ring-amber-500 focus-visible:ring-offset-2 ${toggleClass}`}
        >
          <span
            className={`inline-block h-5 w-5 transform rounded-full bg-white shadow-sm transition-transform duration-200 ${knobTransform}`}
          />
        </button>
      </div>
    </div>
  );
}
