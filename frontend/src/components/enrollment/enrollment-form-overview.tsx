"use client";

import {
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import {
  Check,
  Eye,
  ExternalLink,
  Lock,
  Pencil,
  Plus,
  TextCursorInput,
  Trash2,
} from "lucide-react";
import { isRenameOf } from "~/components/enrollment/form-template-draft";
import {
  openTemplatePreview,
  useTemplatePreviewHref,
} from "~/components/enrollment/form-template-preview";
import { Button, buttonClassName } from "~/components/ui/button";
import { ConceptIconTile } from "~/components/ui/concept-icon-tile";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import type { FormErrorInput } from "~/components/ui/form-error";
import { FormModal } from "~/components/ui/form-modal";
import { useFormModalPending } from "~/components/ui/use-form-modal-pending";
import { IconTile } from "~/components/ui/icon-tile";
import { Input } from "~/components/ui/input";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { useApiFormError } from "~/contexts/ToastContext";
import { calendarPeriodsConcept } from "~/lib/concepts/calendarPeriods";
import { enrollmentsConcept } from "~/lib/concepts/enrollments";
import { permissionsConcept } from "~/lib/concepts/permissions";
import { formatBerlinDate } from "~/lib/date-helpers";
import type { FormSchema } from "~/lib/enrollment-form-schema-api";
import type { Phase } from "~/lib/enrollment-phase-api";
import { createLogger } from "~/lib/logger";
import { useTenantAwarePath } from "~/lib/tenant-path";

const logger = createLogger({ component: "EnrollmentFormEditor" });

export function EnrollmentFormsOverview({
  templates,
  phases,
  onCreate,
  onEdit,
  onPreview,
  onRename,
  onDelete,
}: Readonly<{
  templates: FormSchema[];
  phases: Phase[];
  onCreate: () => void;
  onEdit: (schema: FormSchema) => void;
  onPreview: (schema: FormSchema) => void;
  onRename: (schema: FormSchema) => void;
  onDelete: (schema: FormSchema) => void;
}>) {
  const assignedTemplateCount = templates.filter((schema) =>
    phases.some((phase) => phase.form_schema_id === schema.id),
  ).length;

  return (
    // Flex-Spalte: als Editor-Wurzel einer Tenant-Seite reicht sie den
    // Platz an die Fläche weiter, die bis zur Unterkante wächst
    // (`.moto-tenant-body`).
    <div className="flex flex-col space-y-5">
      {/* Karte als Flex-Spalte und das Raster darin `flex-1`: wächst die
          Karte bis zur Unterkante, läuft die graue Seitenspalte mit bis
          dorthin, statt über dem Kartenrand zu enden. */}
      <SectionCard
        padding="none"
        className="flex flex-col"
        bodyClassName="flex min-h-0 flex-1 flex-col"
      >
        <div className="grid flex-1 gap-0 lg:grid-cols-[minmax(0,1fr)_360px] xl:grid-cols-[minmax(0,1fr)_400px]">
          <div className="compact:p-4 space-y-6 p-5 max-sm:p-4">
            <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
              <div>
                <h2 className="text-base font-semibold text-gray-900">
                  Anmeldeformulare verwalten
                </h2>
                <p className="mt-2 max-w-3xl text-sm leading-6 text-gray-600">
                  Das Basisformular ist immer vorhanden. Eigene Vorlagen nutzen
                  Sie, wenn eine Anmeldephase abweichende Pflichtangaben oder
                  Zusatzfragen braucht.
                </p>
              </div>
              <Button
                type="button"
                onClick={onCreate}
                variant="primary"
                size="md"
                className="inline-flex items-center justify-center gap-2 whitespace-nowrap"
              >
                <Plus className="h-4 w-4" aria-hidden="true" />
                Neue Vorlage
              </Button>
            </div>

            <section className="space-y-3">
              <div className="flex items-end justify-between gap-3">
                <div>
                  <h3 className="text-base font-semibold text-gray-900">
                    Formularvorlagen
                  </h3>
                  <p className="mt-1 text-sm text-gray-600">
                    Jede Anmeldephase nutzt entweder das Basisformular oder eine
                    eigene Vorlage mit angepassten Pflichtangaben und
                    Zusatzfragen.
                  </p>
                </div>
              </div>

              <TemplateOverviewList
                templates={templates}
                phases={phases}
                onEdit={onEdit}
                onPreview={onPreview}
                onRename={onRename}
                onDelete={onDelete}
              />
            </section>
          </div>

          <aside className="moto-dotted-background moto-dotted-background--split compact:p-4 border-t border-gray-100 p-5 max-sm:p-4 lg:border-t-0 lg:border-l">
            <OverviewGuide
              templateCount={templates.length}
              assignedTemplateCount={assignedTemplateCount}
            />
          </aside>
        </div>
      </SectionCard>
    </div>
  );
}

function TemplateOverviewList({
  templates,
  phases,
  onEdit,
  onPreview,
  onRename,
  onDelete,
}: Readonly<{
  templates: FormSchema[];
  phases: Phase[];
  onEdit: (schema: FormSchema) => void;
  onPreview: (schema: FormSchema) => void;
  onRename: (schema: FormSchema) => void;
  onDelete: (schema: FormSchema) => void;
}>) {
  return (
    <SectionCard padding="none">
      <div className="divide-y divide-gray-100">
        <BaseTemplateOverviewRow />
        {templates.map((schema) => (
          <TemplateOverviewRow
            key={schema.id}
            schema={schema}
            onEdit={() => onEdit(schema)}
            onPreview={() => onPreview(schema)}
            onRename={() => onRename(schema)}
            onDelete={() => onDelete(schema)}
            isAssigned={phases.some(
              (phase) => phase.form_schema_id === schema.id,
            )}
          />
        ))}
      </div>
    </SectionCard>
  );
}

function BaseTemplateOverviewRow() {
  const tenantPath = useTenantAwarePath();

  return (
    <article className="grid gap-4 px-4 py-4 md:grid-cols-[minmax(0,1fr)_auto] md:items-center">
      <div className="min-w-0">
        <div className="flex items-center gap-2">
          <IconTile size="xs" icon={<Lock aria-hidden="true" />} />
          <h4 className="truncate text-sm font-semibold text-gray-900">
            Basisformular
          </h4>
          <StatusBadge tone="gray" compact label="System" />
        </div>
        <p className="mt-1 text-xs leading-5 text-gray-500">
          Elternteil, Kind, Klassenstufe und gewünschtes Betreuungsangebot.
        </p>
        <div className="mt-3 flex flex-wrap gap-x-4 gap-y-1 text-xs text-gray-500">
          <UsageLine title="Standard" status="base" />
          <span>Systemformular</span>
          <span>Keine Anpassungen</span>
        </div>
      </div>
      <div className="flex justify-start gap-2 md:justify-end">
        <a
          href={tenantPath("/anmeldung/preview?base=1")}
          target="_blank"
          rel="noreferrer"
          className={buttonClassName({
            variant: "outline",
            size: "md",
            className: "gap-2",
          })}
        >
          <ExternalLink className="h-4 w-4" aria-hidden="true" />
          Vorschau
        </a>
      </div>
    </article>
  );
}

function TemplateOverviewRow({
  schema,
  onEdit,
  onPreview,
  onRename,
  onDelete,
  isAssigned,
}: Readonly<{
  schema: FormSchema;
  onEdit: () => void;
  onPreview: () => void;
  onRename: () => void;
  onDelete: () => void;
  isAssigned: boolean;
}>) {
  const previewHref = useTemplatePreviewHref();
  const requiredCount = schema.fields.filter((field) =>
    Boolean(field.required),
  ).length;
  const usageStatus = isAssigned ? "assigned" : "ready";
  const questionLabel =
    schema.fields.length === 1 ? "1 Frage" : `${schema.fields.length} Fragen`;
  const usageTitle = isAssigned ? "In Phase verwendet" : "Nicht verwendet";

  return (
    <article className="grid gap-4 px-4 py-4 md:grid-cols-[minmax(0,1fr)_auto] md:items-center">
      <div className="min-w-0">
        <div className="flex items-center gap-2">
          <ConceptIconTile concept={enrollmentsConcept} size="xs" />
          <h4 className="truncate text-sm font-semibold text-gray-900">
            {schema.name}
          </h4>
        </div>
        <p className="mt-1 text-xs leading-5 text-gray-500">
          Eigene Vorlage für Pflichtangaben und Zusatzfragen.
        </p>
        <div className="mt-3 flex flex-wrap gap-x-4 gap-y-1 text-xs text-gray-500">
          <UsageLine title={usageTitle} status={usageStatus} />
          <span>Erstellt {formatBerlinDate(schema.created_at)}</span>
          <span>{questionLabel}</span>
          {requiredCount > 0 ? <span>{requiredCount} Pflicht</span> : null}
        </div>
      </div>
      <div className="flex justify-start gap-2 md:justify-end">
        <OverflowMenu
          ariaLabel={`Aktionen für ${schema.name}`}
          items={[
            {
              label: "Prüfen",
              icon: <Eye className="h-4 w-4" aria-hidden />,
              onClick: onPreview,
            },
            {
              label: "Vorschau",
              icon: <ExternalLink className="h-4 w-4" aria-hidden />,
              onClick: () => openTemplatePreview(previewHref(schema.id)),
            },
            {
              label: "Bearbeiten",
              icon: <Pencil className="h-4 w-4" aria-hidden />,
              onClick: onEdit,
            },
            {
              label: "Umbenennen",
              icon: <TextCursorInput className="h-4 w-4" aria-hidden />,
              onClick: onRename,
            },
            {
              label: "Löschen",
              icon: <Trash2 className="h-4 w-4" aria-hidden />,
              onClick: onDelete,
              destructive: true,
            },
          ]}
        />
      </div>
    </article>
  );
}

type TemplateStatus = "base" | "assigned" | "ready";

function UsageLine({
  title,
  status,
}: Readonly<{
  title: string;
  status: TemplateStatus;
}>) {
  const statusMap: Record<TemplateStatus, { textClassName: string }> = {
    base: {
      textClassName: "text-gray-700",
    },
    assigned: {
      textClassName: "text-moto-green-vivid",
    },
    ready: {
      textClassName: "text-gray-700",
    },
  };
  const config = statusMap[status];

  return (
    <span
      className={`inline-flex items-center gap-2 font-medium ${config.textClassName}`}
    >
      {title}
    </span>
  );
}

function OverviewGuide({
  templateCount,
  assignedTemplateCount,
}: Readonly<{
  templateCount: number;
  assignedTemplateCount: number;
}>) {
  return (
    <div className="sticky top-6 space-y-4">
      <div>
        <h2 className="text-base font-semibold text-gray-900">
          Erst prüfen, dann erweitern
        </h2>
        <p className="mt-2 text-sm leading-6 text-gray-600">
          Legen Sie nur dann eine eigene Vorlage an, wenn das Basisformular
          nicht reicht. So bleibt die Elternansicht kurz und verständlich.
        </p>
      </div>

      {/* Der Erklärtext stand als freier Absatz unter der Karte und ist jetzt
          ihre description. */}
      <SectionCard
        title="Nächste Schritte"
        description="Für eine neue Halbjahresanmeldung brauchen Sie oft keine eigene Formularvorlage. Anmeldephase und Betreuungsangebote steuern den eigentlichen Ablauf."
        headingLevel={3}
      >
        <div className="space-y-3">
          <GuideStep
            icon={<MotoConceptIcon concept={permissionsConcept} size={18} />}
            title="Basisformular prüfen"
            done
          />
          <GuideStep
            icon={<MotoConceptIcon concept={enrollmentsConcept} size={18} />}
            title="Vorlage nur bei Bedarf"
            done={templateCount > 0}
          />
          <GuideStep
            icon={
              <MotoConceptIcon concept={calendarPeriodsConcept} size={18} />
            }
            title="Vorlage in Anmeldephase wählen"
            done={assignedTemplateCount > 0}
          />
        </div>
      </SectionCard>
    </div>
  );
}

export function GuideStep({
  icon,
  title,
  done,
}: Readonly<{ icon: ReactNode; title: string; done: boolean }>) {
  return (
    <div className="flex items-center gap-3">
      <IconTile size="xs" tone={done ? "green" : "neutral"} icon={icon} />
      <span className="min-w-0 flex-1 text-sm font-medium text-gray-700">
        {title}
      </span>
      {done && (
        <>
          <Check
            className="text-moto-green-strong h-4 w-4 shrink-0"
            aria-hidden="true"
          />
          <span className="sr-only">Erledigt</span>
        </>
      )}
    </div>
  );
}

export function DeleteSchemaDialog({
  schema,
  deleting,
  error,
  onClose,
  onConfirm,
}: Readonly<{
  schema: FormSchema | null;
  deleting: boolean;
  error: FormErrorInput;
  onClose: () => void;
  onConfirm: () => void;
}>) {
  return (
    <ConfirmDeleteModal
      isOpen={schema !== null}
      title="Formularvorlage löschen"
      description={
        <div className="space-y-3 leading-6">
          <p>
            Die Vorlage{" "}
            <span className="font-semibold text-gray-900">{schema?.name}</span>{" "}
            wird dauerhaft gelöscht.
          </p>
          <p>
            Dabei werden alle Versionen dieser Vorlage entfernt. Bereits
            verwendete Vorlagen können nicht gelöscht werden.
          </p>
        </div>
      }
      gate={{ mode: "twoStep" }}
      onConfirm={onConfirm}
      onClose={onClose}
      loading={deleting}
      error={error}
    />
  );
}

export function RenameSchemaDialog({
  schema,
  onClose,
  onConfirm,
}: Readonly<{
  schema: FormSchema | null;
  onClose: () => void;
  onConfirm: (newName: string) => Promise<void>;
}>) {
  const [value, setValue] = useState("");
  const save = useFormModalPending(schema !== null, schema?.id);
  const submitting = save.pending;
  const formRef = useRef<HTMLFormElement>(null);
  const {
    error,
    show: showError,
    fieldError,
    clear: clearError,
  } = useApiFormError(formRef);

  // Reset the field to the current name each time the dialog opens for a
  // schema so the admin edits from the existing value.
  useEffect(() => {
    if (schema) {
      setValue(schema.name);
      clearError();
    }
  }, [schema, clearError]);

  const isOpen = schema !== null;
  const trimmed = value.trim();
  const canSubmit = isRenameOf(schema, value) && !submitting;

  // "Wiederholen" sends the name as it is then.
  const submitRef = useRef<() => Promise<void>>(async () => undefined);
  const submit = async () => {
    if (!canSubmit) return;
    clearError();
    await save.run(async (op) => {
      try {
        await onConfirm(trimmed);
        // Eine späte Antwort schließt nicht den Dialog einer anderen Vorlage
        // (#6533).
        if (op.isCurrent()) onClose();
      } catch (err) {
        logger.warn("schema_rename_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (!op.isCurrent()) return;
        void showError(err, {
          object: "die Formularvorlage",
          retry: () => void submitRef.current(),
        });
      }
    });
  };
  useLayoutEffect(() => {
    submitRef.current = submit;
  });

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      pending={save}
      title="Formular umbenennen"
      size="sm"
      error={error}
      footer={
        <div className="flex justify-end gap-2">
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={onClose}
            disabled={submitting}
          >
            Abbrechen
          </Button>
          <Button
            type="button"
            variant="primary"
            size="md"
            onClick={() => void submit()}
            disabled={!canSubmit}
            isLoading={submitting}
          >
            Speichern
          </Button>
        </div>
      }
    >
      <form
        ref={formRef}
        onSubmit={(event) => {
          event.preventDefault();
          void submit();
        }}
        className="space-y-3"
      >
        <Input
          name="name"
          label="Name"
          type="text"
          value={value}
          onChange={(event) => setValue(event.target.value)}
          error={fieldError("name")}
          placeholder="z. B. Ferienbetreuung Sommer 2026"
          autoFocus
        />
        <p className="text-xs leading-5 text-gray-500">
          Der neue Name gilt für alle Versionen dieser Vorlage. Bereits
          abgeschickte Anmeldungen bleiben unverändert.
        </p>
      </form>
    </FormModal>
  );
}
