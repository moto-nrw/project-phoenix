"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import {
  ArrowLeft,
  CalendarClock,
  HeartPulse,
  MessageCircle,
  Newspaper,
  Plus,
  ShieldCheck,
  UserRound,
  Users,
} from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import { type Child, type ParentNote, listMyChildren } from "~/lib/parent-api";
import { createLogger } from "~/lib/logger";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import {
  type ChildCare,
  NotesModal,
  ParentNotesList,
  PickupTimeModal,
  SickNoteModal,
  SickStatusSummary,
  useChildCare,
} from "~/components/parent/child-care";
import RelatedAccountsPanel from "~/components/parent/related-accounts-panel";

// Quick-actions that are wired to real backend flows. The rest remain
// "coming soon" stubs until their features ship.
const SUPPORTED_ACTIONS: Record<string, "sick" | "notes" | "pickup"> = {
  sick: "sick",
  message: "notes",
  pickupTime: "pickup",
};

// An action is usable only when it's wired AND the child's school has the
// matching feature enabled — otherwise the backend would reject it with 403.
// Pickup changes are the exception: existing guardian-authored rows must stay
// clearable even after the school disables new parent changes.
function isActionEnabled(actionKey: string, care: ChildCare): boolean {
  const target = SUPPORTED_ACTIONS[actionKey];
  if (target === "sick") return care.features.sick_note_enabled;
  if (target === "notes") return care.features.notes_enabled;
  if (target === "pickup") {
    return (
      care.features.pickup_change_enabled ||
      care.careExceptions.some((entry) => entry.source === "guardian")
    );
  }
  return false;
}

const logger = createLogger({ component: "ChildDetail" });

const CHILD_ACTIONS = [
  {
    key: "sick",
    icon: HeartPulse,
    tone: "text-[#D6373E] bg-[#D6373E]/10",
  },
  {
    key: "pickupTime",
    icon: CalendarClock,
    tone: "text-[#5080D8] bg-[#5080D8]/10",
  },
  {
    key: "message",
    icon: MessageCircle,
    tone: "text-[#F78C10] bg-[#F78C10]/10",
  },
  {
    key: "pickupPermission",
    icon: ShieldCheck,
    tone: "text-[#83CD2D] bg-[#83CD2D]/15",
  },
  {
    key: "people",
    icon: Users,
    tone: "text-[#8B5CF6] bg-[#8B5CF6]/10",
  },
  {
    key: "news",
    icon: Newspaper,
    tone: "text-[#5080D8] bg-[#5080D8]/10",
  },
] as const;

interface Props {
  readonly studentId: string;
}

type ChildAction = (typeof CHILD_ACTIONS)[number];
type ChildDetailTranslator = ReturnType<
  typeof useTranslations<"parentChildDetail">
>;

function formatDate(
  iso: string | undefined,
  locale: string,
  t: ChildDetailTranslator,
): string {
  if (!iso) return t("open");
  try {
    return new Intl.DateTimeFormat(locale, {
      day: "2-digit",
      month: "2-digit",
      year: "numeric",
    }).format(new Date(iso));
  } catch {
    return iso;
  }
}

function getServiceRange(
  child: Child,
  locale: string,
  t: ChildDetailTranslator,
): string {
  if (!child.enrolled_from && !child.enrolled_until) return t("open");
  return t("range", {
    from: formatDate(child.enrolled_from, locale, t),
    to: formatDate(child.enrolled_until, locale, t),
  });
}

function getInitials(child: Child): string {
  return `${child.first_name.at(0) ?? ""}${child.last_name.at(0) ?? ""}`.toUpperCase();
}

