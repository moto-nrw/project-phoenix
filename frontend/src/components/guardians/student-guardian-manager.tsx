"use client";

import { useState, useEffect, useCallback } from "react";
import { UserPlus, Loader2 } from "lucide-react";
import GuardianList from "./guardian-list";
import GuardianFormModal from "./guardian-form-modal";
import { GuardianDeleteModal } from "./guardian-delete-modal";
import type {
  GuardianWithRelationship,
  GuardianFormData,
  PhoneType,
} from "@/lib/guardian-helpers";
import { getGuardianFullName } from "@/lib/guardian-helpers";
import type { RelationshipFormData } from "./guardian-form-modal";
import {
  fetchStudentGuardians,
  createGuardian,
  updateGuardian,
  linkGuardianToStudent,
  updateStudentGuardianRelationship,
  removeGuardianFromStudent,
  addGuardianPhoneNumber,
  updateGuardianPhoneNumber,
  deleteGuardianPhoneNumber,
  setGuardianPrimaryPhone,
} from "@/lib/guardian-api";
import { useToast } from "~/contexts/ToastContext";

interface StudentGuardianManagerProps {
  readonly studentId: string;
  readonly readOnly?: boolean;
  readonly onUpdate?: () => void;
}

export default function StudentGuardianManager({
  studentId,
  readOnly = false,
  onUpdate,
}: StudentGuardianManagerProps) {
  const [guardians, setGuardians] = useState<GuardianWithRelationship[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editingGuardian, setEditingGuardian] = useState<
    GuardianWithRelationship | undefined
  >();
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const [deletingGuardian, setDeletingGuardian] = useState<
    GuardianWithRelationship | undefined
  >();
  const [isDeleting, setIsDeleting] = useState(false);
  const { success: toastSuccess } = useToast();

  // Load guardians
  const loadGuardians = useCallback(async () => {
    try {
      setIsLoading(true);
      setError(null);
      const data = await fetchStudentGuardians(studentId);
      setGuardians(data);
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Fehler beim Laden der Erziehungsberechtigten",
      );
    } finally {
      setIsLoading(false);
    }
  }, [studentId]);

  useEffect(() => {
    loadGuardians().catch(() => {
      // Error already handled in loadGuardians
    });
  }, [loadGuardians]);

  // Handle create guardian(s) - supports multiple guardians at once
  const handleCreateGuardians = async (
    guardians: Array<{
      id: string;
      guardianData: GuardianFormData;
      relationshipData: RelationshipFormData;
      phoneNumbers?: Array<{
        phoneNumber: string;
        phoneType: PhoneType;
        label?: string;
        isPrimary: boolean;
      }>;
    }>,
    onEntryCreated?: (entryId: string) => void,
  ) => {
    let successCount = 0;
    try {
      // Create all guardians sequentially to ensure proper error handling
      for (const {
        id,
        guardianData,
        relationshipData,
        phoneNumbers,
      } of guardians) {
        // Create guardian profile
        const newGuardian = await createGuardian(guardianData);

        // Link to student
        await linkGuardianToStudent(studentId, {
          guardianProfileId: newGuardian.id,
          ...relationshipData,
        });

        // Add phone numbers if provided
        if (phoneNumbers && phoneNumbers.length > 0) {
          for (const phone of phoneNumbers) {
            await addGuardianPhoneNumber(newGuardian.id, {
              phoneNumber: phone.phoneNumber,
              phoneType: phone.phoneType,
              label: phone.label,
              isPrimary: phone.isPrimary,
            });
          }
        }

        // Remove successfully created entry from modal (enables retry without duplicates)
        onEntryCreated?.(id);
        successCount++;
      }
    } finally {
      // Only reload and notify parent if at least one guardian was created
      // This prevents false success signals on complete failure
      if (successCount > 0) {
        await loadGuardians();
        onUpdate?.();
        toastSuccess(
          successCount === 1
            ? "Erziehungsberechtigte/r erfolgreich hinzugefügt"
            : `${successCount} Erziehungsberechtigte erfolgreich hinzugefügt`,
        );
      }
    }
  };

  // Handle edit guardian - takes array but only uses first entry (edit mode has single entry)
  const handleEditGuardian = async (
    guardians: Array<{
      id: string;
      guardianData: GuardianFormData;
      relationshipData: RelationshipFormData;
      phoneNumbers?: Array<{
        phoneNumber: string;
        phoneType: PhoneType;
        label?: string;
        isPrimary: boolean;
        id?: string; // Existing phone ID (if editing)
      }>;
    }>,
    _onEntryCreated?: (entryId: string) => void,
  ) => {
    if (!editingGuardian) return;

    const first = guardians[0];
    if (!first) return;

    const { guardianData, relationshipData, phoneNumbers } = first;

    // Update guardian profile and relationship
    await updateGuardian(editingGuardian.id, guardianData);
    await updateStudentGuardianRelationship(
      editingGuardian.relationshipId,
      relationshipData,
    );

    // Sync phone numbers if provided
    if (phoneNumbers) {
      await syncGuardianPhoneNumbers(
        editingGuardian.id,
        phoneNumbers,
        editingGuardian.phoneNumbers ?? [],
      );
    }

    // Reload guardians
    await loadGuardians();
    onUpdate?.();
    setEditingGuardian(undefined);
    toastSuccess("Erziehungsberechtigte/r erfolgreich aktualisiert");
  };

  // Helper: Sync phone numbers (add/update/delete)
  const syncGuardianPhoneNumbers = async (
    guardianId: string,
    formPhones: Array<{
      phoneNumber: string;
      phoneType: PhoneType;
      label?: string;
      isPrimary: boolean;
      id?: string;
    }>,
    existingPhones: Array<{ id: string; isPrimary: boolean }>,
  ) => {
    const existingPhoneIds = new Set(existingPhones.map((p) => p.id));

    // Process phones and track primary
    const primaryPhoneId = await processPhoneUpdates(
      guardianId,
      formPhones,
      existingPhoneIds,
    );

    // Delete removed phones
    await deleteRemovedPhones(guardianId, formPhones, existingPhones);

    // Update primary if changed
    await updatePrimaryIfNeeded(guardianId, primaryPhoneId, existingPhones);
  };

  // Helper: Process phone additions/updates
  const processPhoneUpdates = async (
    guardianId: string,
    formPhones: Array<{
      phoneNumber: string;
      phoneType: PhoneType;
      label?: string;
      isPrimary: boolean;
      id?: string;
    }>,
    existingPhoneIds: Set<string>,
  ): Promise<string | null> => {
    let primaryPhoneId: string | null = null;

    for (const phone of formPhones) {
      const isNew =
        !phone.id || phone.id.includes("-") || !existingPhoneIds.has(phone.id);
      const resultId = isNew
        ? (await addGuardianPhoneNumber(guardianId, phone)).id
        : (await updateGuardianPhoneNumber(guardianId, phone.id!, phone),
          phone.id!);

      if (phone.isPrimary) primaryPhoneId = resultId;
    }

    return primaryPhoneId;
  };

  // Helper: Delete phones removed from form
  const deleteRemovedPhones = async (
    guardianId: string,
    formPhones: Array<{ id?: string }>,
    existingPhones: Array<{ id: string }>,
  ) => {
    const keepIds = new Set(
      formPhones.filter((p) => p.id && !p.id.includes("-")).map((p) => p.id),
    );
    for (const existing of existingPhones) {
      if (!keepIds.has(existing.id)) {
        await deleteGuardianPhoneNumber(guardianId, existing.id);
      }
    }
  };

  // Helper: Update primary phone if changed
  const updatePrimaryIfNeeded = async (
    guardianId: string,
    primaryPhoneId: string | null,
    existingPhones: Array<{ id: string; isPrimary: boolean }>,
  ) => {
    if (!primaryPhoneId) return;
    const existingPrimary = existingPhones.find((p) => p.isPrimary);
    if (existingPrimary?.id !== primaryPhoneId) {
      await setGuardianPrimaryPhone(guardianId, primaryPhoneId);
    }
  };

  // Handle delete guardian - open confirmation modal
  const handleDeleteClick = (guardian: GuardianWithRelationship) => {
    setDeletingGuardian(guardian);
    setShowDeleteModal(true);
  };

  // Confirm delete guardian
  const handleConfirmDelete = async () => {
    if (!deletingGuardian) return;

    const deletedName = getGuardianFullName(deletingGuardian);
    setIsDeleting(true);
    try {
      await removeGuardianFromStudent(studentId, deletingGuardian.id);
      await loadGuardians();
      onUpdate?.();
      setShowDeleteModal(false);
      setDeletingGuardian(undefined);
      toastSuccess(`${deletedName} wurde erfolgreich entfernt`);
    } catch (err) {
      alert(
        err instanceof Error
          ? err.message
          : "Fehler beim Entfernen der/des Erziehungsberechtigten",
      );
    } finally {
      setIsDeleting(false);
    }
  };

  // Cancel delete
  const handleCancelDelete = () => {
    setShowDeleteModal(false);
    setDeletingGuardian(undefined);
  };

  // Open modal for creating
  const handleOpenCreateModal = () => {
    setEditingGuardian(undefined);
    setIsModalOpen(true);
  };

  // Open modal for editing
  const handleOpenEditModal = (guardian: GuardianWithRelationship) => {
    setEditingGuardian(guardian);
    setIsModalOpen(true);
  };

  // Close modal
  const handleCloseModal = () => {
    setIsModalOpen(false);
    setEditingGuardian(undefined);
  };

  // Only show full-page loader on initial load (no data yet)
  // During refreshes, keep UI mounted to preserve modal state
  if (isLoading && guardians.length === 0) {
    return (
      <div className="flex items-center justify-center py-12">
        <Loader2 className="h-8 w-8 animate-spin text-gray-600" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-red-700">
        {error}
      </div>
    );
  }

  return (
    <div className="rounded-2xl border border-gray-100 bg-white/50 p-4 backdrop-blur-sm sm:p-6">
      {/* Header with Icon and Add Button */}
      <div className="mb-4 flex items-center justify-between gap-2">
        <div className="flex min-w-0 flex-1 items-center gap-2 sm:gap-3">
          <div className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg bg-purple-100 text-purple-600 sm:h-10 sm:w-10">
            <svg
              className="h-5 w-5"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"
              />
            </svg>
          </div>
          <h2 className="truncate text-base font-semibold text-gray-900 sm:text-lg">
            Erziehungsberechtigte
          </h2>
        </div>
        <div className="flex flex-shrink-0 items-center gap-2">
          {readOnly && (
            <span className="inline-flex items-center gap-1 rounded-md bg-gray-100 px-2 py-1 text-xs font-medium text-gray-600 sm:px-2.5">
              <svg
                className="h-3 w-3 sm:h-3.5 sm:w-3.5"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
                />
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"
                />
              </svg>
              <span className="hidden sm:inline">Nur Ansicht</span>
              <span className="sm:hidden">Ansicht</span>
            </span>
          )}
          {!readOnly && (
            <button
              onClick={handleOpenCreateModal}
              className="inline-flex items-center gap-2 rounded-lg bg-gray-900 p-2 text-white transition-all duration-200 hover:bg-gray-700 hover:shadow-lg active:scale-[0.99] sm:gap-2 sm:px-4 sm:py-2 sm:hover:scale-[1.01]"
              title="Erziehungsberechtigte/n hinzufügen"
            >
              <UserPlus className="h-4 w-4" />
              <span className="hidden text-sm font-medium sm:inline">
                Hinzufügen
              </span>
            </button>
          )}
        </div>
      </div>

      {/* Guardian List */}
      <div className="space-y-3">
        <GuardianList
          guardians={guardians}
          onEdit={readOnly ? undefined : handleOpenEditModal}
          onDelete={readOnly ? undefined : handleDeleteClick}
          readOnly={readOnly}
          showRelationship={true}
        />
      </div>

      {/* Form Modal */}
      <GuardianFormModal
        isOpen={isModalOpen}
        onClose={handleCloseModal}
        onSubmit={editingGuardian ? handleEditGuardian : handleCreateGuardians}
        initialData={editingGuardian}
        mode={editingGuardian ? "edit" : "create"}
      />

      {/* Delete Confirmation Modal */}
      <GuardianDeleteModal
        isOpen={showDeleteModal}
        onClose={handleCancelDelete}
        onConfirm={handleConfirmDelete}
        guardianName={
          deletingGuardian ? getGuardianFullName(deletingGuardian) : ""
        }
        isLoading={isDeleting}
      />
    </div>
  );
}
