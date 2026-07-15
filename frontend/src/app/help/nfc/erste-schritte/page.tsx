import Image from "next/image";
import {
  BadgeCheck,
  KeyRound,
  Nfc,
  PlugZap,
  QrCode,
  ShieldCheck,
  Wrench,
} from "lucide-react";

const fullGuideUrl = "https://moto-ogs.de/help/nfc";

const steps = [
  {
    number: "1",
    title: "Tablet montieren und einschalten",
    icon: PlugZap,
    text: "Vor der Montage einen Platz mit Strom und guter WLAN-Reichweite wählen. Optional mit Wand- oder Standhalterung montieren und die Angaben des Herstellers beachten. Danach Strom einstecken.",
    detail:
      "Das Gerät startet automatisch und verbindet sich mit dem hinterlegten WLAN.",
  },
  {
    number: "2",
    title: "Mit Geräte-PIN anmelden",
    icon: KeyRound,
    text: "Auf dem Startbildschirm auf Anmelden tippen und die 4-stellige PIN eingeben. Die Standard-PIN bei Auslieferung ist 1234. Nach der vierten Ziffer prüft das Tablet automatisch.",
    detail:
      "Wir empfehlen, die Standard-PIN nach der ersten Anmeldung unter Einstellungen, Geräte, OGS Geräte-PIN zu ändern.",
  },
  {
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
    "Versteckter Onepager für den Versandkarton: erste Schritte für das moto NFC-Tablet.",
};

export default function NfcQuickstartPage() {
  return (
    <main className="moto-dotted-background moto-dotted-background--guide min-h-screen overflow-x-hidden px-4 py-8 text-gray-950 print:p-0">
      <section className="relative mx-auto flex min-h-[297mm] w-full max-w-[210mm] flex-col overflow-hidden rounded-[28px] print:h-[297mm] print:min-h-0 print:max-w-none">
        <header className="relative overflow-hidden rounded-[28px] bg-[#17231F] text-white">
          <div
            className="absolute right-0 bottom-0 h-full w-[42%] bg-[#F3F4F6]"
            aria-hidden="true"
          />
          <div className="relative grid min-h-[168px] grid-cols-[1fr_290px] gap-8 px-10 pt-9 pb-8 print:min-h-[142px] print:grid-cols-[1fr_240px] print:px-8 print:pt-6 print:pb-6">
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
              <h1 className="mt-5 text-[44px] leading-[0.98] font-semibold tracking-normal print:mt-4 print:text-[38px]">
                Erste Schritte
              </h1>
              <p className="mt-4 max-w-[390px] text-base leading-7 text-white/78 print:mt-3 print:max-w-[360px] print:text-sm print:leading-6">
                NFC-Tablet in drei kurzen Schritten einsatzbereit machen und die
                ersten Armbänder den Kindern zuweisen.
              </p>
            </div>
            <div className="relative flex items-center justify-center">
              <Image
                src="/help/screens/nfc-tablet-willkommen.webp"
                alt=""
                width={960}
                height={960}
                className="relative z-10 w-[286px] max-w-none -translate-y-5 object-contain print:w-[238px] print:-translate-y-4"
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
                      "flex h-8 w-9 items-center justify-center bg-[#315C9B] text-sm font-bold text-white print:h-7 print:w-8 print:text-xs",
                      isRight
                        ? "order-2 [clip-path:polygon(0_0,100%_0,100%_100%,0_100%,18%_50%)]"
                        : "order-1 [clip-path:polygon(0_0,100%_0,82%_50%,100%_100%,0_100%)]",
                    ].join(" ")}
                  >
                    {step.number}
                  </span>
                  <div
                    className={[
                      "flex h-16 w-16 items-center justify-center rounded-3xl bg-[#83CD2D]/16 text-[#3F6F12] print:h-12 print:w-12 print:rounded-2xl",
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
                  key={step.number}
                  className={[
                    "relative flex",
                    isRight ? "justify-end" : "justify-start",
                  ].join(" ")}
                >
                  <div
                    className={[
                      "absolute top-8 hidden h-3 w-3 items-center justify-center rounded-full bg-[#83CD2D] md:flex print:flex",
                      isRight
                        ? "left-1/2 -translate-x-[calc(100%+2px)]"
                        : "left-1/2 translate-x-2",
                    ].join(" ")}
                    aria-hidden="true"
                  />

                  <div
                    className={[
                      "relative w-full rounded-3xl border border-gray-200 bg-white p-5 md:w-[78%] print:w-[78%] print:rounded-2xl print:p-4",
                      isRight ? "md:mr-0 print:mr-0" : "",
                    ].join(" ")}
                  >
                    <div
                      className={[
                        "absolute top-5 print:top-4",
                        isRight
                          ? "right-5 print:right-4"
                          : "left-5 print:left-4",
                      ].join(" ")}
                    >
                      {marker}
                    </div>
                    <div
                      className={[
                        "min-w-0",
                        isRight ? "pr-32 print:pr-24" : "pl-32 print:pl-24",
                      ].join(" ")}
                    >
                      <h2 className="text-[22px] leading-tight font-semibold tracking-normal print:text-[19px]">
                        {step.title}
                      </h2>
                      <p className="mt-3 text-[15px] leading-6 text-gray-700 print:mt-2 print:text-[13px] print:leading-5">
                        {step.text}
                      </p>
                      <p className="mt-3 rounded-2xl bg-[#F9FAFB] px-4 py-3 text-sm leading-5 font-medium text-gray-700 print:mt-2 print:rounded-xl print:px-3 print:py-2 print:text-xs print:leading-4">
                        {step.detail}
                      </p>
                    </div>
                  </div>
                </article>
              );
            })}
          </div>

          <section className="mt-7 grid grid-cols-[1fr_190px] gap-5 rounded-[28px] bg-[#5080D8]/10 p-5 print:mt-4 print:grid-cols-[1fr_150px] print:gap-4 print:rounded-2xl print:p-4">
            <div>
              <div className="flex items-center gap-2 text-[#315C9B]">
                <QrCode className="h-5 w-5" aria-hidden="true" />
                <h2 className="text-lg font-semibold tracking-normal print:text-base">
                  Ausführliche Anleitung öffnen
                </h2>
              </div>
              <p className="mt-3 text-sm leading-6 text-gray-700 print:mt-2 print:text-xs print:leading-5">
                Dort finden Sie die vollständige Schritt-für-Schritt-Anleitung
                mit Bildern: Geräte anmelden, Armbänder zuweisen, Check-in und
                Check-out nutzen und typische Fehler schnell beheben.
              </p>
              <p className="mt-4 text-xs font-semibold text-gray-500">
                {fullGuideUrl.replace("https://", "")}
              </p>
            </div>

            <div className="rounded-3xl border border-gray-200 bg-white p-4 text-center print:rounded-2xl print:p-3">
              <Image
                src="/help/quickstart/nfc-guide-qr.svg"
                alt=""
                width={128}
                height={128}
                className="mx-auto h-32 w-32 print:h-24 print:w-24"
              />
              <p className="mt-2 text-[11px] leading-4 font-semibold text-gray-700">
                QR-Code scannen
              </p>
            </div>
          </section>

          <section className="mt-6 grid grid-cols-3 gap-3 print:hidden">
            {hints.map((hint, index) => {
              const Icon =
                index === 0 ? ShieldCheck : index === 1 ? BadgeCheck : Wrench;
              return (
                <div
                  key={hint}
                  className="rounded-2xl border border-gray-200 bg-[#F9FAFB] p-4 print:rounded-xl print:p-3"
                >
                  <Icon className="h-5 w-5 text-[#3F6F12]" aria-hidden="true" />
                  <p className="mt-3 text-xs leading-5 font-medium text-gray-700 print:mt-2 print:text-[11px] print:leading-4">
                    {hint}
                  </p>
                </div>
              );
            })}
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
    </main>
  );
}
