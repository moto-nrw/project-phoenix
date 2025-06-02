"use client";

import { type ReactNode } from "react";
import { ResponsiveLayout } from "@/components/dashboard";
import { SearchFilter } from "./search-filter";
import { DatabasePageHeader } from "./database-page-header";
import { DatabaseListSection } from "./database-list-section";

export interface DatabaseListPageProps<T = unknown> {
  // Page metadata
  userName: string;
  
  // Header
  title: string;
  description: string;
  
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
}

export function DatabaseListPage<T = unknown>({
  userName: _userName,
  title,
  description,
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
}: DatabaseListPageProps<T>) {
  // Loading state
  if (loading) {
    return (
      <ResponsiveLayout>
        <div className="max-w-7xl mx-auto p-4 md:p-6 lg:p-8 pb-24 lg:pb-8">
          <div className="flex flex-col items-center justify-center py-12 md:py-16">
            <div className="flex flex-col items-center gap-4">
              <div className="h-10 w-10 md:h-12 md:w-12 animate-spin rounded-full border-b-2 border-t-2 border-blue-500"></div>
              <p className="text-sm md:text-base text-gray-600">Daten werden geladen...</p>
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
        <div className="max-w-7xl mx-auto p-4 md:p-6 lg:p-8 pb-24 lg:pb-8">
          <div className="flex flex-col items-center justify-center py-8 md:py-12">
            <div className="max-w-md w-full rounded-lg bg-red-50 p-4 md:p-6 text-red-800 shadow-md">
              <h2 className="mb-2 text-lg md:text-xl font-semibold">Fehler</h2>
              <p className="text-sm md:text-base">{error}</p>
              {onRetry && (
                <button
                  onClick={onRetry}
                  className="mt-4 w-full md:w-auto rounded-lg bg-red-100 px-4 py-2 text-sm md:text-base text-red-800 transition-colors hover:bg-red-200 active:scale-[0.98]"
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
  const hasSearchOrFilters = searchValue !== "" ? true : filters !== undefined;
  const defaultEmptyTitle = hasSearchOrFilters 
    ? "Keine Ergebnisse gefunden" 
    : `Keine ${itemLabel?.plural ?? "Einträge"} vorhanden`;
  const defaultEmptyMessage = hasSearchOrFilters
    ? "Versuchen Sie einen anderen Suchbegriff."
    : `Fügen Sie ${itemLabel?.singular ? `einen neuen ${itemLabel.singular}` : "einen neuen Eintrag"} hinzu, um zu beginnen.`;

  // Main content
  return (
    <ResponsiveLayout>
      <div className="max-w-7xl mx-auto p-4 md:p-6 lg:p-8 pb-24 lg:pb-8">
        <DatabasePageHeader title={title} description={description} />
        
        <SearchFilter
          searchPlaceholder={searchPlaceholder}
          searchValue={searchValue}
          onSearchChange={onSearchChange}
          filters={filters}
          addButton={addButton}
        />
        
        {/* Info Section */}
        {infoSection && (
          <div className="mb-6 md:mb-8 rounded-lg border border-blue-200 bg-blue-50 p-3 md:p-4">
            <div className="flex">
              {infoSection.icon && (
                <div className="flex-shrink-0">
                  {infoSection.icon}
                </div>
              )}
              <div className={infoSection.icon ? "ml-2 md:ml-3 flex-1" : "flex-1"}>
                <h3 className="text-xs md:text-sm font-medium text-blue-800">{infoSection.title}</h3>
                <div className="mt-0.5 md:mt-1 text-xs md:text-sm text-blue-700">
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
              const key = (item as any).id ? `item-${(item as any).id}` : `item-${index}`;
              return (
                <div key={key}>
                  {renderItem(item, index)}
                </div>
              );
            })
          ) : (
            <div className="py-8 md:py-12 text-center">
              <div className="flex flex-col items-center gap-4">
                {emptyIcon ?? (
                  <svg className="h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
                  </svg>
                )}
                <div>
                  <h3 className="text-base md:text-lg font-medium text-gray-900 mb-1">
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
                onClick={() => onPageChange(Math.max(1, pagination.current_page - 1))}
                disabled={pagination.current_page === 1}
                className="relative inline-flex items-center rounded-md border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                Zurück
              </button>
              <button
                onClick={() => onPageChange(Math.min(pagination.total_pages, pagination.current_page + 1))}
                disabled={pagination.current_page === pagination.total_pages}
                className="relative ml-3 inline-flex items-center rounded-md border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                Weiter
              </button>
            </div>
            <div className="hidden sm:flex sm:flex-1 sm:items-center sm:justify-between">
              <div>
                <p className="text-sm text-gray-700">
                  Zeige{' '}
                  <span className="font-medium">
                    {(pagination.current_page - 1) * pagination.page_size + 1}
                  </span>{' '}
                  bis{' '}
                  <span className="font-medium">
                    {Math.min(pagination.current_page * pagination.page_size, pagination.total_records)}
                  </span>{' '}
                  von{' '}
                  <span className="font-medium">{pagination.total_records}</span>{' '}
                  Ergebnissen
                </p>
              </div>
              <div>
                <nav className="isolate inline-flex -space-x-px rounded-md shadow-sm" aria-label="Pagination">
                  <button
                    onClick={() => onPageChange(Math.max(1, pagination.current_page - 1))}
                    disabled={pagination.current_page === 1}
                    className="relative inline-flex items-center rounded-l-md px-2 py-2 text-gray-400 ring-1 ring-inset ring-gray-300 hover:bg-gray-50 focus:z-20 focus:outline-offset-0 disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    <span className="sr-only">Vorherige</span>
                    <svg className="h-5 w-5" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                      <path fillRule="evenodd" d="M12.79 5.23a.75.75 0 01-.02 1.06L8.832 10l3.938 3.71a.75.75 0 11-1.04 1.08l-4.5-4.25a.75.75 0 010-1.08l4.5-4.25a.75.75 0 011.06.02z" clipRule="evenodd" />
                    </svg>
                  </button>
                  
                  {/* Page numbers */}
                  {Array.from({ length: Math.min(7, pagination.total_pages) }, (_, i) => {
                    const startPage = Math.max(1, Math.min(pagination.current_page - 3, pagination.total_pages - 6));
                    const displayPage = startPage + i;
                    
                    if (displayPage > pagination.total_pages) return null;
                    
                    return (
                      <button
                        key={displayPage}
                        onClick={() => onPageChange(displayPage)}
                        className={`relative inline-flex items-center px-4 py-2 text-sm font-semibold ${
                          displayPage === pagination.current_page
                            ? 'z-10 bg-blue-600 text-white focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-600'
                            : 'text-gray-900 ring-1 ring-inset ring-gray-300 hover:bg-gray-50 focus:z-20 focus:outline-offset-0'
                        }`}
                      >
                        {displayPage}
                      </button>
                    );
                  })}
                  
                  <button
                    onClick={() => onPageChange(Math.min(pagination.total_pages, pagination.current_page + 1))}
                    disabled={pagination.current_page === pagination.total_pages}
                    className="relative inline-flex items-center rounded-r-md px-2 py-2 text-gray-400 ring-1 ring-inset ring-gray-300 hover:bg-gray-50 focus:z-20 focus:outline-offset-0 disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    <span className="sr-only">Nächste</span>
                    <svg className="h-5 w-5" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                      <path fillRule="evenodd" d="M7.21 14.77a.75.75 0 01.02-1.06L11.168 10 7.23 6.29a.75.75 0 111.04-1.08l4.5 4.25a.75.75 0 010 1.08l-4.5 4.25a.75.75 0 01-1.06-.02z" clipRule="evenodd" />
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