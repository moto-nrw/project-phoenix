"use client";

import {
  type Dispatch,
  type SetStateAction,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { AlertCircle, Check, Loader2 } from "lucide-react";
import { PencilSimpleIcon } from "@phosphor-icons/react/ssr";
import { useLocale, useTranslations } from "next-intl";

import { Input } from "~/components/ui/input";
import { Textarea } from "~/components/ui/textarea";
import { Alert } from "~/components/ui/alert";
import { Checkbox } from "~/components/ui/checkbox";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { SUPPORTED_LOCALES } from "~/i18n/locales";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  type ChildFeatures,
  type ChildMasterData,
  type MasterDataChange,
  type MasterDataChangeInput,
  getChildFeatures,
  getChildMasterData,
  submitMasterDataRequest,
  updateMasterDataField,
} from "~/lib/parent-api";
import { ParentSection } from "~/components/parent/shell/parent-section";
import { OgsVisibleBadge } from "~/components/parent/ogs-visible-badge";
import { ParentSectionSkeleton } from "~/components/parent/parent-page";
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
const CONTACT_METHODS = ["email", "phone", "mobile", "sms"] as const;

type SaveStatus = "idle" | "saving" | "saved" | "error";

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
  readonly error: string | null;
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
  const t = useTranslations("parentMasterData");

  if (loading) {
    if (area === "details") {
      return (
        <div className="grid grid-cols-1 items-start gap-5 lg:grid-cols-2 [&_.animate-pulse]:animate-none">
          <ParentSectionSkeleton rows={2} />
          <ParentSectionSkeleton rows={2} />
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
    return <Alert type="error" message={t("loadError")} />;
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
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [md, feats] = await Promise.all([
        getChildMasterData(studentId),
        getChildFeatures(studentId),
      ]);
      setData(md);
      setFeatures(feats);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      logger.warn("child_master_data_load_failed", {
        error: message,
        student_id: studentId,
      });
      setError(message);
    } finally {
      setLoading(false);
    }
  }, [studentId]);

  useEffect(() => {
    if (!enabled) return;
    void load();
  }, [enabled, load]);

  return { data, features, loading, error, setData };
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

  if (area === "departure") {
    return (
      <DepartureSection
        studentId={studentId}
        childName={childName}
        data={data}
        features={features}
        pending={pendingByField.get("departure/allowed_departure_modes")}
        onApplied={onApplied}
      />
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

  if (area === "contact") return contactSection;

  return (
    <div className="grid grid-cols-1 items-start gap-5 lg:grid-cols-2">
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
    </div>
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
  onEdited,
}: Readonly<{
  studentId: string;
  label: string;
  value: string;
  pending?: MasterDataChange;
  onEdited?: () => void;
}>) {
  const t = useTranslations("parentMasterData");
  const [editing, setEditing] = useState(false);
  // Nur Textwerte lassen sich hier ändern. Die Abholarten sind eine Tabelle
  // und werden im eigenen Abschnitt geändert.
  const editableValue =
    typeof pending?.new_value === "string" ? pending.new_value : null;
  return (
    <div className="min-w-0">
      <dt className="flex min-h-6 flex-wrap items-center gap-2 text-sm text-gray-500">
        <span>{label}</span>
        {pending && (
          <div className="flex flex-wrap items-center gap-2">
            <StatusBadge label={t("pendingBadge")} tone="orange" />
            <RequestSharingControl
              studentId={studentId}
              requestType="master_data"
              requestId={pending.id}
              isSelf={pending.is_self === true}
            />
            {pending.is_self === true && editableValue !== null && (
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
          </div>
        )}
      </dt>
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
      <dd className="mt-1 text-base font-medium break-words text-gray-900">
        {value}
      </dd>
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
  const [status, setStatus] = useState<SaveStatus>("idle");
  const [message, setMessage] = useState<string | null>(null);
  const [recipientIds, setRecipientIds] = useState<string[]>([]);
  const [requestSaved, setRequestSaved] = useState(false);
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
  const displayedModes = hasAccompanied
    ? [...DEPARTURE_REQUEST_MODES, "accompanied"]
    : DEPARTURE_REQUEST_MODES;

  useEffect(() => {
    const previous = departureBase.current;
    setModes((draft) =>
      departureModesEqual(draft, previous) || pending ? current : draft,
    );
    departureBase.current = current;
  }, [current, pending]);

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

  const submit = async () => {
    if (!changed || !requestable) return;
    setStatus("saving");
    setMessage(null);
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
      setRequestSaved(true);
      setStatus("saved");
      setMessage(t("requestSubmitted"));
      try {
        const next = await getChildMasterData(studentId);
        onApplied(next);
      } catch (refreshErr) {
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
      setMessage(t("requestError"));
    }
  };

  return (
    <ParentSection
      title={t("sections.departure", { name: childName })}
      description={t("departureDescription")}
      concept="pickup"
    >
      {pending && <StatusBadge label={t("pendingBadge")} tone="orange" />}
      {pending && (
        <RequestSharingControl
          studentId={studentId}
          requestType="master_data"
          requestId={pending.id}
          isSelf={pending.is_self === true}
        />
      )}
      {hasAccompanied && (
        <p className="text-sm text-gray-600">
          {t("departureReadOnlyAccompanied")}
        </p>
      )}
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {DEPARTURE_DAYS.map((day) => (
          <fieldset
            key={day}
            className="rounded-xl border border-gray-200 bg-gray-50/70 p-3"
          >
            <legend className="px-1 text-sm font-semibold text-gray-900">
              {t(`departureDays.${day}`)}
            </legend>
            <div className="mt-1 space-y-1">
              {displayedModes.map((mode) => (
                <label
                  key={mode}
                  htmlFor={`departure-${day}-${mode}`}
                  className={
                    hasAccompanied
                      ? "flex min-h-11 cursor-default items-center gap-3 rounded-lg px-2 text-sm text-gray-700"
                      : "flex min-h-11 cursor-pointer items-center gap-3 rounded-lg px-2 text-sm text-gray-700 hover:bg-white"
                  }
                >
                  <Checkbox
                    id={`departure-${day}-${mode}`}
                    aria-label={`${t(`departureDays.${day}`)} ${t(`departureModes.${mode}`)}`}
                    checked={
                      mode === "accompanied"
                        ? (data.allowed_departure_modes?.[day] ?? []).includes(
                            mode,
                          )
                        : (modes[day] ?? []).includes(mode)
                    }
                    disabled={!requestable}
                    onChange={() => toggle(day, mode)}
                  />
                  <span>{t(`departureModes.${mode}`)}</span>
                </label>
              ))}
            </div>
          </fieldset>
        ))}
      </div>
      {requestable && !requestSaved && (
        <RequestSharingSelector
          studentId={studentId}
          selected={recipientIds}
          onChange={setRecipientIds}
        />
      )}
      {features.master_data_request_enabled ? (
        <div className="flex flex-col-reverse items-stretch gap-3 border-t border-gray-100 pt-4 sm:flex-row sm:items-center sm:justify-end">
          {message && (
            <span
              role="status"
              className={
                status === "error"
                  ? "text-moto-red-strong text-sm"
                  : "text-moto-green-strong text-sm"
              }
            >
              {message}
            </span>
          )}
          <Button
            type="button"
            variant="primary"
            size="md"
            className="min-h-11 sm:min-h-0"
            disabled={
              !changed || !requestable || status === "saving" || requestSaved
            }
            onClick={() => void submit()}
          >
            {t("requestButton")}
          </Button>
        </div>
      ) : (
        <p className="text-xs text-gray-500">{t("requestDisabled")}</p>
      )}
      {pending && <p className="text-xs text-gray-500">{t("pendingNotice")}</p>}
    </ParentSection>
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
  const [local, setLocal] = useState(value);
  const [status, setStatus] = useState<SaveStatus>("idle");
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
    } else if (
      latestValue.current === value &&
      inFlightValue.current === null
    ) {
      setStatus("saved");
    }
  }, [value]);

  const doSave = useCallback(
    async (next: string) => {
      if (next === savedValue.current) {
        setStatus("idle");
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
        }
      } catch {
        setStatus("error");
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
    [onSave],
  );

  const handleChange = (next: string) => {
    setLocal(next);
    latestValue.current = next;
    setStatus("idle");
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
        <SaveIndicator status={status} />
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
  const [local, setLocal] = useState(value);
  const [status, setStatus] = useState<SaveStatus>("idle");
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
    } else if (
      latestValue.current === value &&
      inFlightValue.current === null
    ) {
      setStatus("saved");
    }
  }, [value]);

  const doSave = useCallback(
    async (next: string) => {
      if (next === savedValue.current) {
        setStatus("idle");
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
        }
      } catch {
        setStatus("error");
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
    [onSave],
  );

  const handleChange = (next: string) => {
    setLocal(next);
    latestValue.current = next;
    setStatus("idle");
    void doSave(next);
  };

  return (
    <div>
      <div className="flex items-center justify-between">
        <span className="text-sm font-medium text-gray-700">{label}</span>
        <SaveIndicator status={status} />
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

function SaveIndicator({ status }: Readonly<{ status: SaveStatus }>) {
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
        className="text-moto-red-strong inline-flex items-center gap-1 text-xs font-medium"
      >
        <AlertCircle className="h-3 w-3" aria-hidden="true" />
        {t("saveError")}
      </span>
    );
  }
  return null;
}
