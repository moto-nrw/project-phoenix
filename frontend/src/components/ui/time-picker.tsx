"use client";

import { useState, useEffect, useCallback } from "react";

interface TimePickerProps {
  value: string;
  onChange: (value: string) => void;
  min?: string;
  className?: string;
}

export function TimePicker({ value, onChange, min, className }: TimePickerProps) {
  // Round minutes up to next 5-minute interval
  const roundToNext5Minutes = (timeStr: string) => {
    const [h, m] = timeStr.split(":");
    let hours = parseInt(h ?? "0", 10);
    let minutes = parseInt(m ?? "0", 10);
    
    // Round up to next 5-minute interval
    if (minutes % 5 !== 0) {
      minutes = Math.ceil(minutes / 5) * 5;
      
      // Handle overflow to next hour
      if (minutes >= 60) {
        minutes = 0;
        hours = (hours + 1) % 24;
      }
    }
    
    return {
      hours: hours.toString().padStart(2, "0"),
      minutes: minutes.toString().padStart(2, "0")
    };
  };

  // Initialize state from value prop
  const getInitialTime = () => {
    if (value) {
      return roundToNext5Minutes(value);
    }
    return { hours: "00", minutes: "00" };
  };

  const initialTime = getInitialTime();
  const [hours, setHours] = useState(initialTime.hours);
  const [minutes, setMinutes] = useState(initialTime.minutes);
  
  // Notify parent of the initial rounded time
  useEffect(() => {
    const initialValue = `${initialTime.hours}:${initialTime.minutes}`;
    if (value && initialValue !== value) {
      onChange(initialValue);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []); // Only run once on mount

  // Update local state when value prop changes from outside
  useEffect(() => {
    if (value && value !== `${hours.padStart(2, "0")}:${minutes.padStart(2, "0")}`) {
      const rounded = roundToNext5Minutes(value);
      setHours(rounded.hours);
      setMinutes(rounded.minutes);
      // Also notify parent of the rounded value
      onChange(`${rounded.hours}:${rounded.minutes}`);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value]); // Don't include hours/minutes in deps to prevent circular updates

  // Notify parent of changes
  const updateTime = useCallback((newHours: string, newMinutes: string) => {
    const newValue = `${newHours}:${newMinutes}`;
    onChange(newValue);
  }, [onChange]);

  const incrementHours = () => {
    const currentHours = parseInt(hours || "0", 10);
    const newHoursNum = (currentHours + 1) % 24;
    const newHoursStr = newHoursNum.toString().padStart(2, "0");
    setHours(newHoursStr);
    updateTime(newHoursStr, minutes || "00");
  };

  const decrementHours = () => {
    const currentHours = parseInt(hours || "0", 10);
    const newHoursNum = currentHours === 0 ? 23 : currentHours - 1;
    const newHoursStr = newHoursNum.toString().padStart(2, "0");
    setHours(newHoursStr);
    updateTime(newHoursStr, minutes || "00");
  };

  const incrementMinutes = () => {
    const currentMinutes = parseInt(minutes || "0", 10);
    const newMinutesNum = (currentMinutes + 5) % 60;
    const newMinutesStr = newMinutesNum.toString().padStart(2, "0");
    setMinutes(newMinutesStr);
    updateTime(hours || "00", newMinutesStr);
  };

  const decrementMinutes = () => {
    const currentMinutes = parseInt(minutes || "0", 10);
    const newMinutesNum = currentMinutes < 5 ? 55 : currentMinutes - 5;
    const newMinutesStr = newMinutesNum.toString().padStart(2, "0");
    setMinutes(newMinutesStr);
    updateTime(hours || "00", newMinutesStr);
  };

  // Check if time is valid based on min constraint
  const isValidTime = () => {
    if (!min) return true;
    const currentTime = `${hours}:${minutes}`;
    return currentTime >= min;
  };

  return (
    <div className={`flex items-center justify-center space-x-2 ${className}`}>
      {/* Hours */}
      <div className="flex flex-col items-center">
        <button
          type="button"
          onClick={incrementHours}
          className="p-1 text-gray-600 hover:text-gray-900 hover:bg-gray-100 rounded transition-colors"
          aria-label="Stunde erhöhen"
        >
          <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 15l7-7 7 7" />
          </svg>
        </button>
        <input
          type="text"
          value={hours}
          onChange={(e) => {
            const val = e.target.value.replace(/\D/g, "");
            
            if (val === "") {
              // Allow empty field while typing
              setHours("");
            } else if (val.length === 1) {
              // Single digit: could be typing "05" or just "5"
              if (parseInt(val, 10) <= 23) {
                setHours(val);
              }
            } else if (val.length === 2) {
              // Two digits: validate full number
              if (parseInt(val, 10) <= 23) {
                setHours(val);
                updateTime(val, minutes);
              }
            }
          }}
          onBlur={() => {
            // On blur, ensure we have a valid value
            if (hours === "") {
              setHours("00");
              updateTime("00", minutes);
            } else if (hours.length === 1) {
              // Single digit: pad with leading zero
              const paddedHours = hours.padStart(2, "0");
              setHours(paddedHours);
              updateTime(paddedHours, minutes);
            }
          }}
          className="w-16 text-center text-2xl font-semibold border rounded-md py-2 focus:outline-none focus:ring-2 focus:ring-blue-500"
          maxLength={2}
        />
        <button
          type="button"
          onClick={decrementHours}
          className="p-1 text-gray-600 hover:text-gray-900 hover:bg-gray-100 rounded transition-colors"
          aria-label="Stunde verringern"
        >
          <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
          </svg>
        </button>
      </div>

      {/* Separator */}
      <div className="text-2xl font-semibold">:</div>

      {/* Minutes */}
      <div className="flex flex-col items-center">
        <button
          type="button"
          onClick={incrementMinutes}
          className="p-1 text-gray-600 hover:text-gray-900 hover:bg-gray-100 rounded transition-colors"
          aria-label="Minuten erhöhen"
        >
          <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 15l7-7 7 7" />
          </svg>
        </button>
        <input
          type="text"
          value={minutes}
          onChange={(e) => {
            const val = e.target.value.replace(/\D/g, "");
            
            if (val === "") {
              // Allow empty field while typing
              setMinutes("");
            } else if (val.length === 1) {
              // Single digit: could be typing "05" or just "5"
              if (parseInt(val, 10) <= 59) {
                setMinutes(val);
              }
            } else if (val.length === 2) {
              // Two digits: validate full number
              if (parseInt(val, 10) <= 59) {
                setMinutes(val);
                updateTime(hours, val);
              }
            }
          }}
          onBlur={() => {
            // On blur, ensure we have a valid value
            if (minutes === "") {
              setMinutes("00");
              updateTime(hours, "00");
            } else if (minutes.length === 1) {
              // Single digit: pad with leading zero
              const paddedMinutes = minutes.padStart(2, "0");
              setMinutes(paddedMinutes);
              updateTime(hours, paddedMinutes);
            }
          }}
          className="w-16 text-center text-2xl font-semibold border rounded-md py-2 focus:outline-none focus:ring-2 focus:ring-blue-500"
          maxLength={2}
        />
        <button
          type="button"
          onClick={decrementMinutes}
          className="p-1 text-gray-600 hover:text-gray-900 hover:bg-gray-100 rounded transition-colors"
          aria-label="Minuten verringern"
        >
          <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
          </svg>
        </button>
      </div>

      {/* Time label */}
      <div className="ml-4 text-sm text-gray-600">Uhr</div>

      {/* Warning if time is invalid */}
      {!isValidTime() && (
        <div className="ml-4 text-sm text-red-600">
          <svg className="w-5 h-5 inline" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
          </svg>
        </div>
      )}
    </div>
  );
}