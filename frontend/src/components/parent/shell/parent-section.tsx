"use client";

import type React from "react";
import { SectionCard } from "~/components/ui/section-card";
import { ConceptIconTile } from "~/components/ui/concept-icon-tile";
import type { MotoConceptKey } from "~/lib/moto-concepts";

/** Einheitliche Abschnittskarte fuer die Eltern-App. */
export function ParentSection({
  title,
  description,
  actions,
  children,
  bare = false,
  level = 2,
  concept,
  prominent = false,
}: Readonly<{
  title: string;
  description?: string;
  actions?: React.ReactNode;
  children?: React.ReactNode;
  /** Ohne eigene Kartenflaeche, wenn der Inhalt selbst schon Karten sind. */
  bare?: boolean;
  /** 3 fuer Bloecke, die unter einem anderen Abschnitt haengen. */
  level?: 2 | 3;
  concept?: MotoConceptKey;
  prominent?: boolean;
}>) {
  if (!bare) {
    return (
      <SectionCard
        title={title}
        description={description}
        leading={
          concept ? (
            <ConceptIconTile concept={concept} variant="section" />
          ) : undefined
        }
        actions={
          actions ? (
            <div className="w-full sm:w-auto [&>*]:w-full sm:[&>*]:w-auto">
              {actions}
            </div>
          ) : undefined
        }
        headingLevel={level}
        titleClassName={
          prominent
            ? "flex min-h-9 items-center text-xl leading-tight tracking-tight sm:min-h-10"
            : undefined
        }
        bodyClassName="mt-5 space-y-5"
      >
        {children}
      </SectionCard>
    );
  }

  const Heading = level === 3 ? "h3" : "h2";
  return (
    <section className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="min-w-0">
          <Heading className="text-base font-semibold text-gray-900">
            {title}
          </Heading>
          {description && (
            <p className="mt-1 text-sm leading-6 text-gray-600">
              {description}
            </p>
          )}
        </div>
        {actions}
      </div>
      {children}
    </section>
  );
}

export function ParentSubsection({
  title,
  description,
  actions,
  children,
}: Readonly<{
  title: string;
  description?: string;
  actions?: React.ReactNode;
  children?: React.ReactNode;
}>) {
  return (
    <section className="rounded-xl bg-gray-50 p-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0">
          <h3 className="text-sm font-semibold text-gray-900">{title}</h3>
          {description ? (
            <p className="mt-1 max-w-2xl text-sm leading-6 text-gray-600">
              {description}
            </p>
          ) : null}
        </div>
        {actions ? (
          <div className="w-full shrink-0 sm:w-auto [&>*]:w-full sm:[&>*]:w-auto">
            {actions}
          </div>
        ) : null}
      </div>
      {children ? <div className="mt-4 space-y-4">{children}</div> : null}
    </section>
  );
}
