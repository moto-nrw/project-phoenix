"use client";

import { useState, useEffect, useRef } from "react";
import { X, Plus, Star, Trash2 } from "lucide-react";
import { Modal } from "~/components/ui/modal";
import type {
  GuardianFormData,
  GuardianWithRelationship,
  PhoneType,
} from "@/lib/guardian-helpers";
import { RELATIONSHIP_TYPES, PHONE_TYPE_LABELS } from "@/lib/guardian-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "GuardianForm" });

export interface RelationshipFormData {
  readonly relationshipType: string;
  readonly isPrimary: boolean;
  readonly isEmergencyContact: boolean;
  readonly canPickup: boolean;
  readonly pickupNotes?: string;
  readonly emergencyPriority: number;
}

// Phone entry type for the form
// Exported for testing
export interface PhoneEntry {
  id: string;
  phoneNumber: string;
  phoneType: PhoneType;
  label: string;
  isPrimary: boolean;
}

// Entry type for multi-guardian form
// Exported for testing
export interface GuardianEntry {
  id: string; // Unique ID for React key
  firstName: string;
  lastName: string;
  email: string;
  phoneNumbers: PhoneEntry[];
  relationshipType: string;
  isEmergencyContact: boolean;
  // Relationship flags (preserved in edit mode)
  isPrimary: boolean;
  canPickup: boolean;
  emergencyPriority: number;
}

// Create empty phone entry
function createEmptyPhone(isPrimary = false): PhoneEntry {
  return {
    id: crypto.randomUUID(),
    phoneNumber: "",
    phoneType: "mobile",
    label: "",
    isPrimary,
  };
}

// Create empty guardian entry
// Exported for testing
export function createEmptyEntry(): GuardianEntry {
  return {
    id: crypto.randomUUID(),
    firstName: "",
    lastName: "",
    email: "",
    phoneNumbers: [createEmptyPhone(true)], // Start with one primary phone
    relationshipType: "parent",
    isEmergencyContact: false,
    isPrimary: false,
    canPickup: true,
    emergencyPriority: 1,
  };
}

// Helper: Update a phone in an entry
function updatePhoneInEntry(
  entry: GuardianEntry,
  phoneId: string,
  field: keyof PhoneEntry,
  value: string | boolean,
): GuardianEntry {
  return {
    ...entry,
    phoneNumbers: entry.phoneNumbers.map((phone) =>
      phone.id === phoneId ? { ...phone, [field]: value } : phone,
    ),
  };
}

// Helper: Set primary phone in an entry
function setPrimaryPhoneInEntry(
  entry: GuardianEntry,
  phoneId: string,
): GuardianEntry {
  return {
    ...entry,
    phoneNumbers: entry.phoneNumbers.map((phone) => ({
      ...phone,
      isPrimary: phone.id === phoneId,
    })),
  };
}

// Helper: Add a phone to an entry
function addPhoneToEntry(entry: GuardianEntry): GuardianEntry {
  return {
    ...entry,
    phoneNumbers: [...entry.phoneNumbers, createEmptyPhone(false)],
  };
}

// Helper: Remove a phone from an entry
function removePhoneFromEntry(
  entry: GuardianEntry,
  phoneId: string,
): GuardianEntry {
  const newPhones = entry.phoneNumbers.filter((p) => p.id !== phoneId);
  // If we removed the primary, make the first one primary
  const hasPrimary = newPhones.some((p) => p.isPrimary);
  if (!hasPrimary && newPhones.length > 0) {
    newPhones[0] = { ...newPhones[0]!, isPrimary: true };
  }
  return {
    ...entry,
    phoneNumbers: newPhones,
  };
}

