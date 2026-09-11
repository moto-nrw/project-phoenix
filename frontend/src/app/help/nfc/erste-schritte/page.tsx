import Image from "next/image";
import Link from "next/link";
import {
  ArrowRight,
  BadgeCheck,
  BookOpen,
  KeyRound,
  Nfc,
  PlugZap,
  ShieldCheck,
  Wrench,
} from "lucide-react";
import { HelpHeader } from "~/components/help/guide-components";
import { HelpSearchInline } from "~/components/help/help-search";

const quickstartPdf = {
  href: "/help/pdfs/nfc-erste-schritte.pdf",
  download: "moto Anleitung - NFC Erste Schritte.pdf",
};

const steps = [
  {
    id: "tablet-montieren-und-einschalten",
    number: "1",
    title: "Tablet montieren und einschalten",
    icon: PlugZap,
    text: "Vor der Montage einen Platz mit Strom und guter WLAN-Reichweite wählen. Optional mit Wand- oder Standhalterung montieren und die Angaben des Herstellers beachten. Danach Strom einstecken.",
    detail:
      "Das Gerät startet automatisch und verbindet sich mit dem hinterlegten WLAN.",
  },
  {
    id: "mit-geraete-pin-anmelden",
    number: "2",
    title: "Mit Geräte-PIN anmelden",
    icon: KeyRound,
    text: "Auf dem Startbildschirm auf Anmelden tippen und die 4-stellige PIN eingeben. Die Standard-PIN bei Auslieferung ist 1234. Nach der vierten Ziffer prüft das Tablet automatisch.",
    detail:
      "Wir empfehlen, die Standard-PIN nach der ersten Anmeldung unter Einstellungen, Geräte, OGS Geräte-PIN zu ändern.",
  },
  {
    id: "armbaender-zuweisen",
    number: "3",
    title: "Armbänder zuweisen",
    icon: Nfc,
    text: "Auf dem Startbildschirm Armband identifizieren tippen, dann Scan starten. Armband ruhig an das Lesegerät halten, Person auswählen tippen, Kind auswählen und mit Armband zuweisen bestätigen.",
    detail:
      "Falls das Armband schon vergeben ist, heißt der Button Anderer Person zuweisen. Die Armbänder müssen angekommen sein und die Kinder müssen in moto angelegt sein.",
  },
] as const;

const hints = [
  "Armband flach und mittig auf den NFC-Sensor legen.",
  "Bei rotem WLAN-Symbol die Internetverbindung prüfen.",
  "Wenn die App hängt: Gerät 10 Sekunden vom Strom trennen.",
] as const;

export const metadata = {
  title: "moto NFC-Tablet: Erste Schritte",
  description:
    "Onepager für den Versandkarton: erste Schritte für das moto NFC-Tablet.",
};

