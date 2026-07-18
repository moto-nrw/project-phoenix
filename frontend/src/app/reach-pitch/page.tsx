import Image from "next/image";
import type { ReactNode } from "react";
import {
  Building2,
  ClipboardList,
  DoorOpen,
  HeartHandshake,
  MessageCircle,
  Radar,
  ScanLine,
  ShieldCheck,
  Users,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";

export const metadata = {
  title: "moto REACH Pitch Night",
  description:
    "Pitch-Präsentation für die REACH Pitch Night 2026 im moto Design.",
};

const colors = {
  green: "#83CD2D",
  greenText: "#3F6F12",
  blue: "#5080D8",
  orange: "#F78C10",
  red: "#FF3130",
  purple: "#7C3AED",
} as const;

type Tone = "green" | "blue" | "orange" | "red" | "purple" | "gray";

const toneClass: Record<
  Tone,
  {
    readonly bg: string;
    readonly text: string;
    readonly border: string;
    readonly dot: string;
  }
> = {
  green: {
    bg: "bg-[#83CD2D]/14",
    text: "text-[#3F6F12]",
    border: "border-[#83CD2D]/25",
    dot: "bg-[#83CD2D]",
  },
  blue: {
    bg: "bg-[#5080D8]/12",
    text: "text-[#315C9B]",
    border: "border-[#5080D8]/25",
    dot: "bg-[#5080D8]",
  },
  orange: {
    bg: "bg-[#F78C10]/12",
    text: "text-[#9B5609]",
    border: "border-[#F78C10]/25",
    dot: "bg-[#F78C10]",
  },
  red: {
    bg: "bg-[#FF3130]/10",
    text: "text-[#CC2626]",
    border: "border-[#FF3130]/20",
    dot: "bg-[#FF3130]",
  },
  purple: {
    bg: "bg-[#7C3AED]/10",
    text: "text-[#6D28D9]",
    border: "border-[#7C3AED]/20",
    dot: "bg-[#7C3AED]",
  },
  gray: {
    bg: "bg-gray-100",
    text: "text-gray-700",
    border: "border-gray-200",
    dot: "bg-gray-400",
  },
};

const marketStats = [
  {
    value: "$643M",
    label: "Child-Care-Software-Markt 2026",
    tone: "blue" as Tone,
  },
  {
    value: "$1.38B",
    label: "Prognose bis 2035",
    tone: "green" as Tone,
  },
  {
    value: "264k",
    label: "zusätzliche Ganztagsplätze bis 2029/30",
    tone: "orange" as Tone,
  },
];

const productCards = [
  {
    title: "NFC-Tablet",
    body: "Kinder melden sich selbstständig an und sehen, was heute relevant ist.",
    image: "/help/screens/nfc-hauptbildschirm.webp",
    tone: "green" as Tone,
  },
  {
    title: "Betreuer-App",
    body: "Das Team sieht Kinder, Räume, Hinweise und Abholung im laufenden Tag.",
    image: "/help/screens/meine-gruppen.webp",
    tone: "blue" as Tone,
  },
  {
    title: "OGS-Büro",
    body: "Leitung und Verwaltung pflegen Stammdaten, Gruppen, Angebote und Planung.",
    image: "/help/screens/datenverwaltung.webp",
    tone: "orange" as Tone,
  },
];

const modules = [
  {
    title: "OGS",
    price: "0,99 Euro",
    unit: "pro Kind und Monat",
    body: "Anwesenheit, Räume, Kinderprofile",
    tone: "green" as Tone,
  },
  {
    title: "Eltern",
    price: "0,49 Euro",
    unit: "pro Kind und Monat",
    body: "Krankmeldung, Abholung, Kommunikation",
    tone: "orange" as Tone,
  },
  {
    title: "Personal",
    price: "199 Euro",
    unit: "pro Standort und Monat",
    body: "Dienstplan, Zeiterfassung, Vertretung",
    tone: "blue" as Tone,
  },
];

const progressItems = [
  {
    title: "NFC schneller einführbar",
    body: "Quickstart, Onepager und Hilfesuche wurden ausgebaut.",
    icon: ScanLine,
    tone: "green" as Tone,
  },
  {
    title: "Planung wird belastbarer",
    body: "Schichtarten, Vertretungen, Plan/Ist und Änderungsprotokoll sind weiter gereift.",
    icon: ClipboardList,
    tone: "blue" as Tone,
  },
  {
    title: "Eltern- und Anmeldeflüsse wachsen",
    body: "Abholung, Abwesenheit und Betreuungsangebote wurden im Alltag robuster.",
    icon: MessageCircle,
    tone: "orange" as Tone,
  },
  {
    title: "Echte Zahlen ergänzen",
    body: "Pilotstandorte, Interviews, Umsatz und Testimonials gehören hier als nächste Schärfung rein.",
    icon: Radar,
    tone: "red" as Tone,
  },
];

const askItems = [
  {
    title: "Pilotkontakte",
    body: "Träger, OGS-Leitungen und Kommunen",
    icon: DoorOpen,
    tone: "green" as Tone,
  },
  {
    title: "Beta-Feedback",
    body: "Teams, die echte Nachmittage testen",
    icon: Users,
    tone: "blue" as Tone,
  },
  {
    title: "Netzwerk",
    body: "Vergabe, Datenschutz und Bildungsträger",
    icon: HeartHandshake,
    tone: "orange" as Tone,
  },
];

function Slide({
  eyebrow,
  children,
  index,
  className = "",
}: {
  readonly eyebrow?: string;
  readonly children: ReactNode;
  readonly index: number;
  readonly className?: string;
}) {
  return (
    <section
      className={`pitch-slide moto-dotted-background moto-dotted-background--fullscreen relative mx-auto grid aspect-video w-full max-w-[1440px] overflow-hidden rounded-[28px] border border-gray-200 bg-gray-50 shadow-sm print:rounded-none print:border-0 print:shadow-none ${className}`}
    >
      <div className="relative z-10 flex h-full flex-col p-[5.2%]">
        <div className="flex items-center justify-between gap-6">
          <div className="flex min-h-8 items-center gap-3">
            <Image
              src="/images/moto-logo-mit-schriftzug.webp"
              alt="moto"
              width={150}
              height={48}
              priority={index === 1}
              className="h-8 w-auto object-contain"
            />
            {eyebrow ? (
              <span className="rounded-full border border-[#83CD2D]/25 bg-white/80 px-3 py-1 text-[clamp(0.62rem,0.95vw,0.86rem)] font-bold tracking-[0.08em] text-[#3F6F12] uppercase shadow-sm backdrop-blur">
                {eyebrow}
              </span>
            ) : null}
          </div>
          <span className="text-[clamp(0.62rem,0.9vw,0.82rem)] font-semibold tracking-normal text-gray-400">
            {String(index).padStart(2, "0")}
          </span>
        </div>
        <div className="flex min-h-0 flex-1 flex-col">{children}</div>
      </div>
    </section>
  );
}

function Eyebrow({ children }: { readonly children: ReactNode }) {
  return (
    <p className="text-[clamp(0.7rem,1vw,0.9rem)] font-bold tracking-[0.08em] text-[#3F6F12] uppercase">
      {children}
    </p>
  );
}

function Headline({
  children,
  className = "",
}: {
  readonly children: ReactNode;
  readonly className?: string;
}) {
  return (
    <h1
      className={`mt-3 max-w-[15ch] text-[clamp(2rem,5.4vw,5.25rem)] leading-[0.96] font-semibold tracking-normal text-gray-950 ${className}`}
    >
      {children}
    </h1>
  );
}

function Lead({
  children,
  className = "",
}: {
  readonly children: ReactNode;
  readonly className?: string;
}) {
  return (
    <p
      className={`mt-5 max-w-[48rem] text-[clamp(1rem,1.55vw,1.45rem)] leading-[1.45] text-gray-600 ${className}`}
    >
      {children}
    </p>
  );
}

function Surface({
  children,
  className = "",
}: {
  readonly children: ReactNode;
  readonly className?: string;
}) {
  return (
    <div
      className={`moto-content-surface rounded-2xl border border-gray-200 bg-white/88 shadow-sm backdrop-blur-md ${className}`}
    >
      {children}
    </div>
  );
}

function ToneIcon({
  icon: Icon,
  tone,
}: {
  readonly icon: LucideIcon;
  readonly tone: Tone;
}) {
  const toneClasses = toneClass[tone];
  return (
    <span
      className={`flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl ${toneClasses.bg} ${toneClasses.text}`}
    >
      <Icon className="h-5 w-5" aria-hidden="true" />
    </span>
  );
}

function Label({
  children,
  tone = "green",
}: {
  readonly children: ReactNode;
  readonly tone?: Tone;
}) {
  const toneClasses = toneClass[tone];
  return (
    <span
      className={`inline-flex w-fit items-center rounded-full border px-3 py-1 text-[clamp(0.66rem,0.9vw,0.82rem)] font-semibold ${toneClasses.border} ${toneClasses.bg} ${toneClasses.text}`}
    >
      {children}
    </span>
  );
}

function MarketLine() {
  const points = [
    { x: 0, y: 76 },
    { x: 25, y: 64 },
    { x: 50, y: 48 },
    { x: 75, y: 32 },
    { x: 100, y: 5 },
  ];
  const path = points
    .map((point, index) => `${index === 0 ? "M" : "L"} ${point.x} ${point.y}`)
    .join(" ");
  return (
    <svg
      viewBox="0 0 100 86"
      className="h-full w-full overflow-visible"
      role="img"
      aria-label="Wachsender Markt von 2026 bis 2035"
    >
      <path
        d="M0 80 H100"
        stroke="#E5E7EB"
        strokeWidth="1"
        vectorEffect="non-scaling-stroke"
      />
      <path
        d="M0 56 H100"
        stroke="#E5E7EB"
        strokeWidth="1"
        vectorEffect="non-scaling-stroke"
      />
      <path
        d="M0 32 H100"
        stroke="#E5E7EB"
        strokeWidth="1"
        vectorEffect="non-scaling-stroke"
      />
      <path
        d={path}
        fill="none"
        stroke={colors.green}
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="4"
        vectorEffect="non-scaling-stroke"
      />
      {points.map((point) => (
        <circle
          key={`${point.x}-${point.y}`}
          cx={point.x}
          cy={point.y}
          r="3.2"
          fill={colors.green}
        />
      ))}
    </svg>
  );
}

function SoftwareGapChart() {
  const kitaPath = "M 12 62 C 27 58, 36 43, 48 31 S 70 12, 90 8";
  const ogsPath = "M 12 60 C 29 59, 46 58, 62 58 S 78 57, 90 56";
  return (
    <div className="relative flex h-full min-h-[28rem] flex-col overflow-hidden rounded-[2rem] border border-gray-200 bg-white shadow-sm">
      <div className="absolute inset-x-8 top-8 h-24 rounded-full bg-[#5080D8]/8 blur-3xl" />
      <div className="relative flex items-start justify-between gap-6 border-b border-gray-100 px-7 pt-6 pb-4">
        <div>
          <p className="text-[clamp(0.68rem,0.86vw,0.82rem)] font-bold tracking-[0.08em] text-gray-400 uppercase">
            Entwicklung Kinderbetreuungssoftware
          </p>
          <p className="mt-1 max-w-[24rem] text-[clamp(0.84rem,1vw,0.96rem)] leading-snug font-semibold text-gray-500">
            Softwaremarkt und -nutzung
          </p>
        </div>
        <div className="grid gap-2">
          {[
            { label: "Kita-Software", color: "#5080D8", text: "#315C9B" },
            { label: "OGS-Software", color: "#F78C10", text: "#9B5609" },
          ].map((item) => (
            <div
              key={item.label}
              className="flex items-center gap-2 rounded-full border border-gray-100 bg-gray-50/80 px-3 py-1.5 text-[clamp(0.72rem,0.9vw,0.86rem)] font-semibold"
              style={{ color: item.text }}
            >
              <span
                className="h-2.5 w-2.5 rounded-full"
                style={{ backgroundColor: item.color }}
              />
              {item.label}
            </div>
          ))}
        </div>
      </div>
      <div
        className="relative min-h-0 flex-1 px-7 pt-4 pb-6"
        role="img"
        aria-label="Schematische Entwicklung von Kita-Software und OGS-Software"
      >
        <svg
          viewBox="0 0 100 72"
          className="h-full w-full overflow-visible"
          aria-hidden="true"
        >
          <text
            x="5.2"
            y="35"
            textAnchor="middle"
            transform="rotate(-90 5.2 35)"
            className="fill-gray-400 text-[2.25px] font-bold"
          >
            Software-Reife
          </text>
          {[14, 26, 38, 50, 62].map((y) => (
            <path
              key={y}
              d={`M12 ${y} H92`}
              stroke="#E5E7EB"
              strokeWidth="0.45"
              vectorEffect="non-scaling-stroke"
            />
          ))}
          <path
            d="M12 62 H92"
            stroke="#D1D5DB"
            strokeWidth="0.8"
            vectorEffect="non-scaling-stroke"
          />
          <path
            d="M12 8 V62"
            stroke="#D1D5DB"
            strokeWidth="0.8"
            vectorEffect="non-scaling-stroke"
          />
          <path
            d="M74 6.8 V62"
            stroke="#83CD2D"
            strokeDasharray="2 2"
            strokeWidth="0.9"
            vectorEffect="non-scaling-stroke"
          />
          <path
            d={kitaPath}
            fill="none"
            stroke="#5080D8"
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth="3.4"
            vectorEffect="non-scaling-stroke"
          />
          <path
            d={ogsPath}
            fill="none"
            stroke="#F78C10"
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth="3.4"
            vectorEffect="non-scaling-stroke"
          />
          {[
            { cx: 12, cy: 62, color: "#5080D8" },
            { cx: 48, cy: 31, color: "#5080D8" },
            { cx: 90, cy: 8, color: "#5080D8" },
            { cx: 12, cy: 60, color: "#F78C10" },
            { cx: 90, cy: 56, color: "#F78C10" },
          ].map((point) => (
            <circle
              key={`${point.cx}-${point.cy}-${point.color}`}
              cx={point.cx}
              cy={point.cy}
              r="1.45"
              fill={point.color}
              stroke="#FFFFFF"
              strokeWidth="1.1"
              vectorEffect="non-scaling-stroke"
            />
          ))}
          <g>
            <rect
              x="58.5"
              y="0.3"
              width="31"
              height="5.4"
              rx="2.7"
              fill="#FFFFFF"
              stroke="#83CD2D"
              strokeOpacity="0.32"
              vectorEffect="non-scaling-stroke"
            />
            <text
              x="74"
              y="3"
              textAnchor="middle"
              dominantBaseline="middle"
              className="fill-[#3F6F12] text-[1.85px] font-bold"
            >
              Ab 2026: Rechtsanspruch
            </text>
          </g>
          <g>
            <rect
              x="77"
              y="14.4"
              width="14"
              height="4.2"
              rx="2.1"
              fill="#5080D8"
              opacity="0.1"
            />
            <text
              x="84"
              y="16.5"
              textAnchor="middle"
              dominantBaseline="middle"
              className="fill-[#315C9B] text-[1.85px] font-bold"
            >
              reifer Markt
            </text>
          </g>
          <g>
            <rect
              x="70"
              y="50"
              width="17"
              height="4.2"
              rx="2.1"
              fill="#F78C10"
              opacity="0.12"
            />
            <text
              x="78.5"
              y="52.1"
              textAnchor="middle"
              dominantBaseline="middle"
              className="fill-[#9B5609] text-[1.85px] font-bold"
            >
              operative Lücke
            </text>
          </g>
          <text
            x="12"
            y="68.5"
            textAnchor="middle"
            className="fill-gray-400 text-[2.6px] font-bold"
          >
            2010
          </text>
          <text
            x="74"
            y="68.5"
            textAnchor="middle"
            className="fill-gray-400 text-[2.6px] font-bold"
          >
            2026
          </text>
          <text
            x="92"
            y="68.5"
            textAnchor="middle"
            className="fill-gray-400 text-[2.6px] font-bold"
          >
            heute
          </text>
        </svg>
      </div>
    </div>
  );
}

function GroupLandingMockup() {
  return (
    <div className="relative h-[clamp(28rem,44vw,40rem)] w-full">
      <div className="absolute inset-x-[8%] bottom-[10%] h-[24%] rounded-full bg-[#83CD2D]/13 blur-3xl" />
      <div className="absolute inset-y-[-6%] right-[-6%] left-[-8%] flex items-center">
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img
          src="/pitch/reach/landing-devices.svg"
          alt="moto Plattform auf Laptop, Tablet und Smartphone"
          className="block h-auto w-full object-contain drop-shadow-[0_26px_60px_rgba(15,23,42,0.18)]"
        />
      </div>
    </div>
  );
}

function HeroDeviceMockup() {
  return <GroupLandingMockup />;
}

export default function ReachPitchPage() {
  return (
    <main className="min-h-screen overflow-x-hidden bg-gray-50 px-4 py-6 text-gray-950 sm:px-6 lg:px-8 print:bg-white print:p-0">
      <div className="mx-auto flex w-full max-w-[1500px] flex-col gap-8 print:gap-0">
        <Slide index={1} className="bg-white">
          <div className="grid min-h-0 flex-1 grid-cols-[0.72fr_1.28fr] items-center gap-[4%]">
            <div>
              <Eyebrow>REACH Pitch Night</Eyebrow>
              <Headline>Das Betriebssystem für OGS.</Headline>
              <Lead>
                moto verbindet alles, was den offenen Ganztag am Laufen hält:
                Kinder, Eltern, Team, Planung und{" "}
                <span className="whitespace-nowrap">Check-in.</span>
              </Lead>
              <div className="mt-8 flex flex-wrap gap-3">
                <Label>Zettel raus. moto rein.</Label>
                <Label tone="blue">04.08.2026</Label>
              </div>
            </div>
            <div className="relative min-h-0" aria-label="moto Gruppenansicht">
              <HeroDeviceMockup />
            </div>
          </div>
        </Slide>

        <Slide index={2} eyebrow="Problem">
          <div className="grid min-h-0 flex-1 grid-cols-[1fr_0.95fr] items-center gap-[5%]">
            <div className="relative h-[70%] min-h-[360px] overflow-hidden rounded-3xl border border-gray-200 shadow-sm">
              <Image
                src="/pitch/reach/ogs-paper-desk.png"
                alt="Papierlisten auf einem OGS-Schreibtisch"
                fill
                sizes="50vw"
                className="object-cover"
              />
            </div>
            <Surface className="p-8">
              <Label tone="red">Chaos</Label>
              <h1 className="mt-5 text-[clamp(2rem,4.2vw,4.3rem)] leading-[1.02] font-semibold tracking-normal text-gray-950">
                Der Nachmittag startet mit Papier.
              </h1>
              <p className="mt-5 text-[clamp(1rem,1.45vw,1.35rem)] leading-relaxed text-gray-600">
                Anmeldungen, Abholzeiten, Hinweise und Listen liegen verteilt.
                Sobald sich etwas ändert, beginnt das Nachtragen.
              </p>
              <div className="mt-7 flex flex-wrap gap-2">
                <Label tone="red">Papier</Label>
                <Label tone="orange">Excel</Label>
                <Label tone="blue">Zuruf</Label>
              </div>
            </Surface>
          </div>
        </Slide>

        <Slide index={3} eyebrow="Problem">
          <div className="grid min-h-0 flex-1 grid-cols-[0.95fr_1fr] items-center gap-[5%]">
            <Surface className="p-8">
              <Label tone="orange">Massiver Zeitverlust</Label>
              <h1 className="mt-5 text-[clamp(2rem,4.2vw,4.3rem)] leading-[1.02] font-semibold tracking-normal text-gray-950">
                Wenn ein Kind gesucht wird, steht der Betrieb.
              </h1>
              <p className="mt-5 text-[clamp(1rem,1.45vw,1.35rem)] leading-relaxed text-gray-600">
                Ein Elternteil wartet. Eine Betreuungskraft läuft los. Die
                Leitung schaut in Listen. Aus einer kleinen Frage wird schnell
                ein echter Zeitfresser.
              </p>
              <div className="mt-7 flex flex-wrap gap-2">
                <Label tone="orange">Wo ist Emma?</Label>
                <Label tone="blue">Abholung</Label>
                <Label tone="red">Suchzeit</Label>
              </div>
            </Surface>
            <div className="relative h-[70%] min-h-[360px] overflow-hidden rounded-3xl border border-gray-200 shadow-sm">
              <Image
                src="/pitch/reach/ogs-pickup-hallway.png"
                alt="Abholsituation im Schulflur"
                fill
                sizes="50vw"
                className="object-cover"
              />
            </div>
          </div>
        </Slide>

        <Slide index={4} eyebrow="Problem">
          <div className="grid min-h-0 flex-1 grid-cols-[0.72fr_1.28fr] items-center gap-[5%]">
            <div>
              <Headline className="mt-5 max-w-[12ch] text-[clamp(2rem,4.8vw,4.75rem)]">
                Kita hat Software. OGS hat Druck.
              </Headline>
              <Lead className="max-w-[35rem]">
                Kita-Software ist etabliert. Der offene Ganztag wächst jetzt in
                denselben Bedarf hinein, aber mit eigenen Abläufen und deutlich
                mehr Druck.
              </Lead>
              <Surface className="mt-7 max-w-[29rem] p-4">
                <p className="text-[clamp(0.9rem,1.18vw,1.1rem)] leading-snug font-semibold text-gray-800">
                  Ab 2026 wächst nicht nur die Zahl der Plätze. Es wächst auch
                  der Anspruch an Planung, Nachweis und Verlässlichkeit.
                </p>
                <div className="mt-4 flex flex-wrap gap-1.5 [&>span]:px-2.5 [&>span]:text-[clamp(0.62rem,0.82vw,0.76rem)]">
                  <Label tone="blue">Mehr Plätze</Label>
                  <Label tone="orange">Mehr Dokumentation</Label>
                  <Label>Mehr Koordination</Label>
                </div>
              </Surface>
            </div>
            <SoftwareGapChart />
          </div>
        </Slide>

        <Slide index={5} eyebrow="Lösung">
          <div className="relative flex min-h-0 flex-1 flex-col justify-center">
            <div className="pointer-events-none absolute top-[4%] right-[3%] w-[clamp(15rem,22vw,20rem)] opacity-80">
              <Image
                src="/pitch/reach/app-device-mockups.svg"
                alt=""
                width={6727}
                height={4490}
                className="h-auto w-full"
              />
            </div>
            <Headline className="relative z-10 max-w-[17ch]">
              moto ist das Betriebssystem für den OGS-Alltag.
            </Headline>
            <Lead className="relative z-10 max-w-[52rem]">
              Ein System, das den laufenden Betreuungstag verbindet: Kinder,
              Eltern, Team, Leitung und Träger.
            </Lead>
            <Surface className="relative z-10 mt-12 p-8">
              <div>
                <div className="grid grid-cols-5 items-start">
                  {[
                    { label: "Kinder", icon: Users, tone: "green" as Tone },
                    {
                      label: "Eltern",
                      icon: MessageCircle,
                      tone: "orange" as Tone,
                    },
                    {
                      label: "Team",
                      icon: ShieldCheck,
                      tone: "purple" as Tone,
                    },
                    {
                      label: "Leitung",
                      icon: ClipboardList,
                      tone: "red" as Tone,
                    },
                    {
                      label: "Träger",
                      icon: Building2,
                      tone: "blue" as Tone,
                    },
                  ].map((node, nodeIndex, list) => (
                    <div
                      key={node.label}
                      className="grid grid-rows-[44px_auto] items-start text-center"
                    >
                      <div className="flex items-center gap-3">
                        <div
                          className={`h-px flex-1 ${nodeIndex === 0 ? "bg-transparent" : "bg-gray-200"}`}
                          aria-hidden="true"
                        />
                        <ToneIcon icon={node.icon} tone={node.tone} />
                        <div
                          className={`h-px flex-1 ${nodeIndex === list.length - 1 ? "bg-transparent" : "bg-gray-200"}`}
                          aria-hidden="true"
                        />
                      </div>
                      <p className="mt-3 text-[clamp(0.92rem,1.24vw,1.15rem)] font-semibold text-gray-900">
                        {node.label}
                      </p>
                    </div>
                  ))}
                </div>
              </div>
              <div className="mx-auto mt-9 w-fit rounded-full bg-gray-950 px-6 py-3 text-[clamp(0.9rem,1.2vw,1.12rem)] font-semibold text-white">
                Wer ist da? Wo ist das Kind? Was ändert sich jetzt?
              </div>
            </Surface>
          </div>
        </Slide>

        <Slide index={6} eyebrow="Produkt">
          <div className="flex min-h-0 flex-1 flex-col justify-center">
            <Headline className="max-w-[18ch]">
              Die App bildet Bewegung ab, nicht nur Stammdaten.
            </Headline>
            <div className="mt-9 grid grid-cols-3 gap-5">
              {productCards.map((card) => {
                const tone = toneClass[card.tone];
                return (
                  <Surface key={card.title} className={`p-4 ${tone.border}`}>
                    <div className="relative aspect-[16/10] overflow-hidden rounded-xl border border-gray-200 bg-white">
                      <Image
                        src={card.image}
                        alt={card.title}
                        fill
                        sizes="30vw"
                        className="object-contain p-3"
                      />
                    </div>
                    <h2 className="mt-4 text-[clamp(1.1rem,1.55vw,1.45rem)] font-semibold text-gray-950">
                      {card.title}
                    </h2>
                    <p className="mt-2 text-[clamp(0.78rem,1.02vw,0.98rem)] leading-relaxed text-gray-600">
                      {card.body}
                    </p>
                  </Surface>
                );
              })}
            </div>
            <p className="mx-auto mt-8 max-w-[58rem] text-center text-[clamp(1rem,1.35vw,1.25rem)] leading-relaxed font-semibold text-gray-800">
              Ein Scan oder eine Änderung aktualisiert den gemeinsamen
              Tageszustand. Das Team sieht sofort, was relevant ist.
            </p>
          </div>
        </Slide>

        <Slide index={7} eyebrow="Geschäftsmodell">
          <div className="flex min-h-0 flex-1 flex-col justify-center">
            <Headline className="max-w-[18ch]">
              Modularer SaaS-Umsatz, Hardware nach Bedarf.
            </Headline>
            <div className="mt-10 grid grid-cols-3 gap-5">
              {modules.map((module) => {
                const tone = toneClass[module.tone];
                return (
                  <Surface key={module.title} className={`p-6 ${tone.border}`}>
                    <p
                      className={`text-[clamp(1rem,1.35vw,1.25rem)] font-bold ${tone.text}`}
                    >
                      {module.title}
                    </p>
                    <p className="mt-7 text-[clamp(2rem,3.3vw,3.5rem)] leading-none font-semibold text-gray-950">
                      {module.price}
                    </p>
                    <p className="mt-3 text-[clamp(0.82rem,1.06vw,1rem)] text-gray-500">
                      {module.unit}
                    </p>
                    <p className="mt-6 text-[clamp(0.9rem,1.16vw,1.1rem)] leading-relaxed text-gray-700">
                      {module.body}
                    </p>
                  </Surface>
                );
              })}
            </div>
            <div className="mx-auto mt-9 rounded-full bg-gray-950 px-8 py-4 text-center text-[clamp(0.95rem,1.2vw,1.15rem)] font-semibold text-white">
              Beispiel: 120 Kinder plus Personalmodul, rund 377 Euro
              Monatsumsatz pro Standort
            </div>
          </div>
        </Slide>

        <Slide index={8} eyebrow="Markt">
          <div className="grid min-h-0 flex-1 grid-cols-[1.05fr_0.95fr] items-end gap-[6%]">
            <div className="pb-[2%]">
              <Headline>Betreuung wird Infrastruktur.</Headline>
              <Lead>
                Der Markt digitalisiert sich. Gleichzeitig entsteht mit dem
                Rechtsanspruch auf Ganztag ab 2026 ein operativer Druck, den
                klassische Kita- oder Schulsoftware nur teilweise abbildet.
              </Lead>
              <Surface className="mt-10 h-[24vh] min-h-[170px] p-6">
                <MarketLine />
              </Surface>
              <div className="mt-3 flex justify-between px-3 text-xs font-medium text-gray-400">
                <span>2026</span>
                <span>2028</span>
                <span>2030</span>
                <span>2032</span>
                <span>2035</span>
              </div>
            </div>
            <Surface className="grid gap-4 p-6">
              {marketStats.map((stat) => {
                const tone = toneClass[stat.tone];
                return (
                  <div
                    key={stat.value}
                    className="border-b border-gray-100 pb-4 last:border-b-0 last:pb-0"
                  >
                    <p
                      className={`text-[clamp(2rem,4vw,4.25rem)] leading-none font-semibold ${tone.text}`}
                    >
                      {stat.value}
                    </p>
                    <p className="mt-2 max-w-[17rem] text-[clamp(0.86rem,1.18vw,1.1rem)] leading-snug text-gray-600">
                      {stat.label}
                    </p>
                  </div>
                );
              })}
            </Surface>
          </div>
        </Slide>

        <Slide index={9} eyebrow="Team">
          <div className="flex min-h-0 flex-1 flex-col justify-center">
            <Headline className="max-w-[18ch]">
              Team-Slide folgt als nächster Baustein.
            </Headline>
            <Lead className="max-w-[52rem]">
              Platzhalter für Gründerteam, Rollen, Umsetzungskraft und die
              wichtigsten offenen Belege.
            </Lead>
          </div>
        </Slide>

        <Slide index={10} eyebrow="Fortschritt">
          <div className="flex min-h-0 flex-1 flex-col justify-center">
            <Headline className="max-w-[20ch]">
              In den letzten drei Monaten wurde aus Demo immer mehr Betrieb.
            </Headline>
            <div className="mt-9 grid grid-cols-2 gap-4">
              {progressItems.map((item) => (
                <Surface key={item.title} className="flex gap-4 p-5">
                  <ToneIcon icon={item.icon} tone={item.tone} />
                  <div>
                    <h2 className="text-[clamp(1rem,1.35vw,1.25rem)] font-semibold text-gray-950">
                      {item.title}
                    </h2>
                    <p className="mt-2 text-[clamp(0.78rem,1.02vw,0.98rem)] leading-relaxed text-gray-600">
                      {item.body}
                    </p>
                  </div>
                </Surface>
              ))}
            </div>
          </div>
        </Slide>

        <Slide index={11} eyebrow="Ask">
          <div className="grid min-h-0 flex-1 grid-cols-[1fr_0.35fr] items-center gap-[5%]">
            <div>
              <Headline className="max-w-[17ch]">
                Wir suchen die nächsten OGS-Realitäten.
              </Headline>
              <Lead className="max-w-[50rem]">
                Nicht perfekte Testumgebungen. Echte Nachmittage mit Kindern,
                Räumen, Abholung, Elternkommunikation und Leitung unter
                Zeitdruck.
              </Lead>
              <div className="mt-9 grid grid-cols-3 gap-4">
                {askItems.map((item) => (
                  <Surface key={item.title} className="p-5">
                    <ToneIcon icon={item.icon} tone={item.tone} />
                    <h2 className="mt-4 text-[clamp(1rem,1.35vw,1.25rem)] font-semibold text-gray-950">
                      {item.title}
                    </h2>
                    <p className="mt-2 text-[clamp(0.76rem,1vw,0.95rem)] leading-relaxed text-gray-600">
                      {item.body}
                    </p>
                  </Surface>
                ))}
              </div>
              <p className="mt-9 text-[clamp(1rem,1.35vw,1.25rem)] font-semibold text-gray-950">
                moto.nrw
              </p>
            </div>
            <div className="flex justify-center">
              <div className="relative aspect-square w-full max-w-[230px] rounded-full bg-[#83CD2D]/16 p-8">
                <Image
                  src="/pitch/reach/moto-pin.png"
                  alt="moto Bildmarke"
                  width={300}
                  height={300}
                  className="h-full w-full object-contain"
                />
              </div>
            </div>
          </div>
        </Slide>
      </div>
    </main>
  );
}
