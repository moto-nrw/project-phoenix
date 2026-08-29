"use client";

import Image from "next/image";
import Link from "next/link";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import { ArrowLeft, ChevronRight, CircleHelp, Info } from "lucide-react";
import { useCallback, useEffect, useMemo } from "react";
import { cn } from "~/lib/utils";
import {
  getPrototypeTopics,
  PROTOTYPE_GROUP_LABELS,
  type PrototypePresenceMode,
  type PrototypeRole,
  type PrototypeSchoolyard,
  type PrototypeTopic,
} from "./prototype-data";

function isRole(value: string | null): value is PrototypeRole {
  return value === "caregiver" || value === "lead";
}

function isPresenceMode(value: string | null): value is PrototypePresenceMode {
  return value === "detailed" || value === "binary";
}

function isSchoolyard(value: string | null): value is PrototypeSchoolyard {
  return value === "enabled" || value === "disabled";
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

function PrototypeMobileHeader() {
  return (
    <header className="border-b border-gray-200 bg-white px-4 py-4 lg:hidden print:hidden">
      <div className="flex items-center gap-3">
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
}: Readonly<{
  topic: PrototypeTopic;
  topicsById: ReadonlyMap<string, PrototypeTopic>;
  hrefFor: (topicId?: string) => string;
}>) {
  const related = topic.related
    .map((id) => topicsById.get(id))
    .filter((item): item is PrototypeTopic => item != null);
  return (
    <section className="mt-8">
      <h2 className="text-lg font-semibold text-gray-950">Das passt dazu</h2>
      <div className="mt-3 grid gap-3 sm:grid-cols-2">
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

function SidebarPrototype({
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

export function HelpPrototype() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const params = useParams<{ topic?: string[] }>();

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

  const topics = useMemo(
    () => getPrototypeTopics(presenceMode, schoolyard),
    [presenceMode, schoolyard],
  );
  const topicId = params.topic?.[0];
  const topic = topics.find((item) => item.id === topicId);

  const hrefFor = useCallback(
    (nextTopicId?: string) => {
      const queryParams = new URLSearchParams(searchParams.toString());
      queryParams.delete("variant");
      queryParams.set("role", role);
      queryParams.set("presence_mode", presenceMode);
      queryParams.set("schoolyard", schoolyard);
      const nextPath = nextTopicId
        ? `/help/prototype/${encodeURIComponent(nextTopicId)}`
        : "/help/prototype";
      return `${nextPath}?${queryParams.toString()}`;
    },
    [presenceMode, role, schoolyard, searchParams],
  );

  useEffect(() => {
    if (!topicId && window.location.hash) {
      const legacyTopicId = window.location.hash.slice(1);
      if (topics.some((item) => item.id === legacyTopicId)) {
        router.replace(hrefFor(legacyTopicId));
        return;
      }
    }
    if (searchParams.has("variant")) {
      router.replace(hrefFor(topicId), { scroll: false });
    }
  }, [hrefFor, router, searchParams, topicId, topics]);

  const invalidTopic = topicId != null && topic == null;

  return (
    <div className="moto-dotted-background moto-dotted-background--fullscreen min-h-screen bg-gray-50 pb-12">
      <PrototypeMobileHeader />
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
      ) : (
        <SidebarPrototype
          topic={topic}
          topics={topics}
          role={role}
          hrefFor={hrefFor}
        />
      )}
    </div>
  );
}
