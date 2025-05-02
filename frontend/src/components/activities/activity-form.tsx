'use client';

import { useState, useEffect } from 'react';
import type { Activity, ActivityCategory, ActivityTime } from '@/lib/activity-api';

// Helper component for selecting a supervisor
const SupervisorSelector = ({ 
  value, 
  onChange, 
  label,
  supervisors = []
}: { 
  value: string; 
  onChange: (value: string) => void; 
  label: string;
  supervisors?: Array<{id: string, name: string}>; 
}) => {
  return (
    <div>
      <label className="block text-sm font-medium text-gray-700 mb-1">
        {label}
      </label>
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="w-full px-4 py-2 rounded-lg border border-gray-300 focus:outline-none focus:ring-2 focus:ring-purple-500 transition-all duration-200"
      >
        <option value="">Supervisor auswählen</option>
        {supervisors.map(supervisor => (
          <option key={supervisor.id} value={supervisor.id}>
            {supervisor.name}
          </option>
        ))}
      </select>
    </div>
  );
};

// Helper component for selecting a category
const CategorySelector = ({ 
  value, 
  onChange, 
  label,
  categories = []
}: { 
  value: string; 
  onChange: (value: string) => void; 
  label: string;
  categories?: ActivityCategory[]; 
}) => {
  return (
    <div>
      <label className="block text-sm font-medium text-gray-700 mb-1">
        {label}
      </label>
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="w-full px-4 py-2 rounded-lg border border-gray-300 focus:outline-none focus:ring-2 focus:ring-purple-500 transition-all duration-200"
      >
        <option value="">Kategorie auswählen</option>
        {categories.map(category => (
          <option key={category.id} value={category.id}>
            {category.name}
          </option>
        ))}
      </select>
    </div>
  );
};

