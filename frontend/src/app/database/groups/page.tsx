'use client';

import { useSession } from 'next-auth/react';
import { redirect, useRouter } from 'next/navigation';
import { useState, useEffect } from 'react';
import { DataListPage } from '@/components/dashboard';
import { GroupListItem } from '@/components/groups';
import type { Group } from '@/lib/api';
import { groupService } from '@/lib/api';

export default function GroupsPage() {
  const router = useRouter();
  const [groups, setGroups] = useState<Group[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchFilter, setSearchFilter] = useState('');
  
  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect('/login');
    },
  });

  // Function to fetch groups with optional filters
  const fetchGroups = async (search?: string) => {
    try {
      setLoading(true);
      
      // Prepare filters for API call
      const filters = {
        search: search ?? undefined
      };
      
      try {
        // Fetch from the real API using our group service
        const data = await groupService.getGroups(filters);
        
        if (data.length === 0 && !search) {
          console.log('No groups returned from API, checking connection');
        }
        
        setGroups(data);
        setError(null);
      } catch (apiErr) {
        console.error('API error when fetching groups:', apiErr);
        setError('Fehler beim Laden der Gruppendaten. Bitte versuchen Sie es später erneut.');
        setGroups([]);
      }
    } catch (err) {
      console.error('Error fetching groups:', err);
      setError('Fehler beim Laden der Gruppendaten. Bitte versuchen Sie es später erneut.');
      setGroups([]);
    } finally {
      setLoading(false);
    }
  };

  // Initial data load
  useEffect(() => {
    void fetchGroups();
  }, []);

  // Handle search filter changes
  useEffect(() => {
    // Debounce search to avoid too many API calls
    const timer = setTimeout(() => {
      void fetchGroups(searchFilter);
    }, 300);
    
    return () => clearTimeout(timer);
  }, [searchFilter]);

  if (status === 'loading' || loading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p>Loading...</p>
      </div>
    );
  }

  const handleSelectGroup = (group: Group) => {
    router.push(`/database/groups/${group.id}`);
  };

  const renderGroup = (group: Group) => (
    <GroupListItem group={group} onClick={() => handleSelectGroup(group)} />
  );

  // Show error if loading failed
  if (error) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center p-4">
        <div className="bg-red-50 text-red-800 p-4 rounded-lg max-w-md">
          <h2 className="font-semibold mb-2">Fehler</h2>
          <p>{error}</p>
          <button 
            onClick={() => fetchGroups()} 
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
      title="Gruppenauswahl"
      sectionTitle="Gruppe auswählen"
      backUrl="/database"
      newEntityLabel="Neue Gruppe erstellen"
      newEntityUrl="/database/groups/new"
      data={groups}
      onSelectEntityAction={handleSelectGroup}
      renderEntity={renderGroup}
    />
  );
}