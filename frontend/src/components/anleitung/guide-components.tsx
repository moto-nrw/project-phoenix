import Link from "next/link";
import {
  ArrowLeft,
  Check,
  Download,
  Image as ImageIcon,
  ListChecks,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import type { GuideChapter, GuideSection, GuideTone } from "./guide-data";
import { PrintButton } from "./print-button";

const toneClasses: Record<
  GuideTone,
  {
    readonly soft: string;
    readonly text: string;
    readonly border: string;
    readonly ring: string;
  }
> = {
  blue: {
    soft: "bg-[#5080D8]/10",
    text: "text-[#315C9B]",
    border: "border-[#5080D8]/20",
    ring: "ring-[#5080D8]/20",
  },
  green: {
    soft: "bg-[#83CD2D]/14",
    text: "text-[#3F6F12]",
    border: "border-[#83CD2D]/25",
    ring: "ring-[#83CD2D]/25",
  },
  orange: {
    soft: "bg-[#F78C10]/12",
    text: "text-[#9B5609]",
    border: "border-[#F78C10]/25",
    ring: "ring-[#F78C10]/25",
  },
  red: {
    soft: "bg-[#FF3130]/10",
    text: "text-[#CC2626]",
    border: "border-[#FF3130]/20",
    ring: "ring-[#FF3130]/20",
  },
  purple: {
    soft: "bg-purple-500/10",
    text: "text-purple-700",
    border: "border-purple-500/20",
    ring: "ring-purple-500/20",
  },
  gray: {
    soft: "bg-gray-100",
    text: "text-gray-700",
    border: "border-gray-200",
    ring: "ring-gray-200",
  },
};

export function GuideShell({
  eyebrow,
  title,
  description,
  chapters,
  activeVariant,
  children,
}: {
  readonly eyebrow: string;
  readonly title: string;
  readonly description: string;
  readonly chapters: readonly GuideChapter[];
  readonly activeVariant: "standard" | "nfc";
  readonly children?: React.ReactNode;
}) {
  return (
    <main className="moto-dotted-background moto-dotted-background--fullscreen min-h-screen overflow-x-hidden">
      <div className="relative mx-auto w-full max-w-7xl px-4 py-4 sm:px-6 sm:py-6 lg:px-8">
        <header className="print:hidden">
          <div className="flex flex-col gap-4 rounded-3xl border border-gray-200 bg-white/88 p-4 shadow-sm backdrop-blur-md sm:p-5 lg:flex-row lg:items-center lg:justify-between">
            <Link
              href="/anleitung"
              className="inline-flex w-fit items-center gap-2 rounded-lg px-2 py-1 text-sm font-semibold text-gray-600 transition-colors hover:bg-gray-100 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
            >
              <ArrowLeft className="h-4 w-4" aria-hidden="true" />
              Anleitung
            </Link>
            <nav className="flex flex-wrap gap-2" aria-label="Varianten">
              <VariantLink
                href="/anleitung/standard"
                active={activeVariant === "standard"}
              >
                moto Standard
              </VariantLink>
              <VariantLink
                href="/anleitung/nfc"
                active={activeVariant === "nfc"}
              >
                moto NFC
              </VariantLink>
              <PrintButton />
            </nav>
          </div>
        </header>

        <section className="py-8 sm:py-10 lg:py-12">
          <div className="grid gap-8 lg:grid-cols-[minmax(0,1fr)_340px] lg:items-start">
            <div className="min-w-0">
              <p className="text-sm font-bold tracking-[0.08em] text-[#83CD2D] uppercase">
                {eyebrow}
              </p>
              <h1 className="mt-3 max-w-4xl text-4xl leading-tight font-semibold tracking-normal text-gray-950 sm:text-5xl">
                {title}
              </h1>
              <p className="mt-4 max-w-3xl text-base leading-7 text-gray-600 sm:text-lg">
                {description}
              </p>
              {children}
            </div>
            <aside className="moto-content-surface rounded-3xl border p-4 shadow-sm lg:sticky lg:top-6 print:hidden">
              <div className="mb-3 flex items-center gap-2">
                <ListChecks
                  className="h-4 w-4 text-gray-500"
                  aria-hidden="true"
                />
                <h2 className="text-sm font-semibold text-gray-900">Kapitel</h2>
              </div>
              <ol className="space-y-1">
                {chapters.map((chapter, index) => {
                  const Icon = chapter.icon;
                  const tone = toneClasses[chapter.tone];
                  return (
                    <li key={chapter.id}>
                      <a
                        href={`#${chapter.id}`}
                        className="group flex items-center gap-3 rounded-xl px-3 py-2 text-sm text-gray-600 transition-colors hover:bg-gray-50 hover:text-gray-950 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                      >
                        <span
                          className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-lg ${tone.soft} ${tone.text}`}
                        >
                          <Icon className="h-4 w-4" aria-hidden="true" />
                        </span>
                        <span className="min-w-0 flex-1 truncate">
                          {index + 1}. {chapter.title}
                        </span>
                      </a>
                    </li>
                  );
                })}
              </ol>
            </aside>
          </div>
        </section>

        <div className="space-y-12 pb-16 print:space-y-8 print:pb-0">
          {chapters.map((chapter, index) => (
            <GuideChapterBlock
              key={chapter.id}
              chapter={chapter}
              index={index}
            />
          ))}
        </div>
      </div>
    </main>
  );
}

function VariantLink({
  href,
  active,
  children,
}: {
  readonly href: string;
  readonly active: boolean;
  readonly children: React.ReactNode;
}) {
  return (
    <Link
      href={href}
      className={`inline-flex h-10 items-center rounded-lg border px-3 text-sm font-semibold shadow-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${
        active
          ? "border-[#83CD2D]/40 bg-[#83CD2D]/16 text-[#3F6F12]"
          : "border-gray-200 bg-white text-gray-700 hover:border-gray-300 hover:bg-gray-50"
      }`}
    >
      {children}
    </Link>
  );
}

function InlineText({ text }: { readonly text: string }) {
  const parts = text.split(/(`[^`]+`)/g);
  return (
    <>
      {parts.map((part, index) =>
        part.startsWith("`") && part.endsWith("`") ? (
          <code
            key={`${part}-${index}`}
            className="rounded-md bg-gray-100 px-1.5 py-0.5 text-[0.92em] font-semibold text-gray-800"
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

function GuideChapterBlock({
  chapter,
  index,
}: {
  readonly chapter: GuideChapter;
  readonly index: number;
}) {
  const tone = toneClasses[chapter.tone];
  const Icon = chapter.icon;

  return (
    <section id={chapter.id} className="scroll-mt-6">
      <div className="mb-5 grid gap-4 lg:grid-cols-[320px_minmax(0,1fr)] lg:items-end">
        <div>
          <div
            className={`mb-4 flex h-12 w-12 items-center justify-center rounded-2xl ${tone.soft} ${tone.text} ring-1 ${tone.ring}`}
          >
            <Icon className="h-6 w-6" aria-hidden="true" />
          </div>
          <p className="text-sm font-semibold text-gray-500">
            Kapitel {index + 1}
          </p>
          <h2 className="mt-1 text-3xl font-semibold tracking-normal text-gray-950">
            {chapter.title}
          </h2>
        </div>
        <p className="max-w-2xl text-base leading-7 text-gray-600">
          {chapter.description}
        </p>
      </div>
      <div className="space-y-5">
        {chapter.sections.map((section, sectionIndex) => (
          <GuideSectionCard
            key={section.id}
            section={section}
            tone={chapter.tone}
            variant={sectionIndex % 3}
          />
        ))}
      </div>
    </section>
  );
}

function GuideSectionCard({
  section,
  tone,
  variant,
}: {
  readonly section: GuideSection;
  readonly tone: GuideTone;
  readonly variant: number;
}) {
  if (variant === 1 && section.columns) {
    return (
      <article className="moto-content-surface overflow-hidden rounded-3xl border shadow-sm">
        <div className="grid lg:grid-cols-[minmax(0,0.9fr)_minmax(0,1.25fr)]">
          <div className="border-b border-gray-100 p-5 sm:p-6 lg:border-r lg:border-b-0">
            <SectionHeader section={section} tone={tone} />
            {section.callout ? <Callout callout={section.callout} /> : null}
          </div>
          <div className="grid gap-3 p-5 sm:p-6 md:grid-cols-2 xl:grid-cols-3">
            {section.columns.map((column) => (
              <div
                key={column.title}
                className="rounded-2xl border border-gray-200 bg-gray-50/70 p-4"
              >
                <h4 className="text-sm font-semibold text-gray-950">
                  <InlineText text={column.title} />
                </h4>
                <ul className="mt-3 space-y-2 text-sm leading-6 text-gray-600">
                  {column.items.map((item) => (
                    <li key={item} className="flex gap-2">
                      <Check
                        className="mt-1 h-4 w-4 shrink-0 text-[#83CD2D]"
                        aria-hidden="true"
                      />
                      <span>
                        <InlineText text={item} />
                      </span>
                    </li>
                  ))}
                </ul>
              </div>
            ))}
          </div>
        </div>
        {section.screenshot ? (
          <ScreenshotPlaceholder title={section.screenshot} compact />
        ) : null}
      </article>
    );
  }

  if (variant === 2 && section.checklist) {
    return (
      <article className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_360px]">
        <div className="moto-content-surface rounded-3xl border p-5 shadow-sm sm:p-6">
          <SectionHeader section={section} tone={tone} />
          {section.steps ? <StepList steps={section.steps} /> : null}
          {section.screenshot ? (
            <ScreenshotPlaceholder title={section.screenshot} />
          ) : null}
        </div>
        <aside className="rounded-3xl border border-[#83CD2D]/25 bg-white p-5 shadow-sm sm:p-6">
          <div className="mb-4 flex items-center gap-2">
            <Check className="h-5 w-5 text-[#83CD2D]" aria-hidden="true" />
            <h3 className="text-base font-semibold text-gray-950">
              Checkliste
            </h3>
          </div>
          <ul className="space-y-3 text-sm leading-6 text-gray-700">
            {section.checklist.map((item) => (
              <li key={item} className="flex gap-3">
                <span className="mt-1 flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-[#83CD2D]/16 text-[#3F6F12]">
                  <Check className="h-3.5 w-3.5" aria-hidden="true" />
                </span>
                <span>
                  <InlineText text={item} />
                </span>
              </li>
            ))}
          </ul>
        </aside>
      </article>
    );
  }

  return (
    <article className="moto-content-surface rounded-3xl border p-5 shadow-sm sm:p-6">
      <div className="grid gap-6 lg:grid-cols-[minmax(0,0.95fr)_minmax(0,1.05fr)]">
        <div>
          <SectionHeader section={section} tone={tone} />
          {section.callout ? <Callout callout={section.callout} /> : null}
          {section.columns ? <ColumnList columns={section.columns} /> : null}
        </div>
        <div>
          {section.steps ? <StepList steps={section.steps} /> : null}
          {section.checklist ? (
            <Checklist items={section.checklist} className="mt-4" />
          ) : null}
          {section.screenshot ? (
            <ScreenshotPlaceholder title={section.screenshot} />
          ) : null}
        </div>
      </div>
    </article>
  );
}

function SectionHeader({
  section,
  tone,
}: {
  readonly section: GuideSection;
  readonly tone: GuideTone;
}) {
  const toneClass = toneClasses[tone];
  return (
    <div>
      <p className={`text-sm font-bold ${toneClass.text}`}>{section.kicker}</p>
      <h3 className="mt-1 text-2xl font-semibold tracking-normal text-gray-950">
        {section.title}
      </h3>
      <p className="mt-3 text-base leading-7 text-gray-600">{section.body}</p>
    </div>
  );
}

function StepList({ steps }: { readonly steps: readonly string[] }) {
  return (
    <ol className="space-y-3">
      {steps.map((step, index) => (
        <li key={step} className="flex gap-3">
          <span className="mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-[#83CD2D]/16 text-xs font-semibold text-[#3F6F12]">
            {index + 1}
          </span>
          <span className="pt-0.5 text-sm leading-6 text-gray-700">
            <InlineText text={step} />
          </span>
        </li>
      ))}
    </ol>
  );
}

function ColumnList({
  columns,
}: {
  readonly columns: NonNullable<GuideSection["columns"]>;
}) {
  return (
    <div className="mt-5 grid gap-3 sm:grid-cols-2">
      {columns.map((column) => (
        <div key={column.title} className="rounded-2xl bg-gray-50 p-4">
          <h4 className="text-sm font-semibold text-gray-950">
            <InlineText text={column.title} />
          </h4>
          <ul className="mt-3 space-y-2 text-sm leading-6 text-gray-600">
            {column.items.map((item) => (
              <li key={item}>
                <InlineText text={item} />
              </li>
            ))}
          </ul>
        </div>
      ))}
    </div>
  );
}

function Checklist({
  items,
  className = "",
}: {
  readonly items: readonly string[];
  readonly className?: string;
}) {
  return (
    <div
      className={`rounded-2xl border border-gray-200 bg-gray-50 p-4 ${className}`}
    >
      <h4 className="mb-3 text-sm font-semibold text-gray-950">Checkliste</h4>
      <ul className="space-y-2 text-sm leading-6 text-gray-700">
        {items.map((item) => (
          <li key={item} className="flex gap-2">
            <Check
              className="mt-1 h-4 w-4 shrink-0 text-[#83CD2D]"
              aria-hidden="true"
            />
            <span>
              <InlineText text={item} />
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}

function Callout({
  callout,
}: {
  readonly callout: NonNullable<GuideSection["callout"]>;
}) {
  const tone = toneClasses[callout.tone ?? "blue"];
  return (
    <div className={`mt-5 rounded-2xl border p-4 ${tone.soft} ${tone.border}`}>
      <h4 className={`text-sm font-semibold ${tone.text}`}>{callout.title}</h4>
      <p className="mt-1 text-sm leading-6 text-gray-700">
        <InlineText text={callout.body} />
      </p>
    </div>
  );
}

function ScreenshotPlaceholder({
  title,
  compact = false,
}: {
  readonly title: string;
  readonly compact?: boolean;
}) {
  return (
    <div
      className={`mt-5 overflow-hidden rounded-2xl border border-dashed border-gray-300 bg-[linear-gradient(135deg,#f8fafc_0%,#ffffff_52%,#f3f4f6_100%)] ${
        compact ? "mx-5 mb-5" : ""
      }`}
    >
      <div className="flex items-center justify-between border-b border-dashed border-gray-200 px-4 py-3">
        <div className="flex items-center gap-2 text-sm font-semibold text-gray-700">
          <ImageIcon className="h-4 w-4 text-gray-400" aria-hidden="true" />
          Screenshot-Platzhalter
        </div>
        <Download className="h-4 w-4 text-gray-300" aria-hidden="true" />
      </div>
      <div className="flex min-h-[150px] items-center justify-center p-6 text-center">
        <p className="max-w-md text-sm leading-6 text-gray-500">{title}</p>
      </div>
    </div>
  );
}

export function LandingFeatureCard({
  title,
  body,
  icon: Icon,
}: {
  readonly title: string;
  readonly body: string;
  readonly icon: LucideIcon;
}) {
  return (
    <div className="rounded-2xl border border-gray-200 bg-white p-4 shadow-sm">
      <div className="flex items-start gap-3">
        <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-gray-100 text-gray-700">
          <Icon className="h-5 w-5" aria-hidden="true" />
        </span>
        <div>
          <h3 className="text-sm font-semibold text-gray-950">{title}</h3>
          <p className="mt-1 text-sm leading-6 text-gray-600">{body}</p>
        </div>
      </div>
    </div>
  );
}
