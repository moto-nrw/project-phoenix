type SlotListProxyRequest = Record<string, unknown>;

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
): Partial<Record<typeof field, string[]>> {
  const value = body[field];
  if (value === undefined) {
    return {};
  }
  if (!Array.isArray(value)) {
    throw new Error(`${field} must be an array`);
  }
  return { [field]: value.map((id) => normalizeID(id, field)) };
}

function normalizeID(value: unknown, field: string): string {
  if (typeof value === "string" && /^\d+$/.test(value) && value !== "0") {
    return value;
  }
  if (typeof value === "number" && Number.isSafeInteger(value) && value > 0) {
    return String(value);
  }
  throw new Error(`${field} must contain positive integer IDs`);
}

function isPlainObject(value: unknown): value is SlotListProxyRequest {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