// Convert GuardianWithRelationship to GuardianEntry
// Exported for testing
export function toEntry(data: GuardianWithRelationship): GuardianEntry {
  // Convert phoneNumbers from Guardian to PhoneEntry format
  let phoneNumbers: PhoneEntry[] = [];

  if (data.phoneNumbers && data.phoneNumbers.length > 0) {
    phoneNumbers = data.phoneNumbers.map((p) => ({
      id: p.id,
      phoneNumber: p.phoneNumber,
      phoneType: p.phoneType,
      label: p.label ?? "",
      isPrimary: p.isPrimary,
    }));
  }

  // Ensure at least one phone entry exists for form
  if (phoneNumbers.length === 0) {
    phoneNumbers.push(createEmptyPhone(true));
  }

  return {
    id: data.id,
    firstName: data.firstName ?? "",
    lastName: data.lastName ?? "",
    email: data.email ?? "",
    phoneNumbers,
    relationshipType: data.relationshipType ?? "parent",
    isEmergencyContact: data.isEmergencyContact ?? false,
    // Preserve relationship flags for edit mode
    isPrimary: data.isPrimary ?? false,
    canPickup: data.canPickup ?? true,
    emergencyPriority: data.emergencyPriority ?? 1,
  };
}

interface GuardianFormModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onSubmit: (
    guardians: Array<{
      id: string;
      guardianData: GuardianFormData;
      relationshipData: RelationshipFormData;
      phoneNumbers: Array<{
        id?: string; // Phone ID (for edit mode)
        phoneNumber: string;
        phoneType: PhoneType;
        label?: string;
        isPrimary: boolean;
      }>;
    }>,
    onEntryCreated?: (entryId: string) => void,
  ) => Promise<void>;
  readonly onDelete?: () => void;
  readonly initialData?: GuardianWithRelationship;
  readonly mode: "create" | "edit";
}

