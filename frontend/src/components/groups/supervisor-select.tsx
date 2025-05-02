'use client';

import { useState, useEffect } from 'react';

interface SupervisorSelectProps {
  selectedSupervisorIds?: string[];
  onSupervisorChange: (supervisorIds: string[]) => void;
  isMulti?: boolean;
  className?: string;
  disabled?: boolean;
}

interface Supervisor {
  id: string;
  name: string;
}

export default function SupervisorSelect({ 
  selectedSupervisorIds = [], 
  onSupervisorChange,
  isMulti = true,
  className = '',
  disabled = false
}: SupervisorSelectProps) {
  const [supervisors, setSupervisors] = useState<Supervisor[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  
  // Fetch supervisors from API
  useEffect(() => {
    const fetchSupervisors = async () => {
      try {
        setLoading(true);
        
        // Make the actual API call
        const response = await fetch('/api/users/supervisors');
        
        if (!response.ok) {
          throw new Error(`Failed to fetch supervisors: ${response.statusText}`);
        }
        
        const data = await response.json() as Supervisor[];
        setSupervisors(data);
        setError(null);
      } catch (err) {
        console.error('Error fetching supervisors:', err);
        setError('Fehler beim Laden der Aufsichtspersonen');
        
        // Fallback to empty array to avoid breaking the UI
        setSupervisors([]);
      } finally {
        setLoading(false);
      }
    };
    
    void fetchSupervisors();
  }, []);
  
  const handleChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    if (isMulti) {
      const selectedOptions = Array.from(e.target.selectedOptions, option => option.value);
      onSupervisorChange(selectedOptions);
    } else {
      onSupervisorChange([e.target.value]);
    }
  };
  
  if (loading) {
    return (
      <div className="animate-pulse h-10 bg-gray-200 rounded-lg w-full"></div>
    );
  }
  
  if (error) {
    return (
      <div className="text-red-500 text-sm">{error}</div>
    );
  }
  
  return (
    <div className={className}>
      <select
        multiple={isMulti}
        value={selectedSupervisorIds}
        onChange={handleChange}
        disabled={disabled}
        className="w-full px-4 py-2 rounded-lg border border-gray-300 focus:outline-none focus:ring-2 focus:ring-purple-500 transition-all duration-200 shadow-sm"
      >
        {supervisors.map(supervisor => (
          <option key={supervisor.id} value={supervisor.id}>
            {supervisor.name}
          </option>
        ))}
      </select>
      {isMulti && (
        <p className="text-xs text-gray-500 mt-1">
          Halten Sie die Strg-Taste gedrückt, um mehrere Optionen auszuwählen
        </p>
      )}
    </div>
  );
}