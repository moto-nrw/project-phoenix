import Image from "next/image";
import type { ReactNode } from "react";

export const metadata = {
  title: "moto Tablet-Montage und Brandschutz",
  description:
    "DIN-A4 Infoblatt zur Montage von moto NFC-Tablets in Schulen und öffentlichen Gebäuden.",
};

const principles = [
  {
    title: "Rettungswege frei halten",
    body: "Tablet, Halterung, Standfuß und Kabel dürfen Flure, Türen und Durchgänge nicht verengen. Auch Sicherheitseinrichtungen müssen sichtbar und erreichbar bleiben.",
  },
  {
    title: "Türen nicht verändern",
    body: "Brand- und Rauchschutztüren, Zargen, Beschläge, Panikfunktion, Türschließer und Türantriebe dürfen nicht beeinträchtigt werden. Eine Montage an solchen Elementen braucht eine ausdrückliche Freigabe.",
  },
  {
    title: "Strom sicher führen",
    body: "Kabel dürfen nicht lose im Laufweg liegen. Sie sollten gegen Zug, Quetschung, scharfe Kanten und mechanische Belastung geschützt werden.",
  },
  {
    title: "Standort freigeben lassen",
    body: "Der konkrete Standort sollte durch Betreiber, Schulträger, Gebäudemanagement oder eine brandschutzkundige Person geprüft und freigegeben werden.",
  },
];

const checklistItems = [
  "Der Standort liegt nicht auf Türblatt, Zarge oder Brandschutzelement, außer es gibt eine ausdrückliche Freigabe.",
  "Die Tür öffnet und schließt vollständig.",
  "Türdrücker, Panikbeschlag, Türschließer, Türantrieb und Bewegungsfläche bleiben frei.",
  "Flure, Türen und Durchgänge bleiben frei nutzbar.",
  "Standfuß oder Tischgerät stehen nicht im Verkehrsweg.",
  "Feuerlöscher, Wandhydranten, Melder, Brandschutzpläne und Fluchtwegschilder bleiben sichtbar und erreichbar.",
  "Kabel liegen nicht lose im Laufweg und sind gegen Zug, Quetschung und mechanische Belastung geschützt.",
  "Die Befestigung passt zum Untergrund und wurde mit der zuständigen Stelle abgestimmt.",
  "Gerät, Netzteil, Kabel und Halterung werden im Betrieb regelmäßig sichtgeprüft.",
];

const mountingTypes = [
  {
    title: "Wandmontage",
    body: "Geeignet, wenn neben dem Raumzugang eine freie Wandfläche vorhanden ist und der Kabelweg sicher geführt werden kann. Nicht auf Türblatt, Zarge oder Brandschutzelementen montieren, außer nach ausdrücklicher Freigabe.",
  },
  {
    title: "Standfuß",
    body: "Sinnvoll, wenn keine Wandmontage möglich oder gewünscht ist. Der Standfuß muss kippsicher stehen und darf nicht im Flur, Türbereich oder in Bewegungsflächen platziert werden.",
  },
  {
    title: "Tischaufsteller",
    body: "Geeignet für Empfang, Sekretariat oder feste Übergabepunkte. Das Kabel muss geschützt geführt werden und darf keine Stolperstelle bilden.",
  },
];

function DocumentPage({
  children,
  label,
}: {
  readonly children: ReactNode;
  readonly label: string;
}) {
  return (
    <section className="info-sheet-page moto-dotted-background moto-dotted-background--fullscreen relative mx-auto grid aspect-[210/297] w-full max-w-[920px] overflow-hidden rounded-[26px] border border-gray-200 bg-gray-50 shadow-sm print:rounded-none print:border-0 print:shadow-none">
      <div className="relative z-10 flex h-full flex-col p-[7.2%]">
        <header className="flex items-center justify-between gap-5">
          <Image
            src="/images/moto-logo-mit-schriftzug.webp"
            alt="moto"
            width={150}
            height={48}
            className="h-8 w-auto object-contain"
          />
          <p className="rounded-full border border-[#83CD2D]/25 bg-white/90 px-3 py-1 text-[clamp(0.62rem,0.9vw,0.78rem)] font-bold tracking-[0.08em] text-[#3F6F12] uppercase shadow-sm backdrop-blur">
            {label}
          </p>
        </header>
        {children}
      </div>
    </section>
  );
}

function TitleBlock({
  eyebrow,
  title,
  lead,
  titleClassName = "",
}: {
  readonly eyebrow: string;
  readonly title: string;
  readonly lead?: string;
  readonly titleClassName?: string;
}) {
  return (
    <div className="mt-6">
      <p className="inline-flex w-fit rounded-full border border-[#83CD2D]/25 bg-[#83CD2D]/12 px-3 py-1 text-[clamp(0.66rem,0.9vw,0.82rem)] font-semibold text-[#3F6F12]">
        {eyebrow}
      </p>
      <h1
        className={`mt-4 max-w-[13ch] text-[clamp(2.05rem,4.7vw,3.72rem)] leading-[0.98] font-semibold tracking-normal text-gray-950 ${titleClassName}`}
      >
        {title}
      </h1>
      {lead ? (
        <p className="mt-3 max-w-[39rem] text-[clamp(0.9rem,1.38vw,1.06rem)] leading-relaxed text-gray-600">
          {lead}
        </p>
      ) : null}
    </div>
  );
}

