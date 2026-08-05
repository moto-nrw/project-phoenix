"use client";

import { useState, useEffect, useRef } from "react";
import { X, Plus, Star, Trash2 } from "lucide-react";
import { Modal } from "~/components/ui/modal";
import { CustomSelect } from "~/components/ui/custom-select";
import { useScrollToError } from "~/lib/hooks/use-scroll-to-error";
import type {
  GuardianFormData,
  GuardianWithRelationship,
  GuardianRole,
  PhoneType,
} from "@/lib/guardian-helpers";
import {
  PHONE_TYPE_LABELS,
  LANGUAGE_PREFERENCES,
} from "@/lib/guardian-helpers";
import {
  GuardianRoleSelect,
  RelationshipTypeSelect,
  RelationshipPermissionsFields,
  defaultGuardianRoleForRelationshipType,
  guardianRoleOperationalDefaults,
} from "~/components/guardians/guardian-relationship-fields";
import { ParentVisibleBadge } from "~/components/ui/parent-visible-badge";
import { PARENT_VISIBLE_HINTS } from "~/lib/parent-visible-fields";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "GuardianForm" });

export interface RelationshipFormData {
  readonly relationshipType: string;
  readonly guardianRole: GuardianRole;
  readonly isPrimary: boolean;
  readonly isEmergencyContact: boolean;
  readonly canPickup: boolean;
  readonly pickupNotes?: string;
  readonly emergencyPriority: number;
}

// Phone entry type for the form
// Exported for testing
interface PhoneEntry {
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
  guardianRole: GuardianRole;
  isEmergencyContact: boolean;
  // Relationship flags (preserved in edit mode)
  isPrimary: boolean;
  canPickup: boolean;
  emergencyPriority: number;
  // Address
  addressStreet: string;
  addressCity: string;
  addressPostalCode: string;
  // Additional
  notes: string;
  languagePreference: string;
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
    guardianRole: "legal_guardian",
    isEmergencyContact: false,
    isPrimary: false,
    canPickup: false,
    emergencyPriority: 1,
    addressStreet: "",
    addressCity: "",
    addressPostalCode: "",
    notes: "",
    languagePreference: "de",
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
    guardianRole: data.guardianRole ?? "custom",
    isEmergencyContact: data.isEmergencyContact ?? false,
    // Preserve relationship flags for edit mode
    isPrimary: data.isPrimary ?? false,
    canPickup: data.canPickup ?? true,
    emergencyPriority: data.emergencyPriority ?? 1,
    addressStreet: data.addressStreet ?? "",
    addressCity: data.addressCity ?? "",
    addressPostalCode: data.addressPostalCode ?? "",
    notes: data.notes ?? "",
    languagePreference: data.languagePreference ?? "de",
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
  // Tracks which fields have validation errors: "entryId:field" or "entryId:phone:phoneId"
  const [fieldErrors, setFieldErrors] = useState<Set<string>>(new Set());
  const [entries, setEntries] = useState<GuardianEntry[]>([createEmptyEntry()]);
  const [newEntryId, setNewEntryId] = useState<string | null>(null);
  const entryRefs = useRef<Map<string, HTMLDivElement>>(new Map());
  const errorRef = useScrollToError(error);

