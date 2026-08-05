"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { AlertCircle, ArrowLeft, Check, Clock, Loader2 } from "lucide-react";
import { useTranslations } from "next-intl";

import { ISODatePicker } from "~/components/ui/date-picker";
import { Input } from "~/components/ui/input";
import { todayISO } from "~/lib/date-helpers";
import { useLocalizedDatePicker } from "~/lib/hooks/use-localized-date-picker";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { SUPPORTED_LOCALES } from "~/i18n/locales";
import { createLogger } from "~/lib/logger";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
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
import { ChildCareScheduleSection } from "~/components/parent/child-care-schedule";
import { Section } from "~/components/parent/child-detail-section";

const logger = createLogger({ component: "ChildMasterData" });

const AUTO_SAVE_DELAY_MS = 1500;
const DEPARTURE_DAYS = ["mon", "tue", "wed", "thu", "fri"] as const;
const DEPARTURE_REQUEST_MODES = ["alone", "bus", "pickup"] as const;
const CONTACT_METHODS = ["email", "phone", "mobile", "sms"] as const;

type SaveStatus = "idle" | "saving" | "saved" | "error";

interface Props {
  readonly studentId: string;
}

export function ChildMasterDataView({ studentId }: Props) {
  const t = useTranslations("parentMasterData");
  const [data, setData] = useState<ChildMasterData | null>(null);
  const [features, setFeatures] = useState<ChildFeatures | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useSetBreadcrumb({ pageTitle: t("title") });

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
    void load();
  }, [load]);

  if (loading) {
    return (
      <div className="mx-auto w-full max-w-7xl space-y-4">
        <div className="h-40 animate-pulse rounded-2xl border border-gray-200 bg-white shadow-sm" />
        <div className="h-64 animate-pulse rounded-2xl border border-gray-200 bg-white shadow-sm" />
      </div>
    );
  }

  if (error || !data || !features) {
    return (
      <div className="mx-auto w-full max-w-7xl">
        <BackBar studentId={studentId} />
        <div className="border-moto-red/20 bg-moto-red/10 text-moto-red-strong mt-4 rounded-2xl border p-5 text-sm shadow-sm">
          {t("loadError")}
        </div>
      </div>
    );
  }

  return (
    <ChildMasterDataContent
      studentId={studentId}
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

function ChildMasterDataContent({
  studentId,
  data,
  features,
  onApplied,
  onDirectApplied,
}: Readonly<{
  studentId: string;
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

  return (
    <div className="mx-auto w-full max-w-7xl space-y-6">
      <div className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm">
        <BackBar studentId={studentId} />
        <div className="p-5 sm:p-6">
          <h1 className="text-2xl font-semibold text-gray-900">{t("title")}</h1>
          <p className="mt-1 text-sm leading-6 text-gray-600">
            {t("subtitle")}
          </p>
        </div>
      </div>

      {/* Track B — child identity (approval required) */}
      <IdentitySection
        studentId={studentId}
        data={data}
        features={features}
        pendingByField={pendingByField}
        onApplied={onApplied}
      />

      {/* Track A — health (direct edit) */}
      <Section title={t("sections.health")} hint={t("editableHint")}>
        <AutoSaveField
          label={t("fields.healthInfo")}
          value={data.health_info ?? ""}
          disabled={!features.master_data_edit_enabled}
          onSave={(v) => saveField("student", "health_info", v)}
        />
      </Section>

      {/* Track A — guardian contact (direct edit) */}
      <Section title={t("sections.contact")} hint={t("editableHint")}>
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
        <div className="grid gap-4 sm:grid-cols-2">
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
        <div className="grid gap-4 sm:grid-cols-[8rem_minmax(0,1fr)]">
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
      </Section>

      <DepartureSection
        studentId={studentId}
        data={data}
        features={features}
        pending={pendingByField.get("departure/allowed_departure_modes")}
        onApplied={onApplied}
      />

      <ChildCareScheduleSection studentId={studentId} />
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
  const [firstName, setFirstName] = useState(data.first_name);
  const [lastName, setLastName] = useState(data.last_name);
  const [birthday, setBirthday] = useState(data.birthday ?? "");
  const [status, setStatus] = useState<SaveStatus>("idle");
  const [message, setMessage] = useState<string | null>(null);

  const requestable = features.master_data_request_enabled;
  const firstNamePending = pendingByField.has("person/first_name");
  const lastNamePending = pendingByField.has("person/last_name");
  const birthdayPending = pendingByField.has("person/birthday");
  const identityBase = useRef({
    firstName: data.first_name,
    lastName: data.last_name,
    birthday: data.birthday ?? "",
  });

  useEffect(() => {
    const previous = identityBase.current;
    const next = {
      firstName: data.first_name,
      lastName: data.last_name,
      birthday: data.birthday ?? "",
    };
    setFirstName((current) =>
      current === previous.firstName || firstNamePending
        ? next.firstName
        : current,
    );
    setLastName((current) =>
      current === previous.lastName || lastNamePending
        ? next.lastName
        : current,
    );
    setBirthday((current) =>
      current === previous.birthday || birthdayPending
        ? next.birthday
        : current,
    );
    identityBase.current = next;
  }, [
    data.first_name,
    data.last_name,
    data.birthday,
    firstNamePending,
    lastNamePending,
    birthdayPending,
  ]);

  const changes = useMemo<MasterDataChangeInput[]>(() => {
    const out: MasterDataChangeInput[] = [];
    if (
      !firstNamePending &&
      firstName.trim() &&
      firstName !== data.first_name
    ) {
      out.push({ target: "person", field_key: "first_name", value: firstName });
    }
    if (!lastNamePending && lastName.trim() && lastName !== data.last_name) {
      out.push({ target: "person", field_key: "last_name", value: lastName });
    }
    if (!birthdayPending && birthday && birthday !== (data.birthday ?? "")) {
      out.push({ target: "person", field_key: "birthday", value: birthday });
    }
    return out;
  }, [
    firstName,
    firstNamePending,
    lastName,
    lastNamePending,
    birthday,
    birthdayPending,
    data.first_name,
    data.last_name,
    data.birthday,
  ]);

  const submit = useCallback(async () => {
    if (changes.length === 0) return;
    setStatus("saving");
    setMessage(null);
    try {
      const submitted = await submitMasterDataRequest(studentId, changes);
      setStatus("saved");
      setMessage(t("requestSubmitted"));
      onApplied({
        ...data,
        pending_changes: mergePendingChanges(data.pending_changes, submitted),
      });
      try {
        const next = await getChildMasterData(studentId);
        onApplied(next);
      } catch (refreshErr) {
        const refreshText =
          refreshErr instanceof Error ? refreshErr.message : String(refreshErr);
        logger.warn("master_data_request_refresh_failed", {
          error: refreshText,
          student_id: studentId,
        });
      }
    } catch (err) {
      const text = err instanceof Error ? err.message : String(err);
      logger.warn("master_data_request_failed", {
        error: text,
        student_id: studentId,
      });
      setStatus("error");
      setMessage(t("requestError"));
    }
  }, [changes, data, studentId, onApplied, t]);

  return (
    <Section title={t("sections.child")} hint={t("requestHint")}>
      <RequestField
        label={t("fields.firstName")}
        value={firstName}
        onChange={setFirstName}
        disabled={!requestable || firstNamePending}
        pending={pendingByField.get("person/first_name")}
      />
      <RequestField
        label={t("fields.lastName")}
        value={lastName}
        onChange={setLastName}
        disabled={!requestable || lastNamePending}
        pending={pendingByField.get("person/last_name")}
      />
      <RequestField
        label={t("fields.birthday")}
        type="date"
        value={birthday}
        onChange={setBirthday}
        disabled={!requestable || birthdayPending}
        pending={pendingByField.get("person/birthday")}
      />

      <div className="flex flex-wrap items-center gap-3">
        <ReadField label={t("fields.schoolClass")} value={data.school_class} />
      </div>

      {requestable ? (
        <div className="flex items-center gap-3">
          <Button
            type="button"
            variant="primary"
            size="md"
            disabled={changes.length === 0 || status === "saving"}
            onClick={() => void submit()}
          >
            {t("requestButton")}
          </Button>
          {message && (
            <span
              className={
                status === "error"
                  ? "text-moto-red-strong text-sm"
                  : "text-moto-green-strong text-sm"
              }
            >
              {message}
            </span>
          )}
        </div>
      ) : (
        <p className="text-xs text-gray-500">{t("requestDisabled")}</p>
      )}
    </Section>
  );
}

function DepartureSection({
  studentId,
  data,
  features,
  pending,
  onApplied,
}: Readonly<{
  studentId: string;
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
      const submitted = await submitMasterDataRequest(studentId, [
        {
          target: "departure",
          field_key: "allowed_departure_modes",
          value: modes,
        },
      ]);
      setStatus("saved");
      setMessage(t("requestSubmitted"));
      onApplied({
        ...data,
        pending_changes: mergePendingChanges(data.pending_changes, submitted),
      });
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
    <Section title={t("sections.departure")} hint={t("requestHint")}>
      <DepartureSummary modes={data.allowed_departure_modes} />
      {pending && (
        <p className="inline-flex items-center gap-1 rounded-full bg-[#EAB308]/15 px-2 py-0.5 text-xs font-semibold text-[#92710b]">
          <Clock className="h-3 w-3" aria-hidden="true" />
          {t("pendingBadge")}
        </p>
      )}
      <div className="overflow-x-auto">
        <table className="w-full min-w-[34rem] border-separate border-spacing-0 text-sm">
          <thead>
            <tr>
              <th className="w-16 py-2 pr-3 text-left text-xs font-semibold tracking-wide text-gray-500 uppercase">
                {t("fields.day")}
              </th>
              {DEPARTURE_REQUEST_MODES.map((mode) => (
                <th
                  key={mode}
                  className="px-2 py-2 text-left text-xs font-semibold tracking-wide text-gray-500 uppercase"
                >
                  {t(`departureModes.${mode}`)}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {DEPARTURE_DAYS.map((day) => (
              <tr key={day}>
                <th className="py-2 pr-3 text-left font-medium text-gray-900">
                  {t(`departureDays.${day}`)}
                </th>
                {DEPARTURE_REQUEST_MODES.map((mode) => (
                  <td key={mode} className="px-2 py-2">
                    <input
                      type="checkbox"
                      aria-label={`${t(`departureDays.${day}`)} ${t(`departureModes.${mode}`)}`}
                      checked={(modes[day] ?? []).includes(mode)}
                      disabled={!requestable}
                      onChange={() => toggle(day, mode)}
                      className="text-moto-green-strong focus:ring-moto-green-strong h-4 w-4 rounded border-gray-300"
                    />
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {features.master_data_request_enabled ? (
        <div className="flex items-center gap-3">
          <Button
            type="button"
            variant="primary"
            size="md"
            disabled={!changed || !requestable || status === "saving"}
            onClick={() => void submit()}
          >
            {t("requestButton")}
          </Button>
          {message && (
            <span
              className={
                status === "error"
                  ? "text-moto-red-strong text-sm"
                  : "text-moto-green-strong text-sm"
              }
            >
              {message}
            </span>
          )}
        </div>
      ) : (
        <p className="text-xs text-gray-500">{t("requestDisabled")}</p>
      )}
      {pending && <p className="text-xs text-gray-500">{t("pendingNotice")}</p>}
    </Section>
  );
}

function AutoSaveField({
  label,
  value,
  type = "text",
  disabled = false,
  onSave,
}: Readonly<{
  label: string;
  value: string;
  type?: string;
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
        <span className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
          {label}
        </span>
        <SaveIndicator status={status} />
      </div>
      <div className="mt-1">
        <Input
          aria-label={label}
          type={type}
          value={local}
          disabled={disabled}
          onChange={(e) => handleChange(e.target.value)}
          onBlur={handleBlur}
        />
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
        <span className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
          {label}
        </span>
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

function RequestField({
  label,
  value,
  onChange,
  type = "text",
  disabled = false,
  pending,
}: Readonly<{
  label: string;
  value: string;
  onChange: (value: string) => void;
  type?: string;
  disabled?: boolean;
  pending?: MasterDataChange;
}>) {
  const t = useTranslations("parentMasterData");
  const datePicker = useLocalizedDatePicker();
  return (
    <div>
      <div className="flex items-center justify-between">
        <span className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
          {label}
        </span>
        {pending && (
          <span className="inline-flex items-center gap-1 rounded-full bg-[#EAB308]/15 px-2 py-0.5 text-xs font-semibold text-[#92710b]">
            <Clock className="h-3 w-3" aria-hidden="true" />
            {t("pendingBadge")}
          </span>
        )}
      </div>
      <div className="mt-1">
        {type === "date" ? (
          // Only the birthday is a date field. It gets the kit calendar with
          // month/year dropdowns (a birth year is far from today) and cannot be
          // in the future.
          <ISODatePicker
            {...datePicker}
            ariaLabel={label}
            value={value}
            disabled={disabled}
            onChange={onChange}
            monthYearNavigation
            max={todayISO()}
            calendarLayout="popover"
            controlSize="lg"
          />
        ) : (
          <Input
            aria-label={label}
            type={type}
            value={value}
            disabled={disabled}
            onChange={(e) => onChange(e.target.value)}
          />
        )}
      </div>
      {pending && (
        <p className="mt-1 text-xs text-gray-500">{t("pendingNotice")}</p>
      )}
    </div>
  );
}

function ReadField({
  label,
  value,
}: Readonly<{ label: string; value: string }>) {
  const t = useTranslations("parentMasterData");
  return (
    <div className="min-w-0">
      <span className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
        {label}
      </span>
      <p className="mt-1 text-sm font-medium text-gray-900">
        {value || t("notSet")}
      </p>
    </div>
  );
}

function DepartureSummary({
  modes,
}: Readonly<{ modes?: Record<string, string[]> }>) {
  const t = useTranslations("parentMasterData");
  return (
    <dl className="grid grid-cols-2 gap-2 sm:grid-cols-5">
      {DEPARTURE_DAYS.map((day) => {
        const dayModes = modes?.[day] ?? [];
        const text =
          dayModes.length > 0
            ? dayModes.map((m) => t(`departureModes.${m}`)).join(", ")
            : t("departureModes.none");
        return (
          <div
            key={day}
            className="rounded-xl border border-gray-200 bg-gray-50/70 p-3"
          >
            <dt className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
              {t(`departureDays.${day}`)}
            </dt>
            <dd className="mt-1 text-sm font-medium text-gray-900">{text}</dd>
          </div>
        );
      })}
    </dl>
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
      <span className="inline-flex items-center gap-1 text-xs text-gray-500">
        <Loader2 className="h-3 w-3 animate-spin" aria-hidden="true" />
      </span>
    );
  }
  if (status === "saved") {
    return (
      <span className="text-moto-green-strong inline-flex items-center gap-1 text-xs font-medium">
        <Check className="h-3 w-3" aria-hidden="true" />
        {t("saved")}
      </span>
    );
  }
  if (status === "error") {
    return (
      <span className="text-moto-red-strong inline-flex items-center gap-1 text-xs font-medium">
        <AlertCircle className="h-3 w-3" aria-hidden="true" />
        {t("saveError")}
      </span>
    );
  }
  return null;
}

function BackBar({ studentId }: Readonly<{ studentId: string }>) {
  const t = useTranslations("parentMasterData");
  return (
    <div className="border-b border-gray-100 px-5 py-3 sm:px-6">
      <Link
        href={`/parents/children/${studentId}`}
        className="inline-flex h-8 items-center gap-2 rounded-lg px-2 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-50 hover:text-gray-900"
      >
        <ArrowLeft className="h-4 w-4" aria-hidden="true" />
        {t("back")}
      </Link>
    </div>
  );
}
