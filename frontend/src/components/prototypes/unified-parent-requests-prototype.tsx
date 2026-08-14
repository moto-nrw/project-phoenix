"use client";

import {
  ArrowLeft,
  ArrowRight,
  Check,
  ChevronRight,
  Clock3,
  History,
  LockKeyhole,
  ShieldAlert,
  Users,
} from "lucide-react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { Modal } from "~/components/ui/modal";
import { Radio } from "~/components/ui/radio";
import { StatusBadge } from "~/components/ui/status-badge";
import {
  GROUP_ROOM_SHADES,
  LOCATION_COLORS,
  MOTO_COLOR_PALETTE,
} from "~/lib/location-helper";

type RequestState = "pending" | "past" | "decided";
type ChildCase = {
  id: string;
  name: string;
  group: string;
  type: string;
  date: string;
  current: string;
  wanted: string;
  count: number;
  urgent?: boolean;
  conflict?: boolean;
  changedAfterRequest?: boolean;
  protected?: boolean;
  state: RequestState;
};

const CASES: ChildCase[] = [
  {
    id: "emma",
    name: "Emma Berger",
    group: "Sonnengruppe",
    type: "Abholzeit",
    date: "Heute",
    current: "14:00 Uhr",
    wanted: "15:30 Uhr",
    count: 1,
    urgent: true,
    state: "pending",
  },
  {
    id: "noah",
    name: "Noah Fischer",
    group: "Regenbogengruppe",
    type: "Abholzeit",
    date: "Heute",
    current: "14:00 Uhr",
    wanted: "14:30 oder 15:00 Uhr",
    count: 2,
    urgent: true,
    conflict: true,
    state: "pending",
  },
  {
    id: "lina",
    name: "Lina Scholz",
    group: "Sonnengruppe",
    type: "Abholart",
    date: "Morgen",
    current: "Wird abgeholt",
    wanted: "Darf allein gehen",
    count: 1,
    changedAfterRequest: true,
    state: "pending",
  },
  {
    id: "malik",
    name: "Malik Yilmaz",
    group: "Wiesengruppe",
    type: "Abholzeit",
    date: "12. August",
    current: "15:00 Uhr",
    wanted: "14:30 Uhr",
    count: 1,
    state: "past",
  },
  {
    id: "sophie",
    name: "Sophie Wagner",
    group: "Regenbogengruppe",
    type: "Betreuungszeit",
    date: "Ab 1. September",
    current: "Montag bis 15:00 Uhr",
    wanted: "Montag bis 16:00 Uhr",
    count: 1,
    state: "decided",
  },
  {
    id: "jonas",
    name: "Jonas Kaya",
    group: "Wiesengruppe",
    type: "Abholberechtigung",
    date: "Ab sofort",
    current: "Nur Eltern",
    wanted: "Zusätzlich Nadine Kaya",
    count: 1,
    protected: true,
    state: "pending",
  },
];

const VARIANTS = [
  { key: "A", label: "Dringlichkeit zuerst" },
  { key: "B", label: "Arbeitsliste mit Vorschau" },
  { key: "C", label: "Kinder als Aufgaben" },
] as const;

function isBulkEligible(item: ChildCase) {
  return (
    item.state === "pending" && !item.conflict && !item.changedAfterRequest
  );
}

function Badge({ item }: { item: ChildCase }) {
  if (item.conflict) return <StatusBadge label="2 Wünsche" tone="red" />;
  if (item.changedAfterRequest)
    return <StatusBadge label="Bitte prüfen" tone="orange" />;
  if (item.state === "past")
    return <StatusBadge label="Vergangen" tone="gray" />;
  if (item.state === "decided")
    return <StatusBadge label="Entschieden" tone="green" />;
  return item.urgent ? (
    <StatusBadge label="Heute" tone="orange" />
  ) : (
    <StatusBadge label="Offen" tone="blue" />
  );
}

