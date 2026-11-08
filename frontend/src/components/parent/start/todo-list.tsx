"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { ChevronDown, ChevronUp } from "lucide-react";
import { StatusBadge } from "~/components/ui/status-badge";
import { Button } from "~/components/ui/button";
import { ListRowLink } from "~/components/ui/list-row-link";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import type { MotoConcept } from "~/lib/moto-concepts";

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
 * Typ-Icons stehen ohne eigene Kachel direkt vor der Zeile. Der Lesestatus
 * folgt dem gemeinsamen Zustand der Navigationszeile; die Aufgaben liefern
 * selbst keine zusätzliche ungelesene Anzahl.
 */

export interface TodoItem {
  readonly key: string;
  readonly concept: MotoConcept;
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
        className="moto-content-surface rounded-2xl border p-5"
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
      className="moto-content-surface overflow-hidden rounded-2xl border p-5"
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
        <span className="shrink-0">
          <StatusBadge
            tone="blue"
            label={t("todo.openCount", { count: items.length })}
          />
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
          className="mt-2 w-full"
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
  const rowProps = {
    icon: (
      <span className="flex size-7 shrink-0 items-center justify-center">
        <MotoConceptIcon
          concept={item.concept}
          tone="blue"
          size={22}
          aria-hidden="true"
        />
      </span>
    ),
    title: (
      <>
        {item.unread && <span className="sr-only">{t("todo.unread")}: </span>}
        {item.title}
      </>
    ),
    subtitle: item.context,
    badge: item.meta ? (
      <span className="text-end text-xs font-medium whitespace-nowrap text-gray-500 tabular-nums">
        {item.meta.date}, {item.meta.time}
      </span>
    ) : undefined,
    unread: item.unread,
    className: "min-h-16 rounded-xl px-3 py-2.5",
  };

  if (item.href) {
    return <ListRowLink href={item.href} {...rowProps} />;
  }

  return <ListRowLink onClick={item.onSelect} {...rowProps} />;
}
