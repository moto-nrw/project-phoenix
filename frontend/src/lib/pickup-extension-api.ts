/**
 * Client for the later-pickup block decisions (#3261). A child who is picked
 * up later than before may be on no block for the extra time; the backend
 * keeps an open task until the Leitung picks one or more blocks, or none.
 * Calls the Next.js proxy routes under /api/timetable/pickup-extensions.
 */

export const PICKUP_EXTENSIONS_SWR_KEY = "pickup-extensions";

type PickupExtensionKind = "day" | "weekday";

interface PickupExtensionBlock {
  readonly id: string;
  readonly title: string;
  readonly startTime: string;
  readonly endTime: string;
}

export interface PickupExtension {
  readonly id: string;
  readonly studentId: string;
  readonly studentName: string;
  readonly kind: PickupExtensionKind;
  /** YYYY-MM-DD, day tasks only. */
  readonly date?: string;
  /** ISO weekday 1-5, weekday tasks only. */
  readonly weekday?: number;
  /** YYYY-MM-DD, weekday tasks only. */
  readonly effectiveFrom?: string;
  /** HH:MM */
  readonly previousPickupTime: string;
  /** HH:MM */
  readonly pickupTime: string;
  readonly blocks: readonly PickupExtensionBlock[];
}

interface BackendPickupExtensionBlock {
  id: number;
  title: string;
  start_time: string;
  end_time: string;
}

interface BackendPickupExtension {
  id: number;
  student_id: number;
  student_name: string;
  kind: PickupExtensionKind;
  date?: string;
  weekday?: number;
  effective_from?: string;
  previous_pickup_time: string;
  pickup_time: string;
  blocks: BackendPickupExtensionBlock[] | null;
}

export class PickupExtensionApiError extends Error {
  readonly status: number;
  readonly code?: string;
  constructor(message: string, status: number, code?: string) {
    super(message);
    this.name = "PickupExtensionApiError";
    this.status = status;
    this.code = code;
  }
}

async function readData<T>(response: Response): Promise<T> {
  if (!response.ok) {
    let message = `Anfrage fehlgeschlagen (HTTP ${response.status})`;
    let code: string | undefined;
    try {
      const body = (await response.json()) as { error?: string; code?: string };
      if (body.error) message = body.error;
      code = body.code;
    } catch {
      // Body wasn't JSON; keep the generic message.
    }
    throw new PickupExtensionApiError(message, response.status, code);
  }
  return (await response.json()) as T;
}

function mapPickupExtension(raw: BackendPickupExtension): PickupExtension {
  return {
    id: String(raw.id),
    studentId: String(raw.student_id),
    studentName: raw.student_name,
    kind: raw.kind,
    ...(raw.date ? { date: raw.date } : {}),
    ...(raw.weekday ? { weekday: raw.weekday } : {}),
    ...(raw.effective_from ? { effectiveFrom: raw.effective_from } : {}),
    previousPickupTime: raw.previous_pickup_time,
    pickupTime: raw.pickup_time,
    blocks: (raw.blocks ?? []).map((block) => ({
      id: String(block.id),
      title: block.title,
      startTime: block.start_time,
      endTime: block.end_time,
    })),
  };
}

/** Open tasks of the school, or of one child when studentId is given. */
export async function fetchPickupExtensions(
  studentId?: string,
): Promise<PickupExtension[]> {
  const query = studentId ? `?student_id=${encodeURIComponent(studentId)}` : "";
  const response = await fetch(`/api/timetable/pickup-extensions${query}`, {
    method: "GET",
    headers: { Accept: "application/json" },
    credentials: "include",
  });
  const responseData = await readData<{
    data: { tasks: BackendPickupExtension[] | null };
  }>(response);
  return (responseData.data.tasks ?? []).map(mapPickupExtension);
}

/**
 * Adds the child to the chosen blocks and closes the task. No block IDs
 * closes it without a change ("Keinem Block zuordnen").
 */
export async function resolvePickupExtension(
  taskId: string,
  blockIds: readonly string[],
): Promise<void> {
  const response = await fetch(
    `/api/timetable/pickup-extensions/${encodeURIComponent(taskId)}/resolve`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      credentials: "include",
      body: JSON.stringify({ block_ids: blockIds.map(Number) }),
    },
  );
  await readData<unknown>(response);
}
