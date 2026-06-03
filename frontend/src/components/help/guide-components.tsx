import Link from "next/link";
import { ArrowLeft, ArrowRight, Check, Info, ListChecks } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import type {
  GuideCallout,
  GuideChapter,
  GuideStep,
  GuideTone,
} from "./guide-data";
import { GuidePdfButton } from "./guide-pdf-button";

type ActivePath = "ersteinrichtung" | "funktionen" | "nfc";

// Maps each guide to its pre-generated PDF under /help/pdfs (see
// scripts/generate-guides.ts). The served paths stay ASCII (route slugs are
// English) to avoid URL-encoding pitfalls; `download` sets the German filename
// the visitor actually sees — matching the page names on the overview.
const pdfByPath: Record<ActivePath, { href: string; download: string }> = {
  ersteinrichtung: {
    href: "/help/pdfs/setup.pdf",
    download: "moto Anleitung – Ersteinrichtung.pdf",
  },
  funktionen: {
    href: "/help/pdfs/features.pdf",
    download: "moto Anleitung – Die App im Alltag.pdf",
  },
  nfc: {
    href: "/help/pdfs/nfc.pdf",
    download: "moto Anleitung – NFC & Tablets.pdf",
  },
};

const nextLinks: Record<
  ActivePath,
  readonly {
    readonly href: string;
    readonly label: string;
    readonly title: string;
    readonly body: string;
    readonly icon: LucideIcon;
  }[]
> = {
  ersteinrichtung: [
    {
      href: "/help/features",
      label: "Als nächstes",
      title: "Die App entdecken",
      body: "Jeder Bereich der App, verständlich erklärt für den Alltag nach der Einrichtung.",
      icon: ArrowRight,
    },
  ],
  funktionen: [
    {
      href: "/help/nfc",
      label: "Optional",
      title: "NFC & Tablets ansehen",
      body: "Zusätzliche Vorbereitung für Einrichtungen mit Tablets oder NFC-Armbändern.",
      icon: ArrowRight,
    },
  ],
  nfc: [
    {
      href: "/help",
      label: "Fertig",
      title: "Zur Übersicht zurück",
      body: "Alle Anleitungsbereiche noch einmal gesammelt an einem Ort.",
      icon: ArrowLeft,
    },
  ],
};

const toneClasses: Record<
  GuideTone,
  { readonly soft: string; readonly text: string; readonly border: string }
> = {
  blue: {
    soft: "bg-[#5080D8]/12",
    text: "text-[#315C9B]",
    border: "border-[#5080D8]/25",
  },
  green: {
    soft: "bg-[#83CD2D]/14",
    text: "text-[#3F6F12]",
    border: "border-[#83CD2D]/25",
  },
  orange: {
    soft: "bg-[#F78C10]/12",
    text: "text-[#9B5609]",
    border: "border-[#F78C10]/25",
  },
  red: {
    soft: "bg-[#FF3130]/10",
    text: "text-[#CC2626]",
    border: "border-[#FF3130]/20",
  },
  purple: {
    soft: "bg-[#7C3AED]/10",
    text: "text-[#6D28D9]",
    border: "border-[#7C3AED]/20",
  },
  gray: {
    soft: "bg-gray-100",
    text: "text-gray-700",
    border: "border-gray-200",
  },
};

