import Link from "next/link";
import { ArrowLeft, ArrowRight, Info, ListChecks } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import type { GuideStep } from "./guide-data";
import { PrintButton } from "./print-button";

type ActivePath = "ersteinrichtung" | "funktionen" | "nfc";

const tabs: readonly {
  readonly href: string;
  readonly label: string;
  readonly key: ActivePath;
}[] = [
  {
    href: "/anleitung/ersteinrichtung",
    label: "Ersteinrichtung",
    key: "ersteinrichtung",
  },
  { href: "/anleitung/funktionen", label: "Die App", key: "funktionen" },
  { href: "/anleitung/nfc", label: "NFC", key: "nfc" },
];

/** Splits a string on `code` spans and renders them as inline <code>. */
function InlineText({ text }: { readonly text: string }) {
  const parts = text.split(/(`[^`]+`)/g);
  return (
    <>
      {parts.map((part, index) =>
        part.startsWith("`") && part.endsWith("`") ? (
          <code
            key={index}
            className="rounded-md bg-gray-100 px-1.5 py-0.5 text-[0.92em] font-semibold text-gray-800 print:border print:border-gray-300 print:bg-white"
          >
            {part.slice(1, -1)}
          </code>
        ) : (
          <span key={index}>{part}</span>
        ),
      )}
    </>
  );
}

/** Landing tile. One per top-level guide section. */
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
      className="group moto-content-surface moto-hover-elevated flex flex-col rounded-2xl border p-6 focus-visible:ring-2 focus-visible:ring-[#83CD2D]/40 focus-visible:outline-none"
    >
      <div className="flex items-start justify-between gap-4">
        <span className="flex h-14 w-14 items-center justify-center rounded-2xl bg-[#83CD2D]/14 text-[#3F6F12]">
          <Icon className="h-7 w-7" aria-hidden="true" />
        </span>
        <ArrowRight
          className="h-5 w-5 text-gray-300 transition-transform group-hover:translate-x-1 group-hover:text-[#83CD2D]"
          aria-hidden="true"
        />
      </div>
      <h2 className="mt-5 text-2xl font-semibold tracking-normal text-gray-950">
        {title}
      </h2>
      <p className="mt-2 text-sm leading-6 text-gray-600">{body}</p>
      <ul className="mt-5 space-y-2 text-sm leading-6 text-gray-700">
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

/** Page frame for a single printable guide section (a flat list of steps). */
export function GuideShell({
  eyebrow,
  title,
  description,
  items,
  activePath,
  numbered,
  note,
}: {
  readonly eyebrow: string;
  readonly title: string;
  readonly description: string;
  readonly items: readonly GuideStep[];
  readonly activePath: ActivePath;
  /** true → number badges (a sequence); false → the item's own icon. */
  readonly numbered: boolean;
  /** Optional highlighted note shown under the description. */
  readonly note?: string;
}) {
  return (
    <main className="moto-dotted-background moto-dotted-background--fullscreen min-h-screen overflow-x-hidden print:bg-white">
      <div className="relative mx-auto w-full max-w-4xl px-4 py-4 sm:px-6 sm:py-6 lg:px-8 print:max-w-none print:px-0 print:py-0">
        <header className="print:hidden">
          <div className="flex flex-col gap-3 rounded-2xl border border-gray-200 bg-white/90 p-3 shadow-sm backdrop-blur-md sm:flex-row sm:items-center sm:justify-between sm:p-4">
            <Link
              href="/anleitung"
              className="inline-flex w-fit items-center gap-2 rounded-lg px-2 py-1 text-sm font-semibold text-gray-600 transition-colors hover:bg-gray-100 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
            >
              <ArrowLeft className="h-4 w-4" aria-hidden="true" />
              Übersicht
            </Link>
            <nav
              className="flex flex-wrap items-center gap-2"
              aria-label="Anleitungsbereiche"
            >
              {tabs.map((tab) => (
                <Link
                  key={tab.key}
                  href={tab.href}
                  aria-current={activePath === tab.key ? "page" : undefined}
                  className={`inline-flex h-9 items-center rounded-lg border px-3 text-sm font-semibold transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${
                    activePath === tab.key
                      ? "border-[#83CD2D]/40 bg-[#83CD2D]/16 text-[#3F6F12]"
                      : "border-gray-200 bg-white text-gray-700 hover:border-gray-300 hover:bg-gray-50"
                  }`}
                >
                  {tab.label}
                </Link>
              ))}
              <PrintButton />
            </nav>
          </div>
        </header>

        <section className="py-8 sm:py-10 print:py-4">
          <p className="text-sm font-bold tracking-[0.08em] text-[#3F6F12] uppercase">
            {eyebrow}
          </p>
          <h1 className="mt-3 text-3xl leading-tight font-semibold tracking-normal text-gray-950 sm:text-4xl print:text-2xl">
            {title}
          </h1>
          <p className="mt-3 max-w-3xl text-base leading-7 text-gray-600 print:text-sm print:leading-6">
            {description}
          </p>

          {note ? (
            <div className="mt-4 flex max-w-3xl items-start gap-2.5 rounded-xl border border-[#5080D8]/25 bg-[#5080D8]/8 p-3 text-sm leading-6 text-gray-700 print:bg-white">
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
              {items.map((item, index) => {
                const Icon = item.icon;
                return (
                  <li key={item.id}>
                    <a
                      href={`#${item.id}`}
                      className="group flex items-start gap-2.5 rounded-lg px-2 py-1.5 text-sm text-gray-600 transition-colors hover:bg-gray-50 hover:text-gray-950 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                    >
                      <span className="mt-px flex h-6 w-6 shrink-0 items-center justify-center rounded-md bg-[#83CD2D]/12 text-xs font-semibold text-[#3F6F12]">
                        {numbered ? (
                          index + 1
                        ) : Icon ? (
                          <Icon className="h-3.5 w-3.5" aria-hidden="true" />
                        ) : (
                          index + 1
                        )}
                      </span>
                      <span className="pt-0.5 leading-5">{item.title}</span>
                    </a>
                  </li>
                );
              })}
            </ol>
          </nav>

          <div className="mt-6 space-y-4 print:mt-6 print:space-y-3">
            {items.map((item, index) => (
              <StepCard
                key={item.id}
                item={item}
                badge={numbered ? String(index + 1) : undefined}
              />
            ))}
          </div>
        </section>
      </div>
    </main>
  );
}

