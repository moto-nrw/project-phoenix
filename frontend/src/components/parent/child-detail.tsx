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
import {
  type Child,
  type ChildFeatures,
  type ParentNote,
  listMyChildren,
} from "~/lib/parent-api";
import { createLogger } from "~/lib/logger";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import {
  type ChildCare,
  NotesModal,
  ParentNotesList,
  SickNoteModal,
  SickStatusSummary,
  useChildCare,
} from "~/components/parent/child-care";

// Quick-actions that are wired to real backend flows. The rest remain
// "coming soon" stubs until their features ship.
const SUPPORTED_ACTIONS: Record<string, "sick" | "notes"> = {
  "Krank melden": "sick",
  "Nachricht schreiben": "notes",
};

// An action is usable only when it's wired AND the child's school has the
// matching feature enabled — otherwise the backend would reject it with 403.
function isActionEnabled(label: string, features: ChildFeatures): boolean {
  const target = SUPPORTED_ACTIONS[label];
  if (target === "sick") return features.sick_note_enabled;
  if (target === "notes") return features.notes_enabled;
  return false;
}

const logger = createLogger({ component: "ChildDetail" });

const CHILD_ACTIONS = [
  {
    label: "Krank melden",
    description: "Tagesmeldung für dieses Kind",
    icon: HeartPulse,
    tone: "text-[#D6373E] bg-[#D6373E]/10",
  },
  {
    label: "Abholzeit ändern",
    description: "Abholung mitteilen",
    icon: CalendarClock,
    tone: "text-[#5080D8] bg-[#5080D8]/10",
  },
  {
    label: "Nachricht schreiben",
    description: "Direkt an die Betreuung",
    icon: MessageCircle,
    tone: "text-[#F78C10] bg-[#F78C10]/10",
  },
  {
    label: "Abholrecht",
    description: "Freigaben verwalten",
    icon: ShieldCheck,
    tone: "text-[#83CD2D] bg-[#83CD2D]/15",
  },
  {
    label: "Personen",
    description: "Kontakte und Eltern",
    icon: Users,
    tone: "text-[#8B5CF6] bg-[#8B5CF6]/10",
  },
  {
    label: "Neuigkeiten",
    description: "Infos zur Gruppe",
    icon: Newspaper,
    tone: "text-[#5080D8] bg-[#5080D8]/10",
  },
] as const;

interface Props {
  readonly studentId: string;
}

function formatDate(iso: string | undefined): string {
  if (!iso) return "Noch offen";
  try {
    return new Intl.DateTimeFormat("de-DE", {
      day: "2-digit",
      month: "2-digit",
      year: "numeric",
    }).format(new Date(iso));
  } catch {
    return iso;
  }
}

function getServiceRange(child: Child): string {
  if (!child.enrolled_from && !child.enrolled_until) return "Noch offen";
  return `${formatDate(child.enrolled_from)} bis ${formatDate(child.enrolled_until)}`;
}

function getInitials(child: Child): string {
  return `${child.first_name.at(0) ?? ""}${child.last_name.at(0) ?? ""}`.toUpperCase();
}

