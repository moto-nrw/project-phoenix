'use client';

import { useSession } from 'next-auth/react';
import { redirect, useRouter } from 'next/navigation';
import { useState, useEffect } from 'react';
import { PageHeader, SectionTitle } from '@/components/dashboard';
import ActivityList from '@/components/activities/activity-list';
import type { Activity, ActivityCategory } from '@/lib/activity-api';
import { activityService } from '@/lib/activity-api';
import Link from 'next/link';

export default function ActivitiesPage() {
  const router = useRouter();
  const [activities, setActivities] = useState<Activity[]>([]);
  const [categories, setCategories] = useState<ActivityCategory[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchFilter, setSearchFilter] = useState('');
  const [categoryFilter, setCategoryFilter] = useState<string | null>(null);
  
  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect('/login');
    },
  });

  // Function to fetch activities with optional filters
  const fetchActivities = async (search?: string, categoryId?: string | null) => {
    try {
      setLoading(true);
      
      // Prepare filters for API call
      const filters = {
        search: search ?? undefined,
        category_id: categoryId ?? undefined
      };
      
      try {
        // Fetch from the real API using our activity service
        const data = await activityService.getActivities(filters);
        
        if (data.length === 0 && !search && !categoryId) {
          console.log('No activities returned from API, checking connection');
        }
        
        setActivities(data);
        setError(null);
      } catch (apiErr) {
        console.error('API error when fetching activities:', apiErr);
        setError('Fehler beim Laden der Aktivitätsdaten. Bitte versuchen Sie es später erneut.');
        setActivities([]);
      }
    } catch (err) {
      console.error('Error fetching activities:', err);
      setError('Fehler beim Laden der Aktivitätsdaten. Bitte versuchen Sie es später erneut.');
      setActivities([]);
    } finally {
      setLoading(false);
    }
  };

  // Function to fetch categories
  const fetchCategories = async () => {
    try {
      const categoriesData = await activityService.getCategories();
      setCategories(categoriesData);
    } catch (err) {
      console.error('Error fetching categories:', err);
    }
  };

  // Initial data load
  useEffect(() => {
    void fetchActivities();
    void fetchCategories();
  }, []);

  // Handle search and category filter changes
  useEffect(() => {
    // Debounce search to avoid too many API calls
    const timer = setTimeout(() => {
      void fetchActivities(searchFilter, categoryFilter);
    }, 300);
    
    return () => clearTimeout(timer);
  }, [searchFilter, categoryFilter]);

  if (status === 'loading' || loading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p>Loading...</p>
      </div>
    );
  }

  const handleSelectActivity = (activity: Activity) => {
    router.push(`/database/activities/${activity.id}`);
  };

  // Show error if loading failed
  if (error) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center p-4">
        <div className="bg-red-50 text-red-800 p-4 rounded-lg max-w-md">
          <h2 className="font-semibold mb-2">Fehler</h2>
          <p>{error}</p>
          <button 
            onClick={() => fetchActivities()} 
            className="mt-4 px-4 py-2 bg-red-100 hover:bg-red-200 text-red-800 rounded transition-colors"
          >
            Erneut versuchen
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen">
      {/* Header */}
      <PageHeader 
        title="Aktivitätenauswahl"
        backUrl="/database"
      />

      {/* Main Content */}
      <main className="max-w-4xl mx-auto p-4">
        {/* Title Section */}
        <div className="mb-8">
          <SectionTitle title="Aktivität auswählen" />
        </div>

        {/* Search and Add Section */}
        <div className="flex flex-col sm:flex-row items-center justify-between gap-4 mb-8">
          <div className="relative w-full sm:max-w-md">
            <input
              type="text"
              placeholder="Suchen..."
              value={searchFilter}
              onChange={(e) => setSearchFilter(e.target.value)}
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
          
          <Link href="/database/activities/new" className="w-full sm:w-auto">
            <button className="group w-full sm:w-auto bg-gradient-to-r from-teal-500 to-blue-600 text-white py-3 px-4 rounded-lg flex items-center gap-2 hover:from-teal-600 hover:to-blue-700 hover:scale-[1.02] hover:shadow-lg transition-all duration-200 justify-center sm:justify-start">
              <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5 transition-transform duration-200 group-hover:rotate-90" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
              </svg>
              <span>Neue Aktivität erstellen</span>
            </button>
          </Link>
        </div>

        {/* Activity List */}
        <ActivityList 
          activities={activities}
          onActivityClick={handleSelectActivity}
          emptyMessage={searchFilter ? `Keine Ergebnisse für "${searchFilter}"` : "Keine Aktivitäten vorhanden."}
        />
      </main>
    </div>
  );
}