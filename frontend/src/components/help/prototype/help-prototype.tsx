"use client";

import Image from "next/image";
import Link from "next/link";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import {
  ArrowLeft,
  ChevronRight,
  CircleHelp,
  ExternalLink,
  Info,
  MessageCircleQuestion,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { ButtonLink } from "~/components/ui/button";
import { cn } from "~/lib/utils";
import {
  getPrototypeTopics,
  PROTOTYPE_GROUP_LABELS,
  type PrototypePresenceMode,
  type PrototypeRole,
  type PrototypeSchoolyard,
  type PrototypeTopic,
} from "./prototype-data";

const CHATGPT_URL = "https://chatgpt.com/";

function chatGptUrlFor(
  documentationUrl: string,
  topic?: PrototypeTopic,
): string {
  const prompt = [
    `Ich sehe mir diese moto-Hilfe an: ${documentationUrl}`,
    topic ? `Das aktuelle Thema ist: ${topic.question}` : null,
    "Helfen Sie mir, die Anleitung zu verstehen.",
    "Nutzen Sie die verlinkte Hilfe als Grundlage.",
    "Erklären Sie die Schritte einfach und konkret.",
    topic ? "Beginnen Sie beim aktuellen Thema." : null,
    topic
      ? "Beantworten Sie auch Fragen zu anderen Themen der moto-Hilfe."
      : "Beantworten Sie Fragen zur gesamten moto-Hilfe.",
    "Fragen Sie kurz nach, wenn meine Frage unklar ist.",
  ]
    .filter((line): line is string => line != null)
    .join("\n");
  const chatGptUrl = new URL(CHATGPT_URL);
  chatGptUrl.searchParams.set("prompt", prompt);
  return chatGptUrl.toString();
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

function ChatGptPrototypeLink({
  className,
  href,
}: Readonly<{ className?: string; href: string }>) {
  return (
    <ButtonLink
      href={href}
      target="_blank"
      rel="noreferrer"
      variant="surface"
      size="md"
      className={cn("gap-2 whitespace-nowrap", className)}
    >
      <MessageCircleQuestion className="h-4 w-4" aria-hidden="true" />
      Frag ChatGPT
      <ExternalLink className="h-3.5 w-3.5" aria-hidden="true" />
    </ButtonLink>
  );
}

function PrototypeMobileHeader({
  chatGptHref,
}: Readonly<{ chatGptHref: string }>) {
  return (
    <header className="border-b border-gray-200 bg-white px-4 py-4 lg:hidden print:hidden">
      <div className="flex items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-3">
          <Link
            href="/help/prototype"
            className="focus-visible:ring-moto-blue rounded-lg focus-visible:ring-2 focus-visible:outline-none"
          >
            <span className="text-xl font-bold tracking-tight text-gray-950">
              moto Hilfe
            </span>
          </Link>
          <span className="bg-moto-orange/12 text-moto-orange-strong hidden rounded-full px-2.5 py-1 text-xs font-bold tracking-wide uppercase sm:inline-flex">
            Prototyp
          </span>
        </div>
        <ChatGptPrototypeLink href={chatGptHref} className="shrink-0 px-3" />
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
        priority
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
  sectionId,
  title = "Das passt dazu",
  documentationStyle = false,
}: Readonly<{
  topic: PrototypeTopic;
  topicsById: ReadonlyMap<string, PrototypeTopic>;
  hrefFor: (topicId?: string) => string;
  sectionId?: string;
  title?: string;
  documentationStyle?: boolean;
}>) {
  const related = topic.related
    .map((id) => topicsById.get(id))
    .filter((item): item is PrototypeTopic => item != null);
  return (
    <section
      id={sectionId}
      className={cn("scroll-mt-8", documentationStyle ? "pt-12" : "mt-8")}
    >
      <h2
        className={cn(
          "font-semibold text-gray-950",
          documentationStyle
            ? "text-2xl tracking-tight text-balance"
            : "text-lg",
        )}
      >
        {title}
      </h2>
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

function DocumentationArticlePrototype({
  topic,
  topicsById,
  hrefFor,
}: Readonly<{
  topic: PrototypeTopic;
  topicsById: ReadonlyMap<string, PrototypeTopic>;
  hrefFor: (topicId?: string) => string;
}>) {
  const tableOfContents = [
    { id: "anleitung", label: "So finden Sie ein Kind", show: true },
    { id: "tipp", label: "Tipp", show: Boolean(topic.note) },
    {
      id: "beispiel",
      label: "So sieht die Suche aus",
      show: Boolean(topic.image),
    },
    { id: "weitere-themen", label: "Weitere Themen", show: true },
  ].filter((item) => item.show);

  return (
    <div className="xl:grid xl:grid-cols-[minmax(0,42rem)_12rem] xl:gap-16">
      <article className="max-w-2xl min-w-0">
        <nav aria-label="Brotkrümelnavigation" className="mb-8">
          <ol className="flex flex-wrap items-center gap-2 text-sm text-gray-500">
            <li>
              <Link href={hrefFor()} className="hover:text-gray-950">
                Hilfe
              </Link>
            </li>
            <li aria-hidden="true">/</li>
            <li>{PROTOTYPE_GROUP_LABELS[topic.group]}</li>
            <li aria-hidden="true">/</li>
            <li className="font-medium text-gray-900">{topic.title}</li>
          </ol>
        </nav>

        <header>
          <p className="text-moto-green-strong text-sm font-bold tracking-wide uppercase">
            {PROTOTYPE_GROUP_LABELS[topic.group]}
          </p>
          <h1 className="mt-3 text-4xl font-semibold tracking-tight text-balance text-gray-950 sm:text-5xl">
            {topic.question}
          </h1>
          <p className="mt-5 text-lg leading-8 text-pretty text-gray-600">
            {topic.summary}
          </p>
        </header>

        <section id="anleitung" className="scroll-mt-8 pt-14">
          <h2 className="text-2xl font-semibold tracking-tight text-balance text-gray-950">
            So finden Sie ein Kind
          </h2>
          <ol className="marker:text-moto-green-strong mt-6 list-decimal space-y-4 pl-6 marker:font-semibold">
            {topic.steps.map((step) => (
              <li key={step} className="pl-2 text-base leading-7 text-gray-700">
                <InlineCode text={step} />
              </li>
            ))}
          </ol>
        </section>

        {topic.note && (
          <section id="tipp" className="scroll-mt-8 pt-12">
            <h2 className="mb-5 text-2xl font-semibold tracking-tight text-gray-950">
              Tipp
            </h2>
            <Alert
              type="info"
              title="Gut zu wissen"
              message={topic.note.replaceAll("`", "")}
              announce="off"
            />
          </section>
        )}

        {topic.image && (
          <section id="beispiel" className="scroll-mt-8 pt-12">
            <h2 className="text-2xl font-semibold tracking-tight text-balance text-gray-950">
              So sieht die Suche aus
            </h2>
            <p className="mt-3 text-base leading-7 text-pretty text-gray-600">
              Im Suchfeld und in den Filtern grenzen Sie die Kinderliste ein.
            </p>
            <div className="mt-6">
              <TopicImage topic={topic} />
            </div>
          </section>
        )}

        <RelatedTopics
          topic={topic}
          topicsById={topicsById}
          hrefFor={hrefFor}
          sectionId="weitere-themen"
          title="Weitere Themen"
          documentationStyle
        />
      </article>

      <aside className="hidden xl:block" aria-label="Auf dieser Seite">
        <div className="sticky top-8 border-l border-gray-200 pl-5">
          <p className="text-sm font-semibold text-gray-950">
            Auf dieser Seite
          </p>
          <nav className="mt-4" aria-label="Abschnitte auf dieser Seite">
            <ul className="space-y-3">
              {tableOfContents.map((item) => (
                <li key={item.id}>
                  <a
                    href={`#${item.id}`}
                    className="text-sm leading-5 text-gray-500 hover:text-gray-950"
                  >
                    {item.label}
                  </a>
                </li>
              ))}
            </ul>
          </nav>
        </div>
      </aside>
    </div>
  );
}

function SidebarPrototype({
  topic,
  topics,
  role,
  hrefFor,
  chatGptHref,
  documentationStyle = false,
}: Readonly<{
  topic?: PrototypeTopic;
  topics: readonly PrototypeTopic[];
  role: PrototypeRole;
  hrefFor: (topicId?: string) => string;
  chatGptHref: string;
  documentationStyle?: boolean;
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
        <div className="border-b border-gray-200 px-5 py-4">
          <ChatGptPrototypeLink href={chatGptHref} className="w-full" />
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto px-5 py-6">
          <p className="text-moto-blue-strong text-xs font-bold tracking-wide uppercase">
            Themen
          </p>
          <div className="mt-4">{topicNavigation}</div>
        </div>
      </aside>

      <main
        className={cn(
          "mx-auto w-full px-4 py-8 sm:px-6 lg:px-12",
          documentationStyle ? "max-w-6xl" : "max-w-5xl",
        )}
      >
        <details className="moto-content-surface mb-6 rounded-2xl border p-4 shadow-sm lg:hidden">
          <summary className="cursor-pointer text-sm font-semibold text-gray-900">
            {topic ? `Thema: ${topic.title}` : "Themen öffnen"}
          </summary>
          <div className="mt-4 border-t border-gray-100 pt-4">
            {topicNavigation}
          </div>
        </details>
        {topic && documentationStyle ? (
          <DocumentationArticlePrototype
            topic={topic}
            topicsById={topicsById}
            hrefFor={hrefFor}
          />
        ) : topic ? (
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
  const documentationStyle =
    topic?.id === "kindersuche" && searchParams.get("article_style") === "docs";

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

  const currentHelpPath = hrefFor(topicId);
  const [documentationUrl, setDocumentationUrl] = useState(currentHelpPath);

  useEffect(() => {
    setDocumentationUrl(
      new URL(currentHelpPath, window.location.origin).toString(),
    );
  }, [currentHelpPath]);

  const chatGptHref = useMemo(
    () => chatGptUrlFor(documentationUrl, topic),
    [documentationUrl, topic],
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
    <div
      className={cn(
        "min-h-screen pb-12",
        documentationStyle
          ? "bg-white"
          : "moto-dotted-background moto-dotted-background--fullscreen bg-gray-50",
      )}
    >
      <PrototypeMobileHeader chatGptHref={chatGptHref} />
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
          chatGptHref={chatGptHref}
          documentationStyle={documentationStyle}
        />
      )}
    </div>
  );
}
