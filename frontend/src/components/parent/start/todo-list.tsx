"use client";

import { useState } from "react";
import Link from "~/components/ui/navigation-link";
import { useTranslations } from "next-intl";
import { ChevronDown, ChevronRight, ChevronUp } from "lucide-react";
import { Button } from "~/components/ui/button";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import type { MotoConceptKey } from "~/lib/moto-concepts";

const INITIAL_TODO_COUNT = 5;

/**
 * Der Bereich "Zu erledigen" der Startseite.
 *
 * Er zeigt ausschliesslich Dinge mit offener Handlung: ungelesene Nachrichten,
 * ungelesene Aushaenge, offene Umfragen, Termineinladungen ohne Antwort. Ist
 * nichts offen, steht dort ein ruhiger Zustand statt einer leeren Liste, und
 * die Ueberschrift entfaellt mit: eine Ueberschrift ueber nichts ist eine
 * Aufgabe, die keine ist.
 *
 * Typ-Icons stehen ohne eigene Kachel direkt vor der Zeile. Ein blauer Punkt
 * am Icon markiert ungelesene Inhalte. So bleiben Inhaltstyp und Lesestatus
 * getrennt erkennbar, ohne zusaetzliche Flaechen pro Eintrag.
 */

export interface TodoItem {
  readonly key: string;
  readonly concept: MotoConceptKey;
  readonly title: string;
  readonly context: string;
  readonly meta?: Readonly<{
    date: string;
    time: string;
  }>;
  readonly unread?: boolean;
  /** Entweder ein Ziel oder eine Handlung, nie beides. */
  readonly href?: string;
  readonly onSelect?: () => void;
}

export function TodoList({ items }: Readonly<{ items: readonly TodoItem[] }>) {
  const t = useTranslations("parentStart");
  const [expanded, setExpanded] = useState(false);

  if (items.length === 0) {
    return (
      <section
        aria-labelledby="parent-todo-empty-heading"
        className="moto-content-surface rounded-2xl border p-5 shadow-sm backdrop-blur-md"
      >
        <h2
          id="parent-todo-empty-heading"
          className="text-lg font-semibold text-gray-900"
        >
          {t("todo.emptyTitle")}
        </h2>
        <p className="mt-1 text-sm leading-6 text-gray-600">
          {t("todo.emptyDescription")}
        </p>
      </section>
    );
  }

  const hiddenCount = Math.max(0, items.length - INITIAL_TODO_COUNT);
  const visibleItems = expanded ? items : items.slice(0, INITIAL_TODO_COUNT);

  return (
    <section
      aria-labelledby="parent-todo-heading"
      className="moto-content-surface overflow-hidden rounded-2xl border p-5 shadow-sm backdrop-blur-md"
    >
      <div className="flex items-start justify-between gap-4">
        <div className="min-w-0">
          <h2
            id="parent-todo-heading"
            className="text-lg font-semibold text-gray-900"
          >
            {t("todo.title")}
          </h2>
          <p className="mt-1 max-w-2xl text-sm leading-6 text-gray-600">
            {t("todo.description")}
          </p>
        </div>
        <span className="bg-moto-blue/10 text-moto-blue-strong inline-flex shrink-0 rounded-full px-2.5 py-1 text-xs font-semibold whitespace-nowrap tabular-nums">
          {t("todo.openCount", { count: items.length })}
        </span>
      </div>
      <ul id="parent-todo-items" className="mt-4 divide-y divide-gray-100">
        {visibleItems.map((item) => (
          <li key={item.key}>
            <TodoRow item={item} />
          </li>
        ))}
      </ul>
      {hiddenCount > 0 && (
        <Button
          type="button"
          variant="ghost"
          size="md"
          className="mt-2 min-h-11 w-full px-3 text-sm font-medium"
          aria-expanded={expanded}
          aria-controls="parent-todo-items"
          onClick={() => setExpanded((current) => !current)}
        >
          {expanded ? (
            <>
              {t("todo.showLess")}
              <ChevronUp className="size-4" aria-hidden="true" />
            </>
          ) : (
            <>
              {hiddenCount === 1
                ? t("todo.showMoreOne")
                : t("todo.showMoreMany", { count: hiddenCount })}
              <ChevronDown className="size-4" aria-hidden="true" />
            </>
          )}
        </Button>
      )}
    </section>
  );
}

/** Eine Zeile, mindestens 64 px hoch und auf ganzer Breite anklickbar. */
function TodoRow({ item }: Readonly<{ item: TodoItem }>) {
  const t = useTranslations("parentStart");
  const inner = (
    <>
      <span className="relative flex size-7 shrink-0 items-center justify-center">
        <MotoConceptIcon
          concept={item.concept}
          tone="blue"
          size={22}
          aria-hidden="true"
        />
        {item.unread && (
          <span
            data-testid="todo-unread-indicator"
            className="bg-moto-blue absolute -top-0.5 -right-0.5 size-3 rounded-full border-2 border-white"
            aria-hidden="true"
          />
        )}
      </span>
      <span className="min-w-0 flex-1">
        {item.unread && <span className="sr-only">{t("todo.unread")}: </span>}
        <span
          className={`block truncate text-[15px] text-gray-900 ${item.unread ? "font-semibold" : "font-medium"}`}
        >
          {item.title}
        </span>
        <span className="mt-0.5 block truncate text-sm text-gray-500">
          {item.context}
        </span>
      </span>
      {item.meta && (
        <span className="shrink-0 text-end text-xs font-medium whitespace-nowrap text-gray-500 tabular-nums">
          {item.meta.date}, {item.meta.time}
        </span>
      )}
      <ChevronRight
        className="hidden size-[18px] shrink-0 text-gray-400 sm:block"
        aria-hidden="true"
      />
    </>
  );

  const shared =
    "relative flex min-h-16 w-full items-center gap-3 rounded-xl px-3 py-2.5 text-left transition-colors hover:bg-gray-50 active:bg-gray-100 focus-visible:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none focus-visible:-outline-offset-2";

  if (item.href) {
    return (
      <Link href={item.href} className={shared}>
        {inner}
      </Link>
    );
  }

  return (
    <button type="button" onClick={item.onSelect} className={shared}>
      {inner}
    </button>
  );
}