function InlineText({ text }: { readonly text: string }) {
  const parts = text.split(/(`[^`]+`)/g);
  return (
    <>
      {parts.map((part, index) =>
        part.startsWith("`") && part.endsWith("`") ? (
          <code
            key={`${part}-${index}`}
            className="rounded-md bg-gray-100 px-1.5 py-0.5 text-[0.92em] font-semibold text-gray-800 print:bg-white print:px-0"
          >
            {part.slice(1, -1)}
          </code>
        ) : (
          <span key={`${part}-${index}`}>{part}</span>
        ),
      )}
    </>
  );
}

export function EntryPointCard({
  href,
  title,
  body,
  icon: Icon,
  points,
}: {
  readonly href: string;
  readonly title: string;
  readonly body: string;
  readonly icon: LucideIcon;
  readonly points: readonly string[];
}) {
  return (
    <Link
      href={href}
      className="group moto-content-surface flex flex-col rounded-2xl border p-5 shadow-sm transition-all duration-200 hover:-translate-y-0.5 hover:border-gray-300 hover:shadow-md focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none sm:p-6"
    >
      <div className="flex items-start justify-between gap-4">
        <span className="flex h-12 w-12 items-center justify-center rounded-2xl bg-[#83CD2D]/14 text-[#3F6F12]">
          <Icon className="h-6 w-6" aria-hidden="true" />
        </span>
        <ArrowRight
          className="h-5 w-5 text-gray-400 transition-transform group-hover:translate-x-1"
          aria-hidden="true"
        />
      </div>
      <h2 className="mt-5 text-xl font-semibold tracking-normal text-gray-950">
        {title}
      </h2>
      <p className="mt-2 text-sm leading-6 text-gray-600">{body}</p>
      <ul className="mt-4 space-y-2 text-sm leading-6 text-gray-700">
        {points.map((point) => (
          <li key={point} className="flex gap-2">
            <span
              className="mt-2 h-1.5 w-1.5 shrink-0 rounded-full bg-[#83CD2D]"
              aria-hidden="true"
            />
            <span>{point}</span>
          </li>
        ))}
      </ul>
    </Link>
  );
}

export function GuideShell({
  eyebrow,
  title,
  description,
  chapters,
  activePath,
  numbered,
  note,
}: {
  readonly eyebrow: string;
  readonly title: string;
  readonly description: string;
  readonly chapters: readonly GuideChapter[];
  readonly activePath: ActivePath;
  /** true → continuous number badges across chapters; false → per-card icon. */
  readonly numbered: boolean;
  /** Optional highlighted note shown under the description. */
  readonly note?: string;
}) {
  // Running start index per chapter so numbered pages count continuously.
  let runningIndex = 0;
  const chapterStartIndex = chapters.map((chapter) => {
    const start = runningIndex;
    runningIndex += chapter.steps.length;
    return start;
  });

  return (
    <main className="moto-dotted-background moto-dotted-background--guide min-h-screen overflow-x-hidden print:bg-white print:before:hidden">
      <div className="relative mx-auto w-full max-w-5xl px-4 py-4 sm:px-6 sm:py-6 lg:px-8 print:max-w-none print:px-0 print:py-0">
        <header className="print:hidden">
          <div className="flex flex-col gap-3 rounded-2xl border border-gray-200 bg-white/90 p-3 shadow-sm backdrop-blur-md sm:flex-row sm:items-center sm:justify-between sm:p-4">
            <Link
              href="/help"
              className="inline-flex w-fit items-center gap-2 rounded-lg px-2 py-1 text-sm font-semibold text-gray-600 transition-colors hover:bg-gray-100 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
            >
              <ArrowLeft className="h-4 w-4" aria-hidden="true" />
              Übersicht
            </Link>
            <div className="flex w-fit flex-wrap items-center gap-2">
              <GuidePdfButton
                href={pdfByPath[activePath].href}
                download={pdfByPath[activePath].download}
              />
            </div>
          </div>
        </header>

        <section className="py-8 sm:py-10 print:py-4">
          <p className="text-sm font-bold tracking-[0.08em] text-[#3F6F12] uppercase">
            {eyebrow}
          </p>
          <h1 className="mt-3 text-3xl leading-tight font-semibold tracking-normal text-gray-950 sm:text-4xl print:text-2xl">
            {title}
          </h1>
          <p className="mt-3 text-base leading-7 text-gray-600 print:text-sm print:leading-6">
            {description}
          </p>

          {note ? (
            <div className="mt-4 flex items-start gap-2.5 rounded-xl border border-[#5080D8]/25 bg-[#5080D8]/8 p-3 text-sm leading-6 text-gray-700 print:bg-white">
              <Info
                className="mt-0.5 h-4 w-4 shrink-0 text-[#315C9B]"
                aria-hidden="true"
              />
              <span>
                <InlineText text={note} />
              </span>
            </div>
          ) : null}

          <nav
            className="mt-6 rounded-2xl border border-gray-200 bg-white/80 p-3 shadow-sm backdrop-blur-md print:hidden"
            aria-label="Auf dieser Seite"
          >
            <div className="mb-2.5 flex items-center gap-2 px-1">
              <ListChecks
                className="h-4 w-4 text-gray-500"
                aria-hidden="true"
              />
              <span className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
                Auf dieser Seite
              </span>
            </div>
            <ol className="grid grid-cols-1 gap-x-6 gap-y-0.5 sm:grid-cols-2 lg:grid-cols-3">
              {chapters.map((chapter, index) => {
                const Icon = chapter.icon;
                const tone = toneClasses[chapter.tone];
                return (
                  <li key={chapter.id}>
                    <a
                      href={`#${chapter.id}`}
                      className="group flex items-start gap-2.5 rounded-lg px-2 py-1.5 text-sm text-gray-600 transition-colors hover:bg-gray-50 hover:text-gray-950 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                    >
                      <span
                        className={`mt-px flex h-6 w-6 shrink-0 items-center justify-center rounded-md ${tone.soft} ${tone.text}`}
                      >
                        <Icon className="h-3.5 w-3.5" aria-hidden="true" />
                      </span>
                      <span className="pt-0.5 leading-5">
                        {index + 1}. {chapter.title}
                      </span>
                    </a>
                  </li>
                );
              })}
            </ol>
          </nav>

          <div className="mt-8 space-y-12 print:mt-6 print:space-y-8">
            {chapters.map((chapter, index) => (
              <ChapterBlock
                key={chapter.id}
                chapter={chapter}
                index={index}
                startIndex={chapterStartIndex[index] ?? 0}
                numbered={numbered}
              />
            ))}
          </div>

          <GuideNextLinks activePath={activePath} />
        </section>
      </div>
    </main>
  );
}