function StepCard({
  item,
  badge,
}: {
  readonly item: GuideStep;
  readonly badge?: string;
}) {
  const Icon = item.icon;
  return (
    <article
      id={item.id}
      className="moto-content-surface scroll-mt-24 rounded-2xl border p-5 shadow-sm sm:p-6 print:[break-inside:avoid] print:border-gray-300 print:p-4 print:shadow-none"
    >
      <div className="flex gap-4">
        <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl bg-[#83CD2D]/14 text-sm font-bold text-[#3F6F12] print:border print:border-gray-300 print:bg-white print:text-gray-900">
          {badge ??
            (Icon ? <Icon className="h-5 w-5" aria-hidden="true" /> : null)}
        </span>
        <div className="min-w-0 flex-1">
          <h2 className="text-lg font-semibold tracking-normal text-gray-950 sm:text-xl">
            {item.title}
          </h2>
          <p className="mt-1 text-sm leading-6 text-gray-600">{item.summary}</p>

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

          <Screenshot image={item.image} caption={item.screenshot} />
        </div>
      </div>
    </article>
  );
}

function Screenshot({
  image,
  caption,
}: {
  readonly image?: string;
  readonly caption: string;
}) {
  if (image) {
    return (
      <figure className="mt-4 overflow-hidden rounded-xl border border-gray-200 shadow-sm print:border-gray-300 print:shadow-none">
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img
          src={image}
          alt={caption}
          loading="lazy"
          className="block w-full"
        />
      </figure>
    );
  }
  return (
    <figure className="mt-4 overflow-hidden rounded-xl border border-dashed border-gray-300 bg-gray-50/60 print:bg-white">
      <figcaption className="flex min-h-[120px] items-center justify-center p-5 text-center">
        <span className="max-w-md text-xs leading-5 text-gray-400">
          Screenshot: {caption}
        </span>
      </figcaption>
    </figure>
  );
}
