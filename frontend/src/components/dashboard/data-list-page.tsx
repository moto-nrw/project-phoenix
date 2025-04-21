'use client';

import { useState } from 'react';
import { PageHeader, SectionTitle } from '@/components/dashboard';
import Link from 'next/link';

// Base interface for all entities
interface BaseEntity {
  id: string;
  name: string;
}

interface DataListPageProps<T extends BaseEntity> {
  title: string;            // Page title (e.g., "Schülerauswahl")
  sectionTitle?: string;    // Section title with gradient (e.g., "Schüler auswählen")
  backUrl: string;          // URL to navigate back to
  newEntityLabel: string;   // Label for new entity button (e.g., "Neuen Schüler erstellen")
  newEntityUrl: string;     // URL to create a new entity
  data: T[];                // Array of entities to display
  onSelectEntity: (entity: T) => void; // Callback when entity is selected
  renderEntity?: (entity: T) => React.ReactNode; // Optional custom renderer for entity
}

export function DataListPage<T extends BaseEntity>({
  title,
  sectionTitle,
  backUrl,
  newEntityLabel,
  newEntityUrl,
  data,
  onSelectEntity,
  renderEntity,
}: DataListPageProps<T>) {
  const [searchTerm, setSearchTerm] = useState('');

  // Filter data based on search term
  const filteredData = data.filter(entity => 
    entity.name.toLowerCase().includes(searchTerm.toLowerCase())
  );

  // Default entity renderer
  const defaultRenderEntity = (entity: T) => (
    <div className="flex justify-between items-center w-full">
      <span>{entity.name}</span>
      <svg 
        xmlns="http://www.w3.org/2000/svg" 
        className="h-5 w-5 text-gray-400" 
        fill="none" 
        viewBox="0 0 24 24" 
        stroke="currentColor"
      >
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
      </svg>
    </div>
  );

  return (
    <div className="min-h-screen">
      {/* Header */}
      <PageHeader 
        title={title}
        backUrl={backUrl}
      />

      {/* Main Content */}
      <main className="max-w-4xl mx-auto p-4">
        {/* Title Section */}
        <div className="mb-8">
          <SectionTitle title={sectionTitle ?? "Auswählen"} />
        </div>

        {/* Search and Add Section */}
        <div className="flex flex-col sm:flex-row items-center justify-between gap-4 mb-8">
          <div className="relative w-full sm:max-w-md">
            <input
              type="text"
              placeholder="Suchen..."
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              className="w-full px-4 py-3 rounded-lg border border-gray-300 focus:outline-none focus:ring-2 focus:ring-blue-500 pl-10 transition-all duration-200 hover:border-gray-400 focus:shadow-md"
            />
            <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
              <svg 
                xmlns="http://www.w3.org/2000/svg" 
                className="h-5 w-5 text-gray-400" 
                fill="none" 
                viewBox="0 0 24 24" 
                stroke="currentColor"
              >
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
              </svg>
            </div>
          </div>
          
          <Link href={newEntityUrl} className="w-full sm:w-auto">
            <button className="group w-full sm:w-auto bg-gradient-to-r from-teal-500 to-blue-600 text-white py-3 px-4 rounded-lg flex items-center gap-2 hover:from-teal-600 hover:to-blue-700 hover:scale-[1.02] hover:shadow-lg transition-all duration-200 justify-center sm:justify-start">
              <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5 transition-transform duration-200 group-hover:rotate-90" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
              </svg>
              <span>{newEntityLabel}</span>
            </button>
          </Link>
        </div>

        {/* Entity List */}
        <div className="space-y-3 w-full">
          {filteredData.length > 0 ? (
            filteredData.map(entity => (
              <div 
                key={entity.id} 
                className="group bg-white border border-gray-100 rounded-lg p-4 shadow-sm hover:shadow-md hover:border-blue-200 hover:translate-y-[-1px] transition-all duration-200 cursor-pointer flex items-center justify-between"
                onClick={() => onSelectEntity(entity)}
              >
                {renderEntity ? renderEntity(entity) : defaultRenderEntity(entity)}
              </div>
            ))
          ) : (
            <div className="text-center py-8">
              <p className="text-gray-500">
                {searchTerm ? 'Keine Ergebnisse gefunden.' : 'Keine Einträge vorhanden.'}
              </p>
            </div>
          )}
        </div>
      </main>
    </div>
  );
}