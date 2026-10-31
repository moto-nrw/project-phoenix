"use client";

import {
  type Dispatch,
  type SetStateAction,
  createContext,
  useCallback,
  useContext,
  useEffect,
  useId,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { AlertCircle, Check, Loader2 } from "lucide-react";
import { PencilSimpleIcon } from "@phosphor-icons/react/ssr";
import { useLocale, useTranslations } from "next-intl";

import { Input } from "~/components/ui/input";
import { Textarea } from "~/components/ui/textarea";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import type { FormError } from "~/components/ui/form-error";
import { Checkbox } from "~/components/ui/checkbox";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { EditActions } from "~/components/ui/edit-actions";
import { ConfirmationModal } from "~/components/ui/modal";
import { useNavigationGuard } from "~/lib/hooks/use-navigation-guard";
import { useApiFormError, useApiLoadError } from "~/contexts/ToastContext";
import { SUPPORTED_LOCALES } from "~/i18n/locales";
import { formatDate } from "~/lib/date-helpers";
import { cn } from "~/lib/utils";
import { createLogger } from "~/lib/logger";
import {
  type ChildFeatures,
  type ChildMasterData,
  type MasterDataChange,
  type MasterDataChangeInput,
  getChildMasterData,
  submitMasterDataRequest,
  updateMasterDataField,
} from "~/lib/parent-api";
import {
  ParentSection,
  ParentSubsection,
} from "~/components/parent/shell/parent-section";
import { OgsVisibleBadge } from "~/components/parent/ogs-visible-badge";
import { ParentSectionSkeleton } from "~/components/parent/parent-page";
import {
  useChildRequests,
  useSharedParentFeatures,
} from "~/components/parent/child-requests-context";
import { StatusBadge } from "~/components/ui/status-badge";
import { ChildMasterDataRequestModal } from "~/components/parent/child-master-data-request-modal";
import {
  RequestSharingControl,
  RequestSharingSelector,
} from "~/components/parent/request-sharing-control";
import { RequestEditModal } from "~/components/parent/request-edit-modal";

const logger = createLogger({ component: "ChildMasterData" });

const AUTO_SAVE_DELAY_MS = 1500;
const DEPARTURE_DAYS = ["mon", "tue", "wed", "thu", "fri"] as const;
const DEPARTURE_REQUEST_MODES = ["alone", "bus", "pickup"] as const;
const DEPARTURE_DISPLAY_MODES = [
  ...DEPARTURE_REQUEST_MODES,
  "accompanied",
] as const;
const CONTACT_METHODS = ["email", "phone", "mobile", "sms"] as const;

type SaveStatus = "idle" | "saving" | "saved" | "error";

// Auto-Save ist hier benannt („Änderungen werden automatisch gespeichert“),
// darf aber nichts verlieren (#3112): jedes Feld meldet, ob etwas noch
// ungespeichert, gerade unterwegs oder fehlgeschlagen ist. Der Abschnitt
// warnt beim Schließen des Tabs, solange etwas offen ist, und fragt vor einem
// Seitenwechsel nach, wenn eine Speicherung fehlgeschlagen ist.
type FieldSaveState = "clean" | "dirty" | "saving" | "error";
type ReportFieldState = (id: string, state: FieldSaveState) => void;
const AutoSaveTracker = createContext<ReportFieldState | null>(null);

function useReportFieldState(state: FieldSaveState) {
  const report = useContext(AutoSaveTracker);
  const id = useId();
  useEffect(() => {
    report?.(id, state);
  }, [report, id, state]);
  useEffect(
    () => () => {
      report?.(id, "clean");
    },
    [report, id],
  );
}

function useUnsavedChangesGuard(
  states: Readonly<Record<string, FieldSaveState>>,
) {
  const values = Object.values(states);
  const hasUnsaved = values.some((state) => state !== "clean");

  return useNavigationGuard(hasUnsaved);
}

interface Props {
  readonly studentId: string;
  /** Der Name des Kindes fuer die Ueberschrift "Angaben zu {Name}". */
  readonly childName: string;
  readonly area?: "details" | "departure" | "contact";
  readonly masterData?: ChildMasterDataState;
}

export interface ChildMasterDataState {
  readonly data: ChildMasterData | null;
  readonly features: ChildFeatures | null;
  readonly loading: boolean;
  /** A failed load from the shared error path (#2518), with retry. */
  readonly error: FormError | null;
  readonly setData: Dispatch<SetStateAction<ChildMasterData | null>>;
}

/**
 * "Angaben zu {Name}": die bisherigen internen Datenfelder in Elternsprache.
 *
 * Liegt seit dem Umbau als Abschnitt IM Kinderbereich statt auf einer eigenen
 * Unterseite; eine Seite pro Datenfeldgruppe hat Eltern nur Klicks gekostet.
 */
export function ChildMasterDataView({
  studentId,
  childName,
  area = "details",
  masterData,
}: Props) {
  const loadedMasterData = useChildMasterData(studentId, !masterData);
  const { data, features, loading, error, setData } =
    masterData ?? loadedMasterData;

  if (loading) {
    if (area === "details") {
      return (
        <div className="grid grid-cols-1 gap-5 lg:grid-cols-2 [&_.animate-pulse]:animate-none">
          <ParentSectionSkeleton rows={2} />
          <ParentSectionSkeleton rows={2} announce={false} />
        </div>
      );
    }
    return (
      <ParentSectionSkeleton
        rows={area === "contact" ? 5 : 3}
        className="[&_.animate-pulse]:animate-none"
      />
    );
  }

  if (error || !data || !features) {
    // Without data the error is the only content; the message arrives once
    // the catalog has loaded.
    return <LoadErrorAlert error={error} />;
  }

  return (
    <ChildMasterDataContent
      studentId={studentId}
      childName={childName}
      area={area}
      data={data}
      features={features}
      onApplied={setData}
      onDirectApplied={(target, field, next) =>
        setData((current) =>
          current
            ? mergeDirectMasterDataField(current, next, target, field)
            : next,
        )
      }
    />
  );
}

export function useChildMasterData(
  studentId: string,
  enabled = true,
): ChildMasterDataState {
  const [data, setData] = useState<ChildMasterData | null>(null);
  const [features, setFeatures] = useState<ChildFeatures | null>(null);
  const sharedFeatures = useSharedParentFeatures();
  const hasSharedFeatures = sharedFeatures !== null;
  const [loading, setLoading] = useState(true);
  const t = useTranslations("parentMasterData");
  const { error, show, clear } = useApiLoadError();
  const childRequests = useChildRequests();

  const load = useCallback(async () => {
    setLoading(true);
    clear();
    try {
      const [md, feats] = await Promise.all([
        getChildMasterData(studentId),
        hasSharedFeatures
          ? Promise.resolve(null)
          : childRequests.features(studentId),
      ]);
      setData(md);
      setFeatures(feats);
    } catch (err) {
      logger.warn("child_master_data_load_failed", {
        error: err instanceof Error ? err.message : String(err),
        student_id: studentId,
      });
      void show(err, {
        object: t("errorObject"),
        retry: () => void loadRef.current(),
      });
    } finally {
      setLoading(false);
    }
  }, [clear, childRequests, hasSharedFeatures, show, studentId, t]);
  // The retry runs the latest load, not the one of the failed attempt.
  const loadRef = useRef(load);
  useLayoutEffect(() => {
    loadRef.current = load;
  });

  useEffect(() => {
    if (!enabled) return;
    void load();
  }, [enabled, load]);

  const {
    error: featureError,
    show: showFeatureError,
    clear: clearFeatureError,
  } = useApiLoadError();
  const sharedFailure = sharedFeatures?.error;
  const reloadFeatures = sharedFeatures?.mutate;
  useEffect(() => {
    if (!enabled || !sharedFailure) {
      clearFeatureError();
      return;
    }
    void showFeatureError(sharedFailure, {
      object: t("errorObject"),
      retry: () => void reloadFeatures?.(),
    });
  }, [
    enabled,
    sharedFailure,
    reloadFeatures,
    showFeatureError,
    clearFeatureError,
    t,
  ]);
  return {
    data,
    features: sharedFeatures
      ? (sharedFeatures.data?.[studentId] ?? null)
      : features,
    loading:
      loading ||
      (!!sharedFeatures && !sharedFeatures.data && !sharedFeatures.error),
    error: error ?? featureError,
    setData,
  };
}

function ChildMasterDataContent({
  studentId,
  childName,
  area,
  data,
  features,
  onApplied,
  onDirectApplied,
}: Readonly<{
  studentId: string;
  childName: string;
  area: "details" | "departure" | "contact";
  data: ChildMasterData;
  features: ChildFeatures;
  onApplied: (next: ChildMasterData) => void;
  onDirectApplied: (
    target: string,
    field: string,
    next: ChildMasterData,
  ) => void;
}>) {
  const t = useTranslations("parentMasterData");
  const pendingByField = useMemo(() => {
    const map = new Map<string, MasterDataChange>();
    for (const c of data.pending_changes) {
      if (c.status === "pending") map.set(`${c.target}/${c.field_key}`, c);
    }
    return map;
  }, [data.pending_changes]);
  const contactEditEnabled = features.master_data_contact_edit_enabled;

  const saveField = useCallback(
    async (target: string, field: string, value: string) => {
      const next = await updateMasterDataField(studentId, target, field, value);
      onDirectApplied(target, field, next);
    },
    [studentId, onDirectApplied],
  );

  const [fieldStates, setFieldStates] = useState<
    Record<string, FieldSaveState>
  >({});
  const reportFieldState = useCallback<ReportFieldState>((id, state) => {
    setFieldStates((prev) => {
      if (prev[id] === state) return prev;
      if (state === "clean" && !(id in prev)) return prev;
      return { ...prev, [id]: state };
    });
  }, []);
  const { pendingHref, confirmNavigation, cancelNavigation } =
    useUnsavedChangesGuard(fieldStates);

  const guarded = (content: React.ReactNode) => (
    <AutoSaveTracker.Provider value={reportFieldState}>
      {content}
      <ConfirmationModal
        mobileSheet
        isOpen={pendingHref !== null}
        onClose={cancelNavigation}
        onConfirm={confirmNavigation}
        title={t("unsaved.title")}
        confirmText={t("unsaved.leave")}
        cancelText={t("unsaved.stay")}
        confirmVariant="danger"
      >
        <p className="text-sm leading-6 text-gray-600">{t("unsaved.body")}</p>
      </ConfirmationModal>
    </AutoSaveTracker.Provider>
  );

  if (area === "departure") {
    return guarded(
      <DepartureSection
        studentId={studentId}
        childName={childName}
        data={data}
        features={features}
        pending={pendingByField.get("departure/allowed_departure_modes")}
        onApplied={onApplied}
      />,
    );
  }

  const contactSection = (
    <ParentSection
      title={t("sections.contact")}
      description={t("editableHint")}
      concept="accounts"
      actions={<OgsVisibleBadge />}
    >
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <AutoSaveField
          label={t("fields.email")}
          type="email"
          value={data.email ?? ""}
          disabled={!contactEditEnabled}
          onSave={(v) => saveField("guardian_profile", "email", v)}
        />
        <AutoSaveField
          label={t("fields.phone")}
          value={data.primary_phone ?? ""}
          disabled={!contactEditEnabled}
          onSave={(v) => saveField("guardian_phone", "primary", v)}
        />
      </div>
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <AutoSaveSelect
          label={t("fields.contactMethod")}
          value={data.preferred_contact_method}
          options={CONTACT_METHODS.map((method) => ({
            value: method,
            label: t(`contactMethods.${method}`),
          }))}
          disabled={!contactEditEnabled}
          onSave={(v) =>
            saveField("guardian_profile", "preferred_contact_method", v)
          }
        />
        <AutoSaveSelect
          label={t("fields.language")}
          value={data.language_preference}
          options={SUPPORTED_LOCALES.map((locale) => ({
            value: locale.code,
            label: locale.label,
          }))}
          disabled={!contactEditEnabled}
          onSave={(v) =>
            saveField("guardian_profile", "language_preference", v)
          }
        />
      </div>
      <AutoSaveField
        label={t("fields.addressStreet")}
        value={data.address_street ?? ""}
        disabled={!contactEditEnabled}
        onSave={(v) => saveField("guardian_profile", "address_street", v)}
      />
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-[8rem_minmax(0,1fr)]">
        <AutoSaveField
          label={t("fields.addressPostalCode")}
          value={data.address_postal_code ?? ""}
          disabled={!contactEditEnabled}
          onSave={(v) =>
            saveField("guardian_profile", "address_postal_code", v)
          }
        />
        <AutoSaveField
          label={t("fields.addressCity")}
          value={data.address_city ?? ""}
          disabled={!contactEditEnabled}
          onSave={(v) => saveField("guardian_profile", "address_city", v)}
        />
      </div>
      {!contactEditEnabled && (
        <p className="text-xs text-gray-500">{t("editDisabled")}</p>
      )}
    </ParentSection>
  );

  if (area === "contact") return guarded(contactSection);

  return guarded(
    <div className="grid grid-cols-1 gap-5 lg:grid-cols-2">
      <IdentitySection
        studentId={studentId}
        data={data}
        features={features}
        pendingByField={pendingByField}
        onApplied={onApplied}
      />

      <ParentSection
        title={t("sections.health")}
        description={t("healthDirectHint")}
        concept="sick"
      >
        <AutoSaveField
          label={t("fields.healthInfo")}
          value={data.health_info ?? ""}
          name="health_info"
          multiline
          placeholder={t("healthPlaceholder")}
          disabled={!features.master_data_edit_enabled}
          onSave={(v) => saveField("student", "health_info", v)}
        />
        {!features.master_data_edit_enabled && (
          <p className="text-xs text-gray-500">{t("editDisabled")}</p>
        )}
      </ParentSection>
    </div>,
  );
}

function IdentitySection({
  studentId,
  data,
  features,
  pendingByField,
  onApplied,
}: Readonly<{
  studentId: string;
  data: ChildMasterData;
  features: ChildFeatures;
  pendingByField: Map<string, MasterDataChange>;
  onApplied: (next: ChildMasterData) => void;
}>) {
  const t = useTranslations("parentMasterData");
  const locale = useLocale();
  const [modalOpen, setModalOpen] = useState(false);

  const refresh = useCallback(() => {
    void (async () => {
      try {
        onApplied(await getChildMasterData(studentId));
      } catch (err) {
        // Deliberately silent: the edit itself was saved and the dialog has
        // confirmed it. This refresh only pulls the newest version in the
        // background; the next page load shows it anyway.
        logger.warn("master_data_refresh_failed", {
          error: err instanceof Error ? err.message : String(err),
          student_id: studentId,
        });
      }
    })();
  }, [onApplied, studentId]);

  const requestable = features.master_data_request_enabled;
  const firstNamePending = pendingByField.get("person/first_name");
  const lastNamePending = pendingByField.get("person/last_name");
  const birthdayPending = pendingByField.get("person/birthday");
  const schoolClassPending = pendingByField.get("student/school_class");

  const submit = async (
    changes: MasterDataChangeInput[],
    recipientIds: string[],
  ) => {
    const submitted = await submitMasterDataRequest(
      studentId,
      changes,
      recipientIds,
    );
    onApplied({
      ...data,
      pending_changes: mergePendingChanges(data.pending_changes, submitted),
    });
    try {
      onApplied(await getChildMasterData(studentId));
    } catch (refreshErr) {
      // Deliberately silent: the request was sent and is already shown as
      // pending from the response. The refresh only fills in details.
      const refreshText =
        refreshErr instanceof Error ? refreshErr.message : String(refreshErr);
      logger.warn("master_data_request_refresh_failed", {
        error: refreshText,
        student_id: studentId,
      });
    }
  };

  const pendingFields = new Set<IdentityFieldKey>();
  if (firstNamePending) pendingFields.add("first_name");
  if (lastNamePending) pendingFields.add("last_name");
  if (birthdayPending) pendingFields.add("birthday");
  if (schoolClassPending) pendingFields.add("school_class");

  return (
    <ParentSection
      title={t("sections.child")}
      description={t("identityDescription")}
      concept="children"
      actions={
        requestable ? (
          <Button
            type="button"
            variant="surface"
            size="md"
            className="gap-2"
            onClick={() => setModalOpen(true)}
          >
            <PencilSimpleIcon size={20} weight="bold" aria-hidden="true" />
            {t("identityRequestButton")}
          </Button>
        ) : undefined
      }
    >
      <dl className="grid grid-cols-1 gap-5 sm:grid-cols-2">
        <IdentityFact
          studentId={studentId}
          label={t("fields.firstName")}
          value={data.first_name}
          pending={firstNamePending}
          onEdited={refresh}
        />
        <IdentityFact
          studentId={studentId}
          label={t("fields.lastName")}
          value={data.last_name}
          pending={lastNamePending}
          onEdited={refresh}
        />
        <IdentityFact
          studentId={studentId}
          label={t("fields.birthday")}
          value={
            data.birthday
              ? formatDate(data.birthday, false, locale)
              : t("notSet")
          }
          pending={birthdayPending}
          formatRequested={(raw) => formatDate(raw, false, locale)}
          onEdited={refresh}
        />
        <IdentityFact
          studentId={studentId}
          label={t("fields.schoolClass")}
          value={data.school_class || t("notSet")}
          pending={schoolClassPending}
          onEdited={refresh}
        />
      </dl>
      {!requestable && (
        <p className="text-xs text-gray-500">{t("requestDisabled")}</p>
      )}
      {modalOpen && (
        <ChildMasterDataRequestModal
          studentId={studentId}
          data={data}
          pendingFields={pendingFields}
          onClose={() => setModalOpen(false)}
          onSubmit={submit}
        />
      )}
    </ParentSection>
  );
}

type IdentityFieldKey =
  "first_name" | "last_name" | "birthday" | "school_class";

function IdentityFact({
  studentId,
  label,
  value,
  pending,
  formatRequested,
  onEdited,
}: Readonly<{
  studentId: string;
  label: string;
  value: string;
  pending?: MasterDataChange;
  /** Zeigt den beantragten Rohwert so an wie den gültigen Wert. */
  formatRequested?: (raw: string) => string;
  onEdited?: () => void;
}>) {
  const t = useTranslations("parentMasterData");
  const requestTitleId = useId();
  const [editing, setEditing] = useState(false);
  // Nur Textwerte lassen sich hier ändern. Die Abholarten sind eine Tabelle
  // und werden im eigenen Abschnitt geändert.
  const editableValue =
    typeof pending?.new_value === "string" ? pending.new_value : null;
  const requestedValue = editableValue
    ? (formatRequested?.(editableValue) ?? editableValue)
    : t("notSet");
  const isSelf = pending?.is_self === true;
  return (
    <div className="min-w-0">
      <dt className="text-sm text-gray-500">{label}</dt>
      <dd className="mt-1 text-base font-medium break-words text-gray-900">
        {value}
      </dd>
      {pending && (
        <dd className="mt-3">
          <div
            role="group"
            aria-labelledby={requestTitleId}
            className="space-y-3 rounded-xl bg-gray-50 p-3"
          >
            <div>
              <p id={requestTitleId} className="text-sm text-gray-500">
                {t("pendingTitle")}
              </p>
              <div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1">
                <span className="min-w-0 text-base font-medium break-words text-gray-900">
                  {requestedValue}
                </span>
                <StatusBadge label={t("pendingBadge")} tone="orange" />
              </div>
            </div>
            {isSelf && (
              <div className="flex flex-wrap items-center gap-2">
                {editableValue !== null && (
                  <Button
                    type="button"
                    variant="outline"
                    size="md"
                    className="max-sm:min-h-11"
                    onClick={() => setEditing(true)}
                  >
                    {t("requestEdit")}
                  </Button>
                )}
                <RequestSharingControl
                  studentId={studentId}
                  requestType="master_data"
                  requestId={pending.id}
                  isSelf={isSelf}
                />
              </div>
            )}
          </div>
        </dd>
      )}
      {pending && editing && editableValue !== null && (
        <RequestEditModal
          studentId={studentId}
          request={{
            type: "master_data",
            id: pending.id,
            label,
            value: editableValue,
          }}
          onClose={() => setEditing(false)}
          onSaved={() => onEdited?.()}
        />
      )}
    </div>
  );
}

function DepartureSection({
  studentId,
  childName,
  data,
  features,
  pending,
  onApplied,
}: Readonly<{
  studentId: string;
  childName: string;
  data: ChildMasterData;
  features: ChildFeatures;
  pending?: MasterDataChange;
  onApplied: (next: ChildMasterData) => void;
}>) {
  const t = useTranslations("parentMasterData");
  const [modes, setModes] = useState(() =>
    normalizeDepartureModes(data.allowed_departure_modes),
  );
  const [editing, setEditing] = useState(false);
  const [status, setStatus] = useState<SaveStatus>("idle");
  const [message, setMessage] = useState<string | null>(null);
  const {
    error: submitError,
    show: showSubmitError,
    clear: clearSubmitError,
  } = useApiFormError();
  const [recipientIds, setRecipientIds] = useState<string[]>([]);
  const editButtonRef = useRef<HTMLButtonElement>(null);
  const firstChoiceRef = useRef<HTMLInputElement>(null);
  const focusEditButton = useRef(false);
  const current = useMemo(
    () => normalizeDepartureModes(data.allowed_departure_modes),
    [data.allowed_departure_modes],
  );
  const departureBase = useRef(current);
  const changed = useMemo(
    () => !departureModesEqual(modes, current),
    [modes, current],
  );
  const hasAccompanied = hasDepartureAccompanied(data.allowed_departure_modes);
  const requestable =
    features.master_data_request_enabled && !pending && !hasAccompanied;
  // Eine offene Anfrage oder ein abgeschalteter Antrag beendet das
  // Bearbeiten, auch wenn er während des Bearbeitens eintrifft.
  const isEditing = editing && requestable;
  const requestedModes = pending
    ? departureModesFromValue(pending.new_value)
    : undefined;
  useReportFieldState(
    !isEditing || !changed
      ? "clean"
      : status === "saving"
        ? "saving"
        : status === "error"
          ? "error"
          : "dirty",
  );

  useEffect(() => {
    if (!changed && status === "error") {
      setStatus("idle");
      setMessage(null);
      clearSubmitError();
    }
  }, [changed, clearSubmitError, status]);

  useEffect(() => {
    const previous = departureBase.current;
    setModes((draft) =>
      departureModesEqual(draft, previous) || pending ? current : draft,
    );
    departureBase.current = current;
  }, [current, pending]);

  useEffect(() => {
    if (isEditing) {
      firstChoiceRef.current?.focus();
    } else if (focusEditButton.current) {
      focusEditButton.current = false;
      editButtonRef.current?.focus();
    }
  }, [isEditing]);

  const toggle = (day: string, mode: string) => {
    setModes((prev) => {
      const next = normalizeDepartureModes(prev);
      const dayModes = new Set(next[day] ?? []);
      if (dayModes.has(mode)) {
        dayModes.delete(mode);
      } else {
        dayModes.add(mode);
      }
      next[day] = DEPARTURE_REQUEST_MODES.filter((m) => dayModes.has(m));
      if (next[day].length === 0) delete next[day];
      return next;
    });
  };

  const startEditing = () => {
    setStatus("idle");
    setMessage(null);
    clearSubmitError();
    setEditing(true);
  };

  const cancelEditing = () => {
    setModes(current);
    setRecipientIds([]);
    setStatus("idle");
    setMessage(null);
    clearSubmitError();
    focusEditButton.current = true;
    setEditing(false);
  };

  const submit = async () => {
    if (!changed || !requestable) return;
    setStatus("saving");
    setMessage(null);
    clearSubmitError();
    try {
      const submitted = await submitMasterDataRequest(
        studentId,
        [
          {
            target: "departure",
            field_key: "allowed_departure_modes",
            value: modes,
          },
        ],
        recipientIds,
      );
      onApplied({
        ...data,
        pending_changes: mergePendingChanges(data.pending_changes, submitted),
      });
      setModes(current);
      setRecipientIds([]);
      setEditing(false);
      setStatus("saved");
      setMessage(t("requestSubmitted"));
      try {
        const next = await getChildMasterData(studentId);
        onApplied(next);
      } catch (refreshErr) {
        // Deliberately silent: the request was sent and is already shown as
        // pending from the response. The refresh only fills in details.
        const refreshText =
          refreshErr instanceof Error ? refreshErr.message : String(refreshErr);
        logger.warn("master_data_departure_request_refresh_failed", {
          error: refreshText,
          student_id: studentId,
        });
      }
    } catch (err) {
      const text = err instanceof Error ? err.message : String(err);
      logger.warn("master_data_departure_request_failed", {
        error: text,
        student_id: studentId,
      });
      setStatus("error");
      void showSubmitError(err, {
        object: t("errorObjectRequest"),
        retry: () => void submitRef.current(),
      });
    }
  };
  // The retry sends the latest selection, not the one of the failed attempt.
  const submitRef = useRef(submit);
  useLayoutEffect(() => {
    submitRef.current = submit;
  });

  return (
    <ParentSection
      title={t("sections.departure", { name: childName })}
      description={t("departureDescription")}
      concept="pickup"
      actions={
        requestable && !isEditing ? (
          <Button
            ref={editButtonRef}
            type="button"
            variant="surface"
            size="md"
            className="gap-2"
            onClick={startEditing}
          >
            <PencilSimpleIcon size={20} weight="bold" aria-hidden="true" />
            {t("departureEdit")}
          </Button>
        ) : undefined
      }
    >
      {isEditing ? (
        <>
          <div
            data-parent-tour="child-departure"
            className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3"
          >
            {DEPARTURE_DAYS.map((day, dayIndex) => (
              <fieldset
                key={day}
                className="rounded-xl border border-gray-200 bg-gray-50/70 p-3"
              >
                <legend className="px-1 text-sm font-semibold text-gray-900">
                  {t(`departureDays.${day}`)}
                </legend>
                <div className="mt-1 space-y-1">
                  {DEPARTURE_REQUEST_MODES.map((mode, modeIndex) => (
                    <label
                      key={mode}
                      htmlFor={`departure-${day}-${mode}`}
                      className="flex min-h-11 cursor-pointer items-center gap-3 rounded-lg px-2 text-sm text-gray-700 hover:bg-white"
                    >
                      <Checkbox
                        ref={
                          dayIndex === 0 && modeIndex === 0
                            ? firstChoiceRef
                            : undefined
                        }
                        id={`departure-${day}-${mode}`}
                        aria-label={`${t(`departureDays.${day}`)} ${t(`departureModes.${mode}`)}`}
                        checked={(modes[day] ?? []).includes(mode)}
                        disabled={status === "saving"}
                        onChange={() => toggle(day, mode)}
                      />
                      <span>{t(`departureModes.${mode}`)}</span>
                    </label>
                  ))}
                </div>
              </fieldset>
            ))}
          </div>
          <RequestSharingSelector
            studentId={studentId}
            selected={recipientIds}
            onChange={setRecipientIds}
          />
          <FormErrorAlert message={submitError} />
          <EditActions
            className="border-t border-gray-100 pt-4 max-sm:flex-col-reverse max-sm:[&>*]:min-h-11 max-sm:[&>*]:w-full"
            onCancel={cancelEditing}
            onSave={() => void submit()}
            saving={status === "saving"}
            disabled={!changed}
            saveLabel={t("requestButton")}
            savingLabel={t("departureSubmitting")}
            cancelLabel={t("departureCancel")}
          />
        </>
      ) : (
        <>
          <DepartureDays modes={data.allowed_departure_modes} tourTarget />
          {hasAccompanied && (
            <p className="text-sm text-gray-600">
              {t("departureReadOnlyAccompanied")}
            </p>
          )}
          {message && (
            <p role="status" className="text-moto-green-strong text-sm">
              {message}
            </p>
          )}
          {pending && (
            <ParentSubsection
              title={t("pendingTitle")}
              actions={
                // Der Wrapper nimmt die volle Breite der Aktionsspalte auf
                // dem Handy, das Badge behält seine eigene Breite.
                <div>
                  <StatusBadge label={t("pendingBadge")} tone="orange" />
                </div>
              }
            >
              {requestedModes && (
                <DepartureDays modes={requestedModes} nested />
              )}
              <p className="text-sm text-gray-500">
                {t("departurePendingNotice")}
              </p>
              <RequestSharingControl
                studentId={studentId}
                requestType="master_data"
                requestId={pending.id}
                isSelf={pending.is_self === true}
              />
            </ParentSubsection>
          )}
          {!features.master_data_request_enabled && (
            <p className="text-xs text-gray-500">{t("requestDisabled")}</p>
          )}
        </>
      )}
    </ParentSection>
  );
}

/** Der Heimweg je Wochentag als reine Anzeige, ohne Auswahlfelder. */
function DepartureDays({
  modes,
  tourTarget = false,
  nested = false,
}: Readonly<{
  modes?: Record<string, readonly string[]>;
  tourTarget?: boolean;
  /** Auf grauem Grund (offene Anfrage) heben sich die Tage weiß ab. */
  nested?: boolean;
}>) {
  const t = useTranslations("parentMasterData");
  return (
    <dl
      data-parent-tour={tourTarget ? "child-departure" : undefined}
      className="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-5"
    >
      {DEPARTURE_DAYS.map((day) => {
        const dayModes = DEPARTURE_DISPLAY_MODES.filter((mode) =>
          (modes?.[day] ?? []).includes(mode),
        );
        return (
          <div
            key={day}
            className={cn(
              "flex items-baseline justify-between gap-3 rounded-xl px-3 py-2.5 sm:block",
              nested ? "bg-white" : "bg-gray-50",
            )}
          >
            <dt className="text-sm text-gray-500">
              {t(`careSchedule.weekdays.${day}`)}
            </dt>
            <dd className="text-right text-sm font-medium text-gray-900 sm:mt-1 sm:text-left">
              {dayModes.length > 0
                ? dayModes.map((mode) => t(`departureModes.${mode}`)).join(", ")
                : t("departureModes.none")}
            </dd>
          </div>
        );
      })}
    </dl>
  );
}

function AutoSaveField({
  label,
  value,
  name,
  type = "text",
  multiline = false,
  placeholder,
  disabled = false,
  onSave,
}: Readonly<{
  label: string;
  value: string;
  name?: string;
  type?: string;
  multiline?: boolean;
  placeholder?: string;
  disabled?: boolean;
  onSave: (value: string) => Promise<void>;
}>) {
  const t = useTranslations("parentMasterData");
  const { error: saveError, show: showSaveError, clear } = useApiFormError();
  const [local, setLocal] = useState(value);
  const [status, setStatus] = useState<SaveStatus>("idle");
  // Ungespeicherte Eingabe (Debounce läuft oder ein Speichern steht aus).
  const [dirty, setDirty] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const savedValue = useRef(value);
  const latestValue = useRef(value);
  const inFlightValue = useRef<string | null>(null);
  const queuedValue = useRef<string | null>(null);

  useEffect(() => {
    const previousSaved = savedValue.current;
    savedValue.current = value;
    if (
      inFlightValue.current === null &&
      latestValue.current === previousSaved
    ) {
      latestValue.current = value;
      setLocal(value);
      setDirty(false);
    } else if (
      latestValue.current === value &&
      inFlightValue.current === null
    ) {
      setStatus("saved");
      clear();
    }
  }, [clear, value]);

  const doSave = useCallback(
    async (next: string) => {
      if (next === savedValue.current) {
        setStatus("idle");
        clear();
        return;
      }
      if (inFlightValue.current === next) {
        return;
      }
      if (inFlightValue.current !== null) {
        queuedValue.current = next;
        setStatus("saving");
        return;
      }

      inFlightValue.current = next;
      setStatus("saving");
      try {
        await onSave(next);
        savedValue.current = next;
        if (latestValue.current === next) {
          setStatus("saved");
          setDirty(false);
          clear();
        }
      } catch (err) {
        if (latestValue.current === next && next !== savedValue.current) {
          setStatus("error");
          // The field's own "Erneut versuchen" sends the latest value, so the
          // alert carries no second retry.
          void showSaveError(err, { object: t("errorObjectChange") });
        }
      } finally {
        inFlightValue.current = null;
        const queued = queuedValue.current;
        queuedValue.current = null;
        if (
          queued !== null &&
          queued === latestValue.current &&
          queued !== savedValue.current
        ) {
          void doSave(queued);
        }
      }
    },
    [clear, onSave, showSaveError, t],
  );

  useReportFieldState(
    status === "saving"
      ? "saving"
      : status === "error"
        ? "error"
        : dirty
          ? "dirty"
          : "clean",
  );

  const retry = () => {
    if (latestValue.current !== savedValue.current) {
      void doSave(latestValue.current);
    }
  };

  const handleChange = (next: string) => {
    setLocal(next);
    latestValue.current = next;
    setStatus("idle");
    clear();
    setDirty(next !== savedValue.current);
    if (timer.current) clearTimeout(timer.current);
    if (inFlightValue.current !== null) {
      queuedValue.current = next;
    } else {
      timer.current = setTimeout(() => void doSave(next), AUTO_SAVE_DELAY_MS);
    }
  };

  const handleBlur = () => {
    if (timer.current) {
      clearTimeout(timer.current);
      timer.current = null;
    }
    if (local !== savedValue.current && inFlightValue.current !== local) {
      void doSave(local);
    }
  };

  return (
    <div>
      <div className="flex items-center justify-between">
        <span className="text-sm font-medium text-gray-700">{label}</span>
        <SaveIndicator status={status} onRetry={retry} />
      </div>
      <div className="mt-1">
        {multiline ? (
          <Textarea
            name={name}
            aria-label={label}
            rows={5}
            className="min-h-32"
            placeholder={placeholder}
            value={local}
            disabled={disabled}
            onChange={(e) => handleChange(e.target.value)}
            onBlur={handleBlur}
          />
        ) : (
          <Input
            name={name}
            aria-label={label}
            controlSize="compact"
            type={type}
            value={local}
            disabled={disabled}
            onChange={(e) => handleChange(e.target.value)}
            onBlur={handleBlur}
          />
        )}
      </div>
      <FormErrorAlert message={saveError} className="mt-2" />
    </div>
  );
}

function AutoSaveSelect({
  label,
  value,
  options,
  disabled = false,
  onSave,
}: Readonly<{
  label: string;
  value: string;
  options: readonly { value: string; label: string }[];
  disabled?: boolean;
  onSave: (value: string) => Promise<void>;
}>) {
  const t = useTranslations("parentMasterData");
  const { error: saveError, show: showSaveError, clear } = useApiFormError();
  const [local, setLocal] = useState(value);
  const [status, setStatus] = useState<SaveStatus>("idle");
  const [dirty, setDirty] = useState(false);
  const savedValue = useRef(value);
  const latestValue = useRef(value);
  const inFlightValue = useRef<string | null>(null);
  const queuedValue = useRef<string | null>(null);

  useEffect(() => {
    const previousSaved = savedValue.current;
    savedValue.current = value;
    if (
      inFlightValue.current === null &&
      latestValue.current === previousSaved
    ) {
      latestValue.current = value;
      setLocal(value);
      setDirty(false);
    } else if (
      latestValue.current === value &&
      inFlightValue.current === null
    ) {
      setStatus("saved");
      clear();
    }
  }, [clear, value]);

  const doSave = useCallback(
    async (next: string) => {
      if (next === savedValue.current) {
        setStatus("idle");
        clear();
        return;
      }
      if (inFlightValue.current === next) {
        return;
      }
      if (inFlightValue.current !== null) {
        queuedValue.current = next;
        setStatus("saving");
        return;
      }

      inFlightValue.current = next;
      setStatus("saving");
      try {
        await onSave(next);
        savedValue.current = next;
        if (latestValue.current === next) {
          setStatus("saved");
          setDirty(false);
          clear();
        }
      } catch (err) {
        if (latestValue.current === next && next !== savedValue.current) {
          setStatus("error");
          // The field's own "Erneut versuchen" sends the latest value, so the
          // alert carries no second retry.
          void showSaveError(err, { object: t("errorObjectChange") });
        }
      } finally {
        inFlightValue.current = null;
        const queued = queuedValue.current;
        queuedValue.current = null;
        if (
          queued !== null &&
          queued === latestValue.current &&
          queued !== savedValue.current
        ) {
          void doSave(queued);
        }
      }
    },
    [clear, onSave, showSaveError, t],
  );

  useReportFieldState(
    status === "saving"
      ? "saving"
      : status === "error"
        ? "error"
        : dirty
          ? "dirty"
          : "clean",
  );

  const retry = () => {
    if (latestValue.current !== savedValue.current) {
      void doSave(latestValue.current);
    }
  };

  const handleChange = (next: string) => {
    setLocal(next);
    latestValue.current = next;
    setStatus("idle");
    clear();
    setDirty(next !== savedValue.current);
    void doSave(next);
  };

  return (
    <div>
      <div className="flex items-center justify-between">
        <span className="text-sm font-medium text-gray-700">{label}</span>
        <SaveIndicator status={status} onRetry={retry} />
      </div>
      <div className="mt-1">
        <CustomSelect
          ariaLabel={label}
          value={local}
          options={options}
          disabled={disabled}
          onChange={handleChange}
        />
      </div>
      <FormErrorAlert message={saveError} className="mt-2" />
    </div>
  );
}

function mergeDirectMasterDataField(
  current: ChildMasterData,
  next: ChildMasterData,
  target: string,
  field: string,
): ChildMasterData {
  if (target === "student" && field === "health_info") {
    return { ...current, health_info: next.health_info };
  }
  if (target === "guardian_phone" && field === "primary") {
    return { ...current, primary_phone: next.primary_phone };
  }
  if (target !== "guardian_profile") {
    return current;
  }
  switch (field) {
    case "email":
      return { ...current, email: next.email };
    case "address_street":
      return { ...current, address_street: next.address_street };
    case "address_city":
      return { ...current, address_city: next.address_city };
    case "address_postal_code":
      return { ...current, address_postal_code: next.address_postal_code };
    case "preferred_contact_method":
      return {
        ...current,
        preferred_contact_method: next.preferred_contact_method,
      };
    case "language_preference":
      return { ...current, language_preference: next.language_preference };
    default:
      return current;
  }
}

function mergePendingChanges(
  current: readonly MasterDataChange[],
  submitted: readonly MasterDataChange[],
): MasterDataChange[] {
  if (submitted.length === 0) return [...current];
  const byKey = new Map<string, MasterDataChange>();
  for (const change of current) {
    byKey.set(`${change.target}/${change.field_key}`, change);
  }
  for (const change of submitted) {
    byKey.set(`${change.target}/${change.field_key}`, change);
  }
  return Array.from(byKey.values());
}

function normalizeDepartureModes(
  modes?: Record<string, readonly string[]>,
): Record<string, string[]> {
  const out: Record<string, string[]> = {};
  for (const day of DEPARTURE_DAYS) {
    const allowed = new Set(modes?.[day] ?? []);
    const ordered = DEPARTURE_REQUEST_MODES.filter((mode) => allowed.has(mode));
    if (ordered.length > 0) out[day] = ordered;
  }
  return out;
}

/** Liest den beantragten Heimweg aus `new_value`; Unbekanntes fällt weg. */
function departureModesFromValue(
  value: unknown,
): Record<string, readonly string[]> | undefined {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return undefined;
  }
  const out: Record<string, string[]> = {};
  for (const [day, modes] of Object.entries(value)) {
    if (Array.isArray(modes)) {
      out[day] = modes.filter(
        (mode): mode is string => typeof mode === "string",
      );
    }
  }
  return out;
}

