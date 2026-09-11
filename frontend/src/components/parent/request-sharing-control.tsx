"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { Modal } from "~/components/ui/modal";
import { useSharingOptions } from "~/components/parent/sharing-options-context";
import {
  getRequestSharing,
  setRequestSharing,
  type ParentRequestShareType,
  type RequestSharingState,
} from "~/lib/parent-api";

interface RequestSharingControlProps {
  readonly studentId: string;
  readonly requestType: ParentRequestShareType;
  readonly requestId: string;
  readonly isSelf: boolean;
}

interface RequestSharingSelectorProps {
  readonly studentId: string;
  readonly selected: readonly string[];
  readonly onChange: (ids: string[]) => void;
}

/**
 * Optional recipient picker shown inside a request form. It never blocks
 * sending: while the list loads, when it fails, and under Familienschutz the
 * selection is simply empty, and the request goes out without recipients.
 */
export function RequestSharingSelector({
  studentId,
  selected,
  onChange,
}: RequestSharingSelectorProps) {
  const t = useTranslations("parentRequestSharing");
  const { state, error } = useSharingOptions(studentId);
  // No choice possible means no recipients: report an empty selection so a
  // stale pick from an earlier state can never travel with the request.
  const unavailable = error || state?.family_protected === true;
  useEffect(() => {
    if (unavailable) onChange([]);
  }, [onChange, unavailable]);
  if (error) return <Alert type="warning" message={t("optionsError")} />;
  if (!state) return <p className="text-sm text-gray-500">{t("loading")}</p>;
  if (state.family_protected)
    return <Alert type="info" message={t("protected")} />;
  return (
    <div className="rounded-xl bg-gray-50 p-3">
      <SharingRecipients
        state={state}
        selected={[...selected]}
        saving={false}
        hint={t("privacyHint")}
        toggle={(id) =>
          onChange(
            selected.includes(id)
              ? selected.filter((item) => item !== id)
              : [...selected, id],
          )
        }
        t={t}
      />
    </div>
  );
}

function useSharingState(
  enabled: boolean,
  studentId: string,
  requestType: ParentRequestShareType,
  requestId: string,
  refreshCount: number,
) {
  const [state, setState] = useState<RequestSharingState | null>(null);
  const [selected, setSelected] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(false);
  useEffect(() => {
    if (!enabled) return;
    let active = true;
    if (refreshCount > 0) {
      setState(null);
      setSelected([]);
    }
    setLoading(true);
    setError(false);
    void (async () => {
      try {
        const next = await getRequestSharing(studentId, requestType, requestId);
        if (active) applySharingState(next, setState, setSelected);
      } catch {
        if (active) setError(true);
      } finally {
        if (active) setLoading(false);
      }
    })();
    return () => {
      active = false;
    };
  }, [enabled, refreshCount, requestId, requestType, studentId]);
  return { state, setState, selected, setSelected, loading, error, setError };
}

function applySharingState(
  next: RequestSharingState,
  setState: (state: RequestSharingState) => void,
  setSelected: (ids: string[]) => void,
) {
  setState(next);
  setSelected(
    next.recipients
      .filter((recipient) => recipient.selected)
      .map((recipient) => recipient.guardian_profile_id),
  );
}

function useSharingSave({
  studentId,
  requestType,
  requestId,
  selected,
  saved,
  failed,
}: Readonly<{
  studentId: string;
  requestType: ParentRequestShareType;
  requestId: string;
  selected: string[];
  saved: (state: RequestSharingState) => void;
  failed: () => void;
}>) {
  const [saving, setSaving] = useState(false);
  const save = useCallback(async () => {
    setSaving(true);
    try {
      saved(
        await setRequestSharing(studentId, requestType, requestId, selected),
      );
    } catch {
      failed();
    } finally {
      setSaving(false);
    }
  }, [failed, requestId, requestType, saved, selected, studentId]);
  return { save, saving };
}