export function ChildDetail({ studentId }: Props) {
  const t = useTranslations("parentChildDetail");
  const [child, setChild] = useState<Child | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const list = await listMyChildren();
      const match = list.find((c) => c.student_id === studentId) ?? null;
      setChild(match);
    } catch (err) {
      const message = err instanceof Error ? err.message : t("unknownError");
      logger.warn("child_detail_load_failed", {
        error: message,
        student_id: studentId,
      });
      setError(message);
    } finally {
      setLoading(false);
    }
  }, [studentId, t]);

  useEffect(() => {
    void load();
  }, [load]);

  if (loading) {
    return <ChildDetailSkeleton />;
  }

  if (error) {
    return (
      <div className="mx-auto max-w-7xl">
        <div className="rounded-2xl border border-[#FF3130]/20 bg-[#FF3130]/10 p-5 text-sm text-[#CC2626] shadow-sm">
          {t("loadError")}
        </div>
      </div>
    );
  }

  if (!child) {
    return (
      <div className="mx-auto max-w-7xl">
        <div className="overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm">
          <BackBar />
          <div className="p-6 text-sm leading-6 text-gray-600">
            {t("notFound")}
          </div>
        </div>
      </div>
    );
  }

  return <ChildDetailContent child={child} />;
}

function ChildDetailContent({ child }: Readonly<{ child: Child }>) {
  const t = useTranslations("parentChildDetail");
  const locale = useLocale();
  const fullName = `${child.first_name} ${child.last_name}`;
  useSetBreadcrumb({ pageTitle: fullName });
  const care = useChildCare(child.student_id);
  const [modal, setModal] = useState<null | "sick" | "notes" | "pickup">(null);

  const openAction = useCallback(
    (actionKey: string) => {
      const target = SUPPORTED_ACTIONS[actionKey];
      if (target && isActionEnabled(actionKey, care)) setModal(target);
    },
    [care],
  );

  const summaryItems = useMemo(
    () => [
      { label: t("schoolLabel"), value: child.school_name },
      { label: t("classLabel"), value: child.school_class || t("notSet") },
      { label: t("careLabel"), value: getServiceRange(child, locale, t) },
    ],
    [child, locale, t],
  );
  const pickupPeople = getPickupPeople(t);

  return (
    <div className="mx-auto w-full max-w-7xl space-y-6">
      <MobileChildAppView
        child={child}
        fullName={fullName}
        care={care}
        onAction={openAction}
      />

      <section className="hidden overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm lg:block">
        <div className="grid gap-0 lg:grid-cols-[minmax(0,1fr)_24rem] xl:grid-cols-[minmax(0,1fr)_30rem]">
          <div>
            <BackBar />
            <div className="p-5 sm:p-6 lg:p-8">
              <div className="flex min-w-0 items-start gap-4">
                <span className="flex h-16 w-16 shrink-0 items-center justify-center rounded-2xl bg-[#83CD2D]/15 text-lg font-semibold text-[#4A7A15]">
                  {getInitials(child) || (
                    <UserRound className="h-7 w-7" aria-hidden="true" />
                  )}
                </span>
                <div className="min-w-0 flex-1">
                  <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
                    {t("childEyebrow")}
                  </p>
                  <h1 className="mt-1 text-3xl font-semibold break-words text-gray-900 sm:text-4xl">
                    {fullName}
                  </h1>
                  <p className="mt-2 text-sm leading-6 break-words text-gray-600 sm:text-base">
                    {child.school_name}
                    {child.school_class ? `, ${child.school_class}` : ""}
                  </p>
                </div>
              </div>
              <div className="mt-7 grid max-w-3xl gap-3 sm:grid-cols-3">
                {CHILD_ACTIONS.slice(0, 3).map((action) => (
                  <DesktopQuickAction
                    key={action.key}
                    action={action}
                    onClick={
                      isActionEnabled(action.key, care)
                        ? () => openAction(action.key)
                        : undefined
                    }
                  />
                ))}
              </div>
            </div>
          </div>
          <div className="moto-dotted-background moto-dotted-background--split border-t border-gray-200 p-5 sm:p-6 lg:border-t-0 lg:border-l">
            <TodayPanel care={care} />
          </div>
        </div>
      </section>

      <div className="grid gap-6 lg:grid-cols-[minmax(0,0.9fr)_minmax(24rem,0.8fr)]">
        <section className="rounded-2xl border border-gray-200 bg-white shadow-sm max-lg:hidden">
          <div className="p-5 sm:p-6">
            <PanelHeader
              eyebrow={t("masterDataEyebrow")}
              title={t("masterDataTitle")}
              description={t("masterDataDescription")}
            />
          </div>
          <dl className="divide-y divide-gray-100 border-t border-gray-100">
            {summaryItems.map((item) => (
              <InfoRow key={item.label} label={item.label} value={item.value} />
            ))}
          </dl>
        </section>

        <div className="grid gap-6 max-lg:hidden">
          <MessagesPanel
            notes={care.notes}
            composeDisabled={!care.features.notes_enabled}
            onCompose={() => setModal("notes")}
          />
          <PickupPeoplePanel people={pickupPeople} />
          <RelatedAccountsPanel
            studentId={child.student_id}
            canInvite={care.features.related_accounts_invite_enabled}
            canRemove={care.features.related_accounts_remove_enabled}
          />
          <NewsPanel />
        </div>
      </div>

      {modal === "sick" && (
        <SickNoteModal
          onClose={() => setModal(null)}
          onSubmit={care.reportSick}
        />
      )}
      {modal === "notes" && (
        <NotesModal
          notes={care.notes}
          onClose={() => setModal(null)}
          onSubmit={care.postNote}
        />
      )}
      {modal === "pickup" && (
        <PickupTimeModal
          careExceptions={care.careExceptions}
          careExceptionsLoaded={care.careExceptionsLoaded}
          pickupChangeEnabled={care.features.pickup_change_enabled}
          onClose={() => setModal(null)}
          onSubmit={care.saveCareException}
          onRemove={care.removeCareException}
        />
      )}
    </div>
  );
}

function MobileChildAppView({
  child,
  fullName,
  care,
  onAction,
}: Readonly<{
  child: Child;
  fullName: string;
  care: ChildCare;
  onAction: (actionKey: string) => void;
}>) {
  const t = useTranslations("parentChildDetail");
  const locale = useLocale();
  const primaryActions = CHILD_ACTIONS.slice(0, 3);
  const pickupPeople = getPickupPeople(t);

  return (
    <div className="space-y-5 lg:hidden">
      <div className="overflow-hidden rounded-[1.75rem] border border-gray-200 bg-white shadow-sm">
        <BackBar />
        <div className="p-5">
          <div className="flex min-w-0 items-center gap-4">
            <span className="flex h-14 w-14 shrink-0 items-center justify-center rounded-3xl bg-[#83CD2D]/15 text-lg font-semibold text-[#4A7A15]">
              {getInitials(child) || (
                <UserRound className="h-6 w-6" aria-hidden="true" />
              )}
            </span>
            <div className="min-w-0">
              <h1 className="text-xl font-semibold break-words text-gray-900">
                {fullName}
              </h1>
              <p className="mt-0.5 text-sm break-words text-gray-600">
                {child.school_name}
                {child.school_class ? `, ${child.school_class}` : ""}
              </p>
              <span className="mt-3 inline-flex max-w-full rounded-full bg-[#83CD2D]/15 px-3 py-1 text-xs font-semibold text-[#4A7A15]">
                {t("careRecorded")}
              </span>
            </div>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-3 gap-3">
        {primaryActions.map((action) => (
          <MobileQuickAction
            key={action.key}
            action={action}
            onClick={
              isActionEnabled(action.key, care)
                ? () => onAction(action.key)
                : undefined
            }
          />
        ))}
      </div>

      <section className="rounded-[1.75rem] border border-gray-200 bg-white p-5 shadow-sm">
        <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
          {t("today.sickLabel")}
        </p>
        <div className="mt-2">
          <SickStatusSummary sickDays={care.sickDays} />
        </div>
      </section>

      <MessagesPanel
        notes={care.notes}
        composeDisabled={!care.features.notes_enabled}
        onCompose={() => onAction("message")}
        mobile
      />

      <section className="rounded-[1.75rem] border border-gray-200 bg-white p-5 shadow-sm">
        <div className="flex items-center justify-between gap-4">
          <h2 className="text-lg font-semibold text-gray-900">
            {t("pickupPeopleTitle")}
          </h2>
          <button
            type="button"
            disabled
            className="inline-flex h-9 items-center rounded-full border border-gray-200 bg-white px-3 text-sm font-semibold text-gray-700 shadow-sm disabled:cursor-not-allowed disabled:opacity-60"
          >
            {t("manage")}
          </button>
        </div>
        <div className="mt-4 flex items-start gap-3 overflow-x-auto pb-1">
          {pickupPeople.map((person) => (
            <PersonBubble key={person.name} person={person} />
          ))}
          <button
            type="button"
            disabled
            className="flex min-w-14 flex-col items-center gap-2 text-xs font-medium text-gray-500 disabled:cursor-not-allowed disabled:opacity-70"
            aria-label={t("addPerson")}
          >
            <span className="flex h-11 w-11 items-center justify-center rounded-full border border-dashed border-gray-300 bg-gray-50 text-gray-500">
              <Plus className="h-5 w-5" aria-hidden="true" />
            </span>
          </button>
        </div>
      </section>

      <RelatedAccountsPanel
        studentId={child.student_id}
        canInvite={care.features.related_accounts_invite_enabled}
        canRemove={care.features.related_accounts_remove_enabled}
        mobile
      />

      <NewsPanel mobile />

      <section className="rounded-[1.75rem] border border-gray-200 bg-white p-5 shadow-sm">
        <h2 className="text-lg font-semibold text-gray-900">
          {t("careLabel")}
        </h2>
        <dl className="mt-4 space-y-3">
          <CompactInfoRow
            label={t("periodLabel")}
            value={getServiceRange(child, locale, t)}
          />
          <CompactInfoRow
            label={t("classLabel")}
            value={child.school_class || t("notSet")}
          />
        </dl>
      </section>
    </div>
  );
}

function getPickupPeople(t: ChildDetailTranslator) {
  return [
    { initials: "MM", name: t("demo.momName"), relation: t("demo.mother") },
    { initials: "PM", name: t("demo.dadName"), relation: t("demo.father") },
    { initials: "OM", name: t("demo.grandmaName"), relation: t("demo.pickup") },
    { initials: "TA", name: t("demo.auntName"), relation: t("demo.pickup") },
  ];
}

function MobileQuickAction({
  action,
  onClick,
}: Readonly<{ action: ChildAction; onClick?: () => void }>) {
  const t = useTranslations("parentChildDetail");
  const Icon = action.icon;
  const enabled = Boolean(onClick);
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={!enabled}
      className={`flex aspect-square min-h-28 flex-col items-center justify-center gap-3 rounded-3xl border p-3 text-center shadow-sm transition-colors ${
        enabled
          ? "border-gray-200 bg-white hover:bg-gray-50"
          : "cursor-not-allowed border-gray-100 bg-white"
      }`}
    >
      <span
        className={`flex h-11 w-11 items-center justify-center rounded-2xl ${action.tone}`}
      >
        <Icon className="h-6 w-6" aria-hidden="true" />
      </span>
      <span className="text-xs leading-4 font-semibold text-gray-900">
        {t(`actions.${action.key}.label`)}
      </span>
    </button>
  );
}

function DesktopQuickAction({
  action,
  onClick,
}: Readonly<{ action: ChildAction; onClick?: () => void }>) {
  const t = useTranslations("parentChildDetail");
  const Icon = action.icon;
  const label = t(`actions.${action.key}.label`);
  const enabled = Boolean(onClick);
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={!enabled}
      className={`flex min-h-24 items-center gap-3 rounded-xl border p-4 text-left transition-colors ${
        enabled
          ? "border-gray-200 bg-white hover:bg-gray-50"
          : "cursor-not-allowed border-gray-200 bg-gray-50/70 opacity-80"
      }`}
      aria-label={enabled ? label : t("comingSoonAria", { label })}
    >
      <span
        className={`flex h-10 w-10 shrink-0 items-center justify-center rounded-lg ${action.tone}`}
      >
        <Icon className="h-5 w-5" aria-hidden="true" />
      </span>
      <span className="min-w-0">
        <span className="block text-sm font-semibold text-gray-900">
          {label}
        </span>
        <span className="mt-0.5 block text-sm leading-5 text-gray-600">
          {t(`actions.${action.key}.description`)}
        </span>
      </span>
    </button>
  );
}

