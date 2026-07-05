"use client";

import { useState, useEffect } from "react";
import { FormModal } from "~/components/ui/form-modal";
import { useToast } from "~/contexts/ToastContext";
import type { ExtendedStudent } from "~/lib/hooks/use-student-data";
import { DepartureSection } from "./student-form-fields";
import {
  busDaysHaveAny,
  pickupDaysHaveAny,
  departureDaysFromLegacy,
  allowedDepartureModesFromDeparture,
  allowedDepartureToBusDays,
  allowedDepartureToDepartureDays,
  allowedDepartureToPickupDays,
  normalizeAllowedDepartureModes,
  allowedModesIncludeAccompanied,
} from "~/lib/student-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "PersonalInfoFormModal" });

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
    const allowedDepartureModes = normalizeAllowedDepartureModes(
      editedStudent.allowed_departure_modes ??
        allowedDepartureModesFromDeparture(
          editedStudent.departure_days ??
            departureDaysFromLegacy(
              editedStudent.bus_days,
              editedStudent.pickup_days,
            ),
        ),
    );
    // "Mit anderem Kind" needs a note saying with whom (#1694).
    if (
      allowedModesIncludeAccompanied(allowedDepartureModes) &&
      !editedStudent.departure_companion_note?.trim()
    ) {
      toast.error("Bitte angeben, mit welchem Kind das Kind nach Hause geht");
      return;
    }
    setIsSaving(true);
    try {
      const busDays = allowedDepartureToBusDays(allowedDepartureModes);
      const pickupDays = allowedDepartureToPickupDays(allowedDepartureModes);
      await onSave({
        ...editedStudent,
        allowed_departure_modes: allowedDepartureModes,
        departure_days: allowedDepartureToDepartureDays(allowedDepartureModes),
        bus_days: busDays,
        buskind: busDaysHaveAny(busDays),
        pickup_days: pickupDays,
        pickup_status: pickupDaysHaveAny(pickupDays)
          ? "Wird abgeholt"
          : "Geht alleine nach Hause",
      });
      onClose();
    } catch (err) {
      logger.error("failed to save personal information", {
        error: err instanceof Error ? err.message : String(err),
      });
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
        <TextInput
          id="modal-student-address-street"
          label="Straße und Hausnummer"
          value={editedStudent.address_street ?? ""}
          onChange={(value) => updateField("address_street", value)}
        />
        <TextInput
          id="modal-student-address-postal-code"
          label="PLZ"
          value={editedStudent.address_postal_code ?? ""}
          onChange={(value) => updateField("address_postal_code", value)}
        />
        <TextInput
          id="modal-student-address-city"
          label="Ort"
          value={editedStudent.address_city ?? ""}
          onChange={(value) => updateField("address_city", value)}
        />
        <DepartureSection
          days={
            editedStudent.allowed_departure_modes ??
            allowedDepartureModesFromDeparture(
              editedStudent.departure_days ??
                departureDaysFromLegacy(
                  editedStudent.bus_days,
                  editedStudent.pickup_days,
                ),
            )
          }
          onChange={(value) => {
            const allowed = normalizeAllowedDepartureModes(value);
            const departure = allowedDepartureToDepartureDays(allowed);
            const busDays = allowedDepartureToBusDays(allowed);
            const pickupDays = allowedDepartureToPickupDays(allowed);
            setEditedStudent((prev) => ({
              ...prev,
              allowed_departure_modes: allowed,
              departure_days: departure,
              bus_days: busDays,
              buskind: busDaysHaveAny(busDays),
              pickup_days: pickupDays,
              pickup_status: pickupDaysHaveAny(pickupDays)
                ? "Wird abgeholt"
                : "Geht alleine nach Hause",
            }));
          }}
          companionNote={editedStudent.departure_companion_note}
          onCompanionNoteChange={(value) =>
            updateField("departure_companion_note", value)
          }
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
        maxLength={255}
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
        maxLength={2000}
      />
    </div>
  );
}