function ChildSummary({
  item,
  active,
  onOpen,
  selected,
  onSelect,
  compact = false,
}: {
  item: ChildCase;
  active: boolean;
  onOpen: () => void;
  selected: boolean;
  onSelect: (checked: boolean) => void;
  compact?: boolean;
}) {
  const eligible = isBulkEligible(item);
  return (
    <article
      className={`rounded-2xl bg-white p-4 shadow-sm ring-1 transition-[box-shadow,ring-color] ${
        active ? "ring-2 ring-gray-900" : "ring-gray-200"
      }`}
      style={
        item.urgent
          ? { borderInlineStart: `5px solid ${LOCATION_COLORS.WARNING}` }
          : undefined
      }
    >
      <div className="flex items-start gap-3">
        <label
          htmlFor={`bulk-${item.id}`}
          className="relative flex min-h-11 min-w-11 cursor-pointer items-center justify-center self-center"
          title={
            eligible
              ? "Für die gemeinsame Freigabe auswählen"
              : "Diese Anfrage braucht eine einzelne Prüfung"
          }
        >
          <Checkbox
            id={`bulk-${item.id}`}
            aria-label={`${item.name} auswählen`}
            checked={selected}
            disabled={!eligible}
            onChange={(event) => onSelect(event.target.checked)}
          />
        </label>
        <button
          type="button"
          onClick={onOpen}
          className="min-w-0 flex-1 rounded-xl text-start focus-visible:ring-2 focus-visible:ring-gray-900 focus-visible:ring-offset-4 focus-visible:outline-none"
          aria-current={active ? "true" : undefined}
        >
          <span className="flex flex-wrap items-start justify-between gap-2">
            <span>
              <span className="block text-base font-semibold text-gray-950">
                {item.name}
              </span>
              <span className="mt-0.5 block text-sm text-gray-600">
                {item.group}
              </span>
            </span>
            <Badge item={item} />
          </span>
          <span className="mt-3 flex items-center justify-between gap-3">
            <span className="min-w-0">
              <span className="block text-sm font-medium text-gray-900">
                {item.type} · {item.date}
              </span>
              {!compact && (
                <span className="mt-1 block text-sm leading-6 text-gray-600">
                  {item.count === 1 ? "1 Anfrage" : `${item.count} Anfragen`} ·
                  Gewünscht: {item.wanted}
                </span>
              )}
            </span>
            <ChevronRight
              className="h-5 w-5 shrink-0 text-gray-500"
              aria-hidden="true"
            />
          </span>
        </button>
      </div>
      {!eligible && item.state === "pending" && (
        <p className="mt-3 ps-14 text-xs leading-5 text-gray-600">
          Nicht gemeinsam freigebbar:{" "}
          {item.conflict
            ? "Es gibt zwei Wünsche."
            : "Der aktuelle Wert hat sich geändert."}
        </p>
      )}
    </article>
  );
}

function ValueComparison({ item }: { item: ChildCase }) {
  return (
    <div className="grid gap-3 sm:grid-cols-2">
      <div className="rounded-xl bg-gray-100 p-4">
        <span className="text-xs font-semibold tracking-wide text-gray-600 uppercase">
          Aktuell
        </span>
        <strong className="mt-1 block text-base text-gray-950">
          {item.current}
        </strong>
      </div>
      <div
        className="rounded-xl p-4"
        style={{ backgroundColor: MOTO_COLOR_PALETTE.green.soft }}
      >
        <span
          className="text-xs font-semibold tracking-wide uppercase"
          style={{ color: GROUP_ROOM_SHADES.text }}
        >
          Gewünscht
        </span>
        <strong className="mt-1 block text-base text-gray-950">
          {item.wanted}
        </strong>
      </div>
    </div>
  );
}