// Helper component for time slots
const TimeSlotEditor = ({
  timeSlots,
  onAdd,
  onRemove
}: {
  timeSlots: ActivityTime[];
  onAdd: (timeSlot: Omit<ActivityTime, 'id' | 'ag_id' | 'created_at'>) => void;
  onRemove: (index: number) => void;
}) => {
  const [weekday, setWeekday] = useState('Monday');
  const [startTime, setStartTime] = useState('');
  const [endTime, setEndTime] = useState('');
  const [timespanId, setTimespanId] = useState('');
  
  const weekdays = [
    'Monday', 'Tuesday', 'Wednesday', 'Thursday', 
    'Friday', 'Saturday', 'Sunday'
  ];
  
  const handleAddTimeSlot = () => {
    if (!weekday || !timespanId) {
      alert('Bitte geben Sie Wochentag und Zeitraum an.');
      return;
    }
    
    onAdd({
      weekday,
      timespan_id: timespanId,
    });
    
    // Reset form
    setStartTime('');
    setEndTime('');
  };
  
  // In a real application, you'd fetch actual timespan IDs from the backend
  // For now, we'll just use a dummy timespan ID
  useEffect(() => {
    if (startTime) {
      // This is just a placeholder - in a real app, you would create or select 
      // a real timespan ID based on the start and end times
      setTimespanId('1');
    } else {
      setTimespanId('');
    }
  }, [startTime, endTime]);
  
  return (
    <div className="space-y-4">
      <h3 className="text-md font-medium text-gray-700">Zeitslots</h3>
      
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Wochentag</label>
          <select
            value={weekday}
            onChange={(e) => setWeekday(e.target.value)}
            className="w-full px-4 py-2 rounded-lg border border-gray-300 focus:outline-none focus:ring-2 focus:ring-purple-500 transition-all duration-200"
          >
            {weekdays.map(day => (
              <option key={day} value={day}>{day}</option>
            ))}
          </select>
        </div>
        
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Startzeit</label>
          <input
            type="time"
            value={startTime}
            onChange={(e) => setStartTime(e.target.value)}
            className="w-full px-4 py-2 rounded-lg border border-gray-300 focus:outline-none focus:ring-2 focus:ring-purple-500 transition-all duration-200"
          />
        </div>
        
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Endzeit</label>
          <input
            type="time"
            value={endTime}
            onChange={(e) => setEndTime(e.target.value)}
            className="w-full px-4 py-2 rounded-lg border border-gray-300 focus:outline-none focus:ring-2 focus:ring-purple-500 transition-all duration-200"
          />
        </div>
      </div>
      
      <div className="flex justify-end">
        <button
          type="button"
          onClick={handleAddTimeSlot}
          className="px-4 py-2 bg-purple-600 text-white rounded-lg hover:bg-purple-700 transition-colors"
        >
          Zeitslot hinzufügen
        </button>
      </div>
      
      {timeSlots.length > 0 && (
        <div className="mt-4">
          <h4 className="text-sm font-medium text-gray-700 mb-2">Vorhandene Zeitslots:</h4>
          <ul className="space-y-2">
            {timeSlots.map((slot, index) => (
              <li 
                key={slot.id || index} 
                className="flex justify-between items-center p-2 bg-gray-50 rounded border border-gray-200"
              >
                <span>
                  {slot.weekday} {slot.timespan?.start_time || ''} 
                  {slot.timespan?.end_time ? ` - ${slot.timespan.end_time}` : ''}
                </span>
                <button
                  type="button"
                  onClick={() => onRemove(index)}
                  className="text-red-500 hover:text-red-700"
                >
                  <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
                    <path fillRule="evenodd" d="M9 2a1 1 0 00-.894.553L7.382 4H4a1 1 0 000 2v10a2 2 0 002 2h8a2 2 0 002-2V6a1 1 0 100-2h-3.382l-.724-1.447A1 1 0 0011 2H9zM7 8a1 1 0 012 0v6a1 1 0 11-2 0V8zm5-1a1 1 0 00-1 1v6a1 1 0 102 0V8a1 1 0 00-1-1z" clipRule="evenodd" />
                  </svg>
                </button>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
};

interface ActivityFormProps {
  initialData?: Partial<Activity>;
  onSubmitAction: (activityData: Partial<Activity>) => Promise<void>;
  onCancelAction: () => void;
  isLoading: boolean;
  formTitle: string;
  submitLabel: string;
  categories?: ActivityCategory[];
  supervisors?: Array<{id: string, name: string}>;
}

export default function ActivityForm({
  initialData,
  onSubmitAction,
  onCancelAction,
  isLoading,
  formTitle,
  submitLabel,
  categories = [],
  supervisors = []
}: ActivityFormProps) {
  const [formData, setFormData] = useState<Partial<Activity>>({
    name: '',
    max_participant: 0,
    is_open_ag: false,
    supervisor_id: '',
    ag_category_id: '',
  });
  
  const [timeSlots, setTimeSlots] = useState<ActivityTime[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (initialData) {
      setFormData({
        name: initialData.name || '',
        max_participant: initialData.max_participant || 0,
        is_open_ag: initialData.is_open_ag || false,
        supervisor_id: initialData.supervisor_id || '',
        ag_category_id: initialData.ag_category_id || '',
      });
      
      if (initialData.times) {
        setTimeSlots(initialData.times);
      }
    }
  }, [initialData]);

  const handleChange = (
    e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>
  ) => {
    const { name, value, type } = e.target as HTMLInputElement;
    
    if (type === 'checkbox') {
      const { checked } = e.target as HTMLInputElement;
      setFormData(prev => ({
        ...prev,
        [name]: checked,
      }));
    } else if (type === 'number') {
      setFormData(prev => ({
        ...prev,
        [name]: parseInt(value) || 0,
      }));
    } else {
      setFormData(prev => ({
        ...prev,
        [name]: value,
      }));
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    
    // Validate form
    if (!formData.name || !formData.max_participant || !formData.supervisor_id || !formData.ag_category_id) {
      setError('Bitte füllen Sie alle Pflichtfelder aus.');
      return;
    }
    
    try {
      setError(null);
      
      // Include time slots in submission data
      const submissionData = {
        ...formData,
        times: timeSlots
      };
      
      // Call the provided submit function with form data
      await onSubmitAction(submissionData);
    } catch (err) {
      console.error('Error submitting form:', err);
      setError('Fehler beim Speichern der Aktivitätsdaten. Bitte versuchen Sie es später erneut.');
    }
  };

  const handleAddTimeSlot = (newTimeSlot: Omit<ActivityTime, 'id' | 'ag_id' | 'created_at'>) => {
    // Generate a temporary ID for UI purposes
    const tempTimeSlot: ActivityTime = {
      id: `temp-${Date.now()}`,
      ag_id: formData.id || 'new',
      created_at: new Date().toISOString(),
      ...newTimeSlot,
    };
    
    setTimeSlots(prev => [...prev, tempTimeSlot]);
  };
  
  const handleRemoveTimeSlot = (index: number) => {
    setTimeSlots(prev => prev.filter((_, i) => i !== index));
  };

  return (
    <div className="bg-white shadow-md rounded-lg overflow-hidden">
      <div className="p-6">
        <h2 className="text-xl font-bold mb-6 text-gray-800">{formTitle}</h2>
        
        {error && (
          <div className="bg-red-50 text-red-800 p-4 rounded-lg mb-6">
            <p>{error}</p>
          </div>
        )}
        
        <form onSubmit={handleSubmit} className="space-y-6">
          <div className="bg-purple-50 p-4 rounded-lg mb-8">
            <h2 className="text-purple-800 text-lg font-medium mb-4">Grundlegende Informationen</h2>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {/* Name field */}
              <div>
                <label htmlFor="name" className="block text-sm font-medium text-gray-700 mb-1">
                  Name*
                </label>
                <input
                  type="text"
                  id="name"
                  name="name"
                  value={formData.name}
                  onChange={handleChange}
                  required
                  className="w-full px-4 py-2 rounded-lg border border-gray-300 focus:outline-none focus:ring-2 focus:ring-purple-500 transition-all duration-200"
                />
              </div>
              
              {/* Max Participants field */}
              <div>
                <label htmlFor="max_participant" className="block text-sm font-medium text-gray-700 mb-1">
                  Maximale Teilnehmer*
                </label>
                <input
                  type="number"
                  id="max_participant"
                  name="max_participant"
                  value={formData.max_participant}
                  onChange={handleChange}
                  min="1"
                  required
                  className="w-full px-4 py-2 rounded-lg border border-gray-300 focus:outline-none focus:ring-2 focus:ring-purple-500 transition-all duration-200"
                />
              </div>
              
              {/* Category selector */}
              <CategorySelector
                value={formData.ag_category_id || ''}
                onChange={(value) => {
                  setFormData(prev => ({
                    ...prev,
                    ag_category_id: value,
                  }));
                }}
                label="Kategorie*"
                categories={categories}
              />
              
              {/* Supervisor selector */}
              <SupervisorSelector
                value={formData.supervisor_id || ''}
                onChange={(value) => {
                  setFormData(prev => ({
                    ...prev,
                    supervisor_id: value,
                  }));
                }}
                label="Leitung*"
                supervisors={supervisors}
              />
              
              {/* Is Open checkbox */}
              <div className="md:col-span-2 flex items-center mt-2">
                <input
                  type="checkbox"
                  id="is_open_ag"
                  name="is_open_ag"
                  checked={formData.is_open_ag}
                  onChange={handleChange}
                  className="h-4 w-4 text-purple-600 rounded border-gray-300 focus:ring-purple-500"
                />
                <label htmlFor="is_open_ag" className="ml-2 block text-sm text-gray-700">
                  Offen für Anmeldungen
                </label>
              </div>
            </div>
          </div>
          
          <div className="bg-blue-50 p-4 rounded-lg mb-8">
            <TimeSlotEditor
              timeSlots={timeSlots}
              onAdd={handleAddTimeSlot}
              onRemove={handleRemoveTimeSlot}
            />
          </div>
          
          {/* Form actions */}
          <div className="flex justify-end pt-4">
            <button
              type="button"
              onClick={onCancelAction}
              className="px-4 py-2 text-gray-700 mr-2 hover:bg-gray-100 rounded-lg transition-colors shadow-sm"
              disabled={isLoading}
            >
              Abbrechen
            </button>
            <button
              type="submit"
              className="px-6 py-2 bg-gradient-to-r from-purple-500 to-indigo-600 text-white rounded-lg hover:from-purple-600 hover:to-indigo-700 hover:shadow-lg transition-all duration-200"
              disabled={isLoading}
            >
              {isLoading ? 'Wird gespeichert...' : submitLabel}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}