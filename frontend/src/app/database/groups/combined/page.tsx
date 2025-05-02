'use client';

import { useSession } from 'next-auth/react';
import { redirect, useRouter } from 'next/navigation';
import { useState, useEffect } from 'react';
import { DataListPage } from '@/components/dashboard';
import type { CombinedGroup } from '@/lib/api';
import { combinedGroupService } from '@/lib/api';

export default function CombinedGroupsPage() {
  const router = useRouter();
  const [combinedGroups, setCombinedGroups] = useState<CombinedGroup[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  
  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect('/login');
    },
  });

  // Function to fetch combined groups
  const fetchCombinedGroups = async () => {
    try {
      setLoading(true);
      
      try {
        // Fetch from the real API using our combined group service
        const data = await combinedGroupService.getCombinedGroups();
        
        if (data.length === 0) {
          console.log('No combined groups returned from API, checking connection');
        }
        
        setCombinedGroups(data);
        setError(null);
      } catch (apiErr) {
        console.error('API error when fetching combined groups:', apiErr);
        setError('Fehler beim Laden der Gruppenkombinationen. Bitte versuchen Sie es später erneut.');
        setCombinedGroups([]);
      }
    } catch (err) {
      console.error('Error fetching combined groups:', err);
      setError('Fehler beim Laden der Gruppenkombinationen. Bitte versuchen Sie es später erneut.');
      setCombinedGroups([]);
    } finally {
      setLoading(false);
    }
  };

  // Initial data load
  useEffect(() => {
    void fetchCombinedGroups();
  }, []);

  if (status === 'loading' || loading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p>Loading...</p>
      </div>
    );
  }

  const handleSelectCombinedGroup = (combinedGroup: CombinedGroup) => {
    router.push(`/database/groups/combined/${combinedGroup.id}`);
  };

  // Custom renderer for combined group items
  const renderCombinedGroup = (combinedGroup: CombinedGroup) => (
    <div className="flex flex-col group-hover:translate-x-1 transition-transform duration-200 w-full">
      <div className="flex items-center justify-between">
        <span className="font-semibold text-gray-900 group-hover:text-blue-600 transition-colors duration-200">
          {combinedGroup.name}
          {combinedGroup.is_active && !combinedGroup.is_expired && (
            <span className="ml-2 px-2 py-0.5 bg-green-100 text-green-800 text-xs rounded-full">
              Aktiv
            </span>
          )}
          {combinedGroup.is_expired && (
            <span className="ml-2 px-2 py-0.5 bg-red-100 text-red-800 text-xs rounded-full">
              Abgelaufen
            </span>
          )}
        </span>
        <svg 
          xmlns="http://www.w3.org/2000/svg" 
          className="h-5 w-5 text-gray-400 group-hover:text-blue-500 group-hover:transform group-hover:translate-x-1 transition-all duration-200" 
          fill="none" 
          viewBox="0 0 24 24" 
          stroke="currentColor"
        >
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
        </svg>
      </div>
      <span className="text-sm text-gray-500">
        Zugriffsmethode: {combinedGroup.access_policy === 'all' ? 'Alle' : 
          combinedGroup.access_policy === 'first' ? 'Erste Gruppe' : 
          combinedGroup.access_policy === 'specific' ? 'Spezifische Gruppe' : 'Manuell'}
        {combinedGroup.group_count !== undefined && ` | Gruppen: ${combinedGroup.group_count}`}
        {combinedGroup.time_until_expiration && ` | Läuft ab in: ${combinedGroup.time_until_expiration}`}
      </span>
    </div>
  );

  // Show error if loading failed
  if (error) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center p-4">
        <div className="bg-red-50 text-red-800 p-4 rounded-lg max-w-md">
          <h2 className="font-semibold mb-2">Fehler</h2>
          <p>{error}</p>
          <button 
            onClick={() => fetchCombinedGroups()} 
            className="mt-4 px-4 py-2 bg-red-100 hover:bg-red-200 text-red-800 rounded transition-colors"
          >
            Erneut versuchen
          </button>
        </div>
      </div>
    );
  }

  return (
    <DataListPage
      title="Gruppenkombinationen"
      sectionTitle="Gruppenkombination auswählen"
      backUrl="/database/groups"
      newEntityLabel="Neue Kombination erstellen"
      newEntityUrl="/database/groups/combined/new"
      data={combinedGroups}
      onSelectEntityAction={handleSelectCombinedGroup}
      renderEntity={renderCombinedGroup}
    />
  );
}