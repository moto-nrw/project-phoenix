import Image from "next/image";
import type { ReactNode } from "react";
import {
  Building2,
  ClipboardList,
  MapPinHouse,
  MessageCircle,
  ShieldCheck,
  Users,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";

export const metadata = {
  title: "moto REACH Pitch Night",
  description:
    "Pitch-Präsentation für die REACH Pitch Night 2026 im moto Design.",
};

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

const marketModelCards = [
  {
    segment: "TAM",
    value: "1,2 Mrd. Euro",
    title: "Child-Care-Software global",
    body: "Wachsende Softwarekategorie für Betreuung, Verwaltung und Elternkommunikation.",
    tone: "blue" as Tone,
  },
  {
    segment: "SAM",
    value: "103 Mio. Euro",
    title: "Deutschland",
    body: "Adressierbarer Betreuungssoftware-Markt aus Kita und Ganztag.",
    tone: "green" as Tone,
  },
  {
    segment: "SOM",
    value: "13,9 Mio. Euro",
    title: "1.000 Standorte",
    body: "Erreichbarer Einstieg über größere OGS-Standorte und Träger.",
    tone: "orange" as Tone,
  },
];

const nfcInfoCards = [
  {
    kicker: "01",
    title: "Freie Bewegung",
    body: "Kinder wechseln selbstbestimmt zwischen Schulhof, Mensa, AGs und Räumen.",
    tone: "green" as Tone,
  },
  {
    kicker: "02",
    title: "Einfacher Check-in",
    body: "NFC macht diese Entscheidungen sichtbar, ohne den Nachmittag zu unterbrechen.",
    tone: "blue" as Tone,
  },
  {
    kicker: "03",
    title: "Mehr Zeit im Team",
    body: "Betreuer müssen weniger suchen und nachtragen, und können mehr begleiten.",
    tone: "orange" as Tone,
  },
];

const modules = [
  {
    title: "OGS",
    price: "0,99 Euro",
    unit: "pro Kind und Monat",
    body: "Anwesenheit, Angebote, Kinderprofile",
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
    body: "Personalverwaltung, Dienstplan, Zeiterfassung",
    tone: "blue" as Tone,
  },
];

const progressItems = [
  {
    value: "8",
    title: "Live-Standorte",
    body: "moto wird täglich im OGS-Betrieb genutzt.",
    tone: "green" as Tone,
  },
  {
    value: "2.000+",
    title: "Kinder und Betreuer",
    body: "arbeiten jeden Tag im Alltag mit moto.",
    tone: "blue" as Tone,
  },
  {
    value: "4",
    title: "Apps",
    body: "moto verbindet Eltern, Betreuer, Leitung und Träger.",
    tone: "orange" as Tone,
  },
  {
    value: "60.000+",
    title: "NFC-Check-ins",
    body: "automatisch erfasst, ohne Listen, Nachfragen oder Nachtragen.",
    tone: "red" as Tone,
  },
];

const teamMembers = [
  {
    initials: "FL",
    imageSrc: "/pitch/reach/team/florian-luettgenau-v7.png",
    name: "Florian Lüttgenau",
    role: "CEO",
    tone: "green" as Tone,
  },
  {
    initials: "YW",
    imageSrc: "/pitch/reach/team/yannick-wenger-v4.png",
    name: "Yannick Wenger",
    role: "CTO",
    tone: "blue" as Tone,
  },
  {
    initials: "CR",
    imageSrc: "/pitch/reach/team/conrad-reintjes.webp",
    name: "Conrad Reintjes",
    role: "CPO",
    tone: "red" as Tone,
  },
  {
    initials: "CK",
    imageSrc: "/pitch/reach/team/christian-kamann.webp",
    name: "Christian Kamann",
    role: "Founding Engineer",
    tone: "orange" as Tone,
  },
  {
    initials: "TH",
    imageSrc: "/pitch/reach/team/tristan-heitger-v3.png",
    name: "Tristan Heitger",
    role: "Founding Engineer",
    tone: "purple" as Tone,
  },
  {
    initials: "JS",
    imageSrc: "/pitch/reach/team/jule-soll-v2.webp",
    name: "Jule Soll",
    role: "Pädagogische Leitung",
    tone: "green" as Tone,
  },
];

const askItems = [
  {
    marker: "Zugang",
    title: "Trägerzugang",
    body: "Kontakte zu OGS-Trägern und Kommunen",
    tone: "green" as Tone,
  },
  {
    marker: "Standorte",
    title: "OGS-Standorte",
    body: "Einrichtungen, die offene Ganztagsbetreuung digital betreiben wollen",
    tone: "blue" as Tone,
  },
  {
    marker: "Partner",
    title: "Partner",
    body: "Menschen mit Ideen, Kontakten oder Ressourcen, die moto unterstützen wollen",
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

function TextMarker({
  children,
  tone = "green",
}: {
  readonly children: ReactNode;
  readonly tone?: Tone;
}) {
  return (
    <p
      className={`text-[clamp(0.72rem,0.94vw,0.86rem)] font-bold tracking-[0.12em] uppercase ${toneClass[tone].text}`}
    >
      {children}
    </p>
  );
}

function MarketLine() {
  const points = [
    { x: 42, y: 70 },
    { x: 104, y: 62 },
    { x: 166, y: 51 },
    { x: 226, y: 35 },
    { x: 286, y: 14 },
  ];
  const path = points
    .map((point, index) => `${index === 0 ? "M" : "L"} ${point.x} ${point.y}`)
    .join(" ");

  return (
    <svg
      viewBox="0 0 300 86"
      className="h-full w-full overflow-visible"
      role="img"
      aria-label="Globaler Child-Care-Software-Markt von 2026 bis 2035"
    >
      {[14, 32, 50, 68].map((y) => (
        <path
          key={y}
          d={`M28 ${y} H290`}
          stroke="#E5E7EB"
          strokeWidth="1"
          vectorEffect="non-scaling-stroke"
        />
      ))}
      <path
        d="M28 10 V72 H290"
        fill="none"
        stroke="#CBD5E1"
        strokeWidth="1.2"
        vectorEffect="non-scaling-stroke"
      />
      <path
        d={path}
        fill="none"
        stroke="#83CD2D"
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
          fill="#83CD2D"
        />
      ))}
      <g className="fill-gray-400 text-[4.7px] font-semibold">
        <text x="2" y="16">
          1,2 Mrd. Euro
        </text>
        <text x="2" y="70">
          0,5 Mrd. Euro
        </text>
        <text x="42" y="83">
          2026
        </text>
        <text x="268" y="83">
          2035
        </text>
      </g>
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
              <TextMarker tone="red">Chaos</TextMarker>
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
              <TextMarker tone="orange">Massiver Zeitverlust</TextMarker>
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
                <div className="grid grid-cols-3 gap-3">
                  {[
                    {
                      title: "Mehr Kinder",
                      body: "Mehr Anmeldungen ab 2026",
                      tone: "blue" as Tone,
                    },
                    {
                      title: "Mehr Nachweis",
                      body: "mehr Dokumentation und Verlässlichkeit",
                      tone: "orange" as Tone,
                    },
                    {
                      title: "Mehr Koordination",
                      body: "mehr Abstimmung im laufenden Betrieb",
                      tone: "green" as Tone,
                    },
                  ].map((item) => (
                    <div key={item.title}>
                      <p
                        className={`text-[clamp(0.72rem,0.9vw,0.82rem)] leading-tight font-bold ${toneClass[item.tone].text}`}
                      >
                        {item.title}
                      </p>
                      <p className="mt-1.5 text-[clamp(0.62rem,0.78vw,0.72rem)] leading-snug text-gray-500">
                        {item.body}
                      </p>
                    </div>
                  ))}
                </div>
              </Surface>
            </div>
            <SoftwareGapChart />
          </div>
        </Slide>

        <Slide index={5} eyebrow="Lösung">
          <div className="relative flex min-h-0 flex-1 flex-col justify-center">
            <div className="pointer-events-none absolute top-[4%] right-[2.5%] w-[clamp(18rem,27vw,24rem)] opacity-80">
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

        <Slide index={6} eyebrow="Lösung">
          <div className="flex min-h-0 flex-1 flex-col justify-center">
            <Headline className="max-w-[18ch]">
              Das O in OGS steht für offen.
            </Headline>
            <div className="mt-9 grid grid-cols-[0.78fr_1.22fr] gap-7">
              <Surface className={`p-4 ${toneClass.green.border}`}>
                <div className="relative aspect-[16/8.6] overflow-hidden rounded-xl border border-gray-200 bg-white">
                  <div className="absolute top-4 right-4 z-10 rounded-full border border-[#83CD2D]/25 bg-[#83CD2D]/12 px-3 py-1 text-xs font-bold text-[#3F6F12] uppercase shadow-sm">
                    NFC
                  </div>
                  <Image
                    src="/help/screens/nfc-tablet-willkommen.webp"
                    alt="NFC-Tablet mit moto Anmeldebildschirm"
                    width={1024}
                    height={1024}
                    sizes="28vw"
                    className="absolute top-[8%] left-[5%] w-[58%] object-contain"
                  />
                  <Image
                    src="/pitch/reach/nfc-wristband.svg"
                    alt="NFC-Armband für den Check-in"
                    width={636}
                    height={474}
                    sizes="16vw"
                    className="absolute right-[7%] bottom-[22%] w-[26%] object-contain drop-shadow-[0_12px_20px_rgba(15,23,42,0.16)]"
                  />
                </div>
                <h2 className="mt-3 text-[clamp(1rem,1.38vw,1.28rem)] font-semibold text-gray-950">
                  Check-in Terminal
                </h2>
                <p className="mt-1.5 text-[clamp(0.72rem,0.94vw,0.88rem)] leading-relaxed text-gray-600">
                  Ein kurzer Scan ersetzt Nachfragen, Abhaken und spätere
                  Korrekturen.
                </p>
              </Surface>
              <div className="flex flex-col justify-center">
                <p className="max-w-[43rem] text-[clamp(1rem,1.36vw,1.28rem)] leading-relaxed font-semibold text-gray-700">
                  Wenn 400 Kinder frei zwischen Schulhof, Mensa, AGs und Räumen
                  wechseln, reichen Listen nicht mehr aus. Mit NFC macht moto
                  offene Konzepte auch bei steigenden Kinderzahlen steuerbar.
                </p>
                <div className="mt-6 grid grid-cols-3 gap-4">
                  {nfcInfoCards.map((card) => {
                    const tone = toneClass[card.tone];
                    return (
                      <Surface
                        key={card.title}
                        className={`p-4 ${tone.border}`}
                      >
                        <p className={`text-sm font-bold ${tone.text}`}>
                          {card.kicker}
                        </p>
                        <h2 className="mt-5 text-[clamp(0.96rem,1.22vw,1.12rem)] leading-tight font-semibold text-gray-950">
                          {card.title}
                        </h2>
                        <p className="mt-2 text-[clamp(0.72rem,0.94vw,0.88rem)] leading-relaxed text-gray-600">
                          {card.body}
                        </p>
                      </Surface>
                    );
                  })}
                </div>
              </div>
            </div>
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
              Beispiel: 150 Kinder plus Personalmodul, rund 421 Euro
              Monatsumsatz pro Standort
            </div>
          </div>
        </Slide>

        <Slide index={8} eyebrow="Markt">
          <div className="grid min-h-0 flex-1 grid-cols-[1.05fr_0.95fr] items-end gap-[6%]">
            <div className="pb-[2%]">
              <Headline>Markt</Headline>
              <Lead>
                Im Ganztag entsteht ein neuer Softwaremarkt. Global wächst die
                Child-Care-Software-Kategorie, in Deutschland schafft der
                Rechtsanspruch einen konkreten operativen Bedarf.
              </Lead>
              <Surface className="mt-10 h-[24vh] min-h-[170px] p-5">
                <MarketLine />
              </Surface>
              <p className="mt-3 px-3 text-[clamp(0.68rem,0.86vw,0.8rem)] font-semibold text-gray-400">
                Globaler Child-Care-Software-Markt, Prognose bis 2035.
              </p>
            </div>
            <Surface className="grid gap-2 p-5">
              {marketModelCards.map((card) => {
                const tone = toneClass[card.tone];
                return (
                  <div
                    key={card.segment}
                    className="border-b border-gray-100 py-2.5 first:pt-0 last:border-b-0 last:pb-0"
                  >
                    <p
                      className={`text-[clamp(0.72rem,0.9vw,0.84rem)] font-bold tracking-[0.08em] uppercase ${tone.text}`}
                    >
                      {card.segment}
                    </p>
                    <p
                      className={`mt-2 text-[clamp(1.45rem,2.7vw,2.85rem)] leading-none font-semibold ${tone.text}`}
                    >
                      {card.value}
                    </p>
                    <h2 className="mt-1.5 text-[clamp(0.88rem,1.08vw,1rem)] leading-tight font-semibold text-gray-950">
                      {card.title}
                    </h2>
                    <p className="mt-1.5 text-[clamp(0.7rem,0.86vw,0.82rem)] leading-snug text-gray-600">
                      {card.body}
                    </p>
                  </div>
                );
              })}
            </Surface>
          </div>
        </Slide>

        <Slide index={9} eyebrow="Meilensteine">
          <div className="flex min-h-0 flex-1 flex-col justify-center">
            <Headline className="max-w-[16ch]">
              moto läuft bereits im OGS-Alltag.
            </Headline>
            <Lead className="max-w-[56rem]">
              moto läuft an acht Standorten und gibt Teams jeden Tag wertvolle
              Zeit zurück.
            </Lead>
            <div className="mt-9 grid grid-cols-2 gap-4">
              {progressItems.map((item) => (
                <Surface key={item.title} className="p-5">
                  <p
                    className={`text-[clamp(2rem,3.45vw,3.55rem)] leading-none font-semibold ${toneClass[item.tone].text}`}
                  >
                    {item.value}
                  </p>
                  <h2 className="mt-3 text-[clamp(1rem,1.35vw,1.25rem)] font-semibold text-gray-950">
                    {item.title}
                  </h2>
                  <p className="mt-2 text-[clamp(0.76rem,0.98vw,0.92rem)] leading-relaxed text-gray-600">
                    {item.body}
                  </p>
                </Surface>
              ))}
            </div>
          </div>
        </Slide>

        <Slide index={10} eyebrow="Team">
          <div className="flex min-h-0 flex-1 flex-col justify-center">
            <Headline className="max-w-[11ch]">Das Team hinter moto.</Headline>
            <Lead className="max-w-[42rem]">
              Produkt, Technik und OGS-Praxis in einem Team, nah am Betrieb und
              schnell in der Umsetzung.
            </Lead>
            <Surface className="mt-14 p-7">
              <div className="grid grid-cols-6 gap-5">
                {teamMembers.map((member) => {
                  const tone = toneClass[member.tone];
                  return (
                    <div key={member.name} className="min-w-0 text-center">
                      <div
                        className={`mx-auto flex aspect-square w-full max-w-[7.6rem] items-center justify-center overflow-hidden rounded-full border ${tone.border} ${tone.bg} ${tone.text} text-[clamp(1.1rem,1.8vw,1.65rem)] font-semibold shadow-[inset_0_1px_0_rgba(255,255,255,0.85)]`}
                      >
                        {member.imageSrc ? (
                          <Image
                            src={member.imageSrc}
                            alt={member.name}
                            width={480}
                            height={480}
                            className="h-full w-full object-cover"
                          />
                        ) : (
                          member.initials
                        )}
                      </div>
                      <h2 className="mt-4 text-[clamp(0.82rem,1vw,0.95rem)] leading-tight font-semibold text-gray-950">
                        {member.name}
                      </h2>
                      <p
                        className={`mt-1 text-[clamp(0.58rem,0.74vw,0.68rem)] font-bold tracking-[0.08em] uppercase ${tone.text}`}
                      >
                        {member.role}
                      </p>
                    </div>
                  );
                })}
              </div>
            </Surface>
          </div>
        </Slide>

        <Slide index={11} eyebrow="Ausblick">
          <div className="grid min-h-0 flex-1 grid-cols-[1fr_0.35fr] items-center gap-[5%]">
            <div>
              <Headline className="max-w-[17ch]">
                Der Anfang ist gemacht. Jetzt denken wir größer.
              </Headline>
              <Lead className="max-w-[50rem]">
                moto läuft bereits im OGS-Alltag. Jetzt suchen wir Menschen, die
                uns helfen, aus acht Standorten den nächsten Rollout zu machen.
              </Lead>
              <div className="mt-9 grid grid-cols-3 gap-4">
                {askItems.map((item) => {
                  const tone = toneClass[item.tone];
                  return (
                    <Surface key={item.title} className="p-5">
                      <p
                        className={`text-[clamp(0.7rem,0.9vw,0.82rem)] font-bold tracking-[0.08em] uppercase ${tone.text}`}
                      >
                        {item.marker}
                      </p>
                      <h2 className="mt-4 text-[clamp(1rem,1.35vw,1.25rem)] font-semibold text-gray-950">
                        {item.title}
                      </h2>
                      <p className="mt-2 text-[clamp(0.76rem,1vw,0.95rem)] leading-relaxed text-gray-600">
                        {item.body}
                      </p>
                    </Surface>
                  );
                })}
              </div>
              <p className="mt-9 text-[clamp(1rem,1.35vw,1.25rem)] font-semibold text-gray-950">
                moto-ogs.de
              </p>
            </div>
            <div className="flex -translate-x-8 -translate-y-10 justify-center">
              <div className="flex aspect-square w-full max-w-[270px] items-center justify-center rounded-[42%] bg-[#83CD2D]/10 text-[#3F6F12] shadow-[inset_0_0_0_1px_rgba(131,205,45,0.12)]">
                <MapPinHouse
                  className="h-[12.5rem] w-[12.5rem] drop-shadow-[0_18px_24px_rgba(63,111,18,0.16)]"
                  strokeWidth={1.25}
                />
              </div>
            </div>
          </div>
        </Slide>
      </div>
    </main>
  );
}