function PrivacyPanel({ item }: { item: ChildCase }) {
  const [share, setShare] = useState(false);
  return (
    <section className="rounded-2xl bg-white p-4 shadow-sm ring-1 ring-gray-200">
      <h3 className="flex items-center gap-2 text-base font-semibold text-gray-950">
        <LockKeyhole className="h-5 w-5" aria-hidden="true" /> Wer sieht die
        Anfrage?
      </h3>
      <div className="mt-4 rounded-xl bg-gray-50 p-3 text-sm leading-6 text-gray-700">
        <p>
          <strong>Alle Sorgeberechtigten sehen:</strong> die neue Abholzeit.
        </p>
        <p className="mt-2">
          <strong>Nur Sie und die OGS sehen:</strong> wer die Anfrage stellt und
          den Grund.
        </p>
      </div>
      {item.protected ? (
        <div
          className="mt-4 flex gap-3 rounded-xl p-3"
          style={{ backgroundColor: MOTO_COLOR_PALETTE.orange.soft }}
        >
          <ShieldAlert
            className="mt-0.5 h-5 w-5 shrink-0"
            style={{ color: MOTO_COLOR_PALETTE.orange.strong }}
            aria-hidden="true"
          />
          <p className="text-sm leading-6 text-gray-800">
            Diese Anfrage kann nicht geteilt werden. Die OGS schützt die Angaben
            der Familie.
          </p>
        </div>
      ) : (
        <label
          htmlFor={`share-${item.id}`}
          className="mt-4 flex min-h-11 cursor-pointer items-start gap-3 rounded-xl p-3 ring-1 ring-gray-200"
        >
          <Checkbox
            id={`share-${item.id}`}
            checked={share}
            onChange={(event) => setShare(event.target.checked)}
          />
          <span className="text-sm leading-6 text-gray-800">
            <strong className="block">Anfrage mit Alex Berger teilen</strong>
            Alex sieht dann auch den Grund und die Antwort der OGS.
          </span>
        </label>
      )}
    </section>
  );
}

