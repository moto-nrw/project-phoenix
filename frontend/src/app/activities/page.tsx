'use client';

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { activityService } from '~/lib/activity-api';
import type { Activity } from '~/lib/activity-api';
import { ActivityList } from '~/components/activities';

export default function ActivitiesPage() {
  const [activities, setActivities] = useState<Activity[]>([]);
  const [categories, setCategories] = useState<Array<{id: string, name: string}>>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState('');
  const [selectedCategory, setSelectedCategory] = useState('');
  
  const router = useRouter();

  useEffect(() => {
    const fetchData = async () => {
      try {
        setLoading(true);
        const [activitiesData, categoriesData] = await Promise.all([
          activityService.getActivities(),
          activityService.getCategories()
        ]);
        setActivities(activitiesData);
        setCategories(categoriesData);
        setError(null);
      } catch (err) {
        console.error('Error fetching activities:', err);
        setError('Fehler beim Laden der Aktivitäten. Bitte versuchen Sie es später erneut.');
      } finally {
        setLoading(false);
      }
    };

    fetchData();
  }, []);

  const handleSearch = (e: React.ChangeEvent<HTMLInputElement>) => {
    setSearch(e.target.value);
  };

  const handleCategoryChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    setSelectedCategory(e.target.value);
  };

  const handleCreateActivity = () => {
    router.push('/activities/new');
  };

  // Filter activities based on search term and selected category
  const filteredActivities = activities.filter(activity => {
    const matchesSearch = activity.name.toLowerCase().includes(search.toLowerCase());
    const matchesCategory = selectedCategory ? activity.ag_category_id === selectedCategory : true;
    return matchesSearch && matchesCategory;
  });

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Aktivitätsgruppen</h1>
        <button
          onClick={handleCreateActivity}
          className="px-4 py-2 bg-gradient-to-r from-purple-500 to-indigo-600 text-white rounded-lg hover:from-purple-600 hover:to-indigo-700 hover:shadow-lg transition-all duration-200"
        >
          Neue Aktivität erstellen
        </button>
      </div>

      <div className="bg-white rounded-lg shadow overflow-hidden mb-8">
        <div className="p-6">
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-6">
            <div className="col-span-2">
              <label htmlFor="search" className="block text-sm font-medium text-gray-700 mb-1">
                Suche
              </label>
              <input
                type="text"
                id="search"
                placeholder="Aktivitätsname suchen..."
                value={search}
                onChange={handleSearch}
                className="w-full px-4 py-2 rounded-lg border border-gray-300 focus:outline-none focus:ring-2 focus:ring-purple-500 transition-all duration-200"
              />
            </div>
            <div>
              <label htmlFor="category" className="block text-sm font-medium text-gray-700 mb-1">
                Kategorie
              </label>
              <select
                id="category"
                value={selectedCategory}
                onChange={handleCategoryChange}
                className="w-full px-4 py-2 rounded-lg border border-gray-300 focus:outline-none focus:ring-2 focus:ring-purple-500 transition-all duration-200"
              >
                <option value="">Alle Kategorien</option>
                {categories.map(category => (
                  <option key={category.id} value={category.id}>
                    {category.name}
                  </option>
                ))}
              </select>
            </div>
          </div>

          {loading ? (
            <div className="text-center py-8">
              <p className="text-gray-500">Aktivitäten werden geladen...</p>
            </div>
          ) : error ? (
            <div className="bg-red-50 text-red-800 p-4 rounded-lg">
              <p>{error}</p>
            </div>
          ) : (
            <ActivityList 
              activities={filteredActivities} 
              emptyMessage="Keine Aktivitäten gefunden. Bitte passen Sie Ihre Filterkriterien an oder erstellen Sie eine neue Aktivität."
            />
          )}
        </div>
      </div>
    </div>
  );
}