function TodayPanel({ care }: Readonly<{ care: ChildCare }>) {
  const t = useTranslations("parentChildDetail");
  const noteCount = care.notes.length;
  return (
    <div className="relative z-10 space-y-4">
      <div>
        <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
          {t("today.title")}
        </p>
        <p className="mt-1 text-sm leading-6 text-gray-600">
          {t("today.description")}
        </p>
      </div>
      <div className="space-y-2">
        <div className="flex items-center gap-3 rounded-xl border border-gray-200 bg-white/85 p-3 shadow-sm">
          <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-[#D6373E]/10 text-[#D6373E]">
            <HeartPulse className="h-5 w-5" aria-hidden="true" />
          </span>
          <div className="min-w-0">
            <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
              {t("today.sickLabel")}
            </p>
            <div className="mt-0.5">
              <SickStatusSummary sickDays={care.sickDays} />
            </div>
          </div>
        </div>
        <div className="flex items-center gap-3 rounded-xl border border-gray-200 bg-white/85 p-3 shadow-sm">
          <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-[#5080D8]/10 text-[#5080D8]">
            <CalendarClock className="h-5 w-5" aria-hidden="true" />
          </span>
          <div className="min-w-0">
            <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
              {t("today.pickupLabel")}
            </p>
            <p className="mt-0.5 text-sm font-semibold text-gray-900">
              {t("today.regularPickup")}
            </p>
          </div>
        </div>
        <div className="flex items-center gap-3 rounded-xl border border-gray-200 bg-white/85 p-3 shadow-sm">
          <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-[#F78C10]/10 text-[#F78C10]">
            <MessageCircle className="h-5 w-5" aria-hidden="true" />
          </span>
          <div className="min-w-0">
            <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
              {t("today.messagesLabel")}
            </p>
            <p className="mt-0.5 text-sm font-semibold text-gray-900">
              {noteCount === 0
                ? t("today.noMessagesSent")
                : t("today.messagesSent", { count: noteCount })}
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}

function MessagesPanel({
  notes,
  onCompose,
  composeDisabled = false,
  mobile = false,
}: Readonly<{
  notes: ParentNote[];
  onCompose: () => void;
  composeDisabled?: boolean;
  mobile?: boolean;
}>) {
  const t = useTranslations("parentChildDetail");
  return (
    <section
      className={
        mobile
          ? "rounded-[1.75rem] border border-gray-200 bg-white p-5 shadow-sm"
          : "rounded-2xl border border-gray-200 bg-white p-5 shadow-sm sm:p-6"
      }
    >
      <div className="flex items-start justify-between gap-4">
        <PanelHeader
          eyebrow={t("messages.eyebrow")}
          title={t("messages.title")}
          description={t("messages.description")}
        />
        {!composeDisabled && (
          <button
            type="button"
            onClick={onCompose}
            className="inline-flex h-9 shrink-0 items-center gap-2 rounded-lg bg-[#F78C10] px-3 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-[#dd7c0c]"
          >
            <MessageCircle className="h-4 w-4" aria-hidden="true" />
            {t("messages.compose")}
          </button>
        )}
      </div>
      <div className="mt-4">
        <ParentNotesList notes={notes} />
      </div>
    </section>
  );
}

function PersonBubble({
  person,
}: Readonly<{
  person: ReturnType<typeof getPickupPeople>[number];
}>) {
  return (
    <div className="flex min-w-14 flex-col items-center gap-2">
      <span className="flex h-11 w-11 items-center justify-center rounded-full bg-gray-100 text-sm font-semibold text-gray-700 ring-1 ring-gray-200">
        {person.initials}
      </span>
      <span className="max-w-12 truncate text-xs font-medium text-gray-700">
        {person.name}
      </span>
    </div>
  );
}

function PickupPeoplePanel({
  people,
}: Readonly<{ people: ReturnType<typeof getPickupPeople> }>) {
  const t = useTranslations("parentChildDetail");
  return (
    <section className="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm sm:p-6">
      <div className="flex items-start justify-between gap-4">
        <PanelHeader
          eyebrow={t("pickupEyebrow")}
          title={t("pickupPeopleTitle")}
          description={t("pickupPeopleDescription")}
        />
        <button
          type="button"
          disabled
          className="inline-flex h-9 shrink-0 items-center gap-2 rounded-lg border border-gray-200 bg-white px-3 text-sm font-semibold text-gray-700 shadow-sm disabled:cursor-not-allowed disabled:opacity-60"
        >
          <Plus className="h-4 w-4" aria-hidden="true" />
          {t("add")}
        </button>
      </div>
      <div className="mt-5 divide-y divide-gray-100 rounded-xl border border-gray-200 bg-gray-50/70">
        {people.map((person) => (
          <div key={person.name} className="flex items-center gap-3 p-3">
            <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-white text-sm font-semibold text-gray-700 ring-1 ring-gray-200">
              {person.initials}
            </span>
            <div className="min-w-0 flex-1">
              <p className="text-sm font-semibold text-gray-900">
                {person.name}
              </p>
              <p className="text-sm text-gray-600">{person.relation}</p>
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}

function NewsPanel({ mobile = false }: Readonly<{ mobile?: boolean }>) {
  const t = useTranslations("parentChildDetail");
  return (
    <section
      className={
        mobile
          ? "rounded-[1.75rem] border border-gray-200 bg-white p-5 shadow-sm"
          : "rounded-2xl border border-gray-200 bg-white p-5 shadow-sm sm:p-6"
      }
    >
      {mobile ? (
        <h2 className="text-lg font-semibold text-gray-900">
          {t("newsTitle")}
        </h2>
      ) : (
        <PanelHeader
          eyebrow={t("newsEyebrow")}
          title={t("newsTitle")}
          description={t("newsDescription")}
        />
      )}
      <div className="mt-4 flex items-center gap-3 rounded-xl border border-gray-200 bg-gray-50/70 p-4">
        <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-white text-gray-600 shadow-sm ring-1 ring-gray-200">
          <Newspaper className="h-5 w-5" aria-hidden="true" />
        </span>
        <div className="min-w-0 flex-1">
          <p className="text-sm font-semibold text-gray-900">
            {t("noNewsTitle")}
          </p>
          <p className="mt-0.5 text-sm leading-5 text-gray-600">
            {t("noNewsDescription")}
          </p>
        </div>
      </div>
    </section>
  );
}

function CompactInfoRow({
  label,
  value,
}: Readonly<{ label: string; value: string }>) {
  return (
    <div className="flex items-start justify-between gap-4 rounded-2xl bg-gray-50 px-4 py-3">
      <dt className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
        {label}
      </dt>
      <dd className="text-right text-sm font-semibold break-words text-gray-900">
        {value}
      </dd>
    </div>
  );
}

function BackBar() {
  const t = useTranslations("parentChildDetail");
  return (
    <div className="border-b border-gray-100 px-5 py-3 sm:px-6">
      <Link
        href="/parents/children"
        className="inline-flex h-8 items-center gap-2 rounded-lg px-2 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-50 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
      >
        <ArrowLeft className="h-4 w-4" aria-hidden="true" />
        {t("back")}
      </Link>
    </div>
  );
}

function InfoRow({ label, value }: Readonly<{ label: string; value: string }>) {
  return (
    <div className="grid gap-1 px-5 py-4 sm:grid-cols-[12rem_minmax(0,1fr)] sm:px-6">
      <dt className="text-xs font-semibold tracking-wide text-gray-500 uppercase sm:pt-0.5">
        {label}
      </dt>
      <dd className="mt-1 text-sm font-medium break-words text-gray-900">
        {value}
      </dd>
    </div>
  );
}

function PanelHeader({
  eyebrow,
  title,
  description,
}: Readonly<{
  eyebrow: string;
  title: string;
  description: string;
}>) {
  return (
    <header>
      <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
        {eyebrow}
      </p>
      <h2 className="mt-1 text-xl font-semibold text-balance text-gray-900">
        {title}
      </h2>
      <p className="mt-1 text-sm leading-6 text-gray-600">{description}</p>
    </header>
  );
}

function ChildDetailSkeleton() {
  return (
    <div className="mx-auto w-full max-w-7xl space-y-6">
      <div className="h-64 animate-pulse rounded-2xl border border-gray-200 bg-white shadow-sm" />
      <div className="grid gap-6 xl:grid-cols-2">
        <div className="h-64 animate-pulse rounded-2xl border border-gray-200 bg-white shadow-sm" />
        <div className="h-64 animate-pulse rounded-2xl border border-gray-200 bg-white shadow-sm" />
      </div>
    </div>
  );
}
