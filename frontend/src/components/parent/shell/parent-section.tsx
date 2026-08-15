"use client";

import type React from "react";

/**
 * Ein Abschnitt der Eltern-App: Ueberschrift 20/600 ueber einer weissen
 * Flaeche.
 *
 * Bewusst ohne Icon-Kachel, ohne Kicker und ohne farbige Flaeche. Die
 * Gliederung entsteht aus Groesse und Abstand, nicht aus Dekoration; Farbe
 * bleibt den Zustaenden vorbehalten. Ersetzt in der Eltern-App die
 * SectionCard-Huelle mit Versalien-Kicker.
 */
export function ParentSection({
  title,
  description,
  actions,
  children,
  bare = false,
}: Readonly<{
  title: string;
  description?: string;
  actions?: React.ReactNode;
  children: React.ReactNode;
  /** Ohne eigene Kartenflaeche, wenn der Inhalt selbst schon Karten sind. */
  bare?: boolean;
}>) {
  return (
    <section className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="min-w-0">
          <h2 className="text-[20px] leading-tight font-semibold text-gray-900">
            {title}
          </h2>
          {description && (
            <p className="mt-1 text-[15px] text-gray-600">{description}</p>
          )}
        </div>
        {actions}
      </div>
      {bare ? (
        children
      ) : (
        <div className="space-y-4 rounded-2xl border border-gray-200 bg-white p-4 shadow-sm sm:p-5">
          {children}
        </div>
      )}
    </section>
  );
}