function hasDepartureAccompanied(modes?: Record<string, readonly string[]>) {
  return DEPARTURE_DAYS.some((day) =>
    (modes?.[day] ?? []).includes("accompanied"),
  );
}

function departureModesEqual(
  a: Record<string, readonly string[]>,
  b: Record<string, readonly string[]>,
) {
  for (const day of DEPARTURE_DAYS) {
    const left = a[day] ?? [];
    const right = b[day] ?? [];
    if (left.length !== right.length) return false;
    for (let i = 0; i < left.length; i++) {
      if (left[i] !== right[i]) return false;
    }
  }
  return true;
}

function SaveIndicator({
  status,
  onRetry,
}: Readonly<{ status: SaveStatus; onRetry?: () => void }>) {
  const t = useTranslations("parentMasterData");
  if (status === "saving") {
    return (
      <span
        role="status"
        className="inline-flex items-center gap-1 text-xs text-gray-500"
      >
        <Loader2 className="h-3 w-3 animate-spin" aria-hidden="true" />
      </span>
    );
  }
  if (status === "saved") {
    return (
      <span
        role="status"
        className="text-moto-green-strong inline-flex items-center gap-1 text-xs font-medium"
      >
        <Check className="h-3 w-3" aria-hidden="true" />
        {t("saved")}
      </span>
    );
  }
  if (status === "error") {
    return (
      <span
        role="status"
        className="text-moto-red-strong inline-flex flex-wrap items-center gap-1 text-xs font-medium"
      >
        <AlertCircle className="h-3 w-3" aria-hidden="true" />
        {onRetry && (
          <Button
            type="button"
            variant="ghost"
            size="compact"
            onClick={onRetry}
            className="text-moto-red-strong"
          >
            {t("unsaved.retry")}
          </Button>
        )}
      </span>
    );
  }
  return null;
}
