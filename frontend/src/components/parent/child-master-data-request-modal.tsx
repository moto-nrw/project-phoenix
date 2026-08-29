"use client";

import { useMemo, useState } from "react";
import type React from "react";
import { CircleNotchIcon, InfoIcon } from "@phosphor-icons/react/ssr";
import { useTranslations } from "next-intl";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { ISODatePicker } from "~/components/ui/date-picker";
import { Input } from "~/components/ui/input";
import { Modal } from "~/components/ui/modal";
import { StatusBadge } from "~/components/ui/status-badge";
import { RequestSharingSelector } from "~/components/parent/request-sharing-control";
import { todayISO } from "~/lib/date-helpers";
import { useLocalizedDatePicker } from "~/lib/hooks/use-localized-date-picker";
import type { ChildMasterData, MasterDataChangeInput } from "~/lib/parent-api";

type IdentityField = "first_name" | "last_name" | "birthday" | "school_class";

export function ChildMasterDataRequestModal({
  studentId,
  data,
  pendingFields,
  onClose,
  onSubmit,
}: Readonly<{
  studentId: string;
  data: ChildMasterData;
  pendingFields: ReadonlySet<string>;
  onClose: () => void;
  onSubmit: (
    changes: MasterDataChangeInput[],
    recipientIds: string[],
  ) => Promise<boolean>;
}>) {
  const t = useTranslations("parentMasterData");
  const ts = useTranslations("parentRequestSharing");
  const datePicker = useLocalizedDatePicker();
  const [firstName, setFirstName] = useState(data.first_name);
  const [lastName, setLastName] = useState(data.last_name);
  const [birthday, setBirthday] = useState(data.birthday ?? "");
  const [schoolClass, setSchoolClass] = useState(data.school_class);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [recipientIds, setRecipientIds] = useState<string[]>([]);
  const [sharingFailed, setSharingFailed] = useState(false);
  const [sharingReady, setSharingReady] = useState(false);

  const fields = useMemo(
    () => [
      {
        key: "first_name" as const,
        target: "person",
        value: firstName.trim(),
        original: data.first_name,
      },
      {
        key: "last_name" as const,
        target: "person",
        value: lastName.trim(),
        original: data.last_name,
      },
      {
        key: "birthday" as const,
        target: "person",
        value: birthday,
        original: data.birthday ?? "",
      },
      {
        key: "school_class" as const,
        target: "student",
        value: schoolClass.trim(),
        original: data.school_class,
      },
    ],
    [birthday, data, firstName, lastName, schoolClass],
  );

  const isPending = (field: IdentityField) => pendingFields.has(field);

  const submit = async () => {
    setError(null);
    const changed = fields.filter(
      (field) => !isPending(field.key) && field.value !== field.original,
    );
    if (changed.some((field) => field.value === "")) {
      setError(t("identityModal.required"));
      return;
    }
    if (changed.length === 0) {
      setError(t("identityModal.noChange"));
      return;
    }
    setSubmitting(true);
    try {
      const sharingSaved = await onSubmit(
        changed.map((field) => ({
          target: field.target,
          field_key: field.key,
          value: field.value,
        })),
        recipientIds,
      );
      if (sharingSaved) onClose();
      else setSharingFailed(true);
    } catch {
      setError(t("requestError"));
    } finally {
      setSubmitting(false);
    }
  };

  const pendingBadge = (field: IdentityField) =>
    isPending(field) ? (
      <StatusBadge label={t("pendingBadge")} tone="orange" />
    ) : null;

  return (
    <Modal
      isOpen
      onClose={onClose}
      title={t("identityModal.title")}
      closeLabel={t("identityModal.close")}
      backdropLabel={t("identityModal.close")}
      isDismissDisabled={submitting}
      mobileSheet
      footer={
        sharingFailed ? (
          <Button type="button" size="md" onClick={onClose}>
            {t("identityModal.close")}
          </Button>
        ) : (
          <>
            <Button
              type="button"
              variant="outline"
              size="md"
              className="hidden sm:inline-flex"
              disabled={submitting}
              onClick={onClose}
            >
              {t("identityModal.cancel")}
            </Button>
            <Button
              type="button"
              size="md"
              className="w-full gap-2 sm:w-auto"
              disabled={submitting || !sharingReady}
              onClick={() => void submit()}
            >
              {submitting && (
                <CircleNotchIcon
                  weight="bold"
                  className="size-4 animate-spin"
                  aria-hidden="true"
                />
              )}
              {submitting
                ? t("identityModal.submitting")
                : t("identityModal.submit")}
            </Button>
          </>
        )
      }
    >
      <div className="space-y-4">
        {sharingFailed ? (
          <Alert type="warning" message={ts("savedButNotShared")} />
        ) : (
          <>
            <p className="text-sm leading-6 text-gray-600">
              {t("identityModal.intro")}
            </p>
            <div className="bg-moto-blue-soft flex items-start gap-2.5 rounded-xl p-3">
              <InfoIcon
                size={20}
                weight="bold"
                className="text-moto-blue-strong mt-0.5 shrink-0"
                aria-hidden="true"
              />
              <div className="min-w-0">
                <p className="text-sm font-semibold text-gray-900">
                  {t("identityModal.noticeTitle")}
                </p>
                <p className="mt-1 text-sm leading-6 text-gray-700">
                  {t("identityModal.noticeBody")}
                </p>
              </div>
            </div>
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <ModalField
                label={t("fields.firstName")}
                badge={pendingBadge("first_name")}
              >
                <Input
                  name="first_name"
                  aria-label={t("fields.firstName")}
                  autoComplete="off"
                  controlSize="compact"
                  value={firstName}
                  disabled={isPending("first_name") || submitting}
                  onChange={(event) => setFirstName(event.target.value)}
                />
              </ModalField>
              <ModalField
                label={t("fields.lastName")}
                badge={pendingBadge("last_name")}
              >
                <Input
                  name="last_name"
                  aria-label={t("fields.lastName")}
                  autoComplete="off"
                  controlSize="compact"
                  value={lastName}
                  disabled={isPending("last_name") || submitting}
                  onChange={(event) => setLastName(event.target.value)}
                />
              </ModalField>
              <ModalField
                label={t("fields.birthday")}
                badge={pendingBadge("birthday")}
              >
                <ISODatePicker
                  {...datePicker}
                  ariaLabel={t("fields.birthday")}
                  value={birthday}
                  disabled={isPending("birthday") || submitting}
                  onChange={setBirthday}
                  monthYearNavigation
                  max={todayISO()}
                  calendarLayout="popover"
                  controlSize="md"
                />
              </ModalField>
              <ModalField
                label={t("fields.schoolClass")}
                badge={pendingBadge("school_class")}
              >
                <Input
                  name="school_class"
                  aria-label={t("fields.schoolClass")}
                  autoComplete="off"
                  controlSize="compact"
                  value={schoolClass}
                  disabled={isPending("school_class") || submitting}
                  onChange={(event) => setSchoolClass(event.target.value)}
                />
              </ModalField>
            </div>
            <RequestSharingSelector
              studentId={studentId}
              selected={recipientIds}
              onChange={setRecipientIds}
              onReadyChange={setSharingReady}
            />
            {error && <Alert type="error" message={error} />}
          </>
        )}
      </div>
    </Modal>
  );
}

function ModalField({
  label,
  badge,
  children,
}: Readonly<{
  label: string;
  badge: React.ReactNode;
  children: React.ReactNode;
}>) {
  return (
    <div>
      <div className="mb-1 flex min-h-6 items-center justify-between gap-2">
        <span className="text-sm font-medium text-gray-700">{label}</span>
        {badge}
      </div>
      {children}
    </div>
  );
}
