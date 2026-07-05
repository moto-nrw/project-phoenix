"use client";

import { useMemo } from "react";
import { Modal } from "~/components/ui/modal";
import { DatabaseForm } from "~/components/ui/database/database-form";
import { configToFormSection, type EntityConfig } from "@/lib/database/types";

type DatabaseFormModalConfig<T> = Pick<
  EntityConfig<T>,
  "form" | "theme" | "labels"
>;

interface DatabaseFormModalProps<T> {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly mode: "create" | "edit";
  readonly config: DatabaseFormModalConfig<T>;
  readonly onSubmit: (data: Partial<T>) => Promise<void>;
  readonly initialData?: Partial<T>;
  readonly isLoading?: boolean;
}

export function DatabaseFormModal<T>({
  isOpen,
  onClose,
  mode,
  config,
  onSubmit,
  initialData,
  isLoading,
}: DatabaseFormModalProps<T>) {
  // Stable identity: DatabaseForm resets its form state whenever the sections
  // array identity changes, which would wipe in-progress edits on every
  // parent re-render (e.g. a loading toggle after a failed save).
  const sections = useMemo(
    () => config.form.sections.map(configToFormSection),
    [config.form.sections],
  );

  // Modal renders nothing while closed; bail out before the title check so a
  // missing label only fails when the modal is actually opened, not at mount.
  if (!isOpen) return null;

  const title =
    mode === "create"
      ? config.labels.createModalTitle
      : config.labels.editModalTitle;
  if (!title) {
    // defineEntityConfig erases literal types, so a missing label can't be
    // excluded statically; fail loudly instead of rendering empty copy.
    throw new Error(`Entity config is missing the ${mode} modal title label`);
  }

  return (
    <Modal isOpen={isOpen} onClose={onClose} title={title}>
      <DatabaseForm<Partial<T>>
        theme={config.theme}
        sections={sections}
        initialData={initialData ?? config.form.defaultValues}
        onSubmit={onSubmit}
        onCancel={onClose}
        isLoading={isLoading}
        submitLabel={mode === "create" ? "Erstellen" : "Speichern"}
        stickyActions
      />
    </Modal>
  );
}