export function ChildDetail({ studentId }: Props) {
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
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.warn("child_detail_load_failed", {
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
    return <ChildDetailSkeleton />;
  }

  if (error) {
    return (
      <div className="mx-auto max-w-7xl">
        <div className="rounded-2xl border border-[#FF3130]/20 bg-[#FF3130]/10 p-5 text-sm text-[#CC2626] shadow-sm">
          Die Kinderdaten konnten nicht geladen werden. Bitte aktualisieren Sie
          die Seite oder versuchen Sie es später erneut.
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
            Kein Kind mit dieser Kennung gefunden. Vermutlich gehört es zu einem
            Schulwechsel oder ist im Elternportal nicht mehr freigeschaltet.
          </div>
        </div>
      </div>
    );
  }

  return <ChildDetailContent child={child} />;
}

function ChildDetailContent({ child }: Readonly<{ child: Child }>) {
  const fullName = `${child.first_name} ${child.last_name}`;
  useSetBreadcrumb({ pageTitle: fullName });
  const care = useChildCare(child.student_id);
  const [modal, setModal] = useState<null | "sick" | "notes">(null);

  const openAction = useCallback(
    (label: string) => {
      const target = SUPPORTED_ACTIONS[label];
      if (target && isActionEnabled(label, care.features)) setModal(target);
    },
    [care.features],
  );

  const summaryItems = useMemo(
    () => [
      { label: "Schule", value: child.school_name },
      { label: "Klasse", value: child.school_class || "Noch nicht hinterlegt" },
      { label: "Betreuung", value: getServiceRange(child) },
    ],
    [child],
  );
  const pickupPeople = getPickupPeople();

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
                    Kind
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
                    key={action.label}
                    action={action}
                    onClick={
                      isActionEnabled(action.label, care.features)
                        ? () => openAction(action.label)
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
              eyebrow="Kinderdaten"
              title="Stammdaten"
              description="Die wichtigsten Angaben zu Kind und Betreuung."
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
  onAction: (label: string) => void;
}>) {
  const primaryActions = CHILD_ACTIONS.slice(0, 3);
  const pickupPeople = getPickupPeople();

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
                Betreuung hinterlegt
              </span>
            </div>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-3 gap-3">
        {primaryActions.map((action) => (
          <MobileQuickAction
            key={action.label}
            action={action}
            onClick={
              isActionEnabled(action.label, care.features)
                ? () => onAction(action.label)
                : undefined
            }
          />
        ))}
      </div>

      <section className="rounded-[1.75rem] border border-gray-200 bg-white p-5 shadow-sm">
        <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
          Tagesmeldung
        </p>
        <div className="mt-2">
          <SickStatusSummary sickDays={care.sickDays} />
        </div>
      </section>

      <MessagesPanel
        notes={care.notes}
        composeDisabled={!care.features.notes_enabled}
        onCompose={() => onAction("Nachricht schreiben")}
        mobile
      />

      <section className="rounded-[1.75rem] border border-gray-200 bg-white p-5 shadow-sm">
        <div className="flex items-center justify-between gap-4">
          <h2 className="text-lg font-semibold text-gray-900">
            Abholberechtigte
          </h2>
          <button
            type="button"
            disabled
            className="inline-flex h-9 items-center rounded-full border border-gray-200 bg-white px-3 text-sm font-semibold text-gray-700 shadow-sm disabled:cursor-not-allowed disabled:opacity-60"
          >
            Verwalten
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
            aria-label="Person hinzufügen"
          >
            <span className="flex h-11 w-11 items-center justify-center rounded-full border border-dashed border-gray-300 bg-gray-50 text-gray-500">
              <Plus className="h-5 w-5" aria-hidden="true" />
            </span>
          </button>
        </div>
      </section>

      <NewsPanel mobile />

      <section className="rounded-[1.75rem] border border-gray-200 bg-white p-5 shadow-sm">
        <h2 className="text-lg font-semibold text-gray-900">Betreuung</h2>
        <dl className="mt-4 space-y-3">
          <CompactInfoRow label="Zeitraum" value={getServiceRange(child)} />
          <CompactInfoRow
            label="Klasse"
            value={child.school_class || "Noch nicht hinterlegt"}
          />
        </dl>
      </section>
    </div>
  );
}

function getPickupPeople() {
  return [
    { initials: "MM", name: "Mama", relation: "Mutter" },
    { initials: "PM", name: "Papa", relation: "Vater" },
    { initials: "OM", name: "Oma", relation: "Abholung" },
    { initials: "TA", name: "Tante", relation: "Abholung" },
  ];
}

function MobileQuickAction({
  action,
  onClick,
}: Readonly<{
  action: (typeof CHILD_ACTIONS)[number];
  onClick?: () => void;
}>) {
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
        {action.label}
      </span>
    </button>
  );
}

function DesktopQuickAction({
  action,
  onClick,
}: Readonly<{
  action: (typeof CHILD_ACTIONS)[number];
  onClick?: () => void;
}>) {
  const Icon = action.icon;
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
      aria-label={enabled ? action.label : `${action.label} bald verfügbar`}
    >
      <span
        className={`flex h-10 w-10 shrink-0 items-center justify-center rounded-lg ${action.tone}`}
      >
        <Icon className="h-5 w-5" aria-hidden="true" />
      </span>
      <span className="min-w-0">
        <span className="block text-sm font-semibold text-gray-900">
          {action.label}
        </span>
        <span className="mt-0.5 block text-sm leading-5 text-gray-600">
          {action.description}
        </span>
      </span>
    </button>
  );
}

