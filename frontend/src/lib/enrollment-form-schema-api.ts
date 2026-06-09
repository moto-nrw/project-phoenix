import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "EnrollmentFormSchemaAPI" });

const SCHEMA_ERROR_MESSAGES: Record<string, string> = {
  "enrollment.schema_has_phases":
    "Diese Formularvorlage wird noch in einer Anmeldephase verwendet.",
  "enrollment.schema_has_requests":
    "Diese Formularvorlage wurde bereits für Anmeldungen verwendet und kann nicht gelöscht werden.",
};

/** Field types accepted by the backend FormFieldType enum. */
export type FormFieldType =
  | "boolean"
  | "number"
  | "text"
  | "textarea"
  | "date"
  | "select"
  | "information"
  | "phone_list"
  | "weekday_schedule"
  | "weekday_boolean"
  | "contact_list";

/**
 * Source of a field's visibility condition. Mirrors the backend
 * ConditionSource* constants in models/enrollment/form_schema.go.
 * - "field": another custom boolean/select field in the same schema
 * - "grade_level": the per-child core field target_grade_level
 * - "care_offering": the child's selected care offering IDs
 */
export type ConditionSource = "field" | "grade_level" | "care_offering";

/** Visibility operators (backend ConditionOp* constants). */
export type ConditionOperator = "eq" | "neq" | "not_empty" | "includes";

/**
 * Makes a field appear only when a controlling value matches. Mirrors
 * backend VisibilityCondition. null/undefined = always visible.
 */
export interface VisibilityCondition {
  source: ConditionSource;
  /** Controlling custom field key (only when source === "field"). */
  field?: string;
  operator: ConditionOperator;
  /** Compared value; omitted for the "not_empty" operator. */
  value?: string | number | boolean | null;
}

/**
 * Reserved targets that link a form field to a downstream Stammdaten
 * column / association table. Empty string (or absent) keeps the
 * field as free custom_data only. Keep in sync with
 * backend/models/enrollment/form_schema.go ReservedTargets.
 *
 * Photo consent and guardian phone number are NOT picker options:
 * the public base form already collects them (consent_flags.photo
 * checkbox + guardian_phone input). The decision service writes
 * them onto the Student / guardian_phone_numbers automatically.
 */
export type FormFieldTarget =
  | ""
  | "student.health_info"
  | "student.extra_info"
  // student.bus_days is the canonical Buskind target (#1582). student.bus is a
  // legacy alias kept so older saved schemas still resolve; it is not offered
  // in the picker for new fields.
  | "student.bus_days"
  | "student.bus"
  | "student.pickup_status"
  | "schedule.pickup"
  | "schedule.arrival"
  | "student.contacts";

export interface ReservedTargetSpec {
  type: FormFieldType;
  appliesToChild: boolean;
  label: string;
}

/** Mirror of the backend ReservedTargets map, for the admin editor. */
export const RESERVED_TARGETS: Record<
  Exclude<FormFieldTarget, "">,
  ReservedTargetSpec
> = {
  "student.health_info": {
    type: "textarea",
    appliesToChild: true,
    label: "Gesundheitsinformationen",
  },
  "student.extra_info": {
    type: "textarea",
    appliesToChild: true,
    label: "Hinweise an die Betreuung",
  },
  "student.bus_days": {
    type: "weekday_boolean",
    appliesToChild: true,
    label: "Buskind",
  },
  "student.bus": {
    type: "weekday_boolean",
    appliesToChild: true,
    label: "Buskind",
  },
  "student.pickup_status": {
    type: "weekday_boolean",
    appliesToChild: true,
    label: "Abholregelung",
  },
  "schedule.pickup": {
    type: "weekday_schedule",
    appliesToChild: true,
    label: "Abholzeiten",
  },
  "schedule.arrival": {
    type: "weekday_schedule",
    appliesToChild: true,
    label: "Ankunftszeiten",
  },
  "student.contacts": {
    type: "contact_list",
    appliesToChild: true,
    label: "Weitere Kontakte / Abholberechtigte / Notfallkontakte",
  },
};

