"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { ChevronRight } from "lucide-react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import type { MotoConceptKey } from "~/lib/moto-concepts";

/**
 * Der Bereich "Zu erledigen" der Startseite.
 *
 * Er zeigt ausschliesslich Dinge mit offener Handlung: ungelesene Nachrichten,
 * ungelesene Aushaenge, offene Umfragen, Termineinladungen ohne Antwort. Ist
 * nichts offen, steht dort ein ruhiger Zustand statt einer leeren Liste, und
 * die Ueberschrift entfaellt mit: eine Ueberschrift ueber nichts ist eine
 * Aufgabe, die keine ist.
 *
 * Die Flaeche bleibt schlicht weiss. Farbe traegt allein das Duotone-Symbol
 * des Konzepts, das die Zeile meint.
 */

export interface TodoItem {
  readonly key: string;
  readonly concept: MotoConceptKey;
  readonly title: string;
  readonly context: string;
  /** Entweder ein Ziel oder eine Handlung, nie beides. */
  readonly href?: string;
  readonly onSelect?: () => void;
}

export function TodoList({ items }: Readonly<{ items: readonly TodoItem[] }>) {
  const t = useTranslations("parentStart");

  if (items.length === 0) {
    return (
      <section className="rounded-2xl border border-gray-200 bg-white p-6 text-center shadow-sm">
        <span className="mx-auto flex size-14 items-center justify-center rounded-full bg-gray-100">
          <MotoConceptIcon concept="dayReport" size={30} />
        </span>
        <p className="mt-3 text-[22px] font-bold text-gray-900">
          {t("todo.emptyTitle")}
        </p>
        <p className="mt-1 text-[15px] text-gray-500">
          {t("todo.emptyDescription")}
        </p>
      </section>
    );
  }

  return (
    <section aria-labelledby="parent-todo-heading">
      <h2
        id="parent-todo-heading"
        className="mb-2 text-[22px] font-bold text-gray-900"
      >
        {t("todo.title")}
      </h2>
      <ul className="divide-y divide-gray-200 overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm">
        {items.map((item) => (
          <li key={item.key}>
            <TodoRow item={item} />
          </li>
        ))}
      </ul>
    </section>
  );
}

/** Eine Zeile, mindestens 72 px hoch und auf ganzer Breite anklickbar. */
function TodoRow({ item }: Readonly<{ item: TodoItem }>) {
  const inner = (
    <>
      <span className="flex size-11 shrink-0 items-center justify-center rounded-xl bg-gray-100">
        <MotoConceptIcon concept={item.concept} size={24} />
      </span>
      <span className="min-w-0 flex-1">
        <span className="block truncate text-[17px] font-semibold text-gray-900">
          {item.title}
        </span>
        <span className="mt-0.5 block truncate text-[15px] text-gray-500">
          {item.context}
        </span>
      </span>
      <ChevronRight
        className="h-5 w-5 shrink-0 text-gray-400"
        aria-hidden="true"
      />
    </>
  );

  const shared =
    "flex min-h-[72px] w-full items-center gap-3 px-4 py-3 text-left transition-colors hover:bg-gray-50 active:bg-gray-100 focus-visible:ring-2 focus-visible:ring-[#5080D8] focus-visible:outline-none focus-visible:-outline-offset-2";

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
