"use client";

import { StatusBadge } from "~/components/ui/status-badge";
import {
  useState,
  useEffect,
  useCallback,
  useLayoutEffect,
  useRef,
} from "react";
import { Eye, Plus, Search } from "lucide-react";
import { Button } from "~/components/ui/button";
import GuardianList from "./guardian-list";
import GuardianFormModal from "./guardian-form-modal";
import GuardianPickerPanel from "./guardian-picker-panel";
import {
  GuardianDeleteModal,
  type GuardianDeleteScope,
} from "./guardian-delete-modal";
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
import { FormModal } from "~/components/ui/form-modal";
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
  fetchStudentGuardianEmailStates,
  liftGuardianEmailBlock,
  resumeGuardianEmails,
  stopGuardianEmails,
} from "@/lib/guardian-api";
import type {
  GuardianEmailState,
  InviteGuardianResult,
} from "@/lib/guardian-api";
import {
  useApiErrorDisplay,
  useApiFormError,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { formErrorMessage } from "~/components/ui/form-error";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { createLogger } from "~/lib/logger";
import { displayEmail } from "~/lib/demo-build";
import { useSession } from "next-auth/react";
import { SectionCard } from "~/components/ui/section-card";
import { StudentPaymentCard } from "./student-payment-card";
import { hasPermission } from "~/lib/auth-utils";
import { parentsConcept } from "~/lib/concepts/parents";
import { Loading } from "~/components/ui/loading";

const logger = createLogger({ component: "StudentGuardianManager" });

// Toast after "Einladen". An existing account gets no registration link,
// only a mail that points to the parents portal login (#3320).
function inviteSuccessMessage(
  outcome: InviteGuardianResult["outcome"],
  name: string,
  email: string,
  confirmRoleUpgrade: boolean,
  mailHeld = false,
): string {
  switch (outcome) {
    case "invited":
      return `Die Einladung an ${email} wird gesendet. Den Stand sehen Sie bei der Person.`;
    case "pending_approval":
      return `Die Anfrage für ${name} wartet auf Freigabe.`;
    case "already_linked":
    case "linked_existing_account":
      if (mailHeld) {
        return `${name} ist verknüpft. An ${email} geht kein Hinweis raus. Den Grund sehen Sie bei der Person.`;
      }
      return confirmRoleUpgrade
        ? `${name} hat jetzt vollen Zugriff.`
        : `${name} hat schon ein Konto. Ein Hinweis zum Elternportal wird an ${email} gesendet.`;
    default:
      return `${name} ist gespeichert.`;
  }
}

type GuardianEmailChange = "stop" | "resume" | "lift";

const EMAIL_CHANGE_COPY: Record<
  GuardianEmailChange,
  {
    readonly title: string;
    readonly confirm: string;
    readonly text: string;
    readonly object: string;
    readonly run: (guardianId: string) => Promise<void>;
    readonly done: (name: string) => string;
  }
> = {
  stop: {
    title: "Mails stoppen?",
    confirm: "Stoppen",
    text: "Diese Person bekommt dann keine Mails der Schule mehr: keine Einladung, keine Elternbriefe, keine Termine. Das gilt auch nach einer neuen Adresse. Anmeldecodes und Mails zum Passwort kommen weiter an. Abrechnungsbriefe des Trägers auch. Nutzt noch jemand diese Adresse, gehen Mails an die Adresse weiter raus.",
    object: "der Stopp der Mails",
    run: stopGuardianEmails,
    done: (name) => `Die Mails an ${name} sind gestoppt.`,
  },
  resume: {
    title: "Mails wieder senden?",
    confirm: "Wieder senden",
    text: "Diese Person bekommt dann wieder Mails der Schule.",
    object: "der Stopp der Mails",
    run: resumeGuardianEmails,
    done: (name) => `${name} bekommt wieder Mails.`,
  },
  lift: {
    title: "Sperre aufheben?",
    confirm: "Aufheben",
    text: "moto sendet dann wieder Mails an diese Adresse. Kommt erneut eine Mail zurück, wird die Adresse wieder gesperrt.",
    object: "die Sperre",
    run: liftGuardianEmailBlock,
    done: (name) => `Die Sperre für ${name} ist aufgehoben.`,
  },
};

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
  // Ladefehler stehen vor Ort, mit Wiederholen (#2517); nie als Leerzustand.
  const [loadFailed, setLoadFailed] = useState(false);
  const loadError = useApiLoadError();
  const showLoadError = loadError.show;
  const clearLoadError = loadError.clear;
  const latestLoadRef = useRef<() => void>(() => undefined);
  const [emailStates, setEmailStates] = useState<
    Record<string, GuardianEmailState> | undefined
  >(undefined);
  const emailStatesLoad = useApiLoadError();
  const showEmailStatesError = emailStatesLoad.show;
  const clearEmailStatesError = emailStatesLoad.clear;
  // Stopping and freeing an address change what reaches a family: ask once.
  const [emailBlockChange, setEmailBlockChange] = useState<{
    guardian: GuardianWithRelationship;
    action: GuardianEmailChange;
  } | null>(null);
  const emailChangeCopy = emailBlockChange
    ? EMAIL_CHANGE_COPY[emailBlockChange.action]
    : null;
  const emailBlockErrors = useApiFormError();
  const [isChangingEmailBlock, setIsChangingEmailBlock] = useState(false);
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [isPickerOpen, setIsPickerOpen] = useState(false);
  const [editingGuardian, setEditingGuardian] = useState<
    GuardianWithRelationship | undefined
  >();
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const [deletingGuardian, setDeletingGuardian] = useState<
    GuardianWithRelationship | undefined
  >();
  // Scope of the deletion (#3110): admins pick it inside the delete dialog,
  // everyone else has only the per-child unlink.
  const [deleteScope, setDeleteScope] = useState<GuardianDeleteScope | null>(
    null,
  );
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
  const { success: toastSuccess } = useToast();
  // Aktionen ohne Formular (Verknüpfen, Einladen): Toast mit Katalogtext.
  const { show: showActionError } = useApiErrorDisplay();
  // Fehler beim Entfernen und bei der Vorschau bleiben im offenen
  // Löschdialog; ein Toast läge unter dem Dialog.
  const deleteErrors = useApiFormError();
  const clearDeleteError = deleteErrors.clear;
  const latestUnlinkRef = useRef<() => void>(() => undefined);
  const latestFullDeleteRef = useRef<() => void>(() => undefined);

  // The full "Komplett löschen" path reaches across every linked child
  // (siblings included), so the backend restricts it to admin wildcards
  // (admin:* / *:*) — mirror that here to only offer it when it would succeed.
  const { data: session } = useSession();
  const canFullDelete = (session?.user?.permissions ?? []).some(
    (p) => p === "admin:*" || p === "*:*",
  );
  // The bank section is its own permission (#2608): maintaining the guardian
  // directory and handling bank data are different jobs, so users:update alone
  // must not reveal an IBAN. Mirror the backend gate, or the card would render
  // and every request behind it 403.
  const canSeePayment = hasPermission(session, "guardians:financial");

  useEffect(() => {
    return () => {
      deletePreviewRequestIdRef.current += 1;
    };
  }, []);

  // The mail state comes on its own: without it the list still shows who to
  // call, and the failure stands above the list with its own retry.
  const loadEmailStates = useCallback(async () => {
    try {
      setEmailStates(await fetchStudentGuardianEmailStates(studentId));
      clearEmailStatesError();
    } catch (err) {
      logger.error("guardian_email_status_load_failed", {
        error: err instanceof Error ? err.message : String(err),
        student_id: studentId,
      });
      void showEmailStatesError(err, {
        object: "der Stand der E-Mails",
        retry: () => void loadEmailStates(),
      });
    }
  }, [studentId, showEmailStatesError, clearEmailStatesError]);

  // Load guardians
  // Never rejects: a failed load shows its error where the list belongs.
  const loadGuardians = useCallback(async () => {
    try {
      setIsLoading(true);
      const data = await fetchStudentGuardians(studentId);
      setGuardians(data);
      setLoadFailed(false);
      clearLoadError();
      void loadEmailStates();
    } catch (err) {
      logger.error("guardians_load_failed", {
        error: err instanceof Error ? err.message : String(err),
        student_id: studentId,
      });
      setLoadFailed(true);
      void showLoadError(err, {
        object: "die Liste der Erziehungsberechtigten",
        retry: () => latestLoadRef.current(),
      });
    } finally {
      setIsLoading(false);
    }
  }, [studentId, showLoadError, clearLoadError, loadEmailStates]);
  useLayoutEffect(() => {
    latestLoadRef.current = () => void loadGuardians();
  });

  useEffect(() => {
    void loadGuardians();
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
  // rethrow so the form modal shows the error and keeps the entries
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
    const first = guardians[0]?.guardianData;
    toastSuccess(
      guardians.length === 1 && first
        ? `${first.firstName} ${first.lastName} ist hinzugefügt.`
        : `${guardians.length} Erziehungsberechtigte sind hinzugefügt.`,
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
        `${getGuardianFullName(guardian)} ist jetzt mit diesem Kind verknüpft.`,
      );
    } catch (err) {
      logger.error("guardian_link_existing_failed", {
        error: err instanceof Error ? err.message : String(err),
        student_id: studentId,
      });
      // Der Suchdialog ist schon zu: die Meldung kommt als Toast.
      void showActionError(err, {
        object: "das Verknüpfen der Person",
        retry: () => void handleSelectExistingGuardian(guardian, relationship),
      });
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
    toastSuccess(
      `Die Angaben von ${guardianData.firstName} ${guardianData.lastName} sind gespeichert.`,
    );
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
    const email = displayEmail(guardian.email);
    if (!email) return;
    setInvitingGuardianId(guardian.id);
    try {
      const result = await inviteGuardianToStudent(studentId, email, {
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
      toastSuccess(
        inviteSuccessMessage(
          result.outcome,
          getGuardianFullName(guardian),
          email,
          confirmRoleUpgrade,
          result.mail_held,
        ),
      );
      await loadEmailStates();
    } catch (err) {
      logger.error("guardian_invite_failed", {
        error: err instanceof Error ? err.message : String(err),
        student_id: studentId,
      });
      void showActionError(err, {
        object: "die Einladung",
        retry: () => void handleInviteGuardian(guardian, confirmRoleUpgrade),
      });
    } finally {
      setInvitingGuardianId(null);
    }
  };

  const closeEmailBlockChange = () => {
    emailBlockErrors.clear();
    setEmailBlockChange(null);
  };

  const confirmEmailBlockChange = async () => {
    if (!emailBlockChange) return;
    const { guardian, action } = emailBlockChange;
    emailBlockErrors.clear();
    setIsChangingEmailBlock(true);
    try {
      await EMAIL_CHANGE_COPY[action].run(guardian.id);
      toastSuccess(
        EMAIL_CHANGE_COPY[action].done(getGuardianFullName(guardian)),
      );
      setEmailBlockChange(null);
      await loadEmailStates();
    } catch (err) {
      logger.error("guardian_email_block_change_failed", {
        error: err instanceof Error ? err.message : String(err),
        student_id: studentId,
        action,
      });
      await emailBlockErrors.show(err, {
        object: EMAIL_CHANGE_COPY[action].object,
        retry: () => void confirmEmailBlockChange(),
      });
    } finally {
      setIsChangingEmailBlock(false);
    }
  };

  // Handle delete guardian - open the modal. Admins get the scope choice
  // inside the dialog; everyone else gets the per-child unlink (their only
  // option), so they never see a single-option "choice".
  const handleDeleteClick = (guardian: GuardianWithRelationship) => {
    deletePreviewRequestIdRef.current += 1;
    clearDeleteError();
    setDeletingGuardian(guardian);
    setDeleteScope(null);
    setFullDeleteWarning(null);
    setFullDeleteAffectedLinkIds([]);
    setIsWarningLoading(false);
    setShowDeleteModal(true);
  };

  // Confirm the per-child unlink. Sibling links and the guardian profile itself
  // are untouched.
  const handleConfirmUnlink = async () => {
    if (!deletingGuardian) return;

    const deletedName = getGuardianFullName(deletingGuardian);
    setIsDeleting(true);
    clearDeleteError();
    try {
      await removeGuardianFromStudent(studentId, deletingGuardian.id);
      await loadGuardians();
      onUpdate?.();
      setShowDeleteModal(false);
      setDeletingGuardian(undefined);
      toastSuccess(`${deletedName} ist von diesem Kind entfernt.`);
    } catch (err) {
      logger.error("guardian_remove_failed", {
        error: err instanceof Error ? err.message : String(err),
        student_id: studentId,
      });
      await deleteErrors.show(err, {
        object: "das Entfernen der Person",
        retry: () => latestUnlinkRef.current(),
      });
    } finally {
      setIsDeleting(false);
    }
  };

  // Scope "full" → fetch the affected-children warning via the READ-ONLY
  // delete-preview endpoint. This never deletes anything by itself — the
  // actual force delete happens only on explicit confirmation in
  // handleConfirmFullDelete. "Endgültig löschen" stays disabled until the
  // warning loads.
  const handleSelectFullDelete = async () => {
    if (!deletingGuardian) return;

    const guardianId = deletingGuardian.id;
    const requestId = deletePreviewRequestIdRef.current + 1;
    deletePreviewRequestIdRef.current = requestId;

    setFullDeleteWarning(null);
    setFullDeleteAffectedLinkIds([]);
    setDeleteScope("full");
    setIsWarningLoading(true);
    clearDeleteError();
    try {
      const preview = await fetchGuardianDeletePreview(guardianId);
      if (deletePreviewRequestIdRef.current !== requestId) return;
      setFullDeleteWarning(preview.warning);
      setFullDeleteAffectedLinkIds(preview.affectedLinkIds);
    } catch (err) {
      if (deletePreviewRequestIdRef.current !== requestId) return;

      logger.error("guardian_full_delete_preview_failed", {
        error: err instanceof Error ? err.message : String(err),
        guardian_id: guardianId,
        student_id: studentId,
      });
      // Preview failed — drop the scope rather than letting the user confirm
      // a delete whose blast radius we never showed. The reason (a 403 for a
      // non-admin included) stays in the dialog; "Wiederholen" asks again.
      handleScopeChange(null, { keepError: true });
      await deleteErrors.show(err, {
        object: "die Prüfung der betroffenen Kinder",
        retry: () => handleScopeChange("full"),
      });
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
    clearDeleteError();
    try {
      await deleteGuardian(deletingGuardian.id, {
        force: true,
        expectedAffectedLinkIds: fullDeleteAffectedLinkIds,
      });
      await loadGuardians();
      onUpdate?.();
      setShowDeleteModal(false);
      setDeletingGuardian(undefined);
      setDeleteScope(null);
      setFullDeleteWarning(null);
      setFullDeleteAffectedLinkIds([]);
      toastSuccess(`${deletedName} ist vollständig gelöscht.`);
    } catch (err) {
      logger.error("guardian_full_delete_failed", {
        error: err instanceof Error ? err.message : String(err),
        student_id: studentId,
      });
      await deleteErrors.show(err, {
        object: "das Löschen der Person",
        retry: () => latestFullDeleteRef.current(),
      });
    } finally {
      setIsDeleting(false);
    }
  };
  // „Wiederholen“ läuft mit dem dann aktuellen Stand des Dialogs.
  useLayoutEffect(() => {
    latestUnlinkRef.current = () => void handleConfirmUnlink();
    latestFullDeleteRef.current = () => void handleConfirmFullDelete();
  });

  // Scope change inside the dialog. Leaving "full" (or dropping the scope
  // after a failed preview) discards the warning so a stale blast radius can
  // never be confirmed; picking "full" starts the preview.
  const handleScopeChange = (
    scope: GuardianDeleteScope | null,
    options: { keepError?: boolean } = {},
  ) => {
    if (scope === "full") {
      void handleSelectFullDelete();
      return;
    }
    if (!options.keepError) clearDeleteError();
    deletePreviewRequestIdRef.current += 1;
    setDeleteScope(scope);
    setFullDeleteWarning(null);
    setFullDeleteAffectedLinkIds([]);
    setIsWarningLoading(false);
  };

  // Cancel delete
  const handleCancelDelete = () => {
    deletePreviewRequestIdRef.current += 1;
    clearDeleteError();
    setShowDeleteModal(false);
    setDeletingGuardian(undefined);
    setDeleteScope(null);
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
  // Bis der Katalogtext des Ladefehlers da ist, bleibt der Ladekreis stehen,
  // nie eine leere Liste.
  const failedWithoutData = loadFailed && guardians.length === 0;
  if (
    (isLoading && guardians.length === 0) ||
    (failedWithoutData && formErrorMessage(loadError.error) === null)
  ) {
    return <Loading message="Bezugspersonen werden geladen …" />;
  }

  if (failedWithoutData) {
    return <LoadErrorAlert error={loadError.error} />;
  }

  return (
    <SectionCard
      className="relative z-10"
      overflow="visible"
      title="Erziehungsberechtigte"
      concept={parentsConcept}
      actions={
        <div className="flex items-center gap-2">
          {readOnly && (
            <>
              <span className="hidden sm:inline-flex">
                <StatusBadge
                  label="Nur Ansicht"
                  tone="gray"
                  icon={<Eye className="h-3 w-3 sm:h-3.5 sm:w-3.5" />}
                />
              </span>
              <span className="inline-flex sm:hidden">
                <StatusBadge
                  label="Ansicht"
                  tone="gray"
                  icon={<Eye className="h-3 w-3 sm:h-3.5 sm:w-3.5" />}
                />
              </span>
            </>
          )}
          {!readOnly && (
            <>
              <Button
                type="button"
                variant="surface"
                size="md"
                onClick={() => setIsPickerOpen(true)}
                className="gap-1"
                title="Vorhandene/n Erziehungsberechtigte/n suchen"
              >
                <Search className="h-4 w-4" />
                <span className="hidden sm:inline">Vorhandene/n suchen</span>
              </Button>
              <Button
                type="button"
                variant="primary"
                size="md"
                onClick={handleOpenCreateModal}
                data-setup-tour="guardian-add"
                className="gap-1"
                title="Erziehungsberechtigte/n hinzufügen"
              >
                <Plus className="h-4 w-4" />
                <span className="hidden sm:inline">Hinzufügen</span>
              </Button>
            </>
          )}
        </div>
      }
    >
      {/* Existing-guardian picker (sibling case): eine Kopf-Aktion mit
          Dialog, kein Formular über der Liste (#3112). Der Dialog schließt,
          sobald die Person gewählt ist; die Verknüpfung läuft dahinter. */}
      {/* oxlint-disable-next-line ui-kit/form-modal-pending -- schließt mit der Auswahl synchron; die Verknüpfung läuft danach außerhalb des Dialogs. */}
      <FormModal
        isOpen={isPickerOpen}
        onClose={() => setIsPickerOpen(false)}
        title="Vorhandene Person suchen"
        size="md"
      >
        {isPickerOpen && (
          <GuardianPickerPanel
            embedded
            onSelect={(guardian, relationship) => {
              setIsPickerOpen(false);
              void handleSelectExistingGuardian(guardian, relationship);
            }}
            onCancel={() => setIsPickerOpen(false)}
            excludeProfileIds={guardians.map((g) => g.id)}
          />
        )}
      </FormModal>

      {/* Ein gescheitertes Neuladen lässt die bekannte Liste stehen. */}
      <LoadErrorAlert error={loadError.error} className="mb-4" />

      {/* Guardian List */}
      <div className="space-y-3">
        <LoadErrorAlert error={emailStatesLoad.error} className="mb-3" />
        <GuardianList
          guardians={guardians}
          onEdit={readOnly ? undefined : handleOpenEditModal}
          onInvite={readOnly ? undefined : (g) => void handleInviteGuardian(g)}
          invitingGuardianId={invitingGuardianId}
          emailStates={emailStates}
          onStopEmails={
            readOnly
              ? undefined
              : (guardian) => setEmailBlockChange({ guardian, action: "stop" })
          }
          onLiftEmailBlock={
            readOnly
              ? undefined
              : (guardian) => setEmailBlockChange({ guardian, action: "lift" })
          }
          onResumeEmails={
            readOnly
              ? undefined
              : (guardian) =>
                  setEmailBlockChange({ guardian, action: "resume" })
          }
          readOnly={readOnly}
          showRelationship={true}
        />
      </div>

      {canSeePayment && (
        <div className="mt-6">
          <StudentPaymentCard
            studentId={studentId}
            guardians={guardians}
            readOnly={readOnly}
            onChanged={() => void loadGuardians()}
          />
        </div>
      )}

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
      <ConfirmationModal
        isOpen={emailBlockChange !== null}
        onClose={closeEmailBlockChange}
        onConfirm={() => void confirmEmailBlockChange()}
        title={emailChangeCopy?.title ?? ""}
        confirmText={emailChangeCopy?.confirm ?? ""}
        cancelText="Abbrechen"
        isConfirmLoading={isChangingEmailBlock}
      >
        <p className="text-sm text-gray-600">{emailChangeCopy?.text}</p>
        <FormErrorAlert message={emailBlockErrors.error} className="mt-3" />
      </ConfirmationModal>

      <GuardianDeleteModal
        isOpen={showDeleteModal}
        onClose={handleCancelDelete}
        guardianName={
          deletingGuardian ? getGuardianFullName(deletingGuardian) : ""
        }
        isLoading={isDeleting}
        canFullDelete={canFullDelete}
        scope={deleteScope}
        onScopeChange={handleScopeChange}
        fullDeleteWarning={fullDeleteWarning}
        isWarningLoading={isWarningLoading}
        onConfirmUnlink={handleConfirmUnlink}
        onConfirmFullDelete={handleConfirmFullDelete}
        error={deleteErrors.error}
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
    </SectionCard>
  );
}
