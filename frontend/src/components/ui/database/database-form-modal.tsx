"use client";

import { Modal } from "~/components/ui/modal";
import { DatabaseForm } from "~/components/ui/database/database-form";
import { configToFormSection, type EntityConfig } from "@/lib/database/types";

type DatabaseFormModalConfig<T> = Pick<EntityConfig<T>, "form" | "theme">;

interface DatabaseFormModalProps<T> {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly title: string;
  readonly config: DatabaseFormModalConfig<T>;
  readonly onSubmit: (data: Partial<T>) => Promise<void>;
  readonly initialData?: Partial<T>;
  readonly isLoading?: boolean;
  readonly error?: string | null;
  readonly submitLabel: string;
  readonly submitButtonGradient?: string;
  readonly stickyActions?: boolean;
}

export function DatabaseFormModal<T>({
  isOpen,
  onClose,
  title,
  config,
  onSubmit,
  initialData,
  isLoading,
  error,
  submitLabel,
  submitButtonGradient,
  stickyActions,
}: DatabaseFormModalProps<T>) {
  return (
    <Modal isOpen={isOpen} onClose={onClose} title={title}>
      <DatabaseForm<Partial<T>>
        theme={config.theme}
        sections={config.form.sections.map(configToFormSection)}
        initialData={initialData ?? config.form.defaultValues}
        onSubmit={onSubmit}
        onCancel={onClose}
        isLoading={isLoading}
        error={error}
        submitLabel={submitLabel}
        submitButtonGradient={submitButtonGradient}
        stickyActions={stickyActions}
      />
    </Modal>
  );
}
