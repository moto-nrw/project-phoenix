type SlotListProxyRequest = Record<string, unknown>;

/**
 * Normalizes the ID-array fields of a slot-list request before it is proxied to
 * the Go backend. The only transformation applied here is coercing numeric IDs
 * to their string form (the backend expects `[]string`); it deliberately never
 * rejects input.
 *
 * Validation of the ID values — non-array fields, non-numeric or non-positive
 * IDs — is left to the backend, which reports those as HTTP 400
 * (`parseSlotListIDList` in `backend/api/timetable/lists.go`). Duplicating that
 * check in the proxy and throwing would surface malformed client input as a
 * misleading 500: the preview route's `createPostHandler` classifies a thrown
 * Error as an unknown server failure, and the export route's catch block also
 * returns 500. Passing the raw value through lets the backend be the single
 * validator and return the correct 400.
 */
export function normalizeSlotListRequestBody(body: unknown): unknown {
  if (!isPlainObject(body)) {
    return body;
  }

  return {
    ...body,
    ...normalizeIDArrayField(body, "instance_ids"),
    ...normalizeIDArrayField(body, "group_ids"),
  };
}

function normalizeIDArrayField(
  body: SlotListProxyRequest,
  field: "instance_ids" | "group_ids",
): Partial<Record<typeof field, unknown>> {
  const value = body[field];
  if (value === undefined) {
    return {};
  }
  // Pass non-array values through untouched so the backend rejects them with a
  // 400 rather than the proxy turning them into a 500.
  if (!Array.isArray(value)) {
    return { [field]: value };
  }
  return { [field]: value.map(coerceID) };
}

// Coerce a numeric ID to its string form; leave everything else (strings,
// booleans, objects, out-of-range numbers) untouched for the backend to
// validate.
function coerceID(value: unknown): unknown {
  if (typeof value === "number" && Number.isSafeInteger(value)) {
    return String(value);
  }
  return value;
}

function isPlainObject(value: unknown): value is SlotListProxyRequest {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
