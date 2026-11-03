// PROTOTYP, nicht für development. Vergleichsseite für die Optik-Entscheidungen
// des Epics #5576 (Fragen Q8, Q9, Q11, Q12, Q13, Q15 aus dem Grilling am
// 03.11.2026). Jede Story stellt „Heute“ und „Vorschlag“ nebeneinander, in
// einem realistischen Ausschnitt statt im leeren Raum. Lebt nur auf dem Branch
// prototype/ui-kit-entscheidungen.
import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { ReactNode } from "react";
import { Suspense } from "react";
import { CalendarDays, Clock, UserCheck, Users } from "lucide-react";
import "react-day-picker/style.css";

import { Alert } from "./alert";
import { SectionCard } from "./section-card";
import { StatCard } from "./stat-card";
import { StatusBadge } from "./status-badge";
import { StatusColorBadge } from "./status-color-badge";
import { LazyDayPicker } from "./lazy-day-picker";
import { EnrollmentStatTile } from "~/components/enrollment/enrollment-stat-tile";
import { LOCATION_COLORS, MOTO_COLOR_PALETTE } from "~/lib/location-helper";

const meta = {
  title: "Prototyp/UI-Kit-Entscheidungen",
  parameters: { layout: "fullscreen" },
} satisfies Meta;

export default meta;
type Story = StoryObj<typeof meta>;

// ---------------------------------------------------------------- Gerüst

function Board({
  question,
  children,
}: {
  readonly question: string;
  readonly children: ReactNode;
}) {
  return (
    <div className="min-h-screen bg-[#f5f6f8] p-6 max-sm:p-3">
      <p className="mb-4 text-sm text-gray-600">{question}</p>
      <div className="grid gap-6 lg:grid-cols-2">{children}</div>
    </div>
  );
}

function Variant({
  label,
  note,
  children,
}: {
  readonly label: string;
  readonly note: string;
  readonly children: ReactNode;
}) {
  return (
    <div className="flex flex-col gap-2">
      <div>
        <span className="rounded-md bg-gray-900 px-2 py-0.5 text-xs font-semibold text-white">
          {label}
        </span>
        <span className="ml-2 text-xs text-gray-500">{note}</span>
      </div>
      {children}
    </div>
  );
}

// ---------------------------------------------------------------- Q8 Warnfarbe

function AmberHandHint({ text }: { readonly text: string }) {
  // So bauen care-offerings-editor (7×), enrollment-form und child-care ihn heute.
  return (
    <div
      className="rounded-xl border p-3 text-sm"
      style={{
        backgroundColor: MOTO_COLOR_PALETTE.amber.soft,
        color: MOTO_COLOR_PALETTE.amber.strong,
        borderColor: `${MOTO_COLOR_PALETTE.amber.base}55`,
      }}
    >
      {text}
    </div>
  );
}

function WarnContext({ hint }: { readonly hint: ReactNode }) {
  return (
    <SectionCard title="Betreuungsangebot Frühbetreuung">
      <div className="flex flex-col gap-3">
        {hint}
        <ul className="divide-y divide-gray-100 text-sm">
          <li className="flex items-center justify-between py-2">
            <span>Mia Schneider</span>
            <StatusColorBadge
              label="Schulhof"
              color={LOCATION_COLORS.SCHOOLYARD}
            />
          </li>
          <li className="flex items-center justify-between py-2">
            <span>Ben Wagner</span>
            <StatusColorBadge
              label="Anfrage offen"
              color={LOCATION_COLORS.WARNING}
            />
          </li>
          <li className="flex items-center justify-between py-2">
            <span>Lea Fischer</span>
            <StatusColorBadge
              label="Gruppenraum"
              color={LOCATION_COLORS.GROUP_ROOM}
            />
          </li>
        </ul>
      </div>
    </SectionCard>
  );
}

const WARN_TEXT =
  "Für dieses Angebot fehlen noch zwei Betreuungskräfte am Freitag.";

export const Q8_Warnfarbe: Story = {
  name: "Q8 Warnfarbe: Hinweis orange, Status amber",
  render: () => (
    <Board question="Q8: Hinweiskästen orange über Alert, Status-Pillen „Achtung“ amber. Steht der orange Hinweis zu nah am orangen „Schulhof“?">
      <Variant label="Heute" note="9 Hinweise von Hand in Amber">
        <WarnContext hint={<AmberHandHint text={WARN_TEXT} />} />
      </Variant>
      <Variant label="Vorschlag" note="Alert type=warning (orange)">
        <WarnContext
          hint={<Alert type="warning" message={WARN_TEXT} announce="off" />}
        />
      </Variant>
    </Board>
  ),
};