interface FormFieldOption {
  label: string;
  value: string;
}

interface FormFieldValidation {
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
  /**
   * Body text for `information` blocks (plain text, newlines preserved).
   * Empty/undefined for every other field type.
   */
  content?: string;
  options?: FormFieldOption[];
  validation?: FormFieldValidation | null;
  sort_order: number;
  applies_to_child?: boolean;
  /**
   * Optional link to a Stammdaten column / association table. When
   * set, the decision service copies the value onto the matching
   * downstream record at approval time. Empty/undefined = free
   * custom field, stays in custom_data.
   */
  target?: FormFieldTarget;
  /**
   * Optional show-if rule. When set, the field is only rendered to
   * parents (and only validated/submitted) when the condition matches.
   * null/undefined = always visible.
   */
  visible_when?: VisibilityCondition | null;
}

export type CoreRequirementKey = "guardian_phone";

export type CoreRequirements = Partial<Record<CoreRequirementKey, boolean>>;

export interface FormSchema {
  id: string;
  name: string;
  version: number;
  is_active: boolean;
  fields: FormField[];
  core_requirements?: CoreRequirements;
  created_by: string;
  created_at: string;
}

interface BackendEnvelope<T> {
  status?: string;
  data?: T;
  message?: string;
  error?: string;
  code?: string;
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

export async function fetchSchemaById(id: string): Promise<FormSchema> {
  const response = await fetch(`${SCHEMA_PATH}/${encodeURIComponent(id)}`, {
    cache: "no-store",
  });
  if (!response.ok) {
    const errorText = await response.text();
    logger.error("schema_fetch_by_id_failed", {
      status: response.status,
      error: errorText,
    });
    throw new Error(`Failed to load form schema (HTTP ${response.status})`);
  }
  return readJSON<FormSchema>(response);
}

export function latestSchemasByName(schemas: FormSchema[]): FormSchema[] {
  const latest = new Map<string, FormSchema>();
  for (const schema of schemas) {
    const existing = latest.get(schema.name);
    if (
      !existing ||
      schema.version > existing.version ||
      (schema.version === existing.version &&
        schema.created_at > existing.created_at)
    ) {
      latest.set(schema.name, schema);
    }
  }
  return [...latest.values()].sort((a, b) =>
    b.created_at.localeCompare(a.created_at),
  );
}

/**
 * Public variant of fetchActiveSchema for the parent enrollment form.
 * Slug-gated, no JWT. Backend resolves the tenant from the URL param
 * and returns only the active schema's fields. Returns null when no
 * schema has been published yet (404), letting the form fall back to
 * core fields only without erroring out.
 */
export interface PublicFormSchema {
  id: string;
  version: number;
  fields: FormField[];
  core_requirements?: CoreRequirements;
}

/**
 * Public Cloudflare Turnstile config for a tenant. enabled mirrors the
 * server-side enrollment.require_captcha setting; site_key is the
 * public Turnstile site key (safe to render in the widget). When
 * site_key is empty the widget is hidden and submit falls through;
 * the backend's IsEnabled check still gates verification.
 */
export interface PublicCaptchaConfig {
  enabled: boolean;
  site_key: string;
}

export async function fetchPublicCaptchaConfig(
  tenantSlug: string,
): Promise<PublicCaptchaConfig | null> {
  const response = await fetch(
    `/api/enrollment/captcha-config/${encodeURIComponent(tenantSlug)}`,
    { cache: "no-store" },
  );
  if (!response.ok) {
    logger.warn("public_captcha_config_failed", {
      status: response.status,
    });
    return null;
  }
  return readJSON<PublicCaptchaConfig>(response);
}

export interface PublicLegalBlock {
  key: "agb" | "data_processing" | "email_contact" | "photo";
  kind: "terms" | "privacy_notice" | "notice" | "consent";
  title: string;
  label: string;
  text: string;
  required: boolean;
}

/**
 * Per-tenant legal documents and the configured blocks shown on the public
 * form. Empty raw strings remain for compatibility; `blocks` is the canonical
 * render/validation contract for the form.
 */
export interface PublicLegalTexts {
  agb: string;
  dsgvo: string;
  email_contact: string;
  photo: string;
  /**
   * Mirrors enrollment.legal_terms_enabled. The AGB block is shown only
   * when this is true and an AGB/Ganztag Info-Brief text is configured.
   */
  terms_enabled: boolean;
  blocks: PublicLegalBlock[];
}

/**
 * Fetches the per-tenant legal documents. Unconfigured texts come back
 * as empty strings and produce no legal block. A non-OK response
 * (settings/DB/JSON failure) THROWS rather than returning null: the
 * caller must fail closed instead of collecting an incomplete legal state.
 */
export async function fetchPublicLegalTexts(
  tenantSlug: string,
): Promise<PublicLegalTexts> {
  const response = await fetch(
    `/api/enrollment/legal/${encodeURIComponent(tenantSlug)}`,
    { cache: "no-store" },
  );
  if (!response.ok) {
    logger.error("public_legal_texts_failed", {
      status: response.status,
    });
    throw new Error(`legal texts request failed: ${response.status}`);
  }
  return readJSON<PublicLegalTexts>(response);
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
 * Creates a new named schema (version 1). Backend rejects when a
 * schema with the same name already exists. Use updateSchema in
 * that case.
 */
export async function createSchema(
  name: string,
  fields: FormField[],
  coreRequirements: CoreRequirements = {},
): Promise<FormSchema> {
  const response = await fetch(SCHEMA_PATH, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name, fields, core_requirements: coreRequirements }),
  });
  if (!response.ok) {
    const payload = (await response
      .json()
      .catch(() => ({}))) as BackendEnvelope<unknown>;
    const message =
      payload.error ?? payload.message ?? `HTTP ${response.status}`;
    logger.error("schema_create_failed", {
      status: response.status,
      message,
    });
    throw new Error(message);
  }
  return readJSON<FormSchema>(response);
}