function DetailPanel({
  item,
  onBack,
}: {
  item: ChildCase;
  onBack: () => void;
}) {
  const [choice, setChoice] = useState(item.conflict ? "1430" : "wanted");
  const [custom, setCustom] = useState("");
  const [decision, setDecision] = useState(
    item.state === "decided" ? "16:00 Uhr" : "",
  );
  const [showCorrection, setShowCorrection] = useState(false);

  return (
    <div className="space-y-5 pb-28 lg:pb-8">
      <div className="flex items-center gap-3 lg:hidden">
        <Button
          type="button"
          variant="ghost"
          size="md"
          onClick={onBack}
          className="min-h-11 px-3"
        >
          <ArrowLeft className="me-2 h-5 w-5" aria-hidden="true" /> Zur Liste
        </Button>
      </div>
      <header>
        <div className="flex flex-wrap items-center gap-2">
          <Badge item={item} />
          <span className="text-sm text-gray-600">
            {item.type} · {item.date}
          </span>
        </div>
        <h2 className="mt-2 text-2xl font-semibold tracking-tight text-gray-950">
          {item.name}
        </h2>
        <p className="mt-1 text-sm text-gray-600">
          {item.group} ·{" "}
          {item.count === 1 ? "1 Anfrage" : `${item.count} Anfragen`}
        </p>
      </header>

      <ValueComparison item={item} />

      {item.changedAfterRequest && (
        <div
          className="flex gap-3 rounded-2xl p-4 ring-1"
          style={{
            backgroundColor: MOTO_COLOR_PALETTE.orange.soft,
            boxShadow: `inset 0 0 0 1px ${LOCATION_COLORS.WARNING}`,
          }}
        >
          <Clock3 className="mt-0.5 h-5 w-5 shrink-0" aria-hidden="true" />
          <p className="text-sm leading-6 text-gray-800">
            <strong className="block">
              Aktueller Wert wurde später geändert
            </strong>
            Prüfen Sie den neuen Stand vor Ihrer Entscheidung.
          </p>
        </div>
      )}

      {item.conflict && (
        <fieldset className="rounded-2xl bg-white p-4 shadow-sm ring-1 ring-gray-200">
          <legend className="px-1 text-base font-semibold text-gray-950">
            Eine Abholzeit festlegen
          </legend>
          <p className="mt-1 text-sm leading-6 text-gray-600">
            Es kann nur eine Abholzeit gelten.
          </p>
          <div className="mt-4 space-y-3">
            {(
              [
                ["1430", "14:30 Uhr", "Wunsch von Mira Fischer"],
                ["1500", "15:00 Uhr", "Wunsch von Daniel Fischer"],
                ["current", "14:00 Uhr", "Nichts ändern"],
                ["custom", "Andere Zeit", "Zeit selbst eintragen"],
              ] as const
            ).map(([value, label, hint]) => (
              <label
                key={value}
                htmlFor={`conflict-${value}`}
                className="flex min-h-14 cursor-pointer items-center gap-3 rounded-xl p-3 ring-1 ring-gray-200 has-[:checked]:ring-2 has-[:checked]:ring-gray-900"
              >
                <Radio
                  id={`conflict-${value}`}
                  name="conflict-result"
                  value={value}
                  checked={choice === value}
                  onChange={() => setChoice(value)}
                />
                <span className="text-sm">
                  <strong className="block text-gray-950">{label}</strong>
                  <span className="text-gray-600">{hint}</span>
                </span>
              </label>
            ))}
          </div>
          {choice === "custom" && (
            <label className="mt-4 block text-sm font-medium text-gray-800">
              Andere Abholzeit
              <input
                value={custom}
                onChange={(event) => setCustom(event.target.value)}
                placeholder="Zum Beispiel 15:30 Uhr"
                className="mt-2 min-h-11 w-full rounded-xl border border-gray-300 bg-white px-3 text-base focus-visible:ring-2 focus-visible:ring-gray-900 focus-visible:outline-none"
              />
            </label>
          )}
        </fieldset>
      )}

      {item.state === "past" ? (
        <div className="rounded-2xl bg-gray-100 p-4">
          <h3 className="font-semibold text-gray-950">Der Tag ist vorbei</h3>
          <p className="mt-1 text-sm leading-6 text-gray-600">
            Die Betreuung wird nicht mehr geändert.
          </p>
          <Button
            type="button"
            variant="secondary"
            className="mt-4 w-full sm:w-auto"
          >
            Als erledigt markieren
          </Button>
        </div>
      ) : item.state === "decided" || decision ? (
        <section className="rounded-2xl bg-white p-4 shadow-sm ring-1 ring-gray-200">
          <h3 className="flex items-center gap-2 font-semibold text-gray-950">
            <History className="h-5 w-5" aria-hidden="true" /> Verlauf
          </h3>
          <ol className="mt-4 space-y-4 text-sm leading-6 text-gray-700">
            <li>
              <strong className="block text-gray-950">
                Heute, 10:24 · Entscheidung
              </strong>
              {decision || "Montag bis 16:00 Uhr"} · Sarah Klein
            </li>
            <li>
              <strong className="block text-gray-950">
                Gestern, 18:42 · Anfrage
              </strong>
              Montag bis 16:00 Uhr · Mira Wagner
            </li>
          </ol>
          {!showCorrection ? (
            <Button
              type="button"
              variant="ghost"
              className="mt-4"
              onClick={() => setShowCorrection(true)}
            >
              Entscheidung korrigieren
            </Button>
          ) : (
            <div className="mt-4 rounded-xl bg-gray-50 p-4">
              <label className="block text-sm font-medium text-gray-800">
                Neues Ergebnis
                <input
                  className="mt-2 min-h-11 w-full rounded-xl border border-gray-300 bg-white px-3 text-base"
                  defaultValue="15:30 Uhr"
                />
              </label>
              <p className="mt-3 text-sm text-gray-600">
                Die alte Entscheidung bleibt im Verlauf. Die Sorgeberechtigten
                erhalten eine Nachricht.
              </p>
              <Button
                type="button"
                variant="primary"
                className="mt-4"
                onClick={() => {
                  setDecision("15:30 Uhr · korrigiert");
                  setShowCorrection(false);
                }}
              >
                Korrektur speichern
              </Button>
            </div>
          )}
        </section>
      ) : (
        <div className="rounded-2xl bg-white p-4 shadow-xl ring-1 ring-gray-200 lg:sticky lg:bottom-4 lg:z-10">
          <p className="text-sm font-medium text-gray-800">
            Ausgewähltes Ergebnis:{" "}
            <strong>
              {item.conflict
                ? choice === "custom"
                  ? custom || "Andere Zeit"
                  : choice === "1430"
                    ? "14:30 Uhr"
                    : choice === "1500"
                      ? "15:00 Uhr"
                      : "Keine Änderung"
                : item.wanted}
            </strong>
          </p>
          <div className="mt-3 flex flex-col-reverse gap-3 sm:flex-row sm:justify-end">
            <Button type="button" variant="outline_danger">
              Ablehnen
            </Button>
            <Button
              type="button"
              variant="success"
              onClick={() => setDecision(item.wanted)}
            >
              Ergebnis übernehmen
            </Button>
          </div>
        </div>
      )}

      <PrivacyPanel item={item} />
    </div>
  );
}

