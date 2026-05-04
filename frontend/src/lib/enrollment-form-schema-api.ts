import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "EnrollmentFormSchemaAPI" });

/** Field types accepted by the backend FormFieldType enum. */
export type FormFieldType =
  | "boolean"
  | "number"
  | "text"
  | "textarea"
  | "date"
  | "select";

export interface FormFieldOption {
  label: string;
  value: string;
}

export interface FormFieldValidation {
  min?: number | null;
  max?: number | null;
  pattern?: string | null;
}

export interface FormField {
  key: string;
  label: string;
  type: FormFieldType;
  required?: boolean;
  help_text?: string;
  options?: FormFieldOption[];
  validation?: FormFieldValidation | null;
  sort_order: number;
  applies_to_child?: boolean;
}

export interface FormSchema {
  id: string;
  version: number;
  is_active: boolean;
  fields: FormField[];
  created_by: string;
  created_at: string;
}

interface BackendEnvelope<T> {
  status?: string;
  data?: T;
  message?: string;
  error?: string;
}

const SCHEMA_PATH = "/api/enrollment/schema";

async function readJSON<T>(response: Response): Promise<T> {
  const raw = (await response.json()) as BackendEnvelope<T>;
  if (
    raw &&
    typeof raw === "object" &&
    "data" in raw &&
    raw.data !== undefined
  ) {
    return raw.data as T;
  }
  return raw as unknown as T;
}

/**
 * Fetches the currently active form schema for the tenant in session.
 * Returns null when no active schema has been published yet (404).
 */
export async function fetchActiveSchema(): Promise<FormSchema | null> {
  const response = await fetch(SCHEMA_PATH, { cache: "no-store" });
  if (response.status === 404) {
    return null;
  }
  if (!response.ok) {
    const errorText = await response.text();
    logger.error("schema_fetch_failed", {
      status: response.status,
      error: errorText,
    });
    throw new Error(`Failed to load form schema (HTTP ${response.status})`);
  }
  return readJSON<FormSchema>(response);
}

/**
 * Lists every schema version for the current tenant (newest first).
 * The Phasen admin page uses this to populate the "use schema X for
 * this phase" dropdown.
 */
export async function listSchemas(): Promise<FormSchema[]> {
  const response = await fetch(`${SCHEMA_PATH}/versions`, {
    cache: "no-store",
  });
  if (!response.ok) {
    const errorText = await response.text();
    logger.error("schema_list_failed", {
      status: response.status,
      error: errorText,
    });
    throw new Error(
      `Failed to load form schema list (HTTP ${response.status})`,
    );
  }
  const list = await readJSON<FormSchema[]>(response);
  return Array.isArray(list) ? list : [];
}

/**
 * Public variant of fetchActiveSchema for the parent enrollment form.
 * Slug-gated, no JWT — backend resolves the tenant from the URL param
 * and returns only the active schema's fields. Returns null when no
 * schema has been published yet (404), letting the form fall back to
 * core fields only without erroring out.
 */
export interface PublicFormSchema {
  id: string;
  version: number;
  fields: FormField[];
}

export async function fetchPublicActiveSchema(
  tenantSlug: string,
  phaseId: string,
): Promise<PublicFormSchema | null> {
  const response = await fetch(
    `/api/enrollment/schema/public/${encodeURIComponent(
      tenantSlug,
    )}/${encodeURIComponent(phaseId)}`,
    { cache: "no-store" },
  );
  if (response.status === 404) {
    return null;
  }
  if (!response.ok) {
    const errorText = await response.text();
    logger.error("public_schema_fetch_failed", {
      status: response.status,
      error: errorText,
    });
    throw new Error(
      `Failed to load public form schema (HTTP ${response.status})`,
    );
  }
  return readJSON<PublicFormSchema>(response);
}

/**
 * Publishes a new schema version with the given fields, marking it
 * active and deactivating the previous version. Returns the newly
 * created schema row.
 */
export async function publishSchema(fields: FormField[]): Promise<FormSchema> {
  const response = await fetch(SCHEMA_PATH, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ fields }),
  });
  if (!response.ok) {
    const payload = (await response
      .json()
      .catch(() => ({}))) as BackendEnvelope<unknown>;
    const message =
      payload.error ?? payload.message ?? `HTTP ${response.status}`;
    logger.error("schema_publish_failed", {
      status: response.status,
      message,
    });
    throw new Error(message);
  }
  return readJSON<FormSchema>(response);
}

/** Convenience: returns a fresh, blank field with sensible defaults. */
export function blankField(sortOrder: number): FormField {
  return {
    key: "",
    label: "",
    type: "text",
    required: false,
    sort_order: sortOrder,
    applies_to_child: false,
  };
}
