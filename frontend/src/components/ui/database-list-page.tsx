"use client";

import { type ReactNode } from "react";
import { ResponsiveLayout } from "@/components/dashboard";
import { getAccentSpinner } from "./database/accents";
import { SearchFilter } from "./search-filter";
import { DatabasePageHeader } from "./database-page-header";
import { DatabaseListSection } from "./database-list-section";

export interface DatabaseListPageProps<T = unknown> {
  // Page metadata
  userName: string;

  // Header
  title: string;
  description: string;
  backUrl?: string;

  // List section
  listTitle: string;

  // Search and filters
  searchPlaceholder: string;
  searchValue: string;
  onSearchChange: (value: string) => void;
  filters?: ReactNode;

  // Add button
  addButton?: {
    label: string;
    href?: string;
    onClick?: () => void;
  };

  // Info section
  infoSection?: {
    title: string;
    content: string;
    icon?: ReactNode;
  };

  // Data and state
  items: T[];
  loading: boolean;
  error?: string | null;
  onRetry?: () => void;

  // Empty state customization
  emptyIcon?: ReactNode;
  emptyTitle?: string;
  emptyMessage?: string;

  // Content
  renderItem: (item: T, index: number) => ReactNode;

  // Optional customization
  itemLabel?: {
    singular: string;
    plural: string;
  };

  // Pagination
  pagination?: {
    current_page: number;
    page_size: number;
    total_pages: number;
    total_records: number;
  } | null;
  onPageChange?: (page: number) => void;
  accent?:
    | "blue"
    | "purple"
    | "green"
    | "red"
    | "indigo"
    | "gray"
    | "amber"
    | "orange"
    | "pink"
    | "yellow";
}

