"use client";

import React, {
  createContext,
  useState,
  useCallback,
  type ReactNode,
} from "react";

interface AlertState {
  isShowing: boolean;
  type?: "success" | "error" | "info" | "warning";
  message?: string;
}

interface AlertContextType {
  alertState: AlertState;
  showAlert: (type: AlertState["type"], message: string) => void;
  hideAlert: () => void;
}

export const AlertContext = createContext<AlertContextType | undefined>(
  undefined,
);

interface AlertProviderProps {
  children: ReactNode;
}

export function AlertProvider({ children }: AlertProviderProps) {
  const [alertState, setAlertState] = useState<AlertState>({
    isShowing: false,
  });

  const showAlert = useCallback((type: AlertState["type"], message: string) => {
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
    <AlertContext.Provider value={value}>{children}</AlertContext.Provider>
  );
}
