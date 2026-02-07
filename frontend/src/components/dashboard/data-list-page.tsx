"use client";

import { useState } from "react";
import { PageHeader, SectionTitle } from "@/components/dashboard";
import Link from "next/link";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "DataListPage" });

// Base interface for all entities
interface BaseEntity {
  id: string;
  name: string;
}

interface DataListPageProps<T extends BaseEntity> {
  readonly title: string; // Page title (e.g., "Schülerauswahl")
  readonly sectionTitle?: string; // Section title with gradient (e.g., "Schüler auswählen")
  readonly backUrl: string; // URL to navigate back to
  readonly newEntityLabel: string; // Label for new entity button (e.g., "Neuen Schüler erstellen")
  readonly newEntityUrl: string; // URL to create a new entity
  readonly data: T[]; // Array of entities to display
  readonly onSelectEntityAction: (entity: T) => void; // Callback when entity is selected
  readonly renderEntity?: (entity: T) => React.ReactNode; // Optional custom renderer for entity
  readonly searchTerm?: string; // Optional controlled search term
  readonly onSearchChange?: (searchTerm: string) => void; // Optional callback for search changes
}

export function DataListPage<T extends BaseEntity>({
  title,
  sectionTitle,
  backUrl,
  newEntityLabel,
  newEntityUrl,
  data,
  onSelectEntityAction,
  renderEntity,
  searchTerm: externalSearchTerm,
  onSearchChange,
}: DataListPageProps<T>) {
  const [internalSearchTerm, setInternalSearchTerm] = useState("");

  logger.debug("render state", {
    data_count: data.length,
    search_term: externalSearchTerm,
  });

  // Use either the controlled or the internal search term
  const searchTerm = externalSearchTerm ?? internalSearchTerm;

  // Handle search input changes
  const handleSearchChange = (value: string) => {
    if (onSearchChange) {
      // If we have an external handler, use it
      onSearchChange(value);
    } else {
      // Otherwise use the internal state
      setInternalSearchTerm(value);
    }
  };

  // Filter data based on search term (only if we're using the internal search)
  const filteredData =
    externalSearchTerm === undefined
      ? data.filter(
          (entity) =>
            entity.name
              ?.toLowerCase()
              .includes(internalSearchTerm.toLowerCase()) ?? false,
        )
      : data; // If external search, don't filter data (already filtered by the parent)

  logger.debug("filtered data", { filtered_count: filteredData.length });

  // Default entity renderer
  const defaultRenderEntity = (entity: T) => (
    <div className="flex w-full items-center justify-between">
      <span>{entity.name || "Unbenannt"}</span>
      <svg
        xmlns="http://www.w3.org/2000/svg"
        className="h-5 w-5 text-gray-400"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path
          strokeLinecap="round"
          strokeLinejoin="round"
          strokeWidth={2}
          d="M9 5l7 7-7 7"
        />
      </svg>
    </div>
  );

  return (
    <div className="min-h-screen">
      {/* Header */}
      <PageHeader title={title} backUrl={backUrl} />

      {/* Main Content */}
      <main className="mx-auto max-w-4xl p-4">
        {/* Title Section */}
        <div className="mb-8">
          <SectionTitle title={sectionTitle ?? "Auswählen"} />
        </div>

        {/* Search and Add Section */}
        <div className="mb-8 flex flex-col items-center justify-between gap-4 sm:flex-row">
          <div className="relative w-full sm:max-w-md">
            <input
              type="text"
              placeholder="Suchen..."
              value={searchTerm}
              onChange={(e) => handleSearchChange(e.target.value)}
              className="w-full rounded-lg border border-gray-300 px-4 py-3 pl-10 transition-all duration-200 hover:border-gray-400 focus:shadow-md focus:ring-2 focus:ring-blue-500 focus:outline-none"
            />
            <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3">
              <svg
                xmlns="http://www.w3.org/2000/svg"
                className="h-5 w-5 text-gray-400"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
                />
              </svg>
            </div>
          </div>

          <Link href={newEntityUrl} className="w-full sm:w-auto">
            <button className="group flex w-full items-center justify-center gap-2 rounded-lg bg-gradient-to-r from-teal-500 to-blue-600 px-4 py-3 text-white transition-all duration-200 hover:scale-[1.02] hover:from-teal-600 hover:to-blue-700 hover:shadow-lg sm:w-auto sm:justify-start">
              <svg
                xmlns="http://www.w3.org/2000/svg"
                className="h-5 w-5 transition-transform duration-200 group-hover:rotate-90"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M12 4v16m8-8H4"
                />
              </svg>
              <span>{newEntityLabel}</span>
            </button>
          </Link>
        </div>

        {/* Entity List */}
        <div className="w-full space-y-3">
          {filteredData.length > 0 ? (
            filteredData.map((entity, index) => {
              const handleClick = () => onSelectEntityAction(entity);
              return (
                <div key={entity.id || `entity-${index}`}>
                  {renderEntity ? (
                    renderEntity(entity)
                  ) : (
                    <button
                      type="button"
                      className="group flex w-full cursor-pointer items-center justify-between rounded-lg border border-gray-100 bg-white p-4 text-left shadow-sm transition-all duration-200 hover:translate-y-[-1px] hover:border-blue-200 hover:shadow-md"
                      onClick={handleClick}
                    >
                      {defaultRenderEntity(entity)}
                    </button>
                  )}
                </div>
              );
            })
          ) : (
            <div className="py-8 text-center">
              <p className="text-gray-500">
                {searchTerm
                  ? "Keine Ergebnisse gefunden."
                  : "Keine Einträge vorhanden."}
              </p>
            </div>
          )}
        </div>
      </main>
    </div>
  );
}