export default function NfcQuickstartPage() {
  return (
    <main className="moto-dotted-background moto-dotted-background--guide min-h-screen overflow-x-hidden px-4 py-8 text-gray-950 print:min-h-0 print:w-[210mm] print:p-0">
      <div className="relative mx-auto flex w-full max-w-5xl flex-col gap-8 print:block print:w-[210mm]">
        <HelpHeader pdf={quickstartPdf} />
        <div className="print:hidden">
          <HelpSearchInline />
        </div>
        <section className="relative mx-auto flex min-h-[297mm] w-full max-w-[210mm] flex-col overflow-hidden rounded-[28px] print:h-[296mm] print:min-h-0 print:w-[210mm]">
          <header className="relative overflow-hidden rounded-[28px] bg-[#17231F] text-white">
            <div
              className="absolute right-0 bottom-0 hidden h-full w-[42%] bg-[#F3F4F6] sm:block"
              aria-hidden="true"
            />
            <div className="relative grid min-h-[168px] grid-cols-1 gap-5 px-6 pt-7 pb-7 sm:grid-cols-[1fr_240px] sm:gap-7 sm:px-8 md:grid-cols-[1fr_290px] md:gap-8 md:px-10 md:pt-9 md:pb-8 print:min-h-[142px] print:grid-cols-[1fr_240px] print:px-8 print:pt-6 print:pb-6">
              <div className="max-w-[430px]">
                <div className="flex items-center gap-3">
                  <span className="flex h-12 w-12 items-center justify-center">
                    <Image
                      src="/images/moto_transparent.webp"
                      alt=""
                      width={1283}
                      height={884}
                      className="h-10 w-auto object-contain"
                      priority
                    />
                  </span>
                  <p className="[font-family:var(--font-moto)] text-[30px] leading-none font-bold tracking-normal text-white">
                    moto
                  </p>
                </div>
                <h1 className="mt-5 text-4xl leading-[0.98] font-semibold tracking-normal sm:text-[44px] print:mt-4 print:text-[38px]">
                  Erste Schritte
                </h1>
                <p className="mt-4 max-w-[390px] text-sm leading-6 text-white/78 sm:text-base sm:leading-7 print:mt-3 print:max-w-[360px] print:text-sm print:leading-6">
                  NFC-Tablet in drei kurzen Schritten einsatzbereit machen und
                  die ersten Armbänder den Kindern zuweisen.
                </p>
              </div>
              <div className="relative flex items-center justify-center">
                <Image
                  src="/help/screens/nfc-tablet-willkommen.webp"
                  alt=""
                  width={960}
                  height={960}
                  className="relative z-10 w-[220px] max-w-full object-contain sm:w-[250px] sm:-translate-y-4 md:w-[286px] md:max-w-none md:-translate-y-5 print:w-[238px] print:-translate-y-4"
                  priority
                />
              </div>
            </div>
          </header>

          <div className="flex flex-1 flex-col px-10 py-9 print:px-8 print:py-6">
            <div className="relative grid gap-4 print:gap-3">
              <div
                className="absolute top-10 bottom-10 left-1/2 hidden w-[2px] -translate-x-1/2 rounded-full bg-[#CBD5E1] md:block print:block"
                aria-hidden="true"
              />
              {steps.map((step, index) => {
                const Icon = step.icon;
                const isRight = index === 1;
                const marker = (
                  <div
                    className={[
                      "flex items-center gap-2",
                      isRight ? "justify-end" : "justify-start",
                    ].join(" ")}
                  >
                    <span
                      className={[
                        "flex h-8 w-9 items-center justify-center bg-[#17231F] text-sm font-bold text-white print:h-7 print:w-8 print:text-xs",
                        isRight
                          ? "order-2 [clip-path:polygon(0_0,100%_0,100%_100%,0_100%,18%_50%)]"
                          : "order-1 [clip-path:polygon(0_0,100%_0,82%_50%,100%_100%,0_100%)]",
                      ].join(" ")}
                    >
                      <span
                        className={[
                          "inline-block leading-none",
                          isRight ? "translate-x-0.5" : "-translate-x-0.5",
                        ].join(" ")}
                      >
                        {step.number}
                      </span>
                    </span>
                    <div
                      className={[
                        "bg-moto-green/16 text-moto-green-strong flex h-16 w-16 items-center justify-center rounded-2xl print:h-12 print:w-12",
                        isRight ? "order-1" : "order-2",
                      ].join(" ")}
                    >
                      <Icon
                        className="h-8 w-8 print:h-6 print:w-6"
                        aria-hidden="true"
                      />
                    </div>
                  </div>
                );

                return (
                  <article
                    id={step.id}
                    key={step.number}
                    className={[
                      "relative flex",
                      isRight ? "justify-end" : "justify-start",
                    ].join(" ")}
                  >
                    <div
                      className={[
                        "bg-moto-green absolute top-8 hidden h-3 w-3 items-center justify-center rounded-full md:flex print:flex",
                        isRight
                          ? "left-1/2 -translate-x-[calc(100%+2px)]"
                          : "left-1/2 translate-x-2",
                      ].join(" ")}
                      aria-hidden="true"
                    />

                    <div
                      className={[
                        // oxlint-disable-next-line ui-kit/no-hand-rolled-surface -- printed guide step card: inside .moto-dotted-background--guide the print rules turn moto-content-surface transparent, which would let the dotted page background show through this bordered card in the PDF
                        "relative w-full rounded-2xl border border-gray-200 bg-white p-5 md:w-[78%] print:w-[78%] print:p-4",
                        isRight ? "md:mr-0 print:mr-0" : "",
                      ].join(" ")}
                    >
                      <div
                        className={[
                          "mb-4 sm:absolute sm:top-5 sm:mb-0 print:absolute print:top-4 print:mb-0",
                          isRight
                            ? "sm:right-5 print:right-4"
                            : "sm:left-5 print:left-4",
                        ].join(" ")}
                      >
                        {marker}
                      </div>
                      <div
                        className={[
                          "min-w-0",
                          isRight
                            ? "sm:pr-32 print:pr-24"
                            : "sm:pl-32 print:pl-24",
                        ].join(" ")}
                      >
                        <h2 className="text-[22px] leading-tight font-semibold tracking-normal print:text-[19px]">
                          {step.title}
                        </h2>
                        <p className="mt-3 text-[15px] leading-6 text-gray-700 print:mt-2 print:text-[13px] print:leading-5">
                          {step.text}
                        </p>
                        <p className="mt-3 rounded-xl bg-gray-50 px-4 py-3 text-sm leading-5 font-medium text-gray-700 print:mt-2 print:px-3 print:py-2 print:text-xs print:leading-4">
                          {step.detail}
                        </p>
                      </div>
                    </div>
                  </article>
                );
              })}
            </div>

            <section className="mt-6 grid grid-cols-1 gap-3 sm:grid-cols-3 print:hidden">
              {hints.map((hint, index) => {
                const Icon =
                  index === 0 ? ShieldCheck : index === 1 ? BadgeCheck : Wrench;
                return (
                  <div
                    key={hint}
                    className="rounded-2xl border border-gray-200 bg-gray-50 p-4 print:rounded-xl print:p-3"
                  >
                    <Icon
                      className="text-moto-green-strong h-5 w-5"
                      aria-hidden="true"
                    />
                    <p className="mt-3 text-xs leading-5 font-medium text-gray-700 print:mt-2 print:text-[11px] print:leading-4">
                      {hint}
                    </p>
                  </div>
                );
              })}
            </section>

            <section className="mt-6 rounded-2xl border border-gray-200 bg-gray-50 p-5 print:mt-4 print:p-4">
              <div className="flex items-start gap-4 print:gap-3">
                <span className="bg-moto-green/16 text-moto-green-strong flex h-12 w-12 shrink-0 items-center justify-center rounded-xl print:h-10 print:w-10">
                  <BookOpen
                    className="h-6 w-6 print:h-5 print:w-5"
                    aria-hidden="true"
                  />
                </span>
                <div className="min-w-0 flex-1">
                  <h2 className="text-[19px] leading-tight font-semibold tracking-normal print:text-[17px]">
                    Weiter zum NFC-Betriebsbuch
                  </h2>
                  <p className="mt-2 text-sm leading-6 text-gray-700 print:mt-1.5 print:text-[13px] print:leading-5">
                    Alles Weitere steht im ausführlichen Handbuch: Aufsicht
                    starten und beenden, Kinder ein- und auschecken,
                    Arbeitszeiten stempeln, Geräteeinstellungen und
                    Fehlerbehebung.
                  </p>
                  <Link
                    href="/help/nfc"
                    className="bg-moto-green hover:bg-moto-green-hover active:bg-moto-green-active mt-3 inline-flex items-center gap-2 rounded-lg px-4 py-2 text-sm font-semibold text-gray-950 transition-colors print:hidden"
                  >
                    NFC-Betriebsbuch öffnen
                    <ArrowRight className="h-4 w-4" aria-hidden="true" />
                  </Link>
                  <p className="mt-2 hidden text-[12px] leading-5 font-medium text-gray-700 print:mt-2 print:block">
                    So finden Sie es: moto im Browser öffnen, unten links auf
                    Hilfe klicken und NFC &amp; Tablets wählen.
                  </p>
                </div>
              </div>
            </section>

            <footer className="mt-auto flex items-end justify-between gap-6 pt-5 text-xs text-gray-500 print:pt-3 print:text-[11px]">
              <p className="max-w-[620px]">
                Für OGS-Teams im Alltag. Persönliche Daten werden nicht auf den
                Armbändern gespeichert.
                <span className="hidden print:inline">
                  {" "}
                  Schnellhilfe: Armband flach auflegen, Internet prüfen, bei
                  eingefrorener App 10 Sekunden vom Strom trennen.
                </span>
              </p>
              <p className="[font-family:var(--font-moto)] text-base leading-none font-bold text-gray-700 print:text-sm">
                moto
              </p>
            </footer>
          </div>
        </section>
      </div>
    </main>
  );
}
