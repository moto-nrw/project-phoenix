"use client";

import { useState, useEffect } from "react";
// import { useRouter } from 'next/navigation';
import type { Group } from "@/lib/api";

// Type for the API response
interface ApiResponse<T> {
  success: boolean;
  message: string;
  data: T;
}

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
  className = "",
  required = false,
  label = "Gruppe",
  includeEmpty = true,
  emptyLabel = "Keine Gruppe auswählen",
}: GroupSelectorProps) {
  const [groups, setGroups] = useState<Group[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchGroups = async () => {
      try {
        setLoading(true);
        // Fetch groups using the list endpoint
        const response = await fetch("/api/groups");

        if (!response.ok) {
          throw new Error(`Error: ${response.status}`);
        }

        const result = await response.json() as ApiResponse<Group[]> | Group[];
        
        // Handle both wrapped and unwrapped responses
        let groupData: Group[];
        if (result && typeof result === 'object' && 'data' in result && !Array.isArray(result)) {
          // Handle wrapped response from our API
          groupData = result.data;
        } else if (Array.isArray(result)) {
          // Handle raw array response
          groupData = result;
        } else {
          throw new Error('Unexpected response format');
        }
        
        setGroups(groupData);
        setError(null);
      } catch (err) {
        console.error("Error fetching groups:", err);
        setError("Fehler beim Laden der Gruppen");
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
    return <div className="mt-1 text-sm text-red-500">{error}</div>;
  }

  return (
    <div className="w-full">
      {label && (
        <label
          htmlFor="group-selector"
          className="mb-1 block text-sm font-medium text-gray-700"
        >
          {label}
          {required && "*"}
        </label>
      )}
      <select
        id="group-selector"
        value={value}
        onChange={handleChange}
        disabled={loading}
        required={required}
        className={`w-full rounded-lg border border-gray-300 px-3 py-2 md:px-4 text-sm md:text-base transition-all duration-200 focus:ring-2 focus:ring-blue-500 focus:outline-none ${className} ${loading ? "opacity-50" : ""}`}
      >
        {includeEmpty && (
          <option value="">{loading ? "Lädt..." : emptyLabel}</option>
        )}

        {groups.map((group) => (
          <option key={group.id} value={group.id}>
            {group.name}
          </option>
        ))}

        {loading && !includeEmpty && (
          <option value="" disabled>
            Lädt...
          </option>
        )}
      </select>
    </div>
  );
}
