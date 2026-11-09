"use client";

// Ein Dokument freigeben (#4692): für die Eltern aller Schulen, einer Schule
// oder einer Gruppe, oder für die Schulen. Unten steht, wer es danach sieht
// und wo.

import { useEffect, useMemo, useRef, useState } from "react";
import { CustomSelect } from "~/components/ui/custom-select";
import { EditActions } from "~/components/ui/edit-actions";
import { FieldLabel } from "~/components/ui/field-label";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { FormModal } from "~/components/ui/form-modal";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { useApiFormError } from "~/contexts/ToastContext";
import type { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { createLogger } from "~/lib/logger";
import {
  releaseTraegerDocument,
  type TraegerDocumentAudience,
  type TraegerDocumentRelease,
  type TraegerDocumentSchool,
} from "~/lib/traeger-documents-api";
import { releaseAudienceLabel, schoolNamesOf } from "./traeger-document-format";
import { InsetPanel } from "~/components/ui/inset-panel";

const logger = createLogger({ component: "TraegerDocumentReleaseModal" });

type Scope = "all" | "school" | "group";

const AUDIENCES = [
  { value: "parents", label: "Eltern" },
  { value: "schools", label: "Schulen" },
] as const;

const PARENT_SCOPES = [
  { value: "all", label: "Alle Schulen" },
  { value: "school", label: "Eine Schule" },
  { value: "group", label: "Eine Gruppe" },
] as const;

const SCHOOL_SCOPES = [
  { value: "all", label: "Alle Schulen" },
  { value: "school", label: "Eine Schule" },
] as const;

/** Wo die Menschen das Dokument nach der Freigabe finden. */
const WHERE: Record<TraegerDocumentAudience, string> = {
  parents: "Die Eltern finden es in der Eltern-App unter „Dokumente“.",
  schools: "Die OGS-Leitung findet es in moto unter „Dateien“.",
};

export function TraegerDocumentReleaseModal({
  isOpen,
  documentId,
  documentTitle,
  schools,
  schoolsLoading,
  schoolsLoadError,
  onClose,
  onReleased,
}: Readonly<{
  isOpen: boolean;
  documentId: string;
  documentTitle: string;
  schools: readonly TraegerDocumentSchool[];
  schoolsLoading: boolean;
  /** Ohne geladene Schulen lässt sich keine Schule und keine Gruppe wählen. */
  schoolsLoadError: ReturnType<typeof useSwrLoadError>;
  onClose: () => void;
  onReleased: (release: TraegerDocumentRelease) => void;
}>) {
  const [audience, setAudience] = useState<TraegerDocumentAudience>("parents");
  const [scope, setScope] = useState<Scope>("all");
  const [schoolId, setSchoolId] = useState("");
  const [groupId, setGroupId] = useState("");
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear: clearErrors } = formErrors;

  useEffect(() => {
    if (!isOpen) return;
    setAudience("parents");
    setScope("all");
    setSchoolId("");
    setGroupId("");
    clearErrors();
  }, [isOpen, clearErrors]);

  const school = schools.find((candidate) => candidate.id === schoolId);
  const groups = useMemo(() => school?.groups ?? [], [school]);
  const names = useMemo(() => schoolNamesOf(schools), [schools]);

  const preview = useMemo(() => {
    if (scope !== "all" && !schoolId) return null;
    if (scope === "group" && !groupId) return null;
    const group = groups.find((candidate) => candidate.id === groupId);
    return releaseAudienceLabel(
      {
        id: "preview",
        audience,
        schoolId: scope === "all" ? null : schoolId,
        groupId: scope === "group" ? groupId : null,
        groupName: group?.name ?? "",
        releasedAt: "",
      },
      names,
    );
  }, [audience, scope, schoolId, groupId, groups, names]);

  const changeAudience = (next: TraegerDocumentAudience) => {
    setAudience(next);
    if (next === "schools" && scope === "group") setScope("school");
    formErrors.clear();
  };

  const submit = async () => {
    formErrors.clear();
    const missing: Record<string, string> = {};
    if (scope !== "all" && !schoolId) {
      missing.school_id = "Bitte wählen Sie eine Schule.";
    }
    if (scope === "group" && !groupId) {
      missing.group_id = "Bitte wählen Sie eine Gruppe.";
    }
    if (Object.keys(missing).length > 0) {
      formErrors.invalid("Bitte prüfen Sie Ihre Angaben.", missing);
      return;
    }
    setSaving(true);
    try {
      onReleased(
        await releaseTraegerDocument(documentId, {
          audience,
          schoolId: scope === "all" ? null : schoolId,
          groupId: scope === "group" ? groupId : null,
        }),
      );
    } catch (err) {
      logger.error("traeger_document_release_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "die Freigabe" });
    } finally {
      setSaving(false);
    }
  };

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      closeDisabled={saving}
      isBackdropDismissDisabled
      title="Dokument freigeben"
      size="md"
      error={formErrors.error}
      footer={
        <EditActions
          onCancel={onClose}
          onSave={() => formRef.current?.requestSubmit()}
          saving={saving}
          saveLabel="Freigeben"
        />
      }
    >
      <form
        ref={formRef}
        noValidate
        className="space-y-5"
        onSubmit={(event) => {
          event.preventDefault();
          void submit();
        }}
      >
        <p className="text-sm text-gray-600">
          Sie geben „{documentTitle}“ frei. Sie können die Freigabe jederzeit
          zurücknehmen.
        </p>
        <div className="space-y-2">
          <FieldLabel>Für wen?</FieldLabel>
          <SegmentedControl
            items={AUDIENCES}
            value={audience}
            onChange={changeAudience}
            ariaLabel="Für wen?"
            fullWidth
          />
        </div>
        <div className="space-y-2">
          <FieldLabel>An welchen Schulen?</FieldLabel>
          <SegmentedControl
            items={audience === "parents" ? PARENT_SCOPES : SCHOOL_SCOPES}
            value={scope}
            onChange={(next) => {
              setScope(next);
              if (next !== "group") setGroupId("");
              formErrors.clear();
            }}
            ariaLabel="An welchen Schulen?"
            fullWidth
          />
        </div>
        {scope !== "all" && schoolsLoadError && (
          <LoadErrorAlert error={schoolsLoadError} actionLayout="responsive" />
        )}
        {scope !== "all" && !schoolsLoadError && (
          <CustomSelect
            id="traeger-document-release-school"
            name="school_id"
            label="Schule"
            required
            placeholder={
              schoolsLoading ? "Schulen werden geladen …" : "Schule wählen"
            }
            disabled={schoolsLoading}
            value={schoolId}
            error={formErrors.fieldError("school_id")}
            options={schools.map((candidate) => ({
              value: candidate.id,
              label: candidate.name,
            }))}
            onChange={(next) => {
              setSchoolId(next);
              setGroupId("");
            }}
            searchable={schools.length > 8}
          />
        )}
        {scope === "group" && !schoolsLoadError && (
          <CustomSelect
            id="traeger-document-release-group"
            name="group_id"
            label="Gruppe"
            required
            disabled={!school || groups.length === 0}
            placeholder={
              !school
                ? "Zuerst eine Schule wählen"
                : groups.length === 0
                  ? "Diese Schule hat noch keine Gruppen"
                  : "Gruppe wählen"
            }
            value={groupId}
            error={formErrors.fieldError("group_id")}
            options={groups.map((group) => ({
              value: group.id,
              label: group.name,
            }))}
            onChange={setGroupId}
            searchable={groups.length > 8}
          />
        )}
        <InsetPanel padding="md">
          <p className="text-xs font-medium text-gray-500">Sichtbar für</p>
          <p className="mt-0.5 text-sm font-semibold text-gray-900">
            {preview ?? "Bitte wählen Sie oben aus."}
          </p>
          <p className="mt-1 text-sm text-gray-600">{WHERE[audience]}</p>
        </InsetPanel>
      </form>
    </FormModal>
  );
}
