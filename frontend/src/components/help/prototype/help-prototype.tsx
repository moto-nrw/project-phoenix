"use client";

import Image from "next/image";
import Link from "next/link";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import {
  ArrowLeft,
  ChevronRight,
  CircleHelp,
  ExternalLink,
  Menu,
  MessageCircleQuestion,
  Search,
  X,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { Button, ButtonLink } from "~/components/ui/button";
import {
  Drawer,
  DrawerClose,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
} from "~/components/ui/drawer";
import { EmptyState } from "~/components/ui/empty-state";
import { Input } from "~/components/ui/input";
import { StatusBadge } from "~/components/ui/status-badge";
import { cn } from "~/lib/utils";
import {
  getPrototypeTopics,
  PROTOTYPE_GROUP_LABELS,
  type PrototypeGroupMode,
  type PrototypePresenceMode,
  type PrototypeRole,
  type PrototypeTopic,
  type PrototypeTopicGroup,
} from "./prototype-data";
import { usePrototypeSidebarScrollRestoration } from "./prototype-sidebar-scroll";

const CHATGPT_URL = "https://chatgpt.com/";
const HELP_CONTENT_ID = "prototype-help-content";
const INTERNAL_APP_ORIGIN = "https://moto.invalid";
const MOBILE_HELP_DRAWER_ID = "prototype-mobile-topic-drawer";
type MobileHelpDrawerMode = "topics" | "search";
type OpenMobileHelpDrawer = (
  mode: MobileHelpDrawerMode,
  trigger: HTMLButtonElement,
) => void;
const DOCUMENTATION_SECTIONS = {
  requirements: { id: "voraussetzungen", label: "Das brauchen Sie" },
  instructions: { id: "anleitung", label: "So geht es" },
  result: { id: "danach", label: "Danach" },
  tip: { id: "tipp", label: "Tipp" },
  differences: {
    id: "unterschiede",
    label: "Wenn es anders aussieht",
  },
  troubleshooting: {
    id: "probleme",
    label: "Wenn es nicht klappt",
  },
  example: { id: "beispiel", label: "So sieht es aus" },
  related: { id: "weitere-themen", label: "Weitere Themen" },
} as const;

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

function isGroupMode(value: string | null): value is PrototypeGroupMode {
  return value === "fixed_groups" || value === "open_care";
}

function appHrefFrom(value: string | null): string {
  if (!value) return "/";
  try {
    const target = new URL(value, INTERNAL_APP_ORIGIN);
    if (target.origin !== INTERNAL_APP_ORIGIN) return "/";
    if (target.pathname === "/help" || target.pathname.startsWith("/help/")) {
      return "/";
    }
    return `${target.pathname}${target.search}${target.hash}`;
  } catch {
    return "/";
  }
}

function normalizeSearchText(value: string): string {
  return value
    .toLocaleLowerCase("de-DE")
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .replaceAll("ß", "ss");
}

function topicMatchesSearch(topic: PrototypeTopic, query: string): boolean {
  const searchText = [
    topic.title,
    topic.question,
    topic.summary,
    ...topic.steps,
    ...(topic.instructionGroups?.flatMap((group) => [
      group.title,
      ...group.steps,
    ]) ?? []),
    topic.note,
  ]
    .filter((value): value is string => value != null)
    .join(" ");

  return normalizeSearchText(searchText).includes(normalizeSearchText(query));
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

function BackToAppPrototypeLink({
  className,
  href,
  label = "Zurück zur App",
}: Readonly<{ className?: string; href: string; label?: string }>) {
  return (
    <ButtonLink
      href={href}
      variant="surface"
      size="md"
      className={cn("gap-2 whitespace-nowrap", className)}
    >
      <ArrowLeft className="h-4 w-4" aria-hidden="true" />
      {label}
    </ButtonLink>
  );
}

function TopicSearch({
  autoFocus = false,
  id,
  inputRef,
  query,
  resultCount,
  onQueryChange,
}: Readonly<{
  autoFocus?: boolean;
  id: string;
  inputRef?: React.Ref<HTMLInputElement>;
  query: string;
  resultCount: number;
  onQueryChange: (query: string) => void;
}>) {
  const hasQuery = query.trim().length > 0;
  const resultText =
    resultCount === 1 ? "1 Thema gefunden." : `${resultCount} Themen gefunden.`;

  return (
    <div>
      <div className="relative">
        <Search
          className="pointer-events-none absolute start-3 bottom-3 h-4 w-4 text-gray-400"
          aria-hidden="true"
        />
        <Input
          ref={inputRef}
          id={id}
          autoFocus={autoFocus}
          label="Hilfe durchsuchen"
          type="search"
          value={query}
          onChange={(event) => onQueryChange(event.target.value)}
          placeholder="Zum Beispiel: Abwesenheit"
          autoComplete="off"
          controlSize="compact"
          className="!h-11 ps-9 pe-3 text-base lg:!h-10 lg:text-sm"
        />
      </div>
      <p
        role="status"
        className={cn("text-xs text-gray-500", hasQuery ? "mt-1.5" : "sr-only")}
      >
        {hasQuery ? resultText : null}
      </p>
    </div>
  );
}

function PrototypeMobileHeader({
  appHref,
  homeHref,
  drawerMode,
  drawerOpen,
  onOpenDrawer,
}: Readonly<{
  appHref: string;
  homeHref: string;
  drawerMode: MobileHelpDrawerMode;
  drawerOpen: boolean;
  onOpenDrawer: OpenMobileHelpDrawer;
}>) {
  return (
    <header className="sticky top-0 z-40 border-b border-gray-200 bg-white/95 px-3 py-2.5 backdrop-blur-md lg:hidden print:hidden">
      <div className="flex items-center gap-1.5">
        <Button
          type="button"
          variant="ghost"
          size="icon"
          aria-label="Themen öffnen"
          aria-controls={MOBILE_HELP_DRAWER_ID}
          aria-expanded={drawerOpen && drawerMode === "topics"}
          onClick={(event) => onOpenDrawer("topics", event.currentTarget)}
          className="min-h-11 min-w-11 shrink-0"
        >
          <Menu className="h-5 w-5" aria-hidden="true" />
        </Button>
        <Link
          href={homeHref}
          className="focus-visible:ring-moto-blue inline-flex min-h-11 min-w-0 items-center rounded-lg px-1 focus-visible:ring-2 focus-visible:outline-none"
        >
          <span className="truncate text-xl font-bold tracking-tight text-gray-950">
            moto Hilfe
          </span>
        </Link>
        <div className="ml-auto flex shrink-0 items-center gap-1.5">
          <Button
            type="button"
            variant="ghost"
            size="icon"
            aria-label="Hilfe durchsuchen"
            aria-controls={MOBILE_HELP_DRAWER_ID}
            aria-expanded={drawerOpen && drawerMode === "search"}
            onClick={(event) => onOpenDrawer("search", event.currentTarget)}
            className="min-h-11 min-w-11"
          >
            <Search className="h-5 w-5" aria-hidden="true" />
          </Button>
          <BackToAppPrototypeLink
            href={appHref}
            label="Zur App"
            className="min-h-11 shrink-0 px-3"
          />
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
}: Readonly<{
  topic: PrototypeTopic;
  topicsById: ReadonlyMap<string, PrototypeTopic>;
  hrefFor: (topicId?: string) => string;
}>) {
  const related = topic.related
    .map((id) => topicsById.get(id))
    .filter((item): item is PrototypeTopic => item != null);
  return (
    <section
      id={DOCUMENTATION_SECTIONS.related.id}
      className="scroll-mt-8 pt-12"
    >
      <h2 className="text-2xl font-semibold tracking-tight text-balance text-gray-950">
        {DOCUMENTATION_SECTIONS.related.label}
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

function DocumentationArticle({
  topic,
  topicsById,
  hrefFor,
}: Readonly<{
  topic: PrototypeTopic;
  topicsById: ReadonlyMap<string, PrototypeTopic>;
  hrefFor: (topicId?: string) => string;
}>) {
  const isDraft = topic.status === "draft";
  const troubleshootingTopic = topic.troubleshooting
    ? topicsById.get(topic.troubleshooting)
    : undefined;
  const tableOfContents = [
    {
      ...DOCUMENTATION_SECTIONS.requirements,
      show: !isDraft && Boolean(topic.requirements?.length),
    },
    {
      ...DOCUMENTATION_SECTIONS.instructions,
      show:
        !isDraft &&
        (topic.steps.length > 0 || Boolean(topic.instructionGroups?.length)),
    },
    {
      ...DOCUMENTATION_SECTIONS.result,
      show: !isDraft && Boolean(topic.result),
    },
    {
      ...DOCUMENTATION_SECTIONS.tip,
      show: !isDraft && Boolean(topic.note),
    },
    {
      ...DOCUMENTATION_SECTIONS.differences,
      show: !isDraft && Boolean(topic.differences?.length),
    },
    {
      ...DOCUMENTATION_SECTIONS.troubleshooting,
      show:
        !isDraft &&
        (Boolean(troubleshootingTopic) ||
          Boolean(topic.troubleshootingDetails?.length)),
    },
    {
      ...DOCUMENTATION_SECTIONS.example,
      show: !isDraft && Boolean(topic.image),
    },
    {
      ...DOCUMENTATION_SECTIONS.related,
      show: topic.related.length > 0,
    },
  ].filter((item) => item.show);

  return (
    <div className="xl:grid xl:grid-cols-[minmax(0,42rem)_12rem] xl:gap-x-16">
      <nav aria-label="Sie sind hier" className="mb-8 xl:col-span-2">
        <ol className="flex flex-wrap items-center gap-2 text-base">
          <li>
            <Link
              href={hrefFor()}
              className="font-medium text-gray-500 transition-colors hover:text-gray-900"
            >
              Hilfe
            </Link>
          </li>
          <li aria-hidden="true">
            <ChevronRight className="h-4 w-4 text-gray-400" />
          </li>
          <li className="font-medium text-gray-500">
            {PROTOTYPE_GROUP_LABELS[topic.group]}
          </li>
          <li aria-hidden="true">
            <ChevronRight className="h-4 w-4 text-gray-400" />
          </li>
          <li aria-current="page" className="font-medium text-gray-900">
            {topic.title}
          </li>
        </ol>
      </nav>

      <article className="max-w-2xl min-w-0">
        <header>
          <p className="text-moto-green-strong text-sm font-bold tracking-wide uppercase">
            {PROTOTYPE_GROUP_LABELS[topic.group]}
          </p>
          <h1 className="mt-3 text-4xl font-semibold tracking-tight text-balance text-gray-950 sm:text-5xl">
            {topic.question}
          </h1>
          <p className="mt-5 text-lg leading-8 text-pretty text-gray-600">
            <InlineCode text={topic.summary} />
          </p>
        </header>

        {isDraft ? (
          <section className="pt-12">
            <Alert
              type="info"
              message="Dieser Artikel ist noch nicht ausgearbeitet. Der Eintrag zeigt bereits die geplante Struktur der Hilfe."
              announce="off"
            />
          </section>
        ) : (
          <>
            {topic.requirements && topic.requirements.length > 0 && (
              <section
                id={DOCUMENTATION_SECTIONS.requirements.id}
                className="scroll-mt-8 pt-14"
              >
                <h2 className="text-2xl font-semibold tracking-tight text-balance text-gray-950">
                  {DOCUMENTATION_SECTIONS.requirements.label}
                </h2>
                <ul className="marker:text-moto-green-strong mt-6 list-disc space-y-3 pl-6">
                  {topic.requirements.map((requirement) => (
                    <li
                      key={requirement}
                      className="pl-2 text-base leading-7 text-gray-700"
                    >
                      <InlineCode text={requirement} />
                    </li>
                  ))}
                </ul>
              </section>
            )}

            <section
              id={DOCUMENTATION_SECTIONS.instructions.id}
              className="scroll-mt-8 pt-14"
            >
              <h2 className="text-2xl font-semibold tracking-tight text-balance text-gray-950">
                {DOCUMENTATION_SECTIONS.instructions.label}
              </h2>
              {topic.instructionGroups && topic.instructionGroups.length > 0 ? (
                <div className="mt-7 space-y-10">
                  {topic.instructionGroups.map((group) => {
                    const items = group.steps.map((step, stepIndex) => (
                      <li
                        key={`${stepIndex}-${step}`}
                        className="pl-2 text-base leading-7 text-gray-700"
                      >
                        <InlineCode text={step} />
                      </li>
                    ));

                    return (
                      <section key={group.title}>
                        <h3 className="text-lg font-semibold text-gray-950">
                          {group.title}
                        </h3>
                        {group.description ? (
                          <p className="mt-2 text-base leading-7 text-gray-600">
                            <InlineCode text={group.description} />
                          </p>
                        ) : null}
                        {group.ordered === false ? (
                          <ul className="marker:text-moto-green-strong mt-4 list-disc space-y-4 pl-6">
                            {items}
                          </ul>
                        ) : (
                          <ol className="marker:text-moto-green-strong mt-4 list-decimal space-y-4 pl-6 marker:font-semibold">
                            {items}
                          </ol>
                        )}
                      </section>
                    );
                  })}
                </div>
              ) : (
                <ol className="marker:text-moto-green-strong mt-6 list-decimal space-y-4 pl-6 marker:font-semibold">
                  {topic.steps.map((step, stepIndex) => (
                    <li
                      key={`${stepIndex}-${step}`}
                      className="pl-2 text-base leading-7 text-gray-700"
                    >
                      <InlineCode text={step} />
                    </li>
                  ))}
                </ol>
              )}
            </section>

            {topic.result && (
              <section
                id={DOCUMENTATION_SECTIONS.result.id}
                className="scroll-mt-8 pt-12"
              >
                <h2 className="text-2xl font-semibold tracking-tight text-gray-950">
                  {DOCUMENTATION_SECTIONS.result.label}
                </h2>
                <p className="mt-5 text-base leading-7 text-gray-700">
                  <InlineCode text={topic.result} />
                </p>
              </section>
            )}
          </>
        )}

        {!isDraft && topic.note && (
          <section
            id={DOCUMENTATION_SECTIONS.tip.id}
            className="scroll-mt-8 pt-12"
          >
            <h2 className="mb-5 text-2xl font-semibold tracking-tight text-gray-950">
              {DOCUMENTATION_SECTIONS.tip.label}
            </h2>
            <Alert
              type="info"
              message={topic.note.replaceAll("`", "")}
              announce="off"
            />
          </section>
        )}

        {!isDraft && topic.differences && topic.differences.length > 0 && (
          <section
            id={DOCUMENTATION_SECTIONS.differences.id}
            className="scroll-mt-8 pt-12"
          >
            <h2 className="text-2xl font-semibold tracking-tight text-gray-950">
              {DOCUMENTATION_SECTIONS.differences.label}
            </h2>
            <ul className="marker:text-moto-green-strong mt-5 list-disc space-y-3 pl-6">
              {topic.differences.map((difference) => (
                <li
                  key={difference}
                  className="pl-2 text-base leading-7 text-gray-700"
                >
                  <InlineCode text={difference} />
                </li>
              ))}
            </ul>
          </section>
        )}

        {!isDraft &&
          (troubleshootingTopic || topic.troubleshootingDetails?.length) && (
            <section
              id={DOCUMENTATION_SECTIONS.troubleshooting.id}
              className="scroll-mt-8 pt-12"
            >
              <h2 className="text-2xl font-semibold tracking-tight text-gray-950">
                {DOCUMENTATION_SECTIONS.troubleshooting.label}
              </h2>
              {topic.troubleshootingDetails &&
                topic.troubleshootingDetails.length > 0 && (
                  <ul className="marker:text-moto-green-strong mt-5 list-disc space-y-3 pl-6">
                    {topic.troubleshootingDetails.map((detail) => (
                      <li
                        key={detail}
                        className="pl-2 text-base leading-7 text-gray-700"
                      >
                        <InlineCode text={detail} />
                      </li>
                    ))}
                  </ul>
                )}
              {troubleshootingTopic && (
                <Link
                  href={hrefFor(troubleshootingTopic.id)}
                  className="moto-content-surface group mt-5 flex items-center justify-between rounded-2xl border p-4 shadow-sm focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                >
                  <span className="font-semibold text-gray-900">
                    {troubleshootingTopic.title}
                  </span>
                  <ChevronRight
                    className="h-4 w-4 shrink-0 text-gray-400 group-hover:text-gray-700"
                    aria-hidden="true"
                  />
                </Link>
              )}
            </section>
          )}

        {!isDraft && topic.image && (
          <section
            id={DOCUMENTATION_SECTIONS.example.id}
            className="scroll-mt-8 pt-12"
          >
            <h2 className="text-2xl font-semibold tracking-tight text-balance text-gray-950">
              {DOCUMENTATION_SECTIONS.example.label}
            </h2>
            <div className="mt-6">
              <TopicImage topic={topic} />
            </div>
          </section>
        )}

        {topic.related.length > 0 && (
          <RelatedTopics
            topic={topic}
            topicsById={topicsById}
            hrefFor={hrefFor}
          />
        )}
      </article>

      <aside className="hidden pt-8 xl:block" aria-label="Auf dieser Seite">
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

function PrototypeMobileDrawer({
  chatGptHref,
  drawerMode,
  drawerOpen,
  filteredTopicCount,
  onCloseAutoFocus,
  onDesktopClose,
  onDrawerOpenChange,
  onSearchQueryChange,
  searchQuery,
  topicNavigation,
}: Readonly<{
  chatGptHref: string;
  drawerMode: MobileHelpDrawerMode;
  drawerOpen: boolean;
  filteredTopicCount: number;
  onCloseAutoFocus: (event: Event) => void;
  onDesktopClose: () => void;
  onDrawerOpenChange: (open: boolean) => void;
  onSearchQueryChange: (query: string) => void;
  searchQuery: string;
  topicNavigation: React.ReactNode;
}>) {
  const closeButtonRef = useRef<HTMLButtonElement | null>(null);

  useEffect(() => {
    if (!drawerOpen || drawerMode !== "topics") return;
    const frame = requestAnimationFrame(() => closeButtonRef.current?.focus());
    return () => cancelAnimationFrame(frame);
  }, [drawerMode, drawerOpen]);

  useEffect(() => {
    const desktopQuery = window.matchMedia("(min-width: 64rem)");
    const closeOnDesktop = () => {
      if (desktopQuery.matches && drawerOpen) onDesktopClose();
    };
    closeOnDesktop();
    desktopQuery.addEventListener("change", closeOnDesktop);
    return () => desktopQuery.removeEventListener("change", closeOnDesktop);
  }, [drawerOpen, onDesktopClose]);

  return (
    <Drawer
      open={drawerOpen}
      onOpenChange={onDrawerOpenChange}
      direction="left"
      shouldScaleBackground={false}
    >
      <DrawerContent
        id={MOBILE_HELP_DRAWER_ID}
        onCloseAutoFocus={onCloseAutoFocus}
        className="w-[calc(100%-2rem)] max-w-sm bg-white lg:hidden"
      >
        <DrawerHeader className="flex flex-row items-start justify-between gap-4 border-b border-gray-200 px-5 py-4 text-left">
          <div>
            <DrawerTitle>
              {drawerMode === "search" ? "Hilfe durchsuchen" : "Hilfethemen"}
            </DrawerTitle>
            <DrawerDescription className="mt-1">
              {drawerMode === "search"
                ? "Suchen Sie nach einer Aufgabe."
                : "Wählen Sie ein Thema."}
            </DrawerDescription>
          </div>
          <DrawerClose asChild>
            <Button
              ref={closeButtonRef}
              type="button"
              variant="ghost"
              size="icon"
              aria-label="Schließen"
              className="min-h-11 min-w-11 shrink-0"
            >
              <X className="h-5 w-5" aria-hidden="true" />
            </Button>
          </DrawerClose>
        </DrawerHeader>

        <div className="min-h-0 flex-1 overflow-y-auto overscroll-contain px-5 py-5 pb-[calc(1.25rem+env(safe-area-inset-bottom))]">
          <div className="space-y-2 border-b border-gray-200 pb-5">
            <TopicSearch
              id="prototype-topic-search-mobile"
              autoFocus={drawerMode === "search"}
              query={searchQuery}
              resultCount={filteredTopicCount}
              onQueryChange={onSearchQueryChange}
            />
            <ChatGptPrototypeLink
              href={chatGptHref}
              className="min-h-11 w-full"
            />
          </div>
          <div className="mt-6 pb-4">{topicNavigation}</div>
        </div>
      </DrawerContent>
    </Drawer>
  );
}

function PrototypeDocumentationShell({
  topic,
  topics,
  filteredTopics,
  invalidTopic,
  role,
  hrefFor,
  appHref,
  chatGptHref,
  searchQuery,
  onSearchQueryChange,
}: Readonly<{
  topic?: PrototypeTopic;
  topics: readonly PrototypeTopic[];
  filteredTopics: readonly PrototypeTopic[];
  invalidTopic: boolean;
  role: PrototypeRole;
  hrefFor: (topicId?: string) => string;
  appHref: string;
  chatGptHref: string;
  searchQuery: string;
  onSearchQueryChange: (query: string) => void;
}>) {
  const topicsById = new Map(topics.map((item) => [item.id, item]));
  const [mobileDrawerOpen, setMobileDrawerOpen] = useState(false);
  const [mobileDrawerMode, setMobileDrawerMode] =
    useState<MobileHelpDrawerMode>("topics");
  const mobileDrawerTriggerRef = useRef<HTMLButtonElement | null>(null);
  const desktopSearchInputRef = useRef<HTMLInputElement | null>(null);
  const mainContentRef = useRef<HTMLElement | null>(null);
  const focusContentAfterTopicNavigationRef = useRef(false);
  const desktopTopicScrollAreaRef = usePrototypeSidebarScrollRestoration();

  const handleMobileDrawerOpenChange = useCallback((open: boolean) => {
    setMobileDrawerOpen(open);
    if (!open) {
      requestAnimationFrame(() => mobileDrawerTriggerRef.current?.focus());
    }
  }, []);

  const handleMobileDrawerDesktopClose = useCallback(() => {
    setMobileDrawerOpen(false);
    requestAnimationFrame(() => desktopSearchInputRef.current?.focus());
  }, []);

  const openMobileDrawer = (
    mode: MobileHelpDrawerMode,
    trigger: HTMLButtonElement,
  ) => {
    mobileDrawerTriggerRef.current = trigger;
    setMobileDrawerMode(mode);
    setMobileDrawerOpen(true);
  };

  const handleTopicClick = (nextTopicId: string) => {
    focusContentAfterTopicNavigationRef.current =
      mobileDrawerOpen && nextTopicId !== topic?.id;
    onSearchQueryChange("");
    handleMobileDrawerOpenChange(false);
  };

  const handleMobileDrawerCloseAutoFocus = useCallback((event: Event) => {
    if (!focusContentAfterTopicNavigationRef.current) return;
    event.preventDefault();
    focusContentAfterTopicNavigationRef.current = false;
    requestAnimationFrame(() =>
      document.getElementById(HELP_CONTENT_ID)?.focus(),
    );
  }, []);

  const groupOrder =
    role === "lead"
      ? (["leitung"] satisfies readonly PrototypeTopicGroup[])
      : ([
          "einstieg",
          "tagesplanung",
          "kinder",
          "gruppen",
          "team",
          "arbeitszeit",
          "nfc",
          "probleme",
        ] satisfies readonly PrototypeTopicGroup[]);
  const topicNavigation = (
    <nav aria-label="Themen im Prototyp" className="space-y-7">
      {filteredTopics.length === 0 && (
        <EmptyState
          variant="compact"
          title="Kein passendes Thema."
          className="px-3"
          action={
            <Button
              type="button"
              variant="ghost"
              size="compact"
              onClick={() => onSearchQueryChange("")}
              className="min-h-11 px-3 text-sm font-semibold underline decoration-gray-300 underline-offset-4 lg:min-h-10"
            >
              Suche löschen
            </Button>
          }
        />
      )}
      {groupOrder.map((group) => {
        const groupTopics = filteredTopics.filter(
          (item) => item.group === group,
        );
        if (groupTopics.length === 0) return null;
        return (
          <div key={group}>
            <p className="mb-2 px-3 text-sm font-bold tracking-tight text-gray-950">
              {PROTOTYPE_GROUP_LABELS[group]}
            </p>
            <div className="space-y-1">
              {groupTopics.map((item) => (
                <Link
                  key={item.id}
                  href={hrefFor(item.id)}
                  onClick={() => handleTopicClick(item.id)}
                  aria-current={item.id === topic?.id ? "page" : undefined}
                  className={cn(
                    // Schriftgewicht bleibt in jedem Zustand gleich: ein Wechsel
                    // auf font-semibold verbreitert die Glyphen, wodurch
                    // mehrzeilige Titel beim Aktivieren neu umbrechen.
                    "flex min-h-11 items-center rounded-lg px-3 py-1.5 text-sm leading-6 font-normal focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none lg:block lg:min-h-0",
                    item.id === topic?.id
                      ? "bg-moto-green-soft text-moto-green-strong"
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
      <ButtonLink
        href={`#${HELP_CONTENT_ID}`}
        variant="primary"
        size="compact"
        onClick={() =>
          requestAnimationFrame(() => mainContentRef.current?.focus())
        }
        className="fixed start-3 top-3 z-[60] -translate-y-20 focus-visible:translate-y-0"
      >
        Zum Inhalt
      </ButtonLink>
      <PrototypeMobileHeader
        appHref={appHref}
        homeHref={hrefFor()}
        drawerMode={mobileDrawerMode}
        drawerOpen={mobileDrawerOpen}
        onOpenDrawer={openMobileDrawer}
      />

      <aside className="fixed inset-y-0 left-0 z-30 hidden w-72 flex-col border-r border-gray-200 bg-white lg:flex">
        <div className="flex h-20 shrink-0 items-center gap-3 border-b border-gray-200 px-6">
          <Link
            href={hrefFor()}
            className="focus-visible:ring-moto-blue rounded-lg focus-visible:ring-2 focus-visible:outline-none"
          >
            <span className="text-xl font-bold tracking-tight text-gray-950">
              moto Hilfe
            </span>
          </Link>
          <StatusBadge label="Prototyp" tone="orange" showDot={false} />
        </div>
        <div className="space-y-2 border-b border-gray-200 px-5 py-4">
          <TopicSearch
            id="prototype-topic-search-desktop"
            inputRef={desktopSearchInputRef}
            query={searchQuery}
            resultCount={filteredTopics.length}
            onQueryChange={onSearchQueryChange}
          />
          <ChatGptPrototypeLink href={chatGptHref} className="w-full" />
        </div>
        <div
          ref={desktopTopicScrollAreaRef}
          className="min-h-0 flex-1 overflow-y-auto px-5 py-6"
        >
          {topicNavigation}
        </div>
        <div className="shrink-0 border-t border-gray-200 px-5 py-4">
          <BackToAppPrototypeLink href={appHref} className="w-full" />
        </div>
      </aside>

      <main
        ref={mainContentRef}
        id={HELP_CONTENT_ID}
        tabIndex={-1}
        className={cn(
          "mx-auto w-full scroll-mt-20 px-4 py-8 sm:px-6 lg:scroll-mt-8 lg:px-12",
          topic ? "max-w-6xl" : "max-w-5xl",
        )}
      >
        {invalidTopic ? (
          <section className="mx-auto max-w-2xl py-8 text-center sm:py-16">
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
            <ButtonLink
              href={hrefFor()}
              variant="primary"
              size="md"
              className="mt-6"
            >
              Zur Übersicht
            </ButtonLink>
          </section>
        ) : topic ? (
          <DocumentationArticle
            topic={topic}
            topicsById={topicsById}
            hrefFor={hrefFor}
          />
        ) : (
          <section className="max-w-4xl">
            <p className="text-moto-green-strong text-sm font-bold tracking-wide uppercase">
              {role === "lead" ? "Für die Leitung" : "Für Betreuungskräfte"}
            </p>
            <h1 className="mt-2 text-4xl font-semibold tracking-tight text-gray-950 sm:text-5xl">
              Was möchten Sie erledigen?
            </h1>
            <p className="mt-4 max-w-2xl text-lg leading-8 text-gray-600">
              Wählen Sie ein Thema. Jede Seite erklärt eine Aufgabe.
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

      <PrototypeMobileDrawer
        chatGptHref={chatGptHref}
        drawerMode={mobileDrawerMode}
        drawerOpen={mobileDrawerOpen}
        filteredTopicCount={filteredTopics.length}
        onCloseAutoFocus={handleMobileDrawerCloseAutoFocus}
        onDesktopClose={handleMobileDrawerDesktopClose}
        onDrawerOpenChange={handleMobileDrawerOpenChange}
        onSearchQueryChange={onSearchQueryChange}
        searchQuery={searchQuery}
        topicNavigation={topicNavigation}
      />
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
    : "unknown";
  const rawGroupMode = searchParams.get("group_mode");
  const groupMode: PrototypeGroupMode = isGroupMode(rawGroupMode)
    ? rawGroupMode
    : "unknown";
  const rawNfcEnabled = searchParams.get("nfc_enabled");
  const nfcEnabled =
    rawNfcEnabled === "true" ? true : rawNfcEnabled === "false" ? false : null;
  const appHref = appHrefFrom(searchParams.get("return_to"));

  const topics = useMemo(
    () => getPrototypeTopics(presenceMode, groupMode, nfcEnabled),
    [groupMode, nfcEnabled, presenceMode],
  );
  const roleTopics = useMemo(
    () =>
      topics.filter(
        (item) => item.audience === "all" || item.audience === role,
      ),
    [role, topics],
  );
  const topicId = params.topic?.[0];
  const topic = topics.find((item) => item.id === topicId);
  const [searchQuery, setSearchQuery] = useState("");
  const filteredTopics = useMemo(() => {
    const query = searchQuery.trim();
    return query
      ? roleTopics.filter((item) => topicMatchesSearch(item, query))
      : roleTopics;
  }, [roleTopics, searchQuery]);

  const hrefFor = useCallback(
    (nextTopicId?: string) => {
      const queryParams = new URLSearchParams(searchParams.toString());
      queryParams.delete("variant");
      queryParams.delete("article_style");
      queryParams.delete("schoolyard");
      queryParams.set("role", role);
      if (nfcEnabled === null) queryParams.delete("nfc_enabled");
      else queryParams.set("nfc_enabled", String(nfcEnabled));
      if (presenceMode === "unknown") queryParams.delete("presence_mode");
      else queryParams.set("presence_mode", presenceMode);
      if (groupMode === "unknown") queryParams.delete("group_mode");
      else queryParams.set("group_mode", groupMode);
      const nextPath = nextTopicId
        ? `/help/prototype/${encodeURIComponent(nextTopicId)}`
        : "/help/prototype";
      return `${nextPath}?${queryParams.toString()}`;
    },
    [groupMode, nfcEnabled, presenceMode, role, searchParams],
  );

  const currentHelpPath = hrefFor(topicId);
  const [documentationUrl, setDocumentationUrl] = useState(currentHelpPath);

  useEffect(() => {
    if (searchParams.has("schoolyard")) {
      router.replace(currentHelpPath, { scroll: false });
    }
  }, [currentHelpPath, router, searchParams]);

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
    if (searchParams.has("variant") || searchParams.has("article_style")) {
      router.replace(hrefFor(topicId), { scroll: false });
    }
  }, [hrefFor, router, searchParams, topicId, topics]);

  const invalidTopic = topicId != null && topic == null;

  return (
    <div
      className={cn(
        "min-h-screen pb-12",
        topic
          ? "bg-white"
          : "moto-dotted-background moto-dotted-background--fullscreen bg-gray-50",
      )}
    >
      <PrototypeDocumentationShell
        topic={topic}
        topics={roleTopics}
        filteredTopics={filteredTopics}
        invalidTopic={invalidTopic}
        role={role}
        hrefFor={hrefFor}
        appHref={appHref}
        chatGptHref={chatGptHref}
        searchQuery={searchQuery}
        onSearchQueryChange={setSearchQuery}
      />
    </div>
  );
}
