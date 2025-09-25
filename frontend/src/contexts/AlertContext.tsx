"use client";

import React, { createContext, useContext, useState, useCallback, type ReactNode } from 'react';

interface AlertState {
  isShowing: boolean;
  type?: 'success' | 'error' | 'info' | 'warning';
  message?: string;
}

interface AlertContextType {
  alertState: AlertState;
  showAlert: (type: AlertState['type'], message: string) => void;
  hideAlert: () => void;
}

export const AlertContext = createContext<AlertContextType | undefined>(undefined);

export function useAlert() {
  const context = useContext(AlertContext);
  if (!context) {
    throw new Error('useAlert must be used within an AlertProvider');
  }
  return context;
}

// Hook specifically for FAB positioning
export function useAlertVisibility() {
  const context = useContext(AlertContext);
  if (!context) {
    // Return default state if not in provider (for backwards compatibility)
    return { isAlertShowing: false };
  }
  return { isAlertShowing: context.alertState.isShowing };
}

interface AlertProviderProps {
  children: ReactNode;
}

export function AlertProvider({ children }: AlertProviderProps) {
  const [alertState, setAlertState] = useState<AlertState>({
    isShowing: false,
  });

  const showAlert = useCallback((type: AlertState['type'], message: string) => {
    setAlertState({
      isShowing: true,
      type,
      message,
    });
  }, []);

  const hideAlert = useCallback(() => {
    setAlertState({
      isShowing: false,
      type: undefined,
      message: undefined,
    });
  }, []);

  const value: AlertContextType = {
    alertState,
    showAlert,
    hideAlert,
  };

  return (
    <AlertContext.Provider value={value}>
      {children}
    </AlertContext.Provider>
  );
}