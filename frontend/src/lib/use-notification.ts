import { useState, useCallback } from "react";

export interface NotificationState {
  message: string | null;
  type: "success" | "error" | "warning" | "info";
}

export function useNotification(autoHideDuration = 3000) {
  const [notification, setNotification] = useState<NotificationState>({
    message: null,
    type: "success",
  });

  const showSuccess = useCallback(
    (message: string) => {
      setNotification({ message, type: "success" });

      if (autoHideDuration > 0) {
        setTimeout(() => {
          setNotification((prev) => ({ ...prev, message: null }));
        }, autoHideDuration);
      }
    },
    [autoHideDuration],
  );

  const showError = useCallback(
    (message: string) => {
      setNotification({ message, type: "error" });

      if (autoHideDuration > 0) {
        setTimeout(() => {
          setNotification((prev) => ({ ...prev, message: null }));
        }, autoHideDuration);
      }
    },
    [autoHideDuration],
  );

  const showWarning = useCallback(
    (message: string) => {
      setNotification({ message, type: "warning" });

      if (autoHideDuration > 0) {
        setTimeout(() => {
          setNotification((prev) => ({ ...prev, message: null }));
        }, autoHideDuration);
      }
    },
    [autoHideDuration],
  );

  const showInfo = useCallback(
    (message: string) => {
      setNotification({ message, type: "info" });

      if (autoHideDuration > 0) {
        setTimeout(() => {
          setNotification((prev) => ({ ...prev, message: null }));
        }, autoHideDuration);
      }
    },
    [autoHideDuration],
  );

  const hideNotification = useCallback(() => {
    setNotification((prev) => ({ ...prev, message: null }));
  }, []);

  return {
    notification,
    showSuccess,
    showError,
    showWarning,
    showInfo,
    hideNotification,
  };
}

// Helper function for standard database operation messages
export const getDbOperationMessage = (
  operation: "create" | "update" | "delete",
  entityName: string,
  entityIdentifier?: string,
): string => {
  const identifier = entityIdentifier ? ` "${entityIdentifier}"` : "";

  switch (operation) {
    case "create":
      return `${entityName}${identifier} wurde erfolgreich erstellt`;
    case "update":
      return `${entityName}${identifier} wurde erfolgreich aktualisiert`;
    case "delete":
      return `${entityName}${identifier} wurde erfolgreich gelöscht`;
  }
};