// ---------------------------------------------------------------- Q9 Pille

function HandPillWithDot({
  label,
  color,
}: {
  readonly label: string;
  readonly color: string;
}) {
  return (
    <span className="inline-flex items-center gap-1.5 rounded-full border border-gray-200 bg-white px-2 py-0.5 text-[11px] font-semibold text-gray-700 uppercase">
      <span
        className="h-1.5 w-1.5 rounded-full"
        style={{ backgroundColor: color }}
      />
      {label}
    </span>
  );
}

function DataTableLikeBadge({
  label,
  color,
}: {
  readonly label: string;
  readonly color: string;
}) {
  return (
    <span
      className="inline-flex items-center rounded-full bg-gray-50 px-3 py-1 text-xs font-medium"
      style={{ color }}
    >
      {label}
    </span>
  );
}

// Prototyp der neuen Töne; im Kit fehlen purple, cyan, outline und size xs.
const EXTRA_TONES = {
  purple: {
    bg: MOTO_COLOR_PALETTE.purple.soft,
    text: MOTO_COLOR_PALETTE.purple.strong,
  },
  cyan: {
    bg: MOTO_COLOR_PALETTE.cyan.soft,
    text: MOTO_COLOR_PALETTE.cyan.strong,
  },
} as const;

function ProtoPill({
  label,
  tone,
  size = "md",
}: {
  readonly label: string;
  readonly tone: keyof typeof EXTRA_TONES | "outline";
  readonly size?: "md" | "xs";
}) {
  const sizeCls =
    size === "xs" ? "px-1.5 py-px text-[12px]" : "px-2.5 py-1 text-xs";
  if (tone === "outline") {
    return (
      <span
        className={`inline-flex items-center rounded-full border border-gray-300 bg-white font-medium text-gray-700 ${sizeCls}`}
      >
        {label}
      </span>
    );
  }
  const t = EXTRA_TONES[tone];
  return (
    <span
      className={`inline-flex items-center rounded-full font-medium ${sizeCls}`}
      style={{ backgroundColor: t.bg, color: t.text }}
    >
      {label}
    </span>
  );
}

function Row({
  name,
  children,
}: {
  readonly name: string;
  readonly children: ReactNode;
}) {
  return (
    <li className="flex items-center justify-between gap-3 py-2 text-sm">
      <span className="text-gray-900">{name}</span>
      <span className="flex flex-wrap justify-end gap-1.5">{children}</span>
    </li>
  );
}

export const Q9_Pille: Story = {
  name: "Q9 Eine Pille",
  render: () => (
    <Board question="Q9: Drei Kit-Pillen und ~80 handgebaute werden eine: StatusBadge, neu mit purple, cyan, outline und size xs. Punkte in Pillen fallen weg.">
      <Variant
        label="Heute"
        note="StatusBadge · StatusColorBadge · DataTable-Badge · handgebaut mit Punkt"
      >
        <SectionCard title="Kinder heute">
          <ul className="divide-y divide-gray-100">
            <Row name="Mia Schneider">
              <StatusBadge label="Anwesend" tone="green" />
            </Row>
            <Row name="Ben Wagner">
              <StatusColorBadge
                label="Entschuldigt"
                color={LOCATION_COLORS.EXCUSED}
              />
            </Row>
            <Row name="Lea Fischer">
              <DataTableLikeBadge
                label="Aktiv"
                color={MOTO_COLOR_PALETTE.green.strong}
              />
            </Row>
            <Row name="Noah Becker">
              <HandPillWithDot
                label="Klassenfahrt"
                color={LOCATION_COLORS.CLASS_TRIP}
              />
            </Row>
            <Row name="Emma Hoffmann">
              <HandPillWithDot
                label="Eingeladen"
                color={LOCATION_COLORS.WARNING}
              />
            </Row>
          </ul>
        </SectionCard>
      </Variant>
      <Variant
        label="Vorschlag"
        note="nur StatusBadge, gleiche Form, neue Töne"
      >
        <SectionCard title="Kinder heute">
          <ul className="divide-y divide-gray-100">
            <Row name="Mia Schneider">
              <StatusBadge label="Anwesend" tone="green" />
            </Row>
            <Row name="Ben Wagner">
              <ProtoPill label="Entschuldigt" tone="purple" />
            </Row>
            <Row name="Lea Fischer">
              <StatusBadge label="Aktiv" tone="green" />
            </Row>
            <Row name="Noah Becker">
              <ProtoPill label="Klassenfahrt" tone="cyan" />
            </Row>
            <Row name="Emma Hoffmann">
              <StatusColorBadge
                label="Eingeladen"
                color={LOCATION_COLORS.WARNING}
              />
            </Row>
            <Row name="Dichtes Raster (size xs)">
              <ProtoPill label="Entwurf" tone="outline" size="xs" />
              <ProtoPill label="Ausflug" tone="cyan" size="xs" />
            </Row>
          </ul>
        </SectionCard>
      </Variant>
    </Board>
  ),
};

