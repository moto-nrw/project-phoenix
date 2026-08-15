"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import {
  CaretRight,
  CheckCircle,
  type Icon,
} from "~/components/parent/shell/parent-icons";

/**
 * Der Bereich "Zu erledigen" der Startseite.
 *
 * Er zeigt ausschliesslich Dinge mit offener Handlung: ungelesene Nachrichten,
 * ungelesene Aushaenge, offene Umfragen, Termineinladungen ohne Antwort. Ist
 * nichts offen, steht dort ein ruhiger Zustand statt einer leeren Liste, und
 * die Ueberschrift entfaellt mit: eine Ueberschrift ueber nichts ist eine
 * Aufgabe, die keine ist.
 *
 * Die Flaeche traegt die feine Punkt-Textur der Website als Flaechenmerkmal.
 * Farbe steckt nur im Icon-Feld, nicht in der Flaeche.
 */

type TodoTone = "blue" | "orange";

export interface TodoItem {
  readonly key: string;
  readonly tone: TodoTone;
  readonly icon: Icon;
  readonly title: string;
  readonly context: string;
  /** Entweder ein Ziel oder eine Handlung, nie beides. */
  readonly href?: string;
  readonly onSelect?: () => void;
}

const TONE_FIELD: Record<TodoTone, string> = {
  blue: "bg-moto-blue-soft text-moto-blue-strong",
  orange: "bg-moto-orange-soft text-moto-orange-strong",
};

export function TodoList({ items }: Readonly<{ items: readonly TodoItem[] }>) {
  const t = useTranslations("parentStart");

  if (items.length === 0) {
    return (
      <section className="moto-dot-texture--soft rounded-2xl border border-gray-200 bg-white p-6 text-center shadow-sm">
        <span className="bg-moto-green-soft text-moto-green-strong mx-auto flex size-12 items-center justify-center rounded-full">
          <CheckCircle size={28} weight="fill" aria-hidden="true" />
        </span>
        <p className="mt-3 text-[20px] font-semibold text-gray-900">
          {t("todo.emptyTitle")}
        </p>
        <p className="mt-1 text-[15px] text-gray-600">
          {t("todo.emptyDescription")}
        </p>
      </section>
    );
  }

  return (
    <section aria-labelledby="parent-todo-heading">
      <h2
        id="parent-todo-heading"
        className="mb-2 text-[20px] font-semibold text-gray-900"
      >
        {t("todo.title")}
      </h2>
      <ul className="moto-dot-texture--soft divide-y divide-gray-200 overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm">
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
  const Icon = item.icon;
  const inner = (
    <>
      <span
        className={`flex size-11 shrink-0 items-center justify-center rounded-xl ${TONE_FIELD[item.tone]}`}
      >
        <Icon size={24} aria-hidden="true" />
      </span>
      <span className="min-w-0 flex-1">
        <span className="block truncate text-[17px] font-semibold text-gray-900">
          {item.title}
        </span>
        <span className="mt-0.5 block truncate text-[15px] text-gray-600">
          {item.context}
        </span>
      </span>
      <CaretRight
        size={20}
        className="shrink-0 text-gray-400"
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
