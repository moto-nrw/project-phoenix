import Image from "next/image";
import type { ReactNode } from "react";
import { Info } from "lucide-react";

export const metadata = {
  title: "moto Tablet-Montage und Brandschutz",
  description:
    "DIN-A4 Infoblatt zur Montage von moto NFC-Tablets in Schulen und öffentlichen Gebäuden.",
};

const principles = [
  {
    title: "Rettungswege frei halten",
    body: "Tablet, Halterung, Standfuß und Kabel dürfen Flure, Türen und Durchgänge nicht verengen. Die NRW-Schulbaurichtlinie nennt für notwendige Flure mindestens 1,50 m nutzbare Breite. Auch Feuerlöscher, Melder, Fluchtwegschilder, Notausgänge und Brandschutzpläne müssen sichtbar und erreichbar bleiben.",
  },
  {
    title: "Türen nicht verändern",
    body: "Brand- und Rauchschutztüren, Zargen, Beschläge, Panikfunktion, Türschließer und Türantriebe dürfen nicht beeinträchtigt werden. Eine Montage an solchen Elementen braucht eine ausdrückliche Freigabe.",
  },
  {
    title: "Strom sicher führen",
    body: "Kabel dürfen nicht lose im Laufweg liegen. Sinnvoll sind feste Kabelwege, zum Beispiel ein Kabelkanal an der Wand oder eine Kabelbrücke für bewegliche Leitungen. In Fluren und Rettungswegen sollte die Kabelführung mit der zuständigen Stelle abgestimmt werden, weil dort auch Anforderungen an Leitungsanlagen relevant sein können.",
  },
  {
    title: "Standort freigeben lassen",
    body: "Der konkrete Standort sollte durch Betreiber, Schulträger, Gebäudemanagement oder eine brandschutzkundige Person geprüft und freigegeben werden.",
  },
];

const checklistItems = [
  "Der Standort liegt nicht auf einer Brand- oder Rauchschutztür, am Türrahmen oder an Teilen der Türanlage, außer es gibt eine ausdrückliche Freigabe.",
  "Die Tür öffnet und schließt vollständig.",
  "Türdrücker, Panikbeschlag, Türschließer, Türantrieb und Bewegungsfläche bleiben frei.",
  "Flure, Türen und Durchgänge bleiben frei nutzbar.",
  "Standfuß oder Tischgerät stehen nur nach Absprache im Flur und blockieren keinen Laufweg.",
  "Feuerlöscher, Wandhydranten, Melder, Brandschutzpläne und Fluchtwegschilder bleiben sichtbar und erreichbar.",
  "Kabel liegen nicht lose im Laufweg. Kabelkanal, Kabelbrücke oder feste Leitungsführung sind mit der zuständigen Stelle abgestimmt.",
  "Wand, Schrauben, Dübel und Halterung passen zusammen. Bohrungen sind mit der zuständigen Stelle abgestimmt.",
  "Gerät, Netzteil, Kabel und Halterung werden regelmäßig auf sichtbare Schäden geprüft.",
];

const mountingTypes = [
  {
    title: "Wandmontage",
    body: "Geeignet, wenn neben dem Raumzugang eine freie Wandfläche vorhanden ist und der Kabelweg sicher geführt werden kann. Nicht auf Brand- oder Rauchschutztüren, Türrahmen oder Teilen der Türanlage montieren, außer nach ausdrücklicher Freigabe.",
  },
  {
    title: "Standfuß",
    body: "Sinnvoll, wenn keine Wandmontage möglich oder gewünscht ist. Der Standfuß muss kippsicher stehen. Eine dauerhafte Aufstellung im Flur, Türbereich oder Laufweg sollte vorher abgestimmt werden.",
  },
  {
    title: "Tischaufsteller",
    body: "Geeignet für Gruppenraum, Sekretariat oder feste Übergabepunkte. Das Kabel muss geschützt geführt werden und darf keine Stolperstelle bilden.",
  },
];

const sourceItems = [
  "NRW-Schulbaurichtlinie, Nr. 3.1, 3.4 und 11",
  "Sichere Schule, Flure: Flucht- und Rettungswege, Einrichtungen, Brandschutz",
  "VV TB NRW / Leitungsanlagenrichtlinie, A 2.2.1.8 und MLAR Abschnitt 3",
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

function InfoBox({ children }: { readonly children: ReactNode }) {
  return (
    <aside className="mt-auto flex gap-3 rounded-2xl border border-[#83CD2D]/20 bg-[#83CD2D]/8 p-4 text-[clamp(0.7rem,0.9vw,0.82rem)] leading-relaxed text-gray-700">
      <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-white text-[#3F6F12] shadow-sm">
        <Info className="h-4 w-4" aria-hidden="true" />
      </span>
      <div>
        <h2 className="text-[clamp(0.8rem,1vw,0.92rem)] font-semibold text-gray-950">
          Wichtige Einordnung
        </h2>
        <p className="mt-1">{children}</p>
      </div>
    </aside>
  );
}

function SourcesBox() {
  return (
    <aside className="mt-auto rounded-2xl border border-gray-200 bg-gray-50/80 p-4 text-[clamp(0.64rem,0.82vw,0.76rem)] leading-relaxed text-gray-600">
      <h2 className="text-[clamp(0.76rem,0.94vw,0.86rem)] font-semibold text-gray-950">
        Quellen zur Orientierung
      </h2>
      <ul className="mt-1.5 grid gap-0.5">
        {sourceItems.map((item) => (
          <li key={item}>{item}</li>
        ))}
      </ul>
    </aside>
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

          <InfoBox>
            Dieses Infoblatt ersetzt keine brandschutzrechtliche Bewertung des
            konkreten Gebäudes. Maßgeblich bleiben Betreiberpflichten, lokales
            Brandschutzkonzept, Landesrecht und Vorgaben des Schulträgers.
          </InfoBox>
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
              Hausmeister, Brandschutzbeauftragte, Fachplanung oder
              Brandschutzdienststelle sinnvoll sein.
            </p>
          </Section>

          <SourcesBox />
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