function GuideNextLinks({ activePath }: { readonly activePath: ActivePath }) {
  const links = nextLinks[activePath];

  return (
    <section className="mt-12 print:hidden" aria-labelledby="guide-next-title">
      <p className="text-sm font-bold tracking-[0.08em] text-[#3F6F12] uppercase">
        Weiter in der Anleitung
      </p>
      <h2
        id="guide-next-title"
        className="mt-2 text-2xl font-semibold tracking-normal text-gray-950"
      >
        Als nächstes
      </h2>
      <div className="mt-4 grid w-full gap-4">
        {links.map((link) => {
          const Icon = link.icon;
          return (
            <Link
              key={link.href}
              href={link.href}
              className="group moto-content-surface flex items-start gap-4 rounded-2xl border p-5 shadow-sm transition-all duration-200 hover:-translate-y-0.5 hover:border-gray-300 hover:shadow-md focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
            >
              <span className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-[#83CD2D]/14 text-[#3F6F12]">
                <Icon
                  className="h-5 w-5 transition-transform group-hover:translate-x-0.5"
                  aria-hidden="true"
                />
              </span>
              <span className="min-w-0">
                <span className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
                  {link.label}
                </span>
                <span className="mt-1 block text-xl font-semibold tracking-normal text-gray-950">
                  {link.title}
                </span>
                <span className="mt-2 block text-sm leading-6 text-gray-600">
                  {link.body}
                </span>
              </span>
            </Link>
          );
        })}
      </div>
    </section>
  );
}

function ChapterBlock({
  chapter,
  index,
  startIndex,
  numbered,
}: {
  readonly chapter: GuideChapter;
  readonly index: number;
  readonly startIndex: number;
  readonly numbered: boolean;
}) {
  const tone = toneClasses[chapter.tone];
  const Icon = chapter.icon;

  return (
    <section id={chapter.id} className="scroll-mt-6 print:[break-inside:avoid]">
      <div className="mb-5 flex items-start gap-4">
        <div
          className={`flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl ${tone.soft} ${tone.text} print:border print:border-gray-300 print:bg-white print:text-gray-900`}
        >
          <Icon className="h-6 w-6" aria-hidden="true" />
        </div>
        <div className="min-w-0">
          <p className="text-sm font-semibold text-gray-500">
            Kapitel {index + 1}
          </p>
          <h2 className="mt-0.5 text-2xl font-semibold tracking-normal text-gray-950 sm:text-3xl print:text-xl">
            {chapter.title}
          </h2>
          <p className="mt-2 text-base leading-7 text-gray-600 print:text-sm print:leading-6">
            {chapter.description}
          </p>
        </div>
      </div>

      <div className="space-y-4 print:space-y-3">
        {chapter.steps.map((step, stepIndex) => (
          <StepCard
            key={step.id}
            item={step}
            tone={chapter.tone}
            badge={numbered ? String(startIndex + stepIndex + 1) : undefined}
          />
        ))}
      </div>
    </section>
  );
}

function StepCard({
  item,
  tone,
  badge,
}: {
  readonly item: GuideStep;
  readonly tone: GuideTone;
  readonly badge?: string;
}) {
  const toneClass = toneClasses[tone];
  const Icon = item.icon;
  return (
    <article
      id={item.id}
      className="moto-content-surface scroll-mt-24 rounded-2xl border p-5 shadow-sm sm:p-6 print:[break-inside:avoid] print:border-gray-300 print:p-4 print:shadow-none"
    >
      <div className="flex gap-4">
        <span
          className={`flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl text-sm font-bold ${toneClass.soft} ${toneClass.text} print:border print:border-gray-300 print:bg-white print:text-gray-900`}
        >
          {badge ??
            (Icon ? <Icon className="h-5 w-5" aria-hidden="true" /> : null)}
        </span>
        <div className="min-w-0 flex-1">
          <h3 className="text-lg font-semibold tracking-normal text-gray-950 sm:text-xl">
            {item.title}
          </h3>
          <p className="mt-1 text-sm leading-6 text-gray-600">
            <InlineText text={item.summary} />
          </p>

          {item.steps ? (
            <ol className="mt-4 space-y-2.5">
              {item.steps.map((step, index) => (
                <li key={index} className="flex gap-3">
                  <span className="mt-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-gray-100 text-xs font-semibold text-gray-600 print:border print:border-gray-300 print:bg-white">
                    {index + 1}
                  </span>
                  <span className="pt-0.5 text-sm leading-6 text-gray-700">
                    <InlineText text={step} />
                  </span>
                </li>
              ))}
            </ol>
          ) : null}

          {item.checklist ? <Checklist items={item.checklist} /> : null}
          {item.callout ? <Callout callout={item.callout} /> : null}

          {item.gallery ? (
            <ScreenshotGallery items={item.gallery} />
          ) : (
            <Screenshot image={item.image} caption={item.screenshot} />
          )}
        </div>
      </div>
    </article>
  );
}

function Checklist({ items }: { readonly items: readonly string[] }) {
  return (
    <div className="mt-4 rounded-xl border border-[#83CD2D]/25 bg-[#83CD2D]/8 p-4 print:border-gray-300 print:bg-white">
      <div className="mb-3 flex items-center gap-2">
        <Check className="h-4 w-4 text-[#3F6F12]" aria-hidden="true" />
        <h4 className="text-sm font-semibold text-gray-950">Checkliste</h4>
      </div>
      <ul className="space-y-2.5 text-sm leading-6 text-gray-700">
        {items.map((item) => (
          <li key={item} className="flex gap-3">
            <span className="mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-[#83CD2D]/16 text-[#3F6F12] print:border print:border-gray-300 print:bg-white">
              <Check className="h-3.5 w-3.5" aria-hidden="true" />
            </span>
            <span>
              <InlineText text={item} />
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}

function Callout({ callout }: { readonly callout: GuideCallout }) {
  const tone = toneClasses[callout.tone ?? "blue"];
  return (
    <div
      className={`mt-4 rounded-xl border p-3 ${tone.soft} ${tone.border} print:bg-white`}
    >
      <h4 className={`text-sm font-semibold ${tone.text}`}>{callout.title}</h4>
      <p className="mt-1 text-sm leading-6 text-gray-700">
        <InlineText text={callout.body} />
      </p>
    </div>
  );
}

function Screenshot({
  image,
  caption,
}: {
  readonly image?: string;
  readonly caption: string;
}) {
  // No image: render nothing. The `caption` still serves as documentation /
  // alt text in the data, but image-less steps show no placeholder box.
  if (!image) {
    return null;
  }
  return (
    <figure className="mt-4 overflow-hidden rounded-xl border border-gray-200 shadow-sm print:border-gray-300 print:shadow-none">
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img src={image} alt={caption} loading="lazy" className="block w-full" />
    </figure>
  );
}

function ScreenshotGallery({
  items,
}: {
  readonly items: readonly {
    readonly image: string;
    readonly caption: string;
  }[];
}) {
  return (
    <div className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-2 print:grid-cols-2 print:gap-3">
      {items.map((item) => (
        <figure
          key={item.image}
          className="overflow-hidden rounded-xl border border-gray-200 bg-white shadow-sm print:[break-inside:avoid] print:border-gray-300 print:shadow-none"
        >
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img
            src={item.image}
            alt={item.caption}
            loading="lazy"
            className="block w-full"
          />
          <figcaption className="border-t border-gray-100 px-3 py-2 text-xs leading-5 text-gray-500 print:border-gray-200">
            {item.caption}
          </figcaption>
        </figure>
      ))}
    </div>
  );
}