  // Reset entries when modal opens/closes or initialData changes
  useEffect(() => {
    if (isOpen) {
      if (initialData) {
        setEntries([toEntry(initialData)]);
      } else {
        setEntries([createEmptyEntry()]);
      }
      setError(null);
      setFieldErrors(new Set());
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

  const updateGuardianRole = (id: string, role: GuardianRole) => {
    const defaults = guardianRoleOperationalDefaults(role);
    setEntries((prev) =>
      prev.map((entry) =>
        entry.id === id ? { ...entry, ...defaults, guardianRole: role } : entry,
      ),
    );
  };

  const updateRelationshipType = (id: string, relationshipType: string) => {
    const guardianRole =
      defaultGuardianRoleForRelationshipType(relationshipType);
    const defaults = guardianRoleOperationalDefaults(guardianRole);
    setEntries((prev) =>
      prev.map((entry) =>
        entry.id === id
          ? { ...entry, ...defaults, relationshipType, guardianRole }
          : entry,
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

  // Validate phone number format (matches backend: digits, spaces, +, -, parens, min 3 digits)
  const validatePhoneNumber = (phone: string): string | null => {
    const trimmed = phone.trim();
    if (trimmed === "") return null; // Empty phones are filtered out before submit

    if (!/^[\d\s+\-()]+$/.test(trimmed)) {
      return "Ungültiges Format (nur Ziffern, Leerzeichen, +, -, Klammern erlaubt)";
    }

    const digitCount = (trimmed.match(/\d/g) ?? []).length;
    if (digitCount < 3) {
      return "Telefonnummer muss mindestens 3 Ziffern enthalten";
    }

    return null;
  };

  // Check if a field has a validation error
  const hasFieldError = (entryId: string, field: string) =>
    fieldErrors.has(`${entryId}:${field}`);

  // Check if a phone field has a validation error
  const hasPhoneError = (entryId: string, phoneId: string) =>
    fieldErrors.has(`${entryId}:phone:${phoneId}`);

  // Validate all entries — stops at first error and highlights only the affected field(s)
  const validateEntries = (): string | null => {
    for (let i = 0; i < entries.length; i++) {
      const entry = entries[i];
      if (!entry) continue;

      const label = entries.length > 1 ? ` (Person ${i + 1})` : "";

      if (!entry.firstName.trim()) {
        setFieldErrors(new Set([`${entry.id}:firstName`]));
        return `Vorname ist erforderlich${label}`;
      }
      if (!entry.lastName.trim()) {
        setFieldErrors(new Set([`${entry.id}:lastName`]));
        return `Nachname ist erforderlich${label}`;
      }

      // Validate email format if provided (basic check, backend does full validation)
      if (entry.email.trim() !== "") {
        const trimmedEmail = entry.email.trim();
        const atIndex = trimmedEmail.indexOf("@");
        const dotIndex = trimmedEmail.lastIndexOf(".");
        if (
          atIndex < 1 ||
          dotIndex < atIndex + 2 ||
          dotIndex >= trimmedEmail.length - 1 ||
          trimmedEmail.includes(" ")
        ) {
          setFieldErrors(new Set([`${entry.id}:email`]));
          return `Ungültiges E-Mail-Format${label}`;
        }
      }

      // Validate phone number formats
      for (const phone of entry.phoneNumbers) {
        const phoneError = validatePhoneNumber(phone.phoneNumber);
        if (phoneError) {
          setFieldErrors(new Set([`${entry.id}:phone:${phone.id}`]));
          return `${phoneError}${label}`;
        }
      }

      // Check for at least one contact method
      const hasEmail = entry.email.trim() !== "";
      const hasPhone = entry.phoneNumbers.some(
        (p) => p.phoneNumber.trim() !== "",
      );

      if (!hasEmail && !hasPhone) {
        const contactErrors = new Set<string>([`${entry.id}:email`]);
        for (const phone of entry.phoneNumbers) {
          contactErrors.add(`${entry.id}:phone:${phone.id}`);
        }
        setFieldErrors(contactErrors);
        return `Mindestens eine Kontaktmöglichkeit ist erforderlich${label}`;
      }
    }

    setFieldErrors(new Set());
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
        addressStreet: entry.addressStreet.trim() || undefined,
        addressCity: entry.addressCity.trim() || undefined,
        addressPostalCode: entry.addressPostalCode.trim() || undefined,
        notes: entry.notes.trim() || undefined,
        languagePreference: entry.languagePreference || "de",
      },
      relationshipData: {
        relationshipType: entry.relationshipType,
        guardianRole: entry.guardianRole,
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
    setFieldErrors(new Set());

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
          <div
            ref={errorRef}
            className="rounded-lg border border-red-200 bg-red-50 p-2 md:p-3"
          >
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
                  <div className="mb-1 flex items-center gap-1">
                    <label
                      htmlFor={`guardian-first-name-${entry.id}`}
                      className={`block text-xs font-medium ${hasFieldError(entry.id, "firstName") ? "text-red-600" : "text-gray-700"}`}
                    >
                      Vorname <span className="text-red-500">*</span>
                    </label>
                    <ParentVisibleBadge
                      compact
                      hint={PARENT_VISIBLE_HINTS.guardianName}
                    />
                  </div>
                  <input
                    id={`guardian-first-name-${entry.id}`}
                    type="text"
                    value={entry.firstName}
                    onChange={(e) =>
                      updateEntry(entry.id, "firstName", e.target.value)
                    }
                    className={`block w-full rounded-lg border bg-white px-3 py-2 text-sm transition-colors focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8] ${hasFieldError(entry.id, "firstName") ? "border-red-400" : "border-gray-200"}`}
                    placeholder="Max"
                    required
                    disabled={isLoading}
                    maxLength={255}
                  />
                </div>

                <div>
                  <div className="mb-1 flex items-center gap-1">
                    <label
                      htmlFor={`guardian-last-name-${entry.id}`}
                      className={`block text-xs font-medium ${hasFieldError(entry.id, "lastName") ? "text-red-600" : "text-gray-700"}`}
                    >
                      Nachname <span className="text-red-500">*</span>
                    </label>
                    <ParentVisibleBadge
                      compact
                      hint={PARENT_VISIBLE_HINTS.guardianName}
                    />
                  </div>
                  <input
                    id={`guardian-last-name-${entry.id}`}
                    type="text"
                    value={entry.lastName}
                    onChange={(e) =>
                      updateEntry(entry.id, "lastName", e.target.value)
                    }
                    className={`block w-full rounded-lg border bg-white px-3 py-2 text-sm transition-colors focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8] ${hasFieldError(entry.id, "lastName") ? "border-red-400" : "border-gray-200"}`}
                    placeholder="Mustermann"
                    required
                    disabled={isLoading}
                    maxLength={255}
                  />
                </div>

                <RelationshipTypeSelect
                  id={`guardian-relationship-type-${entry.id}`}
                  value={entry.relationshipType}
                  onChange={(value) => updateRelationshipType(entry.id, value)}
                  disabled={isLoading}
                  parentVisibleHint={PARENT_VISIBLE_HINTS.guardianName}
                />
                <GuardianRoleSelect
                  id={`guardian-role-${entry.id}`}
                  value={entry.guardianRole}
                  onChange={(value) => updateGuardianRole(entry.id, value)}
                  disabled={isLoading}
                />
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
                <ParentVisibleBadge
                  hint={PARENT_VISIBLE_HINTS.guardianContact}
                />
              </h3>

              {/* Email */}
              <div className="mb-4">
                <label
                  htmlFor={`guardian-email-${entry.id}`}
                  className={`mb-1 block text-xs font-medium ${hasFieldError(entry.id, "email") ? "text-red-600" : "text-gray-700"}`}
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
                  className={`block w-full rounded-lg border bg-white px-3 py-2 text-sm transition-colors focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8] ${hasFieldError(entry.id, "email") ? "border-red-400" : "border-gray-200"}`}
                  placeholder="max.mustermann@example.com"
                  disabled={isLoading}
                  maxLength={255}
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
                      <CustomSelect
                        id={`phone-type-${entry.id}-${phone.id}`}
                        value={phone.phoneType}
                        options={(
                          Object.keys(PHONE_TYPE_LABELS) as PhoneType[]
                        ).map((type) => ({
                          value: type,
                          label: PHONE_TYPE_LABELS[type],
                        }))}
                        onChange={(next) =>
                          updatePhone(
                            entry.id,
                            phone.id,
                            "phoneType",
                            next as PhoneType,
                          )
                        }
                        disabled={isLoading}
                        ariaLabel={`Telefontyp ${phoneIndex + 1}`}
                      />
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
                        className={`block w-full rounded-lg border bg-white px-3 py-1.5 text-sm transition-colors focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8] ${hasPhoneError(entry.id, phone.id) ? "border-red-400" : "border-gray-200"}`}
                        placeholder="+49 170 1234567"
                        disabled={isLoading}
                        aria-label={`Telefonnummer ${phoneIndex + 1}`}
                        maxLength={30}
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
                        maxLength={255}
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

            {/* Address (optional) */}
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
                    d="M17.657 16.657L13.414 20.9a1.998 1.998 0 01-2.827 0l-4.244-4.243a8 8 0 1111.314 0z"
                  />
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M15 11a3 3 0 11-6 0 3 3 0 016 0z"
                  />
                </svg>
                Adresse
                <ParentVisibleBadge
                  hint={PARENT_VISIBLE_HINTS.guardianContact}
                />
              </h3>
              <div className="grid grid-cols-1 gap-3 md:grid-cols-2 md:gap-4">
                <div className="md:col-span-2">
                  <label
                    htmlFor={`guardian-street-${entry.id}`}
                    className="mb-1 block text-xs font-medium text-gray-700"
                  >
                    Straße und Hausnummer
                  </label>
                  <input
                    id={`guardian-street-${entry.id}`}
                    type="text"
                    value={entry.addressStreet}
                    onChange={(e) =>
                      updateEntry(entry.id, "addressStreet", e.target.value)
                    }
                    className="block w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm transition-colors focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8]"
                    placeholder="Musterstr. 1"
                    disabled={isLoading}
                    maxLength={255}
                  />
                </div>
                <div>
                  <label
                    htmlFor={`guardian-postal-${entry.id}`}
                    className="mb-1 block text-xs font-medium text-gray-700"
                  >
                    PLZ
                  </label>
                  <input
                    id={`guardian-postal-${entry.id}`}
                    type="text"
                    value={entry.addressPostalCode}
                    onChange={(e) =>
                      updateEntry(entry.id, "addressPostalCode", e.target.value)
                    }
                    className="block w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm transition-colors focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8]"
                    placeholder="50667"
                    maxLength={5}
                    disabled={isLoading}
                  />
                </div>
                <div>
                  <label
                    htmlFor={`guardian-city-${entry.id}`}
                    className="mb-1 block text-xs font-medium text-gray-700"
                  >
                    Ort
                  </label>
                  <input
                    id={`guardian-city-${entry.id}`}
                    type="text"
                    value={entry.addressCity}
                    onChange={(e) =>
                      updateEntry(entry.id, "addressCity", e.target.value)
                    }
                    className="block w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm transition-colors focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8]"
                    placeholder="Köln"
                    disabled={isLoading}
                    maxLength={255}
                  />
                </div>
              </div>
            </div>

            {/* Additional Info */}
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
                    d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                  />
                </svg>
                Weitere Angaben
              </h3>
              <div className="grid grid-cols-1 gap-3 md:grid-cols-2 md:gap-4">
                <div>
                  <label
                    id={`guardian-language-${entry.id}-label`}
                    htmlFor={`guardian-language-${entry.id}`}
                    className="mb-1 block text-xs font-medium text-gray-700"
                  >
                    Bevorzugte Sprache
                  </label>
                  <CustomSelect
                    id={`guardian-language-${entry.id}`}
                    ariaLabelledBy={`guardian-language-${entry.id}-label`}
                    value={entry.languagePreference}
                    options={LANGUAGE_PREFERENCES}
                    onChange={(next) =>
                      updateEntry(entry.id, "languagePreference", next)
                    }
                    disabled={isLoading}
                  />
                </div>
                <div className="md:col-span-2">
                  <label
                    htmlFor={`guardian-notes-${entry.id}`}
                    className="mb-1 block text-xs font-medium text-gray-700"
                  >
                    Notizen
                  </label>
                  <textarea
                    id={`guardian-notes-${entry.id}`}
                    value={entry.notes}
                    onChange={(e) =>
                      updateEntry(entry.id, "notes", e.target.value)
                    }
                    className="block w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm transition-colors focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8]"
                    placeholder="Interne Notizen zum Erziehungsberechtigten"
                    rows={2}
                    disabled={isLoading}
                    maxLength={2000}
                  />
                </div>
              </div>
            </div>

            {/* Relationship Flags + Emergency Contact (shared with picker) */}
            <RelationshipPermissionsFields
              isPrimary={entry.isPrimary}
              canPickup={entry.canPickup}
              isEmergencyContact={entry.isEmergencyContact}
              onChange={(field, value) => updateEntry(entry.id, field, value)}
              disabled={isLoading}
            />

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
