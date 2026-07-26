// Types and mapping for tenant-defined shift types (Schichtarten, #1836).
// Backend wire format: snake_case, int64 id as number, color as hex string.

export interface BackendShiftType {
  id: number;
  name: string;
  color: string;
  description?: string;
  is_active: boolean;
}

export interface ShiftType {
  id: string;
  name: string;
  /** Hex color, e.g. "#83CD2D" */
  color: string;
  description: string;
  isActive: boolean;
}

export function mapShiftType(data: BackendShiftType): ShiftType {
  return {
    id: data.id.toString(),
    name: data.name,
    color: data.color,
    description: data.description ?? "",
    isActive: data.is_active,
  };
}

/** Builds an id -> shift type lookup for rendering shifts by their type. */
export function indexShiftTypes(
  types: readonly ShiftType[],
): Map<string, ShiftType> {
  const byId = new Map<string, ShiftType>();
  for (const type of types) {
    byId.set(type.id, type);
  }
  return byId;
}
