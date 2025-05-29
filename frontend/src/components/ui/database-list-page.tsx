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
  addButton: {
    label: string;
    href: string;
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
}

export function DatabaseListPage<T = unknown>({
  userName,
  title,
  description,
  listTitle,
  searchPlaceholder,
  searchValue,
  onSearchChange,
  filters,
  addButton,
  items,
  loading,
  error,
  onRetry,
  emptyIcon,
  emptyTitle,
  emptyMessage,
  renderItem,
  itemLabel,
}: DatabaseListPageProps<T>) {
  // Loading state
  if (loading) {
    return (
      <ResponsiveLayout userName={userName}>
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
      <ResponsiveLayout userName={userName}>
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
    <ResponsiveLayout userName={userName}>
      <div className="max-w-7xl mx-auto p-4 md:p-6 lg:p-8 pb-24 lg:pb-8">
        <DatabasePageHeader title={title} description={description} />
        
        <SearchFilter
          searchPlaceholder={searchPlaceholder}
          searchValue={searchValue}
          onSearchChange={onSearchChange}
          filters={filters}
          addButton={addButton}
        />
        
        <DatabaseListSection 
          title={listTitle} 
          itemCount={items.length}
          itemLabel={itemLabel}
        >
          {items.length > 0 ? (
            items.map((item, index) => renderItem(item, index))
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
      </div>
    </ResponsiveLayout>
  );
}