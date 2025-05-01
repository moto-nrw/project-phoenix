'use client';

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import type { Group } from '@/lib/api';

interface GroupSelectorProps {
  value: string;
  onChange: (groupId: string) => void;
  className?: string;
  required?: boolean;
  label?: string;
  includeEmpty?: boolean;
  emptyLabel?: string;
}

export default function GroupSelector({
  value,
  onChange,
  className = '',
  required = false,
  label = 'Gruppe',
  includeEmpty = true,
  emptyLabel = 'Keine Gruppe auswählen',
}: GroupSelectorProps) {
  const [groups, setGroups] = useState<Group[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  
  useEffect(() => {
    const fetchGroups = async () => {
      try {
        setLoading(true);
        // Fetch groups using the public API endpoint
        const response = await fetch('/api/groups/public');
        
        if (!response.ok) {
          throw new Error(`Error: ${response.status}`);
        }
        
        const data = await response.json();
        setGroups(data);
        setError(null);
      } catch (err) {
        console.error('Error fetching groups:', err);
        setError('Fehler beim Laden der Gruppen');
      } finally {
        setLoading(false);
      }
    };
    
    void fetchGroups();
  }, []);
  
  const handleChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    onChange(e.target.value);
  };
  
  if (error) {
    return (
      <div className="text-red-500 text-sm mt-1">
        {error}
      </div>
    );
  }
  
  return (
    <div className="w-full">
      {label && (
        <label htmlFor="group-selector" className="block text-sm font-medium text-gray-700 mb-1">
          {label}{required && '*'}
        </label>
      )}
      <select
        id="group-selector"
        value={value}
        onChange={handleChange}
        disabled={loading}
        required={required}
        className={`w-full px-4 py-2 rounded-lg border border-gray-300 focus:outline-none focus:ring-2 focus:ring-blue-500 transition-all duration-200 ${className} ${loading ? 'opacity-50' : ''}`}
      >
        {includeEmpty && (
          <option value="">{loading ? 'Lädt...' : emptyLabel}</option>
        )}
        
        {groups.map((group) => (
          <option key={group.id} value={group.id}>
            {group.name}
          </option>
        ))}
        
        {loading && !includeEmpty && (
          <option value="" disabled>Lädt...</option>
        )}
      </select>
    </div>
  );
}