function ContentArea({
  children,
  className = "",
}: {
  readonly children: ReactNode;
  readonly className?: string;
}) {
  return (
    <div
      className={`mt-5 min-h-0 flex-1 rounded-[20px] border border-gray-200 bg-white/92 p-6 shadow-sm backdrop-blur-sm ${className}`}
    >
      {children}
    </div>
  );
}

function Section({
  title,
  children,
  className = "",
}: {
  readonly title: string;
  readonly children: ReactNode;
  readonly className?: string;
}) {
  return (
    <section className={className}>
      <h2 className="text-[clamp(0.86rem,1.08vw,0.98rem)] font-semibold text-gray-950">
        {title}
      </h2>
      <div className="mt-1.5 text-[clamp(0.7rem,0.9vw,0.82rem)] leading-relaxed text-gray-600">
        {children}
      </div>
    </section>
  );
}

function TextList({
  items,
  className = "",
}: {
  readonly items: readonly string[];
  readonly className?: string;
}) {
  return (
    <ul className={`grid gap-2 ${className}`}>
      {items.map((item) => (
        <li key={item} className="flex gap-3">
          <span className="mt-[0.62em] h-1.5 w-1.5 shrink-0 rounded-full bg-[#83CD2D]" />
          <span>{item}</span>
        </li>
      ))}
    </ul>
  );
}

function PageOne() {
  return (
    <DocumentPage label="NFC Tablet Montage">
      <TitleBlock
        eyebrow="Orientierung für Schulen und Träger"
        title="Tablet-Montage und Brandschutz."
        lead="moto NFC-Tablets können an der Wand, mit Standfuß oder als Tischgerät eingesetzt werden. Sie enthalten keinen Akku und werden über Netzteil und Kabel betrieben."
      />

      <ContentArea>
        <div className="grid h-full min-h-0 content-start gap-3.5">
          <Section title="Worum es geht">
            <p>
              In Schulen und öffentlichen Gebäuden sollte der konkrete Standort
              vor einer dauerhaften Montage oder Aufstellung intern abgestimmt
              werden. Das gilt besonders in der Nähe von Türen, Fluren,
              Fluchtwegen, Brandschutzeinrichtungen und sicherheitsrelevanter
              Beschilderung.
            </p>
          </Section>

          {principles.map((principle) => (
            <Section key={principle.title} title={principle.title}>
              <p>{principle.body}</p>
            </Section>
          ))}

          <Section title="Empfohlener Ablauf">
            <ol className="grid gap-0.5">
              <li>1. Montagepunkt vorschlagen</li>
              <li>2. Standort intern prüfen</li>
              <li>3. Freigabe festhalten</li>
            </ol>
          </Section>

          <Section title="Wichtige Einordnung">
            <p>
              Dieses Infoblatt ersetzt keine brandschutzrechtliche Bewertung des
              konkreten Gebäudes. Maßgeblich bleiben Betreiberpflichten, lokales
              Brandschutzkonzept, Landesrecht und Vorgaben des Schulträgers.
            </p>
          </Section>
        </div>
      </ContentArea>
    </DocumentPage>
  );
}

function PageTwo() {
  return (
    <DocumentPage label="Prüfpunkte vor Ort">
      <TitleBlock
        eyebrow="Gesprächsgrundlage für den Vor-Ort-Termin"
        title="Was vor der Montage geklärt sein sollte."
        titleClassName="max-w-[17ch] text-[clamp(1.65rem,3.7vw,2.9rem)]"
      />

      <ContentArea>
        <div className="grid h-full min-h-0 content-start gap-3">
          <Section title="Checkliste">
            <TextList items={checklistItems} className="gap-1" />
          </Section>

          {mountingTypes.map((type) => (
            <Section key={type.title} title={type.title}>
              <p>{type.body}</p>
            </Section>
          ))}

          <Section title="Wer sollte beteiligt werden?">
            <p>
              Je nach Gebäude können Schulträger, Gebäudemanagement,
              Hausmeisterei, Brandschutzbeauftragte, Fachplanung oder
              Brandschutzdienststelle sinnvoll sein.
            </p>
          </Section>

          <Section title="Bei Unsicherheit">
            <p className="font-semibold text-gray-800">
              Keine Montage auf Türen, Zargen oder Brandschutzelementen ohne
              ausdrückliche Freigabe.
            </p>
          </Section>
        </div>
      </ContentArea>
    </DocumentPage>
  );
}

export default function NfcTabletMountingFireSafetyPage() {
  return (
    <main className="min-h-screen overflow-x-hidden bg-gray-50 px-4 py-6 text-gray-950 sm:px-6 lg:px-8 print:bg-white print:p-0">
      <div className="mx-auto grid w-full max-w-[1000px] gap-8 print:max-w-none print:gap-0">
        <PageOne />
        <PageTwo />
      </div>
    </main>
  );
}
