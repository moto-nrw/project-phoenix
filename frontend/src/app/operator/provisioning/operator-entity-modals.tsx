"use client";

import {
  RestoreConfirmationModal,
  SoftDeleteConfirmationModal,
  type SoftDeletable,
} from "./soft-delete-shared";

const SCHOOL_DELETE_BULLETS = [
  "Die Schule wird deaktiviert",
  "Alle Zugänge dieser Schule werden gesperrt",
  "Die Schule kann später wiederhergestellt werden",
] as const;

const ORG_DELETE_BULLETS = [
  "Alle Schulen des Trägers müssen vorher gelöscht werden",
  "Der Träger kann später wiederhergestellt werden",
] as const;

interface SoftDeleteWrapperProps<T extends SoftDeletable> {
  readonly target: T;
  readonly inputId: string;
  readonly confirmInput: string;
  readonly onConfirmInputChange: (value: string) => void;
  readonly errorMessage: string;
  readonly isProcessing: boolean;
  readonly onCancel: () => void;
  readonly onConfirm: () => void;
  readonly confirmDisabled?: boolean;
  readonly confirmDisabledReason?: string;
}

export function SchoolSoftDeleteModal<T extends SoftDeletable>(
  props: SoftDeleteWrapperProps<T>,
) {
  return (
    <SoftDeleteConfirmationModal
      {...props}
      entityLabel="Schule"
      entityArticleAccusative="die Schule"
      nameLabel="Geben Sie den Schulnamen ein:"
      warningTitle="Folgende Aktionen werden ausgeführt:"
      warningBullets={SCHOOL_DELETE_BULLETS}
    />
  );
}

export function OrgSoftDeleteModal<T extends SoftDeletable>(
  props: SoftDeleteWrapperProps<T>,
) {
  return (
    <SoftDeleteConfirmationModal
      {...props}
      entityLabel="Träger"
      entityArticleAccusative="den Träger"
      nameLabel="Geben Sie den Trägernamen ein:"
      warningTitle="Hinweis:"
      warningBullets={ORG_DELETE_BULLETS}
    />
  );
}

interface RestoreWrapperProps<T extends SoftDeletable> {
  readonly target: T | null;
  readonly setTarget: (t: T | null) => void;
  readonly onConfirm: () => void;
  readonly isProcessing: boolean;
  readonly errorMessage?: string;
  readonly confirmDisabled?: boolean;
  readonly confirmDisabledReason?: string;
}

export function SchoolRestoreModal<T extends SoftDeletable>(
  props: RestoreWrapperProps<T>,
) {
  return (
    <RestoreConfirmationModal
      {...props}
      entityLabel="Schule"
      entityArticleAccusative="die Schule"
      entityPronounNominative="Die Schule"
      entityPossessiveAccusative="ihren"
    />
  );
}

interface OrgRestoreWrapperProps<
  T extends SoftDeletable,
> extends RestoreWrapperProps<T> {
  readonly extraMessage?: string;
}

export function OrgRestoreModal<T extends SoftDeletable>(
  props: OrgRestoreWrapperProps<T>,
) {
  return (
    <RestoreConfirmationModal
      {...props}
      entityLabel="Träger"
      entityArticleAccusative="den Träger"
      entityPronounNominative="Der Träger"
      entityPossessiveAccusative="seinen"
    />
  );
}
