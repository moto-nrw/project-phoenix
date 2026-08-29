"use client";

import Image from "next/image";
import Link from "next/link";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import {
  ArrowLeft,
  ArrowRight,
  ChevronRight,
  CircleHelp,
  Info,
  Search,
  Sparkles,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { cn } from "~/lib/utils";
import {
  getPrototypeTopics,
  PROTOTYPE_GROUP_LABELS,
  PROTOTYPE_VARIANTS,
  type PrototypePresenceMode,
  type PrototypeRole,
  type PrototypeSchoolyard,
  type PrototypeTopic,
  type PrototypeVariant,
} from "./prototype-data";
import { PrototypeSwitcher } from "./prototype-switcher";

type ModeChoice = "detailed" | "binary-schoolyard" | "binary-door";

const ROLE_ITEMS = [
  { value: "caregiver", label: "Betreuung" },
  { value: "lead", label: "Leitung" },
] as const;

const MODE_ITEMS = [
  { value: "detailed", label: "Mit Räumen" },
  { value: "binary-schoolyard", label: "Einfach mit Schulhof" },
  { value: "binary-door", label: "Einfach ohne Schulhof" },
] as const;

function isVariant(value: string | null): value is PrototypeVariant {
  return value === "a" || value === "b" || value === "c";
}

function isRole(value: string | null): value is PrototypeRole {
  return value === "caregiver" || value === "lead";
}

function isPresenceMode(value: string | null): value is PrototypePresenceMode {
  return value === "detailed" || value === "binary";
}

function isSchoolyard(value: string | null): value is PrototypeSchoolyard {
  return value === "enabled" || value === "disabled";
}

function topicMatches(topic: PrototypeTopic, rawQuery: string): boolean {
  const query = rawQuery.trim().toLocaleLowerCase("de");
  if (!query) return true;
  return [topic.title, topic.question, topic.summary].some((text) =>
    text.toLocaleLowerCase("de").includes(query),
  );
}

function InlineCode({ text }: Readonly<{ text: string }>) {
  const parts = text.split(/(`[^`]+`)/g);
  return (
    <>
      {parts.map((part, index) =>
        part.startsWith("`") && part.endsWith("`") ? (
          <code
            key={`${part}-${index}`}
            className="rounded bg-gray-100 px-1.5 py-0.5 text-[0.9em] font-semibold text-gray-800"
          >
            {part.slice(1, -1)}
          </code>
        ) : (
          part
        ),
      )}
    </>
  );
}

function PrototypeHeader({
  role,
  modeChoice,
  edgeSidebar,
  onRoleChange,
  onModeChange,
}: Readonly<{
  role: PrototypeRole;
  modeChoice: ModeChoice;
  edgeSidebar: boolean;
  onRoleChange: (role: PrototypeRole) => void;
  onModeChange: (mode: ModeChoice) => void;
}>) {
  return (
    <header
      className={cn(
        "sticky top-0 z-20 border-b border-gray-200 bg-white/95 print:hidden",
        edgeSidebar && "lg:ml-72",
      )}
    >
      <div
        className={cn(
          "mx-auto flex w-full max-w-7xl flex-col gap-4 px-4 py-4 sm:px-6 lg:flex-row lg:items-center lg:justify-between lg:px-8",
          edgeSidebar && "lg:max-w-none",
        )}
      >
        <div
          className={cn("flex items-center gap-3", edgeSidebar && "lg:hidden")}
        >
          <Link
            href="/help/prototype"
            className="focus-visible:ring-moto-blue rounded-lg focus-visible:ring-2 focus-visible:outline-none"
          >
            <span className="text-xl font-bold tracking-tight text-gray-950">
              moto Hilfe
            </span>
          </Link>
          <span className="bg-moto-orange/12 text-moto-orange-strong rounded-full px-2.5 py-1 text-xs font-bold tracking-wide uppercase">
            Prototyp
          </span>
        </div>
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
          <div>
            <p className="mb-1 text-xs font-medium text-gray-500">
              Einstieg für
            </p>
            <SegmentedControl
              items={ROLE_ITEMS}
              value={role}
              onChange={onRoleChange}
              ariaLabel="Einstieg wählen"
            />
          </div>
          <div>
            <p className="mb-1 text-xs font-medium text-gray-500">
              Anwesenheit am Tablet
            </p>
            <SegmentedControl
              items={MODE_ITEMS}
              value={modeChoice}
              onChange={onModeChange}
              ariaLabel="Anwesenheitsart wählen"
            />
          </div>
        </div>
      </div>
    </header>
  );
}

function TopicImage({ topic }: Readonly<{ topic: PrototypeTopic }>) {
  if (!topic.image || !topic.imageAlt) return null;
  return (
    <figure className="overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm">
      <Image
        src={topic.image}
        alt={topic.imageAlt}
        width={1280}
        height={800}
        className="h-auto w-full"
      />
      <figcaption className="border-t border-gray-100 px-4 py-3 text-sm text-gray-600">
        {topic.imageAlt}
      </figcaption>
    </figure>
  );
}

function RelatedTopics({
  topic,
  topicsById,
  hrefFor,
  compact = false,
}: Readonly<{
  topic: PrototypeTopic;
  topicsById: ReadonlyMap<string, PrototypeTopic>;
  hrefFor: (topicId?: string) => string;
  compact?: boolean;
}>) {
  const related = topic.related
    .map((id) => topicsById.get(id))
    .filter((item): item is PrototypeTopic => item != null);
  return (
    <section className="mt-8">
      <h2 className="text-lg font-semibold text-gray-950">Das passt dazu</h2>
      <div className={cn("mt-3 grid gap-3", !compact && "sm:grid-cols-2")}>
        {related.map((item) => (
          <Link
            key={item.id}
            href={hrefFor(item.id)}
            className="moto-content-surface group flex items-center justify-between rounded-2xl border p-4 shadow-sm focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
          >
            <span>
              <span className="block text-sm font-semibold text-gray-900">
                {item.title}
              </span>
              <span className="mt-1 block text-xs text-gray-500">
                {item.question}
              </span>
            </span>
            <ChevronRight
              className="h-4 w-4 shrink-0 text-gray-400 group-hover:text-gray-700"
              aria-hidden="true"
            />
          </Link>
        ))}
      </div>
    </section>
  );
}

function VariantA({
  topic,
  topics,
  role,
  hrefFor,
}: Readonly<{
  topic?: PrototypeTopic;
  topics: readonly PrototypeTopic[];
  role: PrototypeRole;
  hrefFor: (topicId?: string) => string;
}>) {
  const topicsById = new Map(topics.map((item) => [item.id, item]));
  const groupOrder =
    role === "lead"
      ? (["alltag", "leitung", "nfc"] as const)
      : (["alltag", "nfc", "leitung"] as const);
  const topicNavigation = (
    <nav aria-label="Themen im Prototyp" className="space-y-5">
      {groupOrder.map((group) => {
        const groupTopics = topics.filter((item) => item.group === group);
        if (groupTopics.length === 0) return null;
        const groupLabel =
          role === "caregiver" && group === "leitung"
            ? "Weitere Themen für die Leitung"
            : PROTOTYPE_GROUP_LABELS[group];
        return (
          <div key={group}>
            <p className="mb-2 text-xs font-semibold text-gray-500">
              {groupLabel}
            </p>
            <div className="space-y-1">
              {groupTopics.map((item) => (
                <Link
                  key={item.id}
                  href={hrefFor(item.id)}
                  aria-current={item.id === topic?.id ? "page" : undefined}
                  className={cn(
                    "block rounded-lg px-3 py-2 text-sm font-medium focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none",
                    item.id === topic?.id
                      ? "bg-gray-900 text-white"
                      : "text-gray-600 hover:bg-gray-100 hover:text-gray-950",
                  )}
                >
                  {item.title}
                </Link>
              ))}
            </div>
          </div>
        );
      })}
    </nav>
  );
  return (
    <div className="w-full lg:pl-72">
      <aside className="fixed inset-y-0 left-0 z-30 hidden w-72 flex-col border-r border-gray-200 bg-white lg:flex">
        <div className="flex h-20 shrink-0 items-center gap-3 border-b border-gray-200 px-6">
          <Link
            href="/help/prototype"
            className="focus-visible:ring-moto-blue rounded-lg focus-visible:ring-2 focus-visible:outline-none"
          >
            <span className="text-xl font-bold tracking-tight text-gray-950">
              moto Hilfe
            </span>
          </Link>
          <span className="bg-moto-orange/12 text-moto-orange-strong rounded-full px-2.5 py-1 text-xs font-bold tracking-wide uppercase">
            Prototyp
          </span>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto px-5 py-6">
          <p className="text-moto-blue-strong text-xs font-bold tracking-wide uppercase">
            Themen
          </p>
          <div className="mt-4">{topicNavigation}</div>
        </div>
      </aside>

      <main className="mx-auto w-full max-w-5xl px-4 py-8 sm:px-6 lg:px-12">
        <details className="moto-content-surface mb-6 rounded-2xl border p-4 shadow-sm lg:hidden">
          <summary className="cursor-pointer text-sm font-semibold text-gray-900">
            {topic ? `Thema: ${topic.title}` : "Themen öffnen"}
          </summary>
          <div className="mt-4 border-t border-gray-100 pt-4">
            {topicNavigation}
          </div>
        </details>
        {topic ? (
          <article className="max-w-3xl">
            <Link
              href={hrefFor()}
              className="mb-5 inline-flex items-center gap-2 text-sm font-medium text-gray-600 hover:text-gray-950"
            >
              <ArrowLeft className="h-4 w-4" aria-hidden="true" />
              Zur Übersicht
            </Link>
            <p className="text-moto-green-strong text-sm font-bold tracking-wide uppercase">
              {PROTOTYPE_GROUP_LABELS[topic.group]}
            </p>
            <h1 className="mt-2 text-3xl font-semibold tracking-tight text-gray-950 sm:text-4xl">
              {topic.question}
            </h1>
            <p className="mt-4 text-lg leading-8 text-gray-600">
              {topic.summary}
            </p>
            <ol className="mt-8 space-y-4">
              {topic.steps.map((step, index) => (
                <li key={step} className="flex gap-4">
                  <span className="bg-moto-green flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-sm font-bold text-gray-950">
                    {index + 1}
                  </span>
                  <p className="pt-1 text-base leading-7 text-gray-800">
                    <InlineCode text={step} />
                  </p>
                </li>
              ))}
            </ol>
            {topic.note && (
              <div className="border-moto-blue/25 bg-moto-blue/8 mt-8 flex gap-3 rounded-2xl border p-4 text-sm leading-6 text-gray-700">
                <Info
                  className="text-moto-blue-strong mt-0.5 h-5 w-5 shrink-0"
                  aria-hidden="true"
                />
                <p>
                  <InlineCode text={topic.note} />
                </p>
              </div>
            )}
            <div className="mt-8">
              <TopicImage topic={topic} />
            </div>
            <RelatedTopics
              topic={topic}
              topicsById={topicsById}
              hrefFor={hrefFor}
            />
          </article>
        ) : (
          <section className="max-w-4xl">
            <p className="text-moto-green-strong text-sm font-bold tracking-wide uppercase">
              {role === "lead" ? "Für die Leitung" : "Für Betreuungskräfte"}
            </p>
            <h1 className="mt-2 text-4xl font-semibold tracking-tight text-gray-950 sm:text-5xl">
              Was möchten Sie erledigen?
            </h1>
            <p className="mt-4 max-w-2xl text-lg leading-8 text-gray-600">
              Wählen Sie links ein Thema. Jede Seite erklärt eine Aufgabe.
            </p>
            <div className="mt-8 grid gap-4 sm:grid-cols-2">
              {topics
                .filter(
                  (item) => item.audience === "all" || item.audience === role,
                )
                .slice(0, 6)
                .map((item) => {
                  const Icon = item.icon;
                  return (
                    <Link
                      key={item.id}
                      href={hrefFor(item.id)}
                      className="moto-content-surface group rounded-2xl border p-5 shadow-sm focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                    >
                      <Icon
                        className="text-moto-blue h-6 w-6"
                        aria-hidden="true"
                      />
                      <h2 className="mt-4 font-semibold text-gray-950">
                        {item.question}
                      </h2>
                      <p className="mt-2 text-sm leading-6 text-gray-600">
                        {item.summary}
                      </p>
                    </Link>
                  );
                })}
            </div>
          </section>
        )}
      </main>
    </div>
  );
}

function VariantB({
  topic,
  topics,
  role,
  hrefFor,
}: Readonly<{
  topic?: PrototypeTopic;
  topics: readonly PrototypeTopic[];
  role: PrototypeRole;
  hrefFor: (topicId?: string) => string;
}>) {
  const topicsById = new Map(topics.map((item) => [item.id, item]));
  const preferred = topics.filter(
    (item) => item.audience === "all" || item.audience === role,
  );
  const additional = topics.filter(
    (item) => item.audience !== "all" && item.audience !== role,
  );
  if (topic) {
    const Icon = topic.icon;
    return (
      <main className="mx-auto w-full max-w-5xl px-4 py-8 sm:px-6 lg:px-8">
        <Link
          href={hrefFor()}
          className="inline-flex items-center gap-2 text-sm font-medium text-gray-600 hover:text-gray-950"
        >
          <ArrowLeft className="h-4 w-4" aria-hidden="true" />
          Alle Aufgaben
        </Link>
        <article className="mt-5 overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm">
          <div className="border-b border-gray-100 p-6 sm:p-8">
            <div className="bg-moto-blue/10 text-moto-blue-strong flex h-12 w-12 items-center justify-center rounded-2xl">
              <Icon className="h-6 w-6" aria-hidden="true" />
            </div>
            <p className="text-moto-blue-strong mt-6 text-sm font-bold tracking-wide uppercase">
              {PROTOTYPE_GROUP_LABELS[topic.group]}
            </p>
            <h1 className="mt-2 text-3xl font-semibold text-gray-950 sm:text-4xl">
              {topic.title}
            </h1>
            <p className="mt-3 text-lg text-gray-600">{topic.summary}</p>
          </div>
          <div className="grid gap-8 p-6 sm:p-8 lg:grid-cols-[minmax(0,1fr)_18rem]">
            <div>
              <h2 className="text-lg font-semibold text-gray-950">
                So geht es
              </h2>
              <div className="mt-4 grid gap-3 sm:grid-cols-2">
                {topic.steps.map((step, index) => (
                  <section
                    key={step}
                    className="rounded-2xl border border-gray-200 bg-gray-50 p-4"
                  >
                    <p className="text-moto-green-strong text-xs font-bold uppercase">
                      Schritt {index + 1}
                    </p>
                    <p className="mt-2 leading-7 text-gray-800">
                      <InlineCode text={step} />
                    </p>
                  </section>
                ))}
              </div>
              {topic.note && (
                <div className="mt-5 flex gap-3 rounded-2xl bg-gray-100 p-4 text-sm leading-6 text-gray-700">
                  <Info
                    className="mt-0.5 h-5 w-5 shrink-0"
                    aria-hidden="true"
                  />
                  <p>
                    <InlineCode text={topic.note} />
                  </p>
                </div>
              )}
            </div>
            <div>
              <TopicImage topic={topic} />
              <RelatedTopics
                topic={topic}
                topicsById={topicsById}
                hrefFor={hrefFor}
                compact
              />
            </div>
          </div>
        </article>
      </main>
    );
  }

  return (
    <main className="mx-auto w-full max-w-6xl px-4 py-10 sm:px-6 lg:px-8">
      <div className="max-w-3xl">
        <p className="text-moto-blue-strong text-sm font-bold tracking-wide uppercase">
          {role === "lead"
            ? "Aufgaben für die Leitung"
            : "Aufgaben in der Betreuung"}
        </p>
        <h1 className="mt-2 text-4xl font-semibold tracking-tight text-gray-950 sm:text-5xl">
          Was möchten Sie jetzt tun?
        </h1>
        <p className="mt-4 text-lg leading-8 text-gray-600">
          Wählen Sie die Aufgabe. Die Anleitung beginnt sofort.
        </p>
      </div>
      <div className="mt-8 grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        {preferred.map((item) => {
          const Icon = item.icon;
          return (
            <Link
              key={item.id}
              href={hrefFor(item.id)}
              className="moto-content-surface group flex min-h-48 flex-col rounded-2xl border p-5 shadow-sm focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
            >
              <div className="flex items-start justify-between">
                <span className="bg-moto-green/15 text-moto-green-strong flex h-10 w-10 items-center justify-center rounded-xl">
                  <Icon className="h-5 w-5" aria-hidden="true" />
                </span>
                <ArrowRight
                  className="h-5 w-5 text-gray-400 group-hover:text-gray-800"
                  aria-hidden="true"
                />
              </div>
              <h2 className="mt-5 text-lg font-semibold text-gray-950">
                {item.question}
              </h2>
              <p className="mt-2 text-sm leading-6 text-gray-600">
                {item.summary}
              </p>
            </Link>
          );
        })}
      </div>
      {additional.length > 0 && (
        <section className="mt-12 border-t border-gray-200 pt-8">
          <h2 className="text-xl font-semibold text-gray-950">
            Weitere Themen für die Leitung
          </h2>
          <p className="mt-2 text-sm text-gray-600">
            Diese Themen bleiben für Einarbeitung und Vertretung erreichbar.
          </p>
          <div className="mt-4 flex flex-wrap gap-2">
            {additional.map((item) => (
              <Link
                key={item.id}
                href={hrefFor(item.id)}
                className="rounded-full border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
              >
                {item.title}
              </Link>
            ))}
          </div>
        </section>
      )}
    </main>
  );
}

function VariantC({
  topic,
  topics,
  role,
  hrefFor,
  query,
  onQueryChange,
}: Readonly<{
  topic?: PrototypeTopic;
  topics: readonly PrototypeTopic[];
  role: PrototypeRole;
  hrefFor: (topicId?: string) => string;
  query: string;
  onQueryChange: (query: string) => void;
}>) {
  const matches = topics.filter((item) => topicMatches(item, query));
  const frequentTopics = topics.filter(
    (item) => item.audience === "all" || item.audience === role,
  );
  const topicsById = new Map(topics.map((item) => [item.id, item]));
  return (
    <main className="mx-auto w-full max-w-2xl px-4 py-8 sm:px-6">
      <div className="text-center">
        <span className="bg-moto-green/15 text-moto-green-strong mx-auto flex h-12 w-12 items-center justify-center rounded-full">
          <Sparkles className="h-6 w-6" aria-hidden="true" />
        </span>
        <p className="text-moto-green-strong mt-4 text-sm font-bold tracking-wide uppercase">
          Schnelle Hilfe
        </p>
        <h1 className="mt-2 text-3xl font-semibold text-gray-950 sm:text-4xl">
          Was möchten Sie wissen?
        </h1>
      </div>
      <div className="relative mt-6">
        <Search
          className="absolute top-1/2 left-4 h-5 w-5 -translate-y-1/2 text-gray-400"
          aria-hidden="true"
        />
        <input
          type="search"
          value={query}
          onChange={(event) => onQueryChange(event.target.value)}
          placeholder="Zum Beispiel: Kind krank melden"
          aria-label="Hilfethemen durchsuchen"
          className="focus:border-moto-blue focus:ring-moto-blue w-full rounded-2xl border border-gray-300 bg-white py-4 pr-4 pl-12 text-base shadow-sm focus:ring-2 focus:outline-none"
        />
      </div>

      {query && (
        <section className="moto-content-surface mt-3 rounded-2xl border p-2 shadow-sm">
          <p className="px-3 py-2 text-xs font-semibold text-gray-500">
            {matches.length === 1
              ? "1 passendes Thema"
              : `${matches.length} passende Themen`}
          </p>
          <div className="space-y-1">
            {matches.map((item) => (
              <Link
                key={item.id}
                href={hrefFor(item.id)}
                onClick={() => onQueryChange("")}
                className="flex items-center justify-between rounded-xl px-3 py-3 text-sm font-medium text-gray-800 hover:bg-gray-100"
              >
                {item.question}
                <ChevronRight
                  className="h-4 w-4 text-gray-400"
                  aria-hidden="true"
                />
              </Link>
            ))}
          </div>
        </section>
      )}

      {topic ? (
        <article className="mt-8">
          <Link
            href={hrefFor()}
            className="inline-flex items-center gap-2 text-sm font-medium text-gray-600 hover:text-gray-950"
          >
            <ArrowLeft className="h-4 w-4" aria-hidden="true" />
            Andere Frage
          </Link>
          <h2 className="mt-5 text-3xl font-semibold tracking-tight text-gray-950">
            {topic.question}
          </h2>
          <p className="mt-3 text-lg leading-8 text-gray-600">
            {topic.summary}
          </p>
          <div className="mt-7 space-y-3">
            {topic.steps.map((step, index) => (
              <div
                key={step}
                className="moto-content-surface flex gap-4 rounded-2xl border p-4 shadow-sm"
              >
                <span className="bg-moto-green flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-sm font-bold text-gray-950">
                  {index + 1}
                </span>
                <p className="pt-1 leading-7 text-gray-800">
                  <InlineCode text={step} />
                </p>
              </div>
            ))}
          </div>
          {topic.note && (
            <div className="mt-4 flex gap-3 rounded-2xl bg-gray-100 p-4 text-sm leading-6 text-gray-700">
              <Info className="mt-0.5 h-5 w-5 shrink-0" aria-hidden="true" />
              <p>
                <InlineCode text={topic.note} />
              </p>
            </div>
          )}
          <div className="mt-6">
            <TopicImage topic={topic} />
          </div>
          <RelatedTopics
            topic={topic}
            topicsById={topicsById}
            hrefFor={hrefFor}
            compact
          />
        </article>
      ) : (
        <section className="mt-8">
          <h2 className="text-lg font-semibold text-gray-950">
            Häufige Fragen
          </h2>
          <div className="mt-3 space-y-2">
            {frequentTopics.slice(0, 7).map((item) => (
              <Link
                key={item.id}
                href={hrefFor(item.id)}
                className="moto-content-surface flex items-center gap-3 rounded-2xl border p-4 shadow-sm focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
              >
                <CircleHelp
                  className="text-moto-blue h-5 w-5 shrink-0"
                  aria-hidden="true"
                />
                <span className="flex-1 font-medium text-gray-900">
                  {item.question}
                </span>
                <ChevronRight
                  className="h-4 w-4 text-gray-400"
                  aria-hidden="true"
                />
              </Link>
            ))}
          </div>
        </section>
      )}
    </main>
  );
}

export function HelpPrototype() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const params = useParams<{ topic?: string[] }>();
  const [query, setQuery] = useState("");

  const rawVariant = searchParams.get("variant");
  const variant: PrototypeVariant = isVariant(rawVariant) ? rawVariant : "a";
  const rawRole = searchParams.get("role");
  const role: PrototypeRole = isRole(rawRole) ? rawRole : "caregiver";
  const rawPresenceMode = searchParams.get("presence_mode");
  const presenceMode: PrototypePresenceMode = isPresenceMode(rawPresenceMode)
    ? rawPresenceMode
    : "detailed";
  const rawSchoolyard = searchParams.get("schoolyard");
  const schoolyard: PrototypeSchoolyard = isSchoolyard(rawSchoolyard)
    ? rawSchoolyard
    : "enabled";
  const modeChoice: ModeChoice =
    presenceMode === "detailed"
      ? "detailed"
      : schoolyard === "enabled"
        ? "binary-schoolyard"
        : "binary-door";

  const topics = useMemo(
    () => getPrototypeTopics(presenceMode, schoolyard),
    [presenceMode, schoolyard],
  );
  const topicId = params.topic?.[0];
  const topic = topics.find((item) => item.id === topicId);

  const hrefFor = useCallback(
    (nextTopicId?: string, overrides?: Record<string, string>) => {
      const queryParams = new URLSearchParams(searchParams.toString());
      queryParams.set("variant", overrides?.variant ?? variant);
      queryParams.set("role", overrides?.role ?? role);
      queryParams.set(
        "presence_mode",
        overrides?.presence_mode ?? presenceMode,
      );
      queryParams.set("schoolyard", overrides?.schoolyard ?? schoolyard);
      const nextPath = nextTopicId
        ? `/help/prototype/${encodeURIComponent(nextTopicId)}`
        : "/help/prototype";
      return `${nextPath}?${queryParams.toString()}`;
    },
    [presenceMode, role, schoolyard, searchParams, variant],
  );

  const replaceState = useCallback(
    (overrides: Record<string, string>) => {
      const nextTopicId = topic?.id;
      router.replace(hrefFor(nextTopicId, overrides), { scroll: false });
    },
    [hrefFor, router, topic?.id],
  );

  useEffect(() => {
    if (topicId || !window.location.hash) return;
    const legacyTopicId = window.location.hash.slice(1);
    if (!topics.some((item) => item.id === legacyTopicId)) return;
    router.replace(hrefFor(legacyTopicId));
  }, [hrefFor, router, topicId, topics]);

  const handleModeChange = (next: ModeChoice) => {
    if (next === "detailed") {
      replaceState({ presence_mode: "detailed", schoolyard: "enabled" });
      return;
    }
    replaceState({
      presence_mode: "binary",
      schoolyard: next === "binary-schoolyard" ? "enabled" : "disabled",
    });
  };

  const invalidTopic = topicId != null && topic == null;

  return (
    <div className="moto-dotted-background moto-dotted-background--fullscreen min-h-screen bg-gray-50 pb-24">
      <PrototypeHeader
        role={role}
        modeChoice={modeChoice}
        edgeSidebar={variant === "a"}
        onRoleChange={(nextRole) => replaceState({ role: nextRole })}
        onModeChange={handleModeChange}
      />

      <div
        className={cn(
          "mx-auto flex w-full max-w-7xl flex-wrap items-center justify-between gap-3 px-4 pt-4 text-xs text-gray-500 sm:px-6 lg:px-8",
          variant === "a" && "lg:ml-72 lg:max-w-none",
        )}
      >
        <p>
          Testzustand: {PROTOTYPE_VARIANTS[variant]} ·{" "}
          {role === "lead" ? "Leitung" : "Betreuung"} ·{" "}
          {modeChoice === "detailed"
            ? "mit Räumen"
            : modeChoice === "binary-schoolyard"
              ? "einfach mit Schulhof"
              : "einfach ohne Schulhof"}
        </p>
        <Link
          href={`${hrefFor()}#kindersuche`}
          className="font-medium text-gray-600 underline decoration-gray-300 underline-offset-4 hover:text-gray-950"
        >
          Alten Hash-Link testen
        </Link>
      </div>

      {invalidTopic ? (
        <main className="mx-auto w-full max-w-2xl px-4 py-16 text-center sm:px-6">
          <CircleHelp
            className="mx-auto h-10 w-10 text-gray-400"
            aria-hidden="true"
          />
          <h1 className="mt-4 text-2xl font-semibold text-gray-950">
            Dieses Thema ist im Prototyp nicht enthalten.
          </h1>
          <p className="mt-2 text-gray-600">
            Öffnen Sie die Übersicht und wählen Sie ein vorhandenes Thema.
          </p>
          <Link
            href={hrefFor()}
            className="mt-6 inline-flex rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white"
          >
            Zur Übersicht
          </Link>
        </main>
      ) : variant === "a" ? (
        <VariantA topic={topic} topics={topics} role={role} hrefFor={hrefFor} />
      ) : variant === "b" ? (
        <VariantB topic={topic} topics={topics} role={role} hrefFor={hrefFor} />
      ) : (
        <VariantC
          topic={topic}
          topics={topics}
          role={role}
          hrefFor={hrefFor}
          query={query}
          onQueryChange={setQuery}
        />
      )}

      <PrototypeSwitcher
        current={variant}
        onChange={(nextVariant) => replaceState({ variant: nextVariant })}
      />
    </div>
  );
}
