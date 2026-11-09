"use client";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import { InsetPanel } from "~/components/ui/inset-panel";
import { useTranslations } from "next-intl";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { Modal } from "~/components/ui/modal";
import { useSharingOptions } from "~/components/parent/sharing-options-context";
import { useApiFormError, useApiLoadError } from "~/contexts/ToastContext";
import {
  getRequestSharing,
  setRequestSharing,
  type ParentRequestShareType,
  type RequestSharingState,
} from "~/lib/parent-api";
import { SpinnerIcon } from "~/components/ui/icons";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { useCareTermTranslations } from "~/lib/care-facility-term";

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
  const t = useCareTermTranslations("parentRequestSharing");
  const { state, failed, error } = useSharingOptions(studentId);
  // No choice possible means no recipients: report an empty selection so a
  // stale pick from an earlier state can never travel with the request.
  const unavailable = failed || state?.family_protected === true;
  useEffect(() => {
    if (unavailable) onChange([]);
  }, [onChange, unavailable]);
  if (failed) {
    // The message arrives once the catalog has loaded; until then the
    // loading line stays, so the spot is never empty.
    return error ? (
      <LoadErrorAlert error={error} />
    ) : (
      <p
        aria-busy="true"
        className="flex items-center gap-2 text-sm text-gray-500"
      >
        <SpinnerIcon className="h-4 w-4" />
        {t("loading")}
      </p>
    );
  }
  if (!state)
    return (
      <p
        aria-busy="true"
        className="flex items-center gap-2 text-sm text-gray-500"
      >
        <SpinnerIcon className="h-4 w-4" />
        {t("loading")}
      </p>
    );
  if (state.family_protected)
    return <Alert type="info" message={t("protected")} />;
  return (
    <InsetPanel>
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
    </InsetPanel>
  );
}

function useSharingState(
  enabled: boolean,
  studentId: string,
  requestType: ParentRequestShareType,
  requestId: string,
  refreshCount: number,
  reload: () => void,
) {
  const t = useCareTermTranslations("parentRequestSharing");
  const [state, setState] = useState<RequestSharingState | null>(null);
  const [selected, setSelected] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const {
    error: loadError,
    show: showLoadError,
    clear: clearLoadError,
  } = useApiLoadError();
  // The retry runs the latest reload, not the one of the failed attempt.
  const reloadRef = useRef(reload);
  useLayoutEffect(() => {
    reloadRef.current = reload;
  });
  useEffect(() => {
    if (!enabled) return;
    let active = true;
    if (refreshCount > 0) {
      setState(null);
      setSelected([]);
    }
    setLoading(true);
    clearLoadError();
    void (async () => {
      try {
        const next = await getRequestSharing(studentId, requestType, requestId);
        if (active) applySharingState(next, setState, setSelected);
      } catch (err) {
        if (active) {
          void showLoadError(err, {
            object: t("errorObjectRecipients"),
            retry: () => reloadRef.current(),
          });
        }
      } finally {
        if (active) setLoading(false);
      }
    })();
    return () => {
      active = false;
    };
  }, [
    clearLoadError,
    enabled,
    refreshCount,
    requestId,
    requestType,
    showLoadError,
    studentId,
    t,
  ]);
  return { state, setState, selected, setSelected, loading, loadError };
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
  failed: (err: unknown) => void;
}>) {
  const [saving, setSaving] = useState(false);
  const save = useCallback(async () => {
    setSaving(true);
    try {
      saved(
        await setRequestSharing(studentId, requestType, requestId, selected),
      );
    } catch (err) {
      failed(err);
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
        <ChoiceTile
          key={recipient.guardian_profile_id}
          selected={selected.includes(recipient.guardian_profile_id)}
          className="min-h-12 py-2 font-normal text-gray-900"
        >
          <Checkbox
            checked={selected.includes(recipient.guardian_profile_id)}
            disabled={saving}
            onChange={() => toggle(recipient.guardian_profile_id)}
          />
          <span>{`${recipient.first_name} ${recipient.last_name}`}</span>
        </ChoiceTile>
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
        size="md"
        disabled={saving}
        onClick={close}
      >
        {t("cancel")}
      </Button>
      <Button
        type="button"
        variant="primary"
        size="md"
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
  const t = useCareTermTranslations("parentRequestSharing");
  const [open, setOpen] = useState(false);
  const [refreshCount, setRefreshCount] = useState(0);
  const reload = useCallback(
    () => setRefreshCount((current) => current + 1),
    [],
  );
  const sharing = useSharingState(
    isSelf,
    studentId,
    requestType,
    requestId,
    refreshCount,
    reload,
  );
  const {
    error: saveError,
    show: showSaveError,
    clear: clearSaveError,
  } = useApiFormError();
  const { setState } = sharing;
  const saved = useCallback(
    (next: RequestSharingState) => {
      setState(next);
      setOpen(false);
    },
    [setState],
  );
  // The retry sends the selection the dialog shows at that moment.
  const saveRef = useRef<() => Promise<void>>(async () => undefined);
  const failed = useCallback(
    (err: unknown) =>
      void showSaveError(err, {
        object: t("errorObjectSelection"),
        retry: () => void saveRef.current(),
      }),
    [showSaveError, t],
  );
  const { save, saving } = useSharingSave({
    studentId,
    requestType,
    requestId,
    selected: sharing.selected,
    saved,
    failed,
  });
  useLayoutEffect(() => {
    saveRef.current = save;
  });
  const toggle = (id: string) =>
    sharing.setSelected((current) =>
      current.includes(id)
        ? current.filter((item) => item !== id)
        : [...current, id],
    );
  const show = () => {
    clearSaveError();
    reload();
    setOpen(true);
  };
  return { open, setOpen, sharing, save, saving, saveError, toggle, show };
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
          <p
            aria-busy="true"
            className="flex items-center gap-2 text-sm text-gray-500"
          >
            <SpinnerIcon className="h-4 w-4" />
            {t("loading")}
          </p>
        ) : null}
        <FormErrorAlert message={control.saveError} />
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
  const t = useCareTermTranslations("parentRequestSharing");
  const control = useRequestSharingControl(props);
  const { sharing } = control;
  if (!props.isSelf) return null;
  if (sharing.loading)
    return (
      <p
        aria-busy="true"
        className="flex items-center gap-2 text-sm text-gray-500"
      >
        <SpinnerIcon className="h-4 w-4" />
        {t("loading")}
      </p>
    );
  if (sharing.loadError && !sharing.state)
    return <LoadErrorAlert error={sharing.loadError} />;
  if (sharing.state?.family_protected)
    return <Alert type="info" message={t("protected")} />;
  if (!sharing.state) return null;
  return (
    <>
      <Button type="button" variant="ghost" size="md" onClick={control.show}>
        {t("button")}
      </Button>
      <SharingDialog control={control} t={t} />
    </>
  );
}
