// Types and mapping for tenant-defined shift types (Schichtarten, #1836).
// Backend wire format: snake_case, int64 id as number, color as hex string.

export interface BackendShiftType {
  id: number;
  name: string;
  color: string;
  description?: string;
  is_active: boolean;
  /** Set on a type the school removed while planned shifts still carry it. */
  archived_at?: string;
  /** Only on a list requested with `with_usage=true`. */
  usage?: { shifts: number; series: number };
}

export interface ShiftType {
  id: string;
  name: string;
  /** Hex color, e.g. "#83CD2D" */
  color: string;
  description: string;
  isActive: boolean;
  /**
   * Set once the school removed the type while planned shifts or series
   * still carried it (#6845). It keeps labelling them but is no longer
   * offered. Optional so fixtures without it stay valid.
   */
  archivedAt?: string | null;
  /** Planned shifts and series carrying the type, when requested. */
  usage?: ShiftTypeUsage;
}

export interface ShiftTypeUsage {
  shifts: number;
  series: number;
}

/** What removing a shift type did (#6845). */
export type ShiftTypeRemoval = "deleted" | "archived";

export function mapShiftType(data: BackendShiftType): ShiftType {
  return {
    id: data.id.toString(),
    name: data.name,
    color: data.color,
    description: data.description ?? "",
    isActive: data.is_active,
    archivedAt: data.archived_at ?? null,
    ...(data.usage
      ? { usage: { shifts: data.usage.shifts, series: data.usage.series } }
      : {}),
  };
}

/**
 * Whether a new shift may get this type: it is switched on and was not
 * removed. Planned shifts keep a type that is no longer offered.
 */
export function isShiftTypeOffered(type: ShiftType): boolean {
  return type.isActive && !type.archivedAt;
}

/** The picker label of a type a shift keeps although it is not offered. */
export function shiftTypeOptionLabel(type: ShiftType): string {
  if (type.archivedAt) return `${type.name} (entfernt)`;
  return type.isActive ? type.name : `${type.name} (inaktiv)`;
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
