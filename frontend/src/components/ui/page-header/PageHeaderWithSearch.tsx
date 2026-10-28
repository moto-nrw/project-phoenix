"use client";

import { PageHeader } from "./PageHeader";
import { PageHeaderSearchLayout } from "./PageHeaderSearchLayout";
import type { PageHeaderWithSearchProps } from "./types";

/**
 * List-page header: title row (with concept icon tile) plus the search,
 * filter and tabs rows of PageHeaderSearchLayout.
 */
export function PageHeaderWithSearch(
  props: Readonly<PageHeaderWithSearchProps>,
) {
  const {
    title,
    concept,
    badge,
    statusIndicator,
    actionButton,
    mobileActionButton,
    overflowMenu,
    ...rows
  } = props;
  return (
    <PageHeaderSearchLayout
      {...rows}
      title={title}
      badge={badge}
      statusIndicator={statusIndicator}
      actionButton={actionButton}
      mobileActionButton={mobileActionButton}
      overflowMenu={overflowMenu}
      titleRow={
        title ? (
          <PageHeader
            title={title}
            concept={concept}
            badge={badge}
            statusIndicator={statusIndicator}
            actionButton={mobileActionButton ?? actionButton}
            overflowMenu={overflowMenu}
          />
        ) : null
      }
    />
  );
}