export function DatabaseListPage<T = unknown>({
  userName: _userName,
  title,
  description,
  backUrl,
  listTitle,
  searchPlaceholder,
  searchValue,
  onSearchChange,
  filters,
  addButton,
  infoSection,
  items,
  loading,
  error,
  onRetry,
  emptyIcon,
  emptyTitle,
  emptyMessage,
  renderItem,
  itemLabel,
  pagination,
  onPageChange,
  accent = "blue",
}: Readonly<DatabaseListPageProps<T>>) {
  const spinnerCls = getAccentSpinner(accent ?? "blue");
  // Loading state
  if (loading) {
    return (
      <ResponsiveLayout>
        <div className="mx-auto max-w-7xl p-4 pb-24 md:p-6 lg:p-8 lg:pb-8">
          <DatabasePageHeader
            title={title}
            description={description}
            backUrl={backUrl}
          />
          <div className="flex flex-col items-center justify-center py-12 md:py-16">
            <div className="flex flex-col items-center gap-4">
              <div
                className={`h-10 w-10 animate-spin rounded-full border-t-2 border-b-2 md:h-12 md:w-12 ${spinnerCls}`}
              ></div>
              <p className="text-sm text-gray-600 md:text-base">
                Daten werden geladen...
              </p>
            </div>
          </div>
        </div>
      </ResponsiveLayout>
    );
  }

  // Error state
  if (error) {
    return (
      <ResponsiveLayout>
        <div className="mx-auto max-w-7xl p-4 pb-24 md:p-6 lg:p-8 lg:pb-8">
          <DatabasePageHeader
            title={title}
            description={description}
            backUrl={backUrl}
          />
          <div className="flex flex-col items-center justify-center py-8 md:py-12">
            <div className="w-full max-w-md rounded-lg bg-red-50 p-4 text-red-800 shadow-md md:p-6">
              <h2 className="mb-2 text-lg font-semibold md:text-xl">Fehler</h2>
              <p className="text-sm md:text-base">{error}</p>
              {onRetry && (
                <button
                  onClick={onRetry}
                  className="mt-4 w-full rounded-lg bg-red-100 px-4 py-2 text-sm text-red-800 transition-colors hover:bg-red-200 active:scale-[0.98] md:w-auto md:text-base"
                >
                  Erneut versuchen
                </button>
              )}
            </div>
          </div>
        </div>
      </ResponsiveLayout>
    );
  }

  // Determine empty state messages
  const hasSearchOrFilters = searchValue !== "" || filters !== undefined;
  const defaultEmptyTitle = hasSearchOrFilters
    ? "Keine Ergebnisse gefunden"
    : `Keine ${itemLabel?.plural ?? "Einträge"} vorhanden`;
  const defaultEmptyMessage = hasSearchOrFilters
    ? "Versuchen Sie einen anderen Suchbegriff."
    : `Fügen Sie ${itemLabel?.singular ? `einen neuen ${itemLabel.singular}` : "einen neuen Eintrag"} hinzu, um zu beginnen.`;

  // Main content
  return (
    <ResponsiveLayout>
      <div className="mx-auto max-w-7xl p-4 pb-24 md:p-6 lg:p-8 lg:pb-8">
        <DatabasePageHeader
          title={title}
          description={description}
          backUrl={backUrl}
        />

        <SearchFilter
          searchPlaceholder={searchPlaceholder}
          searchValue={searchValue}
          onSearchChange={onSearchChange}
          filters={filters}
          addButton={addButton}
          accent={accent}
        />

        {/* Info Section */}
        {infoSection && (
          <div className="mb-6 rounded-lg border border-blue-200 bg-blue-50 p-3 md:mb-8 md:p-4">
            <div className="flex">
              {infoSection.icon && (
                <div className="flex-shrink-0">{infoSection.icon}</div>
              )}
              <div
                className={infoSection.icon ? "ml-2 flex-1 md:ml-3" : "flex-1"}
              >
                <h3 className="text-xs font-medium text-blue-800 md:text-sm">
                  {infoSection.title}
                </h3>
                <div className="mt-0.5 text-xs text-blue-700 md:mt-1 md:text-sm">
                  <p>{infoSection.content}</p>
                </div>
              </div>
            </div>
          </div>
        )}

        <DatabaseListSection
          title={listTitle}
          itemCount={pagination ? pagination.total_records : items.length}
          itemLabel={itemLabel}
        >
          {items.length > 0 ? (
            items.map((item, index) => {
              // Try to use id if available, otherwise fall back to index
              const itemWithId = item as Record<string, unknown>;
              const itemId = itemWithId.id;
              const key =
                itemId &&
                (typeof itemId === "string" || typeof itemId === "number")
                  ? `item-${String(itemId)}`
                  : `item-${index}`;
              return <div key={key}>{renderItem(item, index)}</div>;
            })
          ) : (
            <div className="py-8 text-center md:py-12">
              <div className="flex flex-col items-center gap-4">
                {emptyIcon ?? (
                  <svg
                    className="h-12 w-12 text-gray-400"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10"
                    />
                  </svg>
                )}
                <div>
                  <h3 className="mb-1 text-base font-medium text-gray-900 md:text-lg">
                    {emptyTitle ?? defaultEmptyTitle}
                  </h3>
                  <p className="text-sm text-gray-600">
                    {emptyMessage ?? defaultEmptyMessage}
                  </p>
                </div>
              </div>
            </div>
          )}
        </DatabaseListSection>

        {/* Pagination controls - show if we have pagination data OR if we have exactly pageSize items (might be more pages) */}
        {pagination && pagination.total_pages > 1 && onPageChange && (
          <div className="mt-6 flex items-center justify-between px-4 py-3 sm:px-6">
            <div className="flex flex-1 justify-between sm:hidden">
              <button
                onClick={() =>
                  onPageChange(Math.max(1, pagination.current_page - 1))
                }
                disabled={pagination.current_page === 1}
                className="relative inline-flex items-center rounded-md border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50"
              >
                Zurück
              </button>
              <button
                onClick={() =>
                  onPageChange(
                    Math.min(
                      pagination.total_pages,
                      pagination.current_page + 1,
                    ),
                  )
                }
                disabled={pagination.current_page === pagination.total_pages}
                className="relative ml-3 inline-flex items-center rounded-md border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50"
              >
                Weiter
              </button>
            </div>
            <div className="hidden sm:flex sm:flex-1 sm:items-center sm:justify-between">
              <div>
                <p className="text-sm text-gray-700">
                  Zeige{" "}
                  <span className="font-medium">
                    {(pagination.current_page - 1) * pagination.page_size + 1}
                  </span>{" "}
                  bis{" "}
                  <span className="font-medium">
                    {Math.min(
                      pagination.current_page * pagination.page_size,
                      pagination.total_records,
                    )}
                  </span>{" "}
                  von{" "}
                  <span className="font-medium">
                    {pagination.total_records}
                  </span>{" "}
                  Ergebnissen
                </p>
              </div>
              <div>
                <nav
                  className="isolate inline-flex -space-x-px rounded-md shadow-sm"
                  aria-label="Pagination"
                >
                  <button
                    onClick={() =>
                      onPageChange(Math.max(1, pagination.current_page - 1))
                    }
                    disabled={pagination.current_page === 1}
                    className="relative inline-flex items-center rounded-l-md px-2 py-2 text-gray-400 ring-1 ring-gray-300 ring-inset hover:bg-gray-50 focus:z-20 focus:outline-offset-0 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    <span className="sr-only">Vorherige</span>
                    <svg
                      className="h-5 w-5"
                      viewBox="0 0 20 20"
                      fill="currentColor"
                      aria-hidden="true"
                    >
                      <path
                        fillRule="evenodd"
                        d="M12.79 5.23a.75.75 0 01-.02 1.06L8.832 10l3.938 3.71a.75.75 0 11-1.04 1.08l-4.5-4.25a.75.75 0 010-1.08l4.5-4.25a.75.75 0 011.06.02z"
                        clipRule="evenodd"
                      />
                    </svg>
                  </button>

                  {/* Page numbers */}
                  {Array.from(
                    { length: Math.min(7, pagination.total_pages) },
                    (_, i) => {
                      const startPage = Math.max(
                        1,
                        Math.min(
                          pagination.current_page - 3,
                          pagination.total_pages - 6,
                        ),
                      );
                      const displayPage = startPage + i;

                      if (displayPage > pagination.total_pages) return null;

                      return (
                        <button
                          key={displayPage}
                          onClick={() => onPageChange(displayPage)}
                          className={`relative inline-flex items-center px-4 py-2 text-sm font-semibold ${
                            displayPage === pagination.current_page
                              ? "z-10 bg-blue-600 text-white focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-600"
                              : "text-gray-900 ring-1 ring-gray-300 ring-inset hover:bg-gray-50 focus:z-20 focus:outline-offset-0"
                          }`}
                        >
                          {displayPage}
                        </button>
                      );
                    },
                  )}

                  <button
                    onClick={() =>
                      onPageChange(
                        Math.min(
                          pagination.total_pages,
                          pagination.current_page + 1,
                        ),
                      )
                    }
                    disabled={
                      pagination.current_page === pagination.total_pages
                    }
                    className="relative inline-flex items-center rounded-r-md px-2 py-2 text-gray-400 ring-1 ring-gray-300 ring-inset hover:bg-gray-50 focus:z-20 focus:outline-offset-0 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    <span className="sr-only">Nächste</span>
                    <svg
                      className="h-5 w-5"
                      viewBox="0 0 20 20"
                      fill="currentColor"
                      aria-hidden="true"
                    >
                      <path
                        fillRule="evenodd"
                        d="M7.21 14.77a.75.75 0 01.02-1.06L11.168 10 7.23 6.29a.75.75 0 111.04-1.08l4.5 4.25a.75.75 0 010 1.08l-4.5 4.25a.75.75 0 01-1.06-.02z"
                        clipRule="evenodd"
                      />
                    </svg>
                  </button>
                </nav>
              </div>
            </div>
          </div>
        )}
      </div>
    </ResponsiveLayout>
  );
}