/**
 * Publishes a new version of an existing named schema. The new row
 * inherits the source schema's name and uses max(version)+1 for that
 * name. Older versions stay in place for historical submissions, and
 * phases using an older version of this template are moved to the new
 * schema_id.
 */
export async function updateSchema(
  id: string,
  fields: FormField[],
  coreRequirements?: CoreRequirements,
): Promise<FormSchema> {
  const body =
    coreRequirements === undefined
      ? { fields }
      : { fields, core_requirements: coreRequirements };
  const response = await fetch(`${SCHEMA_PATH}/${encodeURIComponent(id)}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!response.ok) {
    const payload = (await response
      .json()
      .catch(() => ({}))) as BackendEnvelope<unknown>;
    const message =
      payload.error ?? payload.message ?? `HTTP ${response.status}`;
    logger.error("schema_update_failed", {
      status: response.status,
      message,
    });
    throw new Error(message);
  }
  return readJSON<FormSchema>(response);
}

export async function deleteSchema(id: string): Promise<void> {
  const response = await fetch(`${SCHEMA_PATH}/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
  if (response.status === 204) return;
  if (!response.ok) {
    const payload = (await response
      .json()
      .catch(() => ({}))) as BackendEnvelope<unknown>;
    const message =
      (payload.code ? SCHEMA_ERROR_MESSAGES[payload.code] : undefined) ??
      payload.error ??
      payload.message ??
      `HTTP ${response.status}`;
    logger.error("schema_delete_failed", {
      status: response.status,
      message,
      code: payload.code,
    });
    throw new Error(message);
  }
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

/**
 * Convenience: returns a fresh, blank information block. Info blocks
 * collect no answer, so they carry a generated key (the admin never
 * sees it) and a body in `content`. The key is finalised on save.
 */
export function blankInfoField(sortOrder: number): FormField {
  return {
    key: "",
    label: "",
    type: "information",
    content: "",
    required: false,
    sort_order: sortOrder,
    applies_to_child: false,
  };
}