function VariantA(props: VariantProps) {
  const urgent = props.items.filter(
    (item) => item.urgent && item.state === "pending",
  );
  const later = props.items.filter(
    (item) => !item.urgent || item.state !== "pending",
  );
  return (
    <MasterDetail
      {...props}
      list={
        <div className="space-y-7">
          <section>
            <h2 className="mb-3 text-lg font-semibold text-gray-950">
              Heute wichtig{" "}
              <span className="text-gray-500">{urgent.length}</span>
            </h2>
            <div className="space-y-3">
              {urgent.map((item) => (
                <SummaryFromProps key={item.id} item={item} {...props} />
              ))}
            </div>
          </section>
          <section>
            <h2 className="mb-3 text-lg font-semibold text-gray-950">
              Danach <span className="text-gray-500">{later.length}</span>
            </h2>
            <div className="space-y-3">
              {later.map((item) => (
                <SummaryFromProps key={item.id} item={item} {...props} />
              ))}
            </div>
          </section>
        </div>
      }
    />
  );
}

function VariantB(props: VariantProps) {
  return (
    <MasterDetail
      {...props}
      list={
        <div className="overflow-hidden rounded-2xl bg-white shadow-sm ring-1 ring-gray-200">
          <div className="bg-gray-900 px-4 py-3 text-sm font-semibold text-white">
            Offene Arbeitsliste
          </div>
          <div className="divide-y divide-gray-100">
            {props.items.map((item) => (
              <div key={item.id} className="p-3">
                <SummaryFromProps item={item} {...props} compact />
              </div>
            ))}
          </div>
        </div>
      }
    />
  );
}

function VariantC(props: VariantProps) {
  const groups = ["Sonnengruppe", "Regenbogengruppe", "Wiesengruppe"];
  return (
    <MasterDetail
      {...props}
      list={
        <div className="space-y-5">
          {groups.map((group) => {
            const items = props.items.filter((item) => item.group === group);
            return (
              <section
                key={group}
                className="rounded-2xl bg-white p-4 shadow-sm ring-1 ring-gray-200"
              >
                <h2 className="flex items-center gap-2 text-base font-semibold text-gray-950">
                  <Users className="h-5 w-5" aria-hidden="true" />
                  {group}
                  <span className="ms-auto rounded-full bg-gray-100 px-2 py-0.5 text-sm">
                    {items.length}
                  </span>
                </h2>
                <div className="mt-4 space-y-3">
                  {items.map((item) => (
                    <SummaryFromProps
                      key={item.id}
                      item={item}
                      {...props}
                      compact
                    />
                  ))}
                </div>
              </section>
            );
          })}
        </div>
      }
    />
  );
}

type VariantProps = {
  items: ChildCase[];
  activeId: string | null;
  setActiveId: (id: string | null) => void;
  selected: Set<string>;
  toggleSelected: (id: string, checked: boolean) => void;
  restoreFocus: (id: string) => void;
};

function SummaryFromProps({
  item,
  activeId,
  setActiveId,
  selected,
  toggleSelected,
  compact = false,
}: VariantProps & { item: ChildCase; compact?: boolean }) {
  return (
    <div data-child-trigger={item.id}>
      <ChildSummary
        item={item}
        active={activeId === item.id}
        selected={selected.has(item.id)}
        onSelect={(checked) => toggleSelected(item.id, checked)}
        onOpen={() => setActiveId(item.id)}
        compact={compact}
      />
    </div>
  );
}