function TodayPanel({ care }: Readonly<{ care: ChildCare }>) {
  const noteCount = care.notes.length;
  return (
    <div className="relative z-10 space-y-4">
      <div>
        <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
          Heute
        </p>
        <p className="mt-1 text-sm leading-6 text-gray-600">
          Aktuelle Hinweise für dieses Kind.
        </p>
      </div>
      <div className="space-y-2">
        <div className="flex items-center gap-3 rounded-xl border border-gray-200 bg-white/85 p-3 shadow-sm">
          <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-[#D6373E]/10 text-[#D6373E]">
            <HeartPulse className="h-5 w-5" aria-hidden="true" />
          </span>
          <div className="min-w-0">
            <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
              Tagesmeldung
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
              Abholung
            </p>
            <p className="mt-0.5 text-sm font-semibold text-gray-900">
              Reguläre Abholung
            </p>
          </div>
        </div>
        <div className="flex items-center gap-3 rounded-xl border border-gray-200 bg-white/85 p-3 shadow-sm">
          <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-[#F78C10]/10 text-[#F78C10]">
            <MessageCircle className="h-5 w-5" aria-hidden="true" />
          </span>
          <div className="min-w-0">
            <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
              Nachrichten
            </p>
            <p className="mt-0.5 text-sm font-semibold text-gray-900">
              {noteCount === 0
                ? "Keine Nachrichten gesendet"
                : `${noteCount} gesendet`}
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
          eyebrow="Elternportal"
          title="Nachrichten an das Team"
          description="Kurze Mitteilungen an die Betreuung."
        />
        {!composeDisabled && (
          <button
            type="button"
            onClick={onCompose}
            className="inline-flex h-9 shrink-0 items-center gap-2 rounded-lg bg-[#F78C10] px-3 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-[#dd7c0c]"
          >
            <MessageCircle className="h-4 w-4" aria-hidden="true" />
            Schreiben
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
  return (
    <section className="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm sm:p-6">
      <div className="flex items-start justify-between gap-4">
        <PanelHeader
          eyebrow="Abholung"
          title="Abholberechtigte"
          description="Personen, die dieses Kind abholen dürfen."
        />
        <button
          type="button"
          disabled
          className="inline-flex h-9 shrink-0 items-center gap-2 rounded-lg border border-gray-200 bg-white px-3 text-sm font-semibold text-gray-700 shadow-sm disabled:cursor-not-allowed disabled:opacity-60"
        >
          <Plus className="h-4 w-4" aria-hidden="true" />
          Hinzufügen
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
  return (
    <section
      className={
        mobile
          ? "rounded-[1.75rem] border border-gray-200 bg-white p-5 shadow-sm"
          : "rounded-2xl border border-gray-200 bg-white p-5 shadow-sm sm:p-6"
      }
    >
      {mobile ? (
        <h2 className="text-lg font-semibold text-gray-900">Neuigkeiten</h2>
      ) : (
        <PanelHeader
          eyebrow="Aktuelles"
          title="Neuigkeiten"
          description="Meldungen zur Gruppe und zur Betreuung."
        />
      )}
      <div className="mt-4 flex items-center gap-3 rounded-xl border border-gray-200 bg-gray-50/70 p-4">
        <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-white text-gray-600 shadow-sm ring-1 ring-gray-200">
          <Newspaper className="h-5 w-5" aria-hidden="true" />
        </span>
        <div className="min-w-0 flex-1">
          <p className="text-sm font-semibold text-gray-900">
            Keine Neuigkeiten vorhanden
          </p>
          <p className="mt-0.5 text-sm leading-5 text-gray-600">
            Meldungen zur Gruppe erscheinen hier.
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
  return (
    <div className="border-b border-gray-100 px-5 py-3 sm:px-6">
      <Link
        href="/parents/children"
        className="inline-flex h-8 items-center gap-2 rounded-lg px-2 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-50 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
      >
        <ArrowLeft className="h-4 w-4" aria-hidden="true" />
        Zurück zu Meine Kinder
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