export default function GuardianFormModal({
  isOpen,
  onClose,
  onSubmit,
  onDelete,
  initialData,
  mode,
}: GuardianFormModalProps) {
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [entries, setEntries] = useState<GuardianEntry[]>([createEmptyEntry()]);
  const [newEntryId, setNewEntryId] = useState<string | null>(null);
  const entryRefs = useRef<Map<string, HTMLDivElement>>(new Map());

  // Reset entries when modal opens/closes or initialData changes
  useEffect(() => {
    if (isOpen) {
      if (initialData) {
        setEntries([toEntry(initialData)]);
      } else {
        setEntries([createEmptyEntry()]);
      }
      setError(null);
      setNewEntryId(null);
      entryRefs.current.clear();
    }
  }, [isOpen, initialData]);

  // Scroll to newly added entry
  useEffect(() => {
    if (newEntryId) {
      const element = entryRefs.current.get(newEntryId);
      if (element) {
        element.scrollIntoView({ behavior: "smooth", block: "start" });
      }
      setNewEntryId(null);
    }
  }, [newEntryId, entries]);

  // Update a single entry field
  const updateEntry = (
    id: string,
    field: keyof Omit<GuardianEntry, "phoneNumbers">,
    value: string | boolean,
  ) => {
    setEntries((prev) =>
      prev.map((entry) =>
        entry.id === id ? { ...entry, [field]: value } : entry,
      ),
    );
  };

  // Update phone number in an entry
  const updatePhone = (
    entryId: string,
    phoneId: string,
    field: keyof PhoneEntry,
    value: string | boolean,
  ) => {
    setEntries((prev) =>
      prev.map((entry) =>
        entry.id === entryId
          ? updatePhoneInEntry(entry, phoneId, field, value)
          : entry,
      ),
    );
  };

  // Set phone as primary (unset others)
  const setPhonePrimary = (entryId: string, phoneId: string) => {
    setEntries((prev) =>
      prev.map((entry) =>
        entry.id === entryId ? setPrimaryPhoneInEntry(entry, phoneId) : entry,
      ),
    );
  };

  // Add phone number to entry
  const addPhone = (entryId: string) => {
    setEntries((prev) =>
      prev.map((entry) =>
        entry.id === entryId ? addPhoneToEntry(entry) : entry,
      ),
    );
  };

  // Remove phone number from entry
  const removePhone = (entryId: string, phoneId: string) => {
    setEntries((prev) =>
      prev.map((entry) =>
        entry.id === entryId ? removePhoneFromEntry(entry, phoneId) : entry,
      ),
    );
  };

  // Add new empty entry and scroll to it
  const addEntry = () => {
    const newEntry = createEmptyEntry();
    setEntries((prev) => [...prev, newEntry]);
    setNewEntryId(newEntry.id);
  };

  // Remove entry by id (used by UI and partial success callback)
  const removeEntry = (id: string) => {
    setEntries((prev) => prev.filter((entry) => entry.id !== id));
  };

  // Validate all entries
  const validateEntries = (): string | null => {
    for (let i = 0; i < entries.length; i++) {
      const entry = entries[i];
      if (!entry) continue;

      const label = entries.length > 1 ? ` (Person ${i + 1})` : "";

      if (!entry.firstName.trim() || !entry.lastName.trim()) {
        return `Vorname und Nachname sind erforderlich${label}`;
      }

      // Check for at least one contact method
      const hasEmail = entry.email.trim() !== "";
      const hasPhone = entry.phoneNumbers.some(
        (p) => p.phoneNumber.trim() !== "",
      );

      if (!hasEmail && !hasPhone) {
        return `Mindestens eine Kontaktmöglichkeit ist erforderlich${label}`;
      }
    }
    return null;
  };

  // Build submit data from entries
  const buildSubmitData = (): Array<{
    id: string;
    guardianData: GuardianFormData;
    relationshipData: RelationshipFormData;
    phoneNumbers: Array<{
      id?: string;
      phoneNumber: string;
      phoneType: PhoneType;
      label?: string;
      isPrimary: boolean;
    }>;
  }> => {
    return entries.map((entry) => ({
      id: entry.id,
      guardianData: {
        firstName: entry.firstName.trim(),
        lastName: entry.lastName.trim(),
        email: entry.email.trim() || undefined,
      },
      relationshipData: {
        relationshipType: entry.relationshipType,
        isPrimary: entry.isPrimary,
        isEmergencyContact: entry.isEmergencyContact,
        canPickup: entry.canPickup,
        emergencyPriority: entry.emergencyPriority,
      },
      phoneNumbers: entry.phoneNumbers
        .filter((p) => p.phoneNumber.trim() !== "")
        .map((p) => ({
          id: p.id, // Include phone ID for edit mode
          phoneNumber: p.phoneNumber.trim(),
          phoneType: p.phoneType,
          label: p.label.trim() || undefined,
          isPrimary: p.isPrimary,
        })),
    }));
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    const validationError = validateEntries();
    if (validationError) {
      setError(validationError);
      return;
    }

    setIsLoading(true);

    try {
      const submitData = buildSubmitData();
      // Pass removeEntry callback for partial failure handling
      await onSubmit(submitData, removeEntry);
      onClose();
    } catch (err) {
      logger.error("guardian_save_failed", {
        error: err instanceof Error ? err.message : String(err),
        mode,
      });
      setError(err instanceof Error ? err.message : "Fehler beim Speichern");
    } finally {
      setIsLoading(false);
    }
  };

  const modalTitle =
    mode === "create"
      ? "Erziehungsberechtigte/n hinzufügen"
      : "Erziehungsberechtigte/n bearbeiten";

  return (
    <Modal isOpen={isOpen} onClose={onClose} title={modalTitle}>
      <form
        onSubmit={handleSubmit}
        noValidate
        className="space-y-4 md:space-y-6"
      >
        {/* Submit Error */}
        {error && (
          <div className="rounded-lg border border-red-200 bg-red-50 p-2 md:p-3">
            <p className="text-xs text-red-800 md:text-sm">{error}</p>
          </div>
        )}

        {/* Guardian Entries */}
        {entries.map((entry, index) => (
          <div
            key={entry.id}
            ref={(el) => {
              if (el) {
                entryRefs.current.set(entry.id, el);
              } else {
                entryRefs.current.delete(entry.id);
              }
            }}
            className="space-y-4"
          >
            {/* Entry Header (only show for multiple entries) */}
            {entries.length > 1 && (
              <div className="flex items-center justify-between">
                <h4 className="text-sm font-medium text-gray-700">
                  Person {index + 1}
                </h4>
                <button
                  type="button"
                  onClick={() => removeEntry(entry.id)}
                  disabled={isLoading}
                  className="flex items-center gap-1 rounded-lg px-2 py-1 text-xs text-red-600 transition-colors hover:bg-red-50 disabled:opacity-50"
                >
                  <X className="h-3 w-3" />
                  Entfernen
                </button>
              </div>
            )}

            {/* Personal Information */}
            <div className="rounded-xl border border-gray-100 bg-blue-50/30 p-3 md:p-4">
              <h3 className="mb-3 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-4 md:text-sm">
                <svg
                  className="h-3.5 w-3.5 text-blue-600 md:h-4 md:w-4"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"
                  />
                </svg>
                Persönliche Daten
              </h3>
              <div className="grid grid-cols-1 gap-3 md:grid-cols-2 md:gap-4">
                <div>
                  <label
                    htmlFor={`guardian-first-name-${entry.id}`}
                    className="mb-1 block text-xs font-medium text-gray-700"
                  >
                    Vorname <span className="text-red-500">*</span>
                  </label>
                  <input
                    id={`guardian-first-name-${entry.id}`}
                    type="text"
                    value={entry.firstName}
                    onChange={(e) =>
                      updateEntry(entry.id, "firstName", e.target.value)
                    }
                    className="block w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm transition-colors focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8]"
                    placeholder="Max"
                    required
                    disabled={isLoading}
                  />
                </div>

                <div>
                  <label
                    htmlFor={`guardian-last-name-${entry.id}`}
                    className="mb-1 block text-xs font-medium text-gray-700"
                  >
                    Nachname <span className="text-red-500">*</span>
                  </label>
                  <input
                    id={`guardian-last-name-${entry.id}`}
                    type="text"
                    value={entry.lastName}
                    onChange={(e) =>
                      updateEntry(entry.id, "lastName", e.target.value)
                    }
                    className="block w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm transition-colors focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8]"
                    placeholder="Mustermann"
                    required
                    disabled={isLoading}
                  />
                </div>

                <div>
                  <label
                    htmlFor={`guardian-relationship-type-${entry.id}`}
                    className="mb-1 block text-xs font-medium text-gray-700"
                  >
                    Beziehung zum Kind
                  </label>
                  <div className="relative">
                    <select
                      id={`guardian-relationship-type-${entry.id}`}
                      value={entry.relationshipType}
                      onChange={(e) =>
                        updateEntry(
                          entry.id,
                          "relationshipType",
                          e.target.value,
                        )
                      }
                      className="block w-full appearance-none rounded-lg border border-gray-200 bg-white px-3 py-2 pr-10 text-sm transition-colors focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8]"
                      disabled={isLoading}
                    >
                      {RELATIONSHIP_TYPES.map((type) => (
                        <option key={type.value} value={type.value}>
                          {type.label}
                        </option>
                      ))}
                    </select>
                    <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center px-3 text-gray-500">
                      <svg
                        className="h-4 w-4"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                      >
                        <path
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          strokeWidth={2}
                          d="M19 9l-7 7-7-7"
                        />
                      </svg>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            {/* Contact Information */}
            <div className="rounded-xl border border-gray-100 bg-blue-50/30 p-3 md:p-4">
              <h3 className="mb-3 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-4 md:text-sm">
                <svg
                  className="h-3.5 w-3.5 text-blue-600 md:h-4 md:w-4"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
                  />
                </svg>
                Kontaktdaten
              </h3>

              {/* Email */}
              <div className="mb-4">
                <label
                  htmlFor={`guardian-email-${entry.id}`}
                  className="mb-1 block text-xs font-medium text-gray-700"
                >
                  E-Mail
                </label>
                <input
                  id={`guardian-email-${entry.id}`}
                  type="email"
                  value={entry.email}
                  onChange={(e) =>
                    updateEntry(entry.id, "email", e.target.value)
                  }
                  className="block w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm transition-colors focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8]"
                  placeholder="max.mustermann@example.com"
                  disabled={isLoading}
                />
              </div>

              {/* Phone Numbers */}
              <div className="space-y-3">
                <span className="block text-xs font-medium text-gray-700">
                  Telefonnummern
                </span>

                {entry.phoneNumbers.map((phone, phoneIndex) => (
                  <div
                    key={phone.id}
                    className="flex flex-col gap-2 rounded-lg border border-gray-200 bg-white p-3 sm:flex-row sm:items-center"
                  >
                    {/* Phone Type Select */}
                    <div className="w-full sm:w-32">
                      <select
                        id={`phone-type-${entry.id}-${phone.id}`}
                        value={phone.phoneType}
                        onChange={(e) =>
                          updatePhone(
                            entry.id,
                            phone.id,
                            "phoneType",
                            e.target.value as PhoneType,
                          )
                        }
                        className="block w-full appearance-none rounded-lg border border-gray-200 bg-white px-2 py-1.5 text-sm transition-colors focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8]"
                        disabled={isLoading}
                        aria-label={`Telefontyp ${phoneIndex + 1}`}
                      >
                        {(Object.keys(PHONE_TYPE_LABELS) as PhoneType[]).map(
                          (type) => (
                            <option key={type} value={type}>
                              {PHONE_TYPE_LABELS[type]}
                            </option>
                          ),
                        )}
                      </select>
                    </div>

                    {/* Phone Number Input */}
                    <div className="flex-1">
                      <input
                        id={`phone-number-${entry.id}-${phone.id}`}
                        type="tel"
                        value={phone.phoneNumber}
                        onChange={(e) =>
                          updatePhone(
                            entry.id,
                            phone.id,
                            "phoneNumber",
                            e.target.value,
                          )
                        }
                        className="block w-full rounded-lg border border-gray-200 bg-white px-3 py-1.5 text-sm transition-colors focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8]"
                        placeholder="+49 170 1234567"
                        disabled={isLoading}
                        aria-label={`Telefonnummer ${phoneIndex + 1}`}
                      />
                    </div>

                    {/* Label Input (optional) */}
                    <div className="w-full sm:w-28">
                      <input
                        id={`phone-label-${entry.id}-${phone.id}`}
                        type="text"
                        value={phone.label}
                        onChange={(e) =>
                          updatePhone(
                            entry.id,
                            phone.id,
                            "label",
                            e.target.value,
                          )
                        }
                        className="block w-full rounded-lg border border-gray-200 bg-white px-2 py-1.5 text-sm transition-colors focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8]"
                        placeholder="Notiz"
                        disabled={isLoading}
                        aria-label={`Notiz für Nummer ${phoneIndex + 1}`}
                      />
                    </div>

                    {/* Action Buttons */}
                    <div className="flex items-center gap-1">
                      {/* Primary Star */}
                      <button
                        type="button"
                        onClick={() => setPhonePrimary(entry.id, phone.id)}
                        disabled={isLoading || phone.isPrimary}
                        className={`rounded p-1.5 transition-colors ${
                          phone.isPrimary
                            ? "text-yellow-500"
                            : "text-gray-300 hover:text-yellow-400"
                        }`}
                        title={
                          phone.isPrimary ? "Primär" : "Als primär markieren"
                        }
                        aria-label={
                          phone.isPrimary
                            ? "Primäre Nummer"
                            : "Als primäre Nummer markieren"
                        }
                      >
                        <Star
                          className="h-4 w-4"
                          fill={phone.isPrimary ? "currentColor" : "none"}
                        />
                      </button>

                      {/* Delete Button */}
                      {entry.phoneNumbers.length > 1 && (
                        <button
                          type="button"
                          onClick={() => removePhone(entry.id, phone.id)}
                          disabled={isLoading}
                          className="rounded p-1.5 text-gray-400 transition-colors hover:bg-red-50 hover:text-red-600"
                          title="Entfernen"
                          aria-label={`Telefonnummer ${phoneIndex + 1} entfernen`}
                        >
                          <Trash2 className="h-4 w-4" />
                        </button>
                      )}
                    </div>
                  </div>
                ))}

                {/* Add Phone Button */}
                <button
                  type="button"
                  onClick={() => addPhone(entry.id)}
                  disabled={isLoading}
                  className="flex w-full items-center justify-center gap-1.5 rounded-lg border border-dashed border-gray-300 bg-gray-50 py-2 text-xs font-medium text-gray-600 transition-colors hover:border-gray-400 hover:bg-gray-100 disabled:opacity-50"
                >
                  <Plus className="h-3.5 w-3.5" />
                  Weitere Nummer hinzufügen
                </button>
              </div>
            </div>

            {/* Emergency Contact */}
            <div className="rounded-xl border border-red-200 bg-red-50 px-4 py-2">
              <label className="group flex cursor-pointer items-center gap-3">
                <input
                  type="checkbox"
                  checked={entry.isEmergencyContact}
                  onChange={(e) =>
                    updateEntry(
                      entry.id,
                      "isEmergencyContact",
                      e.target.checked,
                    )
                  }
                  className="h-4 w-4 rounded border-gray-300 text-red-600 focus:ring-red-600"
                  disabled={isLoading}
                  aria-label="Als Notfallkontakt markieren"
                />
                <div className="flex items-center gap-2">
                  <svg
                    className="h-5 w-5 text-red-600"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                    />
                  </svg>
                  <span className="text-sm font-medium text-red-900">
                    Notfallkontakt
                  </span>
                </div>
              </label>
            </div>

            {/* Divider between entries */}
            {index < entries.length - 1 && (
              <div className="border-t border-dashed border-gray-300" />
            )}
          </div>
        ))}

        {/* Add More Button (only in create mode) */}
        {mode === "create" && (
          <button
            type="button"
            onClick={addEntry}
            disabled={isLoading}
            className="flex w-full items-center justify-center gap-2 rounded-xl border-2 border-dashed border-gray-300 bg-gray-50 py-2 text-xs font-medium text-gray-600 transition-all duration-200 hover:border-gray-400 hover:bg-gray-100 hover:text-gray-700 disabled:cursor-not-allowed disabled:opacity-50 md:text-sm"
          >
            <svg
              className="h-4 w-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M12 4v16m8-8H4"
              />
            </svg>
            Weiteren hinzufügen
          </button>
        )}

        {/* Action Buttons */}
        <div className="sticky bottom-0 -mx-4 mt-4 -mb-4 flex items-center justify-between gap-2 border-t border-gray-100 bg-white/95 px-4 py-3 backdrop-blur-sm md:-mx-6 md:mt-6 md:-mb-6 md:gap-3 md:px-6 md:py-4">
          {/* Delete button (only in edit mode) */}
          {mode === "edit" && onDelete ? (
            <button
              type="button"
              onClick={onDelete}
              disabled={isLoading}
              className="rounded-lg px-3 py-2 text-xs font-medium text-red-600 transition-colors hover:bg-red-50 disabled:cursor-not-allowed disabled:opacity-50 md:text-sm"
            >
              Entfernen
            </button>
          ) : (
            <div />
          )}
          <div className="flex gap-2 md:gap-3">
            <button
              type="button"
              onClick={onClose}
              disabled={isLoading}
              className="rounded-lg border border-gray-300 px-3 py-2 text-xs font-medium text-gray-700 transition-all duration-200 hover:border-gray-400 hover:bg-gray-50 hover:shadow-md active:scale-100 disabled:cursor-not-allowed disabled:opacity-50 md:px-4 md:text-sm md:hover:scale-105"
            >
              Abbrechen
            </button>
            <button
              type="submit"
              disabled={isLoading}
              className="flex-1 rounded-lg bg-gray-900 px-3 py-2 text-xs font-medium text-white transition-all duration-200 hover:bg-gray-700 hover:shadow-lg active:scale-100 disabled:cursor-not-allowed disabled:opacity-50 md:px-4 md:text-sm md:hover:scale-105"
            >
              {(() => {
                if (isLoading) {
                  return (
                    <span className="flex items-center justify-center gap-2">
                      <svg
                        className="h-4 w-4 animate-spin text-white"
                        fill="none"
                        viewBox="0 0 24 24"
                      >
                        <circle
                          className="opacity-25"
                          cx="12"
                          cy="12"
                          r="10"
                          stroke="currentColor"
                          strokeWidth="4"
                        />
                        <path
                          className="opacity-75"
                          fill="currentColor"
                          d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                        />
                      </svg>
                      Wird gespeichert...
                    </span>
                  );
                }
                if (mode === "create") {
                  return entries.length > 1
                    ? `${entries.length} Personen hinzufügen`
                    : "Hinzufügen";
                }
                return "Speichern";
              })()}
            </button>
          </div>
        </div>
      </form>
    </Modal>
  );
}