function MasterDetail({
  list,
  items,
  activeId,
  setActiveId,
  restoreFocus,
}: VariantProps & { list: React.ReactNode }) {
  const item = items.find((candidate) => candidate.id === activeId) ?? null;
  return (
    <div className="lg:grid lg:grid-cols-[minmax(20rem,0.85fr)_minmax(28rem,1.35fr)] lg:gap-6">
      <div className={item ? "hidden lg:block" : "block"}>{list}</div>
      <div className={item ? "block" : "hidden lg:block"}>
        {item ? (
          <DetailPanel
            key={item.id}
            item={item}
            onBack={() => {
              const id = item.id;
              setActiveId(null);
              requestAnimationFrame(() => restoreFocus(id));
            }}
          />
        ) : (
          <div className="grid min-h-80 place-items-center rounded-2xl bg-white p-8 text-center shadow-sm ring-1 ring-gray-200">
            <div>
              <Check
                className="mx-auto h-10 w-10 text-gray-400"
                aria-hidden="true"
              />
              <h2 className="mt-4 text-lg font-semibold text-gray-950">
                Kind auswählen
              </h2>
              <p className="mt-1 text-sm text-gray-600">
                Hier sehen Sie die Anfrage und entscheiden.
              </p>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

function PrototypeSwitcher({ current }: { current: string }) {
  const router = useRouter();
  const pathname = usePathname();
  const params = useSearchParams();
  const currentIndex = Math.max(
    0,
    VARIANTS.findIndex((item) => item.key === current),
  );
  const move = useCallback(
    (delta: number) => {
      const next =
        VARIANTS[(currentIndex + delta + VARIANTS.length) % VARIANTS.length]!;
      const nextParams = new URLSearchParams(params.toString());
      nextParams.set("variant", next.key);
      router.replace(`${pathname}?${nextParams.toString()}`);
    },
    [currentIndex, params, pathname, router],
  );
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement | null;
      if (target?.matches("input, textarea, [contenteditable='true']")) return;
      if (event.key === "ArrowLeft") move(-1);
      if (event.key === "ArrowRight") move(1);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [move]);
  const info = VARIANTS[currentIndex]!;
  return (
    <div
      className="fixed bottom-[max(5rem,calc(4rem+env(safe-area-inset-bottom)))] left-1/2 z-50 flex -translate-x-1/2 items-center gap-1 rounded-full bg-gray-950 p-1.5 text-white shadow-2xl lg:bottom-[max(1rem,env(safe-area-inset-bottom))]"
      aria-label="Prototyp-Variante wechseln"
    >
      <button
        type="button"
        onClick={() => move(-1)}
        className="grid h-11 w-11 place-items-center rounded-full hover:bg-white/15 focus-visible:ring-2 focus-visible:ring-white focus-visible:outline-none"
        aria-label="Vorherige Variante"
      >
        <ArrowLeft className="h-5 w-5" aria-hidden="true" />
      </button>
      <span className="min-w-36 px-2 text-center text-xs font-medium sm:min-w-52">
        <strong>{info.key}</strong> · {info.label}
      </span>
      <button
        type="button"
        onClick={() => move(1)}
        className="grid h-11 w-11 place-items-center rounded-full hover:bg-white/15 focus-visible:ring-2 focus-visible:ring-white focus-visible:outline-none"
        aria-label="Nächste Variante"
      >
        <ArrowRight className="h-5 w-5" aria-hidden="true" />
      </button>
    </div>
  );
}

export function UnifiedParentRequestsPrototype() {
  const params = useSearchParams();
  const variant = VARIANTS.some((item) => item.key === params.get("variant"))
    ? params.get("variant")!
    : "A";
  const [activeId, setActiveId] = useState<string | null>(null);
  const [selected, setSelected] = useState<Set<string>>(() => new Set());
  const [bulkOpen, setBulkOpen] = useState(false);
  const [bulkDone, setBulkDone] = useState(false);
  const mainRef = useRef<HTMLElement>(null);
  const activeItems = useMemo(
    () => CASES.filter((item) => !(bulkDone && selected.has(item.id))),
    [bulkDone, selected],
  );
  const toggleSelected = (id: string, checked: boolean) =>
    setSelected((current) => {
      const next = new Set(current);
      if (checked) next.add(id);
      else next.delete(id);
      return next;
    });
  const restoreFocus = (id: string) =>
    document
      .querySelector<HTMLElement>(`[data-child-trigger="${id}"] button`)
      ?.focus();
  const props: VariantProps = {
    items: activeItems,
    activeId,
    setActiveId,
    selected,
    toggleSelected,
    restoreFocus,
  };

  return (
    <main ref={mainRef} className="mx-auto w-full max-w-[90rem] pb-28">
      <div className="mb-6 flex flex-wrap items-end justify-between gap-4">
        <div>
          <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
            Prototyp · nicht für den Betrieb
          </p>
          <h1 className="mt-1 text-2xl font-semibold tracking-tight text-gray-950 sm:text-3xl">
            Elternanfragen
          </h1>
          <p className="mt-2 max-w-2xl text-sm leading-6 text-gray-600">
            Anfragen sind nach Kindern gebündelt. Prüfen Sie jeden Wunsch vor
            der Freigabe.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <StatusBadge
            label={`${activeItems.filter((item) => item.state === "pending").length} offen`}
            tone="blue"
          />
          <StatusBadge label="2 heute" tone="orange" />
        </div>
      </div>

      {bulkDone && (
        <div
          role="status"
          className="mb-4 flex items-center gap-3 rounded-xl p-4"
          style={{ backgroundColor: MOTO_COLOR_PALETTE.green.soft }}
        >
          <Check className="h-5 w-5" aria-hidden="true" />
          Die Auswahl wurde gemeinsam freigegeben.
        </div>
      )}

      {selected.size > 0 && (
        <div className="sticky top-3 z-20 mb-5 flex flex-wrap items-center gap-3 rounded-2xl bg-gray-950 p-3 text-white shadow-xl">
          <span className="me-auto text-sm font-medium">
            {selected.size} ausgewählt
          </span>
          <button
            type="button"
            className="min-h-11 rounded-lg px-3 text-sm hover:bg-white/10"
            onClick={() => setSelected(new Set())}
          >
            Auswahl aufheben
          </button>
          <Button
            type="button"
            variant="success"
            size="md"
            onClick={() => setBulkOpen(true)}
          >
            Gemeinsam freigeben
          </Button>
        </div>
      )}

      {variant === "A" && <VariantA {...props} />}
      {variant === "B" && <VariantB {...props} />}
      {variant === "C" && <VariantC {...props} />}

      <Modal
        isOpen={bulkOpen}
        onClose={() => setBulkOpen(false)}
        title={`${selected.size} Anfragen gemeinsam freigeben`}
        footer={
          <div className="flex w-full flex-col-reverse gap-3 sm:flex-row sm:justify-end">
            <Button
              type="button"
              variant="outline"
              onClick={() => setBulkOpen(false)}
            >
              Abbrechen
            </Button>
            <Button
              type="button"
              variant="success"
              onClick={() => {
                setBulkDone(true);
                setBulkOpen(false);
                setActiveId(null);
              }}
            >
              Alle freigeben
            </Button>
          </div>
        }
      >
        <div className="space-y-4 text-sm leading-6 text-gray-700">
          <p>
            Alle ausgewählten Wünsche werden übernommen. Das geschieht nur, wenn
            alle Anfragen noch aktuell sind.
          </p>
          <div className="rounded-xl bg-gray-100 p-3">
            <strong className="block text-gray-950">Gemeinsamer Grund</strong>
            „Geprüft im heutigen Elternanfragen-Eingang“ wird für alle
            Entscheidungen gespeichert.
          </div>
          <p>
            Konflikte und später geänderte Werte konnten nicht ausgewählt
            werden.
          </p>
        </div>
      </Modal>

      <PrototypeSwitcher current={variant} />
    </main>
  );
}
