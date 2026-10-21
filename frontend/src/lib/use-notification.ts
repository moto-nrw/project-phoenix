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