// ---------------------------------------------------------------- Q11 Kennzahlen

export const Q11_Kennzahlen: Story = {
  name: "Q11 Kennzahlen",
  render: () => (
    <Board question="Q11: StatCard inline nach dem Vorbild der ruhigen Anmeldungs-Kachel (EnrollmentStatTile). Laute Kacheln werden ruhiger.">
      <Variant
        label="Heute"
        note="StatCard card: Versalien-Label, große farbige Zahl"
      >
        <div className="grid grid-cols-2 gap-3">
          <StatCard
            label="Anwesend"
            value={84}
            tone="green"
            icon={<UserCheck className="h-5 w-5" />}
          />
          <StatCard
            label="Krank"
            value={6}
            tone="red"
            icon={<Users className="h-5 w-5" />}
          />
          <StatCard
            label="Stunden"
            value="152h"
            hint="von 160h"
            progressPct={95}
            tone="blue"
          />
          <StatCard
            label="Offen"
            value={3}
            tone="orange"
            icon={<Clock className="h-5 w-5" />}
          />
        </div>
      </Variant>
      <Variant
        label="Vorschlag"
        note="StatCard inline = EnrollmentStatTile-Rezept"
      >
        <div className="grid grid-cols-2 gap-3">
          <EnrollmentStatTile label="Anwesend" value={84} icon={UserCheck} />
          <EnrollmentStatTile label="Krank" value={6} icon={Users} />
          <EnrollmentStatTile
            label="Stunden"
            value="152h"
            detail="von 160h"
            icon={CalendarDays}
          />
          <EnrollmentStatTile label="Offen" value={3} icon={Clock} />
        </div>
      </Variant>
    </Board>
  ),
};

// ---------------------------------------------------------------- Q12 SectionLabel

function GroupedList({ labelClass }: { readonly labelClass: string }) {
  const groups = [
    {
      head: "Montag, 3. November",
      meta: "4 Einträge",
      items: ["Frühbetreuung", "Mittagessen"],
    },
    {
      head: "Dienstag, 4. November",
      meta: "2 Einträge",
      items: ["Hausaufgaben", "AG Fußball"],
    },
  ];
  return (
    <SectionCard title="Meine Woche">
      <div className="flex flex-col gap-4">
        {groups.map((g) => (
          <div key={g.head}>
            <div className="mb-1 flex items-baseline justify-between">
              <h3 className={labelClass}>{g.head}</h3>
              <span className="text-xs text-gray-500">{g.meta}</span>
            </div>
            <ul className="divide-y divide-gray-100 text-sm">
              {g.items.map((i) => (
                <li key={i} className="py-2 text-gray-900">
                  {i}
                </li>
              ))}
            </ul>
          </div>
        ))}
      </div>
    </SectionCard>
  );
}

export const Q12_SectionLabel: Story = {
  name: "Q12 SectionLabel",
  render: () => (
    <Board question="Q12: Gruppenköpfe in Listen. Heute 4 Schreibweisen in Versalien; Vorschlag normal geschrieben.">
      <Variant
        label="Heute"
        note="text-xs semibold tracking-wide gray-500 uppercase"
      >
        <GroupedList labelClass="text-xs font-semibold tracking-wide text-gray-500 uppercase" />
      </Variant>
      <Variant label="Vorschlag" note="text-sm font-semibold gray-900">
        <GroupedList labelClass="text-sm font-semibold text-gray-900" />
      </Variant>
    </Board>
  ),
};

