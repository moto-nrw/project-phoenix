"use client";

import type React from "react";
import { useTranslations } from "next-intl";
import { MobilePageHelpTarget } from "~/components/ui/mobile-page-help-target";
import { BackLink } from "~/components/ui/back-link";
import {
  PageBody,
  type PageBodyEmpty,
  type PageBodyError,
} from "~/components/ui/page-body";
import {
  CardSkeleton,
  ListSkeleton,
  SkeletonRegion,
  TenantPageHeaderSkeleton,
} from "~/components/ui/page-skeletons";
import { SectionCard } from "~/components/ui/section-card";

/**
 * Layout primitives shared by every parents-portal page. The portal keeps its
 * own scaffold (ADR 0149, Entscheidung 1): head with a kicker, `text-xl` page
 * title, `text-base` section titles, gray-900 primary actions. Loading, empty
 * and error follow the `TenantPage` contract through the same `PageBody`.
 */

/**
 * Page container: one column, one rhythm, full width.
 *
 * No `max-w-*` cap — the parents portal fills the shell's content area like
 * every tenant page does (`AppShell` already supplies the page padding). Long
 * prose stays readable because the text blocks inside carry their own
 * `max-w-2xl`, not because the whole page is narrowed.
 *
 * `header` stays in every state; `loading`, `error` and `empty` replace the
 * content, as in `TenantPage`.
 */
export function ParentPage({
  header,
  loading = false,
  loadingLabel,
  error,
  empty,
  overlays,
  children,
  className = "",
}: Readonly<{
  /** Der Seitenkopf, meist `ParentPageHeader`. */
  header?: React.ReactNode;
  /** `true`: Kartenskelett; ein Knoten: das Skelett der Fläche aus dem Kit. */
  loading?: boolean | React.ReactNode;
  /** Vorlesbare Ankündigung; ohne Angabe „Inhalt wird geladen“. */
  loadingLabel?: string;
  error?: PageBodyError;
  empty?: PageBodyEmpty;
  /** Dialoge der Seite; sie bleiben in jedem Zustand gemountet. */
  overlays?: React.ReactNode;
  children?: React.ReactNode;
  className?: string;
}>) {
  const hasState = Boolean(loading) || Boolean(error) || Boolean(empty);
  return (
    <div className={`w-full space-y-5 ${className}`}>
      {header}
      {hasState ? (
        <ParentPageState
          loading={loading}
          loadingLabel={loadingLabel}
          error={error}
          empty={empty}
        >
          {children}
        </ParentPageState>
      ) : (
        children
      )}
      {overlays}
    </div>
  );
}

function ParentPageState({
  loading,
  loadingLabel,
  error,
  empty,
  children,
}: Readonly<{
  loading: boolean | React.ReactNode;
  loadingLabel?: string;
  error?: PageBodyError;
  empty?: PageBodyEmpty;
  children?: React.ReactNode;
}>) {
  const t = useTranslations("parentNav");
  return (
    <PageBody
      loading={loading}
      loadingLabel={loadingLabel ?? t("sectionLoading")}
      error={error}
      empty={empty}
    >
      {children}
    </PageBody>
  );
}

/**
 * The page identity block. Detail and list pages use the default white surface.
 * `prominent` is the greeting on the start page, whose head keeps its padding
 * on a phone: the epic leaves that page untouched.
 */
export function ParentPageHeader({
  kicker,
  title,
  description,
  actions,
  backHref,
  backLabel,
  media,
  prominent = false,
}: Readonly<{
  kicker?: string;
  title: string;
  description?: string;
  actions?: React.ReactNode;
  backHref?: string;
  backLabel?: string;
  /** Optional leading element, e.g. a child's initials avatar. */
  media?: React.ReactNode;
  /** Gives dashboard greetings one clear step above section headings. */
  prominent?: boolean;
}>) {
  return (
    // `header` bleibt das Element des Seitenkopfs; die Fläche kommt aus dem Kit.
    <header>
      <SectionCard
        overflow="visible"
        className={prominent ? "compact:p-5 max-sm:p-5" : ""}
      >
        <div className="space-y-3">
          {backHref && backLabel && (
            <BackLink href={backHref} label={backLabel} />
          )}
          <div className="flex flex-wrap items-start justify-between gap-3">
            <SectionCard
              bare
              kicker={kicker}
              title={title}
              description={description}
              leading={media}
              headingLevel={1}
              titleSize={prominent ? "display" : "page"}
              className="min-w-0 flex-1 basis-0"
            />
            {actions ? (
              <div className="contents sm:flex sm:shrink-0 sm:items-center sm:gap-2">
                <div className="flex shrink-0 flex-wrap items-center gap-2 max-sm:order-last max-sm:basis-full">
                  {actions}
                </div>
                <MobilePageHelpTarget />
              </div>
            ) : (
              <MobilePageHelpTarget />
            )}
          </div>
        </div>
      </SectionCard>
    </header>
  );
}

/**
 * Whole-page placeholder from the kit, for a boundary without the page's own
 * head (`Suspense`, auth guard, shell). `rows` is the number of cards below the
 * head.
 */
export function ParentPageSkeleton({ rows = 2 }: Readonly<{ rows?: number }>) {
  const t = useTranslations("parentNav");
  return (
    <SkeletonRegion label={t("sectionLoading")}>
      <div className="w-full space-y-5">
        <TenantPageHeaderSkeleton matches="parent-page" />
        {Array.from({ length: rows }, (_, index) => (
          <ParentSectionSkeleton key={index} announce={false} />
        ))}
      </div>
    </SkeletonRegion>
  );
}

/**
 * One section placeholder from the kit: a card with a head, or list rows with
 * `showHeader={false}`. Announces itself unless another status covers the same
 * loading phase (#4994).
 */
export function ParentSectionSkeleton({
  rows = 3,
  showHeader = true,
  className = "",
  announce = true,
}: Readonly<{
  rows?: number;
  showHeader?: boolean;
  className?: string;
  /** False only when another status covers the same loading phase. */
  announce?: boolean;
}>) {
  const t = useTranslations("parentNav");
  const body = showHeader ? (
    <CardSkeleton rows={rows} />
  ) : (
    <ListSkeleton rows={rows} avatar={false} />
  );
  if (announce) {
    return (
      <SkeletonRegion
        label={t("sectionLoading")}
        testId="parent-page-section-skeleton"
        className={className}
      >
        {body}
      </SkeletonRegion>
    );
  }
  return (
    <div
      data-testid="parent-page-section-skeleton"
      className={className}
      aria-hidden="true"
    >
      {body}
    </div>
  );
}
