"use client";

import { useCallback, useEffect, useState } from "react";
import { Users } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import { type Child, listMyChildren } from "~/lib/parent-api";
import { createLogger } from "~/lib/logger";
import { formatLocalizedDate } from "~/lib/localized-date-format";
import { EmptyState } from "~/components/ui/empty-state";
import { ChildRow, type ChildRowItem } from "~/components/parent/child-row";
import {
  ParentLoadError,
  ParentPage,
  ParentPageHeader,
  ParentPageSkeleton,
} from "~/components/parent/parent-page";

const logger = createLogger({ component: "ParentChildrenPage" });

function formatDate(
  iso: string | undefined,
  locale: string,
  empty: string,
): string {
  if (!iso) return empty;
  return formatLocalizedDate(iso, locale);
}

function formatServiceRange(
  child: Child,
  locale: string,
  empty: string,
  connector: string,
): string {
  if (!child.enrolled_from && !child.enrolled_until) return empty;
  return `${formatDate(child.enrolled_from, locale, empty)} ${connector} ${formatDate(child.enrolled_until, locale, empty)}`;
}

export function ParentChildrenPage() {
  const t = useTranslations("parentChildren");
  const locale = useLocale();
  const [children, setChildren] = useState<Child[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setChildren(await listMyChildren());
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unknown error";
      logger.warn("parent_children_load_failed", { error: message });
      setError(message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  if (loading) {
    return <ParentPageSkeleton rows={1} />;
  }

  if (error) {
    return <ParentLoadError message={t("loadError")} />;
  }

  const items: ChildRowItem[] = children.map((child) => ({
    key: child.student_id,
    name: `${child.first_name} ${child.last_name}`,
    schoolName: child.school_name,
    detail: `${child.school_class ? `${child.school_class} · ` : ""}${t(
      "careRange",
      {
        range: formatServiceRange(
          child,
          locale,
          t("openRange"),
          t("dateRangeConnector"),
        ),
      },
    )}`,
    href: `/parents/children/${child.student_id}`,
  }));

  return (
    <ParentPage>
      <ParentPageHeader
        kicker={t("eyebrow")}
        title={t("title")}
        description={t("description")}
      />

      {items.length === 0 ? (
        <div className="moto-content-surface rounded-2xl border p-5 shadow-sm backdrop-blur-md">
          <EmptyState
            icon={<Users className="h-8 w-8" aria-hidden="true" />}
            title={t("emptyTitle")}
            description={t("emptyDescription")}
            className="py-8"
          />
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
          {items.map((item) => (
            <ChildRow key={item.key} item={item} variant="card" />
          ))}
        </div>
      )}
    </ParentPage>
  );
}