// ---------------------------------------------------------------- Q13 Kalender

const MONTH = new Date(2026, 10, 1);
const RANGE = { from: new Date(2026, 10, 10), to: new Date(2026, 10, 14) };

function Calendar({ tone }: { readonly tone: "green" | "gray" }) {
  const green = tone === "green";
  const pick = green
    ? "[&>button]:!bg-moto-green-fill [&>button]:!text-white"
    : "[&>button]:!bg-gray-900 [&>button]:!text-white";
  const middle = green
    ? "!bg-moto-green/15 [&>button]:!text-moto-green-strong"
    : "!bg-gray-100 [&>button]:!text-gray-900";
  const cell = green ? "w-8 h-8" : "w-9 h-9";
  return (
    <div className="moto-content-surface inline-block rounded-2xl border p-4">
      <Suspense fallback={<div className="h-64 w-64" />}>
        <LazyDayPicker
          mode="range"
          month={MONTH}
          selected={RANGE}
          today={new Date(2026, 10, 3)}
          weekStartsOn={1}
          hideNavigation
          modifiersClassNames={{
            range_start: pick,
            range_end: pick,
            range_middle: middle,
          }}
          classNames={{
            root: "text-sm",
            month_caption: "mb-2 font-semibold text-gray-900",
            weekdays: "flex",
            weekday: `text-gray-500 ${green ? "w-8" : "w-9"} font-normal text-xs text-center`,
            week: "flex w-full mt-1",
            day: `${cell} text-center text-sm p-0 relative`,
            day_button: `${cell} rounded-lg hover:bg-gray-100`,
            today: "font-bold text-moto-blue-strong",
            outside: "text-gray-300",
          }}
        />
      </Suspense>
    </div>
  );
}

export const Q13_Kalender: Story = {
  name: "Q13 Kalender",
  render: () => (
    <Board question="Q13: Auswahl im Kalender. Heute ist der Zeitraum-Kalender grün (h-8), der Einzeldatum-Kalender grau-900 (h-9). Heute-Markierung ist in beiden schon blau.">
      <Variant label="Heute" note="DateRangePicker: grün, Zelle h-8">
        <Calendar tone="green" />
      </Variant>
      <Variant label="Vorschlag" note="beide grau-900, Zelle h-9, heute blau">
        <Calendar tone="gray" />
      </Variant>
    </Board>
  ),
};

// ---------------------------------------------------------------- Q15 Polster

function PaddedCard({
  cls,
  label,
}: {
  readonly cls: string;
  readonly label: string;
}) {
  return (
    <section className={`moto-content-surface rounded-2xl border ${cls}`}>
      <h2 className="text-base font-semibold text-gray-900">Kontaktdaten</h2>
      <p className="mt-1 text-sm text-gray-600">{label}</p>
      <dl className="mt-3 grid grid-cols-2 gap-3 text-sm">
        <div>
          <dt className="text-gray-500">Telefon</dt>
          <dd className="text-gray-900">0251 123456</dd>
        </div>
        <div>
          <dt className="text-gray-500">E-Mail</dt>
          <dd className="truncate text-gray-900">familie@example.com</dd>
        </div>
      </dl>
    </section>
  );
}

export const Q15_Polster: Story = {
  name: "Q15 Kartenpolster",
  render: () => (
    <Board question="Q15: Ein Polster für Karten. SectionCard nutzt schon p-5 / mobil p-4; StatCard und InfoCard weichen ab. Am Telefon vergleichen.">
      <Variant label="Heute" note="p-6 (InfoCard) · p-5 mobil p-3 (StatCard)">
        <div className="flex flex-col gap-3">
          <PaddedCard cls="p-6" label="p-6" />
          <PaddedCard cls="p-5 max-sm:p-3" label="p-5, mobil p-3" />
        </div>
      </Variant>
      <Variant label="Vorschlag" note="überall p-5, mobil p-4 (= SectionCard)">
        <div className="flex flex-col gap-3">
          <PaddedCard cls="p-5 max-sm:p-4" label="p-5, mobil p-4" />
          <PaddedCard cls="p-5 max-sm:p-4" label="p-5, mobil p-4" />
        </div>
      </Variant>
    </Board>
  ),
};
