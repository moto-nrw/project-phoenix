"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { Eye, Loader2, Plus, Search } from "lucide-react";
import GuardianList from "./guardian-list";
import GuardianFormModal from "./guardian-form-modal";
import GuardianPickerPanel from "./guardian-picker-panel";
import { GuardianDeleteModal } from "./guardian-delete-modal";
import type {
  Guardian,
  GuardianWithRelationship,
  GuardianFormData,
  PhoneType,
} from "@/lib/guardian-helpers";
import {
  getGuardianFullName,
  GUARDIAN_ROLE_OPTIONS,
} from "@/lib/guardian-helpers";
import { ConfirmationModal } from "~/components/ui/modal";
import type { RelationshipFormData } from "./guardian-form-modal";
import {
  fetchStudentGuardians,
  createStudentGuardians,
  updateGuardian,
  deleteGuardian,
  linkGuardianToStudent,
  updateStudentGuardianRelationship,
  removeGuardianFromStudent,
  addGuardianPhoneNumber,
  updateGuardianPhoneNumber,
  deleteGuardianPhoneNumber,
  setGuardianPrimaryPhone,
  inviteGuardianToStudent,
  fetchGuardianDeletePreview,
  GuardianApiError,
} from "@/lib/guardian-api";
import { useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { useSession } from "next-auth/react";
import { ConceptSectionHeader } from "~/components/ui/concept-section-header";

const logger = createLogger({ component: "StudentGuardianManager" });

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
  const [isPickerOpen, setIsPickerOpen] = useState(false);
  const [editingGuardian, setEditingGuardian] = useState<
    GuardianWithRelationship | undefined
  >();
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const [deletingGuardian, setDeletingGuardian] = useState<
    GuardianWithRelationship | undefined
  >();
  const [deleteStep, setDeleteStep] = useState<
    "choose" | "confirm-unlink" | "confirm-full"
  >("choose");
  const [fullDeleteWarning, setFullDeleteWarning] = useState<string | null>(
    null,
  );
  const [fullDeleteAffectedLinkIds, setFullDeleteAffectedLinkIds] = useState<
    string[]
  >([]);
  const [isWarningLoading, setIsWarningLoading] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const [invitingGuardianId, setInvitingGuardianId] = useState<string | null>(
    null,
  );
  // Restricted-contact upgrade confirmation (#2172): the invite hit an
  // existing contact whose link has no portal access; ask before upgrading
  // the role to Erziehungsberechtigte/r.
  const [upgradeInvite, setUpgradeInvite] = useState<{
    guardian: GuardianWithRelationship;
    existingRole?: string;
  } | null>(null);
  const deletePreviewRequestIdRef = useRef(0);
  const { success: toastSuccess, error: toastError } = useToast();

  // The full "Komplett löschen" path reaches across every linked child
  // (siblings included), so the backend restricts it to admin wildcards
  // (admin:* / *:*) — mirror that here to only offer it when it would succeed.
  const { data: session } = useSession();
  const canFullDelete = (session?.user?.permissions ?? []).some(
    (p) => p === "admin:*" || p === "*:*",
  );

  useEffect(() => {
    return () => {
      deletePreviewRequestIdRef.current += 1;
    };
  }, []);

  // Load guardians
  const loadGuardians = useCallback(async () => {
    try {
      setIsLoading(true);
      setError(null);
      const data = await fetchStudentGuardians(studentId);
      setGuardians(data);
    } catch (err) {
      logger.error("guardians_load_failed", {
        error: err instanceof Error ? err.message : String(err),
        student_id: studentId,
      });
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

  // Handle create guardian(s) - supports multiple guardians at once.
  //
  // The whole batch is created in ONE backend transaction (#819): every guardian
  // profile, its link to the student, and its phone numbers succeed together or
  // roll back together server-side. This replaces the old per-guardian
  // create→link→add-phones sequence with its client-side rollback, which could
  // orphan a freshly-created profile — a non-admin supervisor cannot delete a
  // guardian once it has no remaining links, so the compensating delete would
  // 403 and leave the profile behind. On any failure nothing is persisted and we
  // rethrow so the form modal shows the (translated) error and keeps the entries
  // for a retry.
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
    _onEntryCreated?: (entryId: string) => void,
  ) => {
    await createStudentGuardians(
      studentId,
      guardians.map(({ guardianData, relationshipData, phoneNumbers }) => ({
        firstName: guardianData.firstName,
        lastName: guardianData.lastName,
        email: guardianData.email,
        addressStreet: guardianData.addressStreet,
        addressCity: guardianData.addressCity,
        addressPostalCode: guardianData.addressPostalCode,
        languagePreference: guardianData.languagePreference,
        notes: guardianData.notes,
        relationshipType: relationshipData.relationshipType,
        guardianRole: relationshipData.guardianRole,
        isPrimary: relationshipData.isPrimary,
        isEmergencyContact: relationshipData.isEmergencyContact,
        canPickup: relationshipData.canPickup,
        pickupNotes: relationshipData.pickupNotes,
        emergencyPriority: relationshipData.emergencyPriority,
        phoneNumbers: phoneNumbers?.map((phone) => ({
          phoneNumber: phone.phoneNumber,
          phoneType: phone.phoneType,
          label: phone.label,
          isPrimary: phone.isPrimary,
        })),
      })),
    );

    await loadGuardians();
    onUpdate?.();
    toastSuccess(
      guardians.length === 1
        ? "Erziehungsberechtigte/r erfolgreich hinzugefügt"
        : `${guardians.length} Erziehungsberechtigte erfolgreich hinzugefügt`,
    );
  };

  // Link an existing guardian chosen from the picker (sibling case). Unlike the
  // create path this never creates a profile — it only links the chosen one with
  // the relationship flags set for THIS child.
  const handleSelectExistingGuardian = async (
    guardian: Guardian,
    relationship: RelationshipFormData,
  ) => {
    try {
      await linkGuardianToStudent(studentId, {
        guardianProfileId: guardian.id,
        ...relationship,
      });
      await loadGuardians();
      onUpdate?.();
      toastSuccess(
        `${getGuardianFullName(guardian)} wurde erfolgreich hinzugefügt`,
      );
    } catch (err) {
      // Log the technical detail; show the user a German message only (the
      // link API throws raw, often English, backend strings).
      logger.error("guardian_link_existing_failed", {
        error: err instanceof Error ? err.message : String(err),
        student_id: studentId,
      });
      toastError("Fehler beim Verknüpfen der/des Erziehungsberechtigten");
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

  // Invite an existing guardian (info already on file) to the parents portal.
  // Uses their on-file email — no re-typing. The backend resolves the existing
  // profile and either sends an invite or links an account that already exists.
  const handleInviteGuardian = async (
    guardian: GuardianWithRelationship,
    confirmRoleUpgrade = false,
  ) => {
    if (!guardian.email) return;
    setInvitingGuardianId(guardian.id);
    try {
      const result = await inviteGuardianToStudent(studentId, guardian.email, {
        confirmRoleUpgrade,
      });
      if (result.outcome === "existing_contact_restricted") {
        // Nothing happened yet: the contact's link carries a restrictive role
        // without portal access. Confirm the upgrade first (#2172).
        setUpgradeInvite({ guardian, existingRole: result.existing_role });
        return;
      }
      await loadGuardians();
      onUpdate?.();
      const name = getGuardianFullName(guardian);
      const message =
        result.outcome === "invited"
          ? `Einladung an ${guardian.email} gesendet`
          : result.outcome === "already_linked"
            ? confirmRoleUpgrade
              ? `${name} hat jetzt vollen Zugriff`
              : `${name} ist bereits verbunden`
            : `${name} wurde mit dem vorhandenen Konto verbunden`;
      toastSuccess(message);
    } catch (err) {
      logger.error("guardian_invite_failed", {
        error: err instanceof Error ? err.message : String(err),
        student_id: studentId,
      });
      toastError("Fehler beim Einladen der/des Erziehungsberechtigten");
    } finally {
      setInvitingGuardianId(null);
    }
  };

  // Handle delete guardian - open the modal. Admins get the choice screen;
  // everyone else goes straight to the per-child unlink confirmation (their
  // only option), so they never see a single-option "choice".
  const handleDeleteClick = (guardian: GuardianWithRelationship) => {
    deletePreviewRequestIdRef.current += 1;
    setDeletingGuardian(guardian);
    setDeleteStep(canFullDelete ? "choose" : "confirm-unlink");
    setFullDeleteWarning(null);
    setIsWarningLoading(false);
    setShowDeleteModal(true);
  };

  // Confirm the per-child unlink. Sibling links and the guardian profile itself
  // are untouched.
  const handleConfirmUnlink = async () => {
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
      logger.error("guardian_remove_failed", {
        error: err instanceof Error ? err.message : String(err),
        student_id: studentId,
      });
      toastError(
        err instanceof Error
          ? err.message
          : "Fehler beim Entfernen der/des Erziehungsberechtigten",
      );
    } finally {
      setIsDeleting(false);
    }
  };

  // Choice → full-delete flow. Switch to the confirmation step IMMEDIATELY (so
  // the modal feels as instant as the unlink path), then fetch the
  // affected-children warning via the READ-ONLY delete-preview endpoint. This
  // never deletes anything by itself — the actual force delete happens only on
  // explicit confirmation in handleConfirmFullDelete. "Endgültig löschen" stays
  // disabled until the warning loads.
  const handleSelectFullDelete = async () => {
    if (!deletingGuardian) return;

    const guardianId = deletingGuardian.id;
    const requestId = deletePreviewRequestIdRef.current + 1;
    deletePreviewRequestIdRef.current = requestId;

    setFullDeleteWarning(null);
    setFullDeleteAffectedLinkIds([]);
    setDeleteStep("confirm-full");
    setIsWarningLoading(true);
    try {
      const preview = await fetchGuardianDeletePreview(guardianId);
      if (deletePreviewRequestIdRef.current !== requestId) return;
      setFullDeleteWarning(preview.warning);
      setFullDeleteAffectedLinkIds(preview.affectedLinkIds);
    } catch (err) {
      if (deletePreviewRequestIdRef.current !== requestId) return;

      logger.error("guardian_full_delete_preview_failed", {
        error: err instanceof Error ? err.message : String(err),
        status: err instanceof GuardianApiError ? err.status : undefined,
        guardian_id: guardianId,
        student_id: studentId,
      });
      // A 403 means the account may not fully delete a guardian — surface that
      // explicitly rather than the generic "could not check children" message.
      if (err instanceof GuardianApiError && err.status === 403) {
        toastError(
          "Sie haben keine Berechtigung, diese Person vollständig zu löschen.",
        );
      } else {
        toastError(
          err instanceof Error
            ? err.message
            : "Fehler beim Prüfen der betroffenen Kinder",
        );
      }
      // Preview failed — drop back to the choice rather than letting the user
      // confirm a delete whose blast radius we never showed.
      handleBack();
    } finally {
      if (deletePreviewRequestIdRef.current === requestId) {
        setIsWarningLoading(false);
      }
    }
  };

  // Confirm the full delete (force): removes the guardian and all of its links.
  const handleConfirmFullDelete = async () => {
    if (!deletingGuardian || !fullDeleteWarning) return;

    const deletedName = getGuardianFullName(deletingGuardian);
    setIsDeleting(true);
    try {
      await deleteGuardian(deletingGuardian.id, {
        force: true,
        expectedAffectedLinkIds: fullDeleteAffectedLinkIds,
      });
      await loadGuardians();
      onUpdate?.();
      setShowDeleteModal(false);
      setDeletingGuardian(undefined);
      setDeleteStep("choose");
      setFullDeleteWarning(null);
      setFullDeleteAffectedLinkIds([]);
      toastSuccess(`${deletedName} wurde vollständig gelöscht`);
    } catch (err) {
      logger.error("guardian_full_delete_failed", {
        error: err instanceof Error ? err.message : String(err),
        student_id: studentId,
      });
      toastError(
        err instanceof Error
          ? err.message
          : "Fehler beim Löschen der/des Erziehungsberechtigten",
      );
    } finally {
      setIsDeleting(false);
    }
  };

  // Back from a confirmation step. Admins return to the choice screen; everyone
  // else has no choice screen, so "back" just closes the modal.
  const handleBack = () => {
    deletePreviewRequestIdRef.current += 1;
    if (canFullDelete) {
      setDeleteStep("choose");
      setFullDeleteWarning(null);
      setFullDeleteAffectedLinkIds([]);
      setIsWarningLoading(false);
    } else {
      handleCancelDelete();
    }
  };

  // Cancel delete
  const handleCancelDelete = () => {
    deletePreviewRequestIdRef.current += 1;
    setShowDeleteModal(false);
    setDeletingGuardian(undefined);
    setDeleteStep("choose");
    setFullDeleteWarning(null);
    setFullDeleteAffectedLinkIds([]);
    setIsWarningLoading(false);
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
      <div className="border-moto-red/20 bg-moto-red/10 text-moto-red-strong rounded-lg border px-4 py-3">
        {error}
      </div>
    );
  }

  return (
    <div className="relative z-10 rounded-2xl border border-gray-100 bg-white/50 p-4 backdrop-blur-sm sm:p-6">
      <ConceptSectionHeader
        className="mb-4"
        title="Erziehungsberechtigte"
        concept="parents"
        actions={
          <div className="flex items-center gap-2">
            {readOnly && (
              <span className="inline-flex items-center gap-1 rounded-md bg-gray-100 px-2 py-1 text-xs font-medium text-gray-600 sm:px-2.5">
                <Eye className="h-3 w-3 sm:h-3.5 sm:w-3.5" aria-hidden="true" />
                <span className="hidden sm:inline">Nur Ansicht</span>
                <span className="sm:hidden">Ansicht</span>
              </span>
            )}
            {!readOnly && (
              <>
                <button
                  type="button"
                  onClick={() => setIsPickerOpen(true)}
                  className="inline-flex items-center gap-1 rounded-lg border border-gray-300 bg-white px-3 py-1.5 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50"
                  title="Vorhandene/n Erziehungsberechtigte/n suchen"
                >
                  <Search className="h-4 w-4" />
                  <span className="hidden sm:inline">Vorhandene/n suchen</span>
                </button>
                <button
                  type="button"
                  onClick={handleOpenCreateModal}
                  className="inline-flex items-center gap-1 rounded-lg bg-gray-900 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-gray-700"
                  title="Erziehungsberechtigte/n hinzufügen"
                >
                  <Plus className="h-4 w-4" />
                  <span className="hidden sm:inline">Hinzufügen</span>
                </button>
              </>
            )}
          </div>
        }
      />

      {/* Existing-guardian picker (sibling case) — inline, not a modal, since a
          search is a light lookup. "Hinzufügen" opens the heavy form modal. */}
      {isPickerOpen && (
        <div className="mb-3">
          <GuardianPickerPanel
            onSelect={(guardian, relationship) => {
              setIsPickerOpen(false);
              void handleSelectExistingGuardian(guardian, relationship);
            }}
            onCancel={() => setIsPickerOpen(false)}
            excludeProfileIds={guardians.map((g) => g.id)}
          />
        </div>
      )}

      {/* Guardian List */}
      <div className="space-y-3">
        <GuardianList
          guardians={guardians}
          onEdit={readOnly ? undefined : handleOpenEditModal}
          onInvite={readOnly ? undefined : (g) => void handleInviteGuardian(g)}
          invitingGuardianId={invitingGuardianId}
          readOnly={readOnly}
          showRelationship={true}
        />
      </div>

      {/* Form Modal */}
      <GuardianFormModal
        isOpen={isModalOpen}
        onClose={handleCloseModal}
        onSubmit={editingGuardian ? handleEditGuardian : handleCreateGuardians}
        onDelete={
          editingGuardian
            ? () => {
                handleCloseModal();
                handleDeleteClick(editingGuardian);
              }
            : undefined
        }
        initialData={editingGuardian}
        mode={editingGuardian ? "edit" : "create"}
      />

      {/* Delete Confirmation Modal */}
      <GuardianDeleteModal
        isOpen={showDeleteModal}
        onClose={handleCancelDelete}
        guardianName={
          deletingGuardian ? getGuardianFullName(deletingGuardian) : ""
        }
        isLoading={isDeleting}
        step={deleteStep}
        canFullDelete={canFullDelete}
        fullDeleteWarning={fullDeleteWarning}
        isWarningLoading={isWarningLoading}
        onSelectUnlink={() => setDeleteStep("confirm-unlink")}
        onSelectFullDelete={handleSelectFullDelete}
        onConfirmUnlink={handleConfirmUnlink}
        onConfirmFullDelete={handleConfirmFullDelete}
        onBack={handleBack}
      />

      {/* Restricted-contact upgrade confirmation (#2172) */}
      <ConfirmationModal
        isOpen={upgradeInvite !== null}
        onClose={() => setUpgradeInvite(null)}
        onConfirm={() => {
          if (!upgradeInvite) return;
          const target = upgradeInvite.guardian;
          setUpgradeInvite(null);
          void handleInviteGuardian(target, true);
        }}
        title="Vollen Zugriff gewähren?"
        confirmText="Zugriff gewähren"
        cancelText="Abbrechen"
      >
        <p className="text-sm text-gray-600">
          {upgradeInvite &&
            `${getGuardianFullName(upgradeInvite.guardian)} ist bisher als ${
              GUARDIAN_ROLE_OPTIONS.find(
                (o) => o.value === upgradeInvite.existingRole,
              )?.label ?? "eingeschränkter Kontakt"
            } für dieses Kind eingetragen, ohne Zugriff auf die Eltern-App.`}{" "}
          Mit der Einladung wird die Rolle auf Erziehungsberechtigte/r
          hochgestuft und die Person erhält vollen Zugriff auf dieses Kind in
          der Eltern-App.
        </p>
      </ConfirmationModal>
    </div>
  );
}