function SharingRecipients({
  state,
  selected,
  saving,
  hint,
  toggle,
  t,
}: Readonly<{
  state: RequestSharingState;
  selected: string[];
  saving: boolean;
  hint?: string;
  toggle: (id: string) => void;
  t: ReturnType<typeof useTranslations>;
}>) {
  if (state.family_protected)
    return <Alert type="info" message={t("protected")} />;
  if (state.recipients.length === 0)
    return <p className="text-sm text-gray-600">{t("noRecipients")}</p>;
  return (
    <fieldset className="space-y-2">
      <legend className="text-sm font-semibold text-gray-900">
        {t("recipientLegend")}
      </legend>
      {hint ? <p className="text-sm text-gray-600">{hint}</p> : null}
      {state.recipients.map((recipient) => (
        <label
          key={recipient.guardian_profile_id}
          className="flex min-h-12 cursor-pointer items-center gap-3 rounded-xl border border-gray-200 px-3 py-2 text-sm text-gray-900"
        >
          <Checkbox
            checked={selected.includes(recipient.guardian_profile_id)}
            disabled={saving}
            onChange={() => toggle(recipient.guardian_profile_id)}
          />
          <span>{`${recipient.first_name} ${recipient.last_name}`}</span>
        </label>
      ))}
    </fieldset>
  );
}

function SharingFooter({
  saving,
  close,
  save,
  t,
}: Readonly<{
  saving: boolean;
  close: () => void;
  save: () => void;
  t: ReturnType<typeof useTranslations>;
}>) {
  return (
    <div className="flex justify-end gap-2">
      <Button
        type="button"
        variant="outline"
        size="touch"
        disabled={saving}
        onClick={close}
      >
        {t("cancel")}
      </Button>
      <Button
        type="button"
        variant="primary"
        size="touch"
        isLoading={saving}
        loadingText={t("saving")}
        onClick={save}
      >
        {t("save")}
      </Button>
    </div>
  );
}

function useRequestSharingControl(props: RequestSharingControlProps) {
  const { studentId, requestType, requestId, isSelf } = props;
  const [open, setOpen] = useState(false);
  const [refreshCount, setRefreshCount] = useState(0);
  const sharing = useSharingState(
    isSelf,
    studentId,
    requestType,
    requestId,
    refreshCount,
  );
  const saved = useCallback(
    (next: RequestSharingState) => {
      sharing.setState(next);
      setOpen(false);
    },
    [sharing],
  );
  const failed = useCallback(() => sharing.setError(true), [sharing]);
  const { save, saving } = useSharingSave({
    studentId,
    requestType,
    requestId,
    selected: sharing.selected,
    saved,
    failed,
  });
  const toggle = (id: string) =>
    sharing.setSelected((current) =>
      current.includes(id)
        ? current.filter((item) => item !== id)
        : [...current, id],
    );
  const show = () => {
    setRefreshCount((current) => current + 1);
    setOpen(true);
  };
  return { open, setOpen, sharing, save, saving, toggle, show };
}

function SharingDialog({
  control,
  t,
}: Readonly<{
  control: ReturnType<typeof useRequestSharingControl>;
  t: ReturnType<typeof useTranslations>;
}>) {
  const { sharing, saving } = control;
  const footer =
    sharing.state && !sharing.state.family_protected ? (
      <SharingFooter
        saving={saving}
        close={() => control.setOpen(false)}
        save={() => void control.save()}
        t={t}
      />
    ) : undefined;
  return (
    <Modal
      isOpen={control.open}
      onClose={() => control.setOpen(false)}
      title={t("title")}
      closeLabel={t("close")}
      backdropLabel={t("close")}
      mobileSheet
      isDismissDisabled={saving}
      footer={footer}
    >
      <div className="space-y-4">
        {sharing.loading ? (
          <p className="text-sm text-gray-500">{t("loading")}</p>
        ) : null}
        {sharing.error ? <Alert type="error" message={t("error")} /> : null}
        {sharing.state ? (
          <SharingRecipients
            state={sharing.state}
            selected={sharing.selected}
            saving={saving}
            hint={t("privacyHint")}
            toggle={control.toggle}
            t={t}
          />
        ) : null}
      </div>
    </Modal>
  );
}

export function RequestSharingControl(props: RequestSharingControlProps) {
  const t = useTranslations("parentRequestSharing");
  const control = useRequestSharingControl(props);
  const { sharing } = control;
  if (!props.isSelf) return null;
  if (sharing.loading)
    return <p className="text-sm text-gray-500">{t("loading")}</p>;
  if (sharing.error && !sharing.state)
    return <Alert type="error" message={t("error")} />;
  if (sharing.state?.family_protected)
    return <Alert type="info" message={t("protected")} />;
  if (!sharing.state) return null;
  return (
    <>
      <Button
        type="button"
        variant="ghost"
        size="md"
        className="max-sm:min-h-11"
        onClick={control.show}
      >
        {t("button")}
      </Button>
      <SharingDialog control={control} t={t} />
    </>
  );
}
