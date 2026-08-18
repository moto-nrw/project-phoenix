import { createLogger } from "~/lib/logger";
import { readEnrollmentError } from "~/lib/enrollment-error-messages";

const logger = createLogger({ component: "EnrollmentFormSchemaAPI" });

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
  | "weekday_mode"
  | "weekday_multi_mode"
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
  // student.departure is the canonical unified target (#1610): per weekday how
  // the child leaves (alone/bus/pickup). student.bus_days / student.bus /
  // student.pickup_status are legacy aliases kept so older saved schemas still
  // resolve; they are not offered in the picker for new fields.
  | "student.allowed_departure_modes"
  | "student.departure"
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
  "student.allowed_departure_modes": {
    type: "weekday_multi_mode",
    appliesToChild: true,
    label: "Erlaubte Heimwege",
  },
  "student.departure": {
    type: "weekday_mode",
    appliesToChild: true,
    label: "Geh- und Abholregelung",
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
  /**
   * Fixed pickup times for a `weekday_schedule` field. When non-empty,
   * the public form renders a dropdown limited to these `HH:MM` values
   * per weekday instead of a free time input, and the backend rejects
   * any off-list time. Empty/undefined = free time entry.
   */
  allowed_times?: string[];
  /**
   * Heimweg-Beschränkung (#2381) for the `weekday_multi_mode` field
   * targeting `student.allowed_departure_modes`: target grade levels
   * whose children may pick at most ONE departure mode per weekday.
   * Parents still choose which one. Empty/undefined = unrestricted
   * multi-select; only valid on that field, the backend rejects it
   * everywhere else.
   */
  single_mode_grades?: number[];
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

type LegalBlockKind = "terms" | "privacy_notice" | "notice" | "consent";

type LegalBlockSource = "standard" | "custom";

export type LegalBlockDisplayMode = "text" | "pdf";

export interface FormLegalBlock {
  key: string;
  kind: LegalBlockKind;
  title: string;
  label: string;
  text: string;
  required: boolean;
  enabled: boolean;
  sort_order: number;
  source?: LegalBlockSource;
  display_mode?: LegalBlockDisplayMode;
  document_url?: string;
}

export interface FormSchema {
  id: string;
  name: string;
  version: number;
  is_active: boolean;
  fields: FormField[];
  core_requirements?: CoreRequirements;
  legal_blocks?: FormLegalBlock[];
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
    throw await readEnrollmentError(
      response,
      "Formularvorlagen konnten nicht geladen werden",
      logger,
      "schema_list_failed",
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
    throw await readEnrollmentError(
      response,
      "Formularvorlage konnte nicht geladen werden",
      logger,
      "schema_fetch_by_id_failed",
    );
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
  legal_blocks?: FormLegalBlock[];
}

export function schemaToPublicFormSchema(schema: FormSchema): PublicFormSchema {
  return {
    id: schema.id,
    version: schema.version,
    fields: schema.fields,
    ...(schema.core_requirements === undefined
      ? {}
      : { core_requirements: schema.core_requirements }),
    ...(schema.legal_blocks === undefined
      ? {}
      : { legal_blocks: schema.legal_blocks }),
  };
}

export interface EnrollmentPreviewBootstrap {
  schema: FormSchema | null;
  assigned_phase_count: number;
  active_assigned_phase_count: number;
}

export async function fetchEnrollmentPreviewBootstrap(params: {
  schemaId?: string | null;
  base?: boolean;
}): Promise<EnrollmentPreviewBootstrap> {
  const search = new URLSearchParams();
  if (params.schemaId) search.set("schemaId", params.schemaId);
  if (params.base) search.set("base", "1");
  const response = await fetch(`${SCHEMA_PATH}/preview?${search.toString()}`, {
    cache: "no-store",
  });
  if (!response.ok) {
    throw await readEnrollmentError(
      response,
      "Formularvorschau konnte nicht geladen werden",
      logger,
      "schema_preview_bootstrap_failed",
    );
  }
  return readJSON<EnrollmentPreviewBootstrap>(response);
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
  key: string;
  kind: LegalBlockKind;
  title: string;
  label: string;
  text: string;
  required: boolean;
  sort_order?: number;
  source?: LegalBlockSource;
}

/**
 * Per-tenant legal documents and the configured blocks shown on the public
 * form. Empty raw strings remain for compatibility; `blocks` is the canonical
 * render/validation contract for the form.
 */
export interface PublicLegalTexts {
  agb: string;
  agb_document_url?: string;
  agb_display_mode?: "text" | "pdf";
  dsgvo: string;
  email_contact: string;
  photo: string;
  terms_enabled: boolean;
  dsgvo_enabled: boolean;
  email_contact_enabled: boolean;
  photo_enabled: boolean;
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
  phaseId?: string,
  options: { lateInviteToken?: string } = {},
): Promise<PublicLegalTexts> {
  const path = `/api/enrollment/legal/${encodeURIComponent(tenantSlug)}`;
  const params = new URLSearchParams();
  if (phaseId) params.set("phaseId", phaseId);
  if (options.lateInviteToken?.trim()) {
    params.set("late_invite", options.lateInviteToken.trim());
  }
  const query = params.toString();
  const url = query ? `${path}?${query}` : path;
  const response = await fetch(url, { cache: "no-store" });
  if (!response.ok) {
    throw await readEnrollmentError(
      response,
      "Rechtstexte konnten nicht geladen werden",
      logger,
      "public_legal_texts_failed",
    );
  }
  return readJSON<PublicLegalTexts>(response);
}

export async function fetchPublicActiveSchema(
  tenantSlug: string,
  phaseId: string,
  options: { lateInviteToken?: string } = {},
): Promise<PublicFormSchema | null> {
  const path = `/api/enrollment/schema/public/${encodeURIComponent(
    tenantSlug,
  )}/${encodeURIComponent(phaseId)}`;
  const trimmedToken = options.lateInviteToken?.trim();
  const url = trimmedToken
    ? `${path}?late_invite=${encodeURIComponent(trimmedToken)}`
    : path;
  const response = await fetch(url, { cache: "no-store" });
  if (response.status === 404) {
    return null;
  }
  if (!response.ok) {
    throw await readEnrollmentError(
      response,
      "Formular konnte nicht geladen werden",
      logger,
      "public_schema_fetch_failed",
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
  legalBlocks?: FormLegalBlock[],
): Promise<FormSchema> {
  const body =
    legalBlocks === undefined
      ? { name, fields, core_requirements: coreRequirements }
      : {
          name,
          fields,
          core_requirements: coreRequirements,
          legal_blocks: legalBlocks,
        };
  const response = await fetch(SCHEMA_PATH, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!response.ok) {
    throw await readEnrollmentError(
      response,
      "Formularvorlage konnte nicht erstellt werden",
      logger,
      "schema_create_failed",
    );
  }
  return readJSON<FormSchema>(response);
}

/**
 * Publishes a new version of an existing named schema. The new row
 * inherits the source schema's name and uses max(version)+1 for that
 * name. Older versions stay in place for historical submissions, and
 * phases using an older version of this template are moved to the new
 * schema_id.
 *
 * Pass `newName` to rename the whole lineage AND publish the new version
 * in ONE backend transaction (a combined "rename + edit" save). A failed
 * publish rolls the rename back, so there is no partial "renamed but
 * content unchanged" state. A 409 (`enrollment.schema_name_exists`) is
 * raised when the new name already identifies a different schema.
 */
export async function updateSchema(
  id: string,
  fields: FormField[],
  coreRequirements?: CoreRequirements,
  legalBlocks?: FormLegalBlock[],
  newName?: string,
): Promise<FormSchema> {
  const body: {
    name?: string;
    fields: FormField[];
    core_requirements?: CoreRequirements;
    legal_blocks?: FormLegalBlock[];
  } = { fields };
  if (newName !== undefined) body.name = newName;
  if (coreRequirements !== undefined) body.core_requirements = coreRequirements;
  if (legalBlocks !== undefined) body.legal_blocks = legalBlocks;
  const response = await fetch(`${SCHEMA_PATH}/${encodeURIComponent(id)}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!response.ok) {
    throw await readEnrollmentError(
      response,
      "Formularvorlage konnte nicht gespeichert werden",
      logger,
      "schema_update_failed",
    );
  }
  return readJSON<FormSchema>(response);
}

export async function uploadEnrollmentLegalDocument(
  file: File,
): Promise<string> {
  const formData = new FormData();
  formData.append("document", file);

  const response = await fetch("/api/enrollment/legal-documents", {
    method: "POST",
    body: formData,
  });

  if (!response.ok) {
    const message =
      response.status === 413
        ? "Die PDF-Datei darf maximal 10 MB groß sein."
        : response.status === 415
          ? "Bitte eine PDF-Datei hochladen."
          : "PDF-Datei konnte nicht hochgeladen werden";
    throw new Error(message);
  }

  const result = (await response.json()) as {
    data?: { document_url?: string };
  };
  const documentURL = result.data?.document_url;
  if (!documentURL) {
    throw new Error("PDF-Datei konnte nicht hochgeladen werden");
  }
  return documentURL;
}

export async function deleteEnrollmentLegalDocument(
  documentURL: string,
  options: { keepalive?: boolean } = {},
): Promise<void> {
  const filename = documentURL.trim().split("/").pop();
  if (!filename) return;

  const response = await fetch(
    `/api/enrollment/legal-documents/${encodeURIComponent(filename)}`,
    { keepalive: options.keepalive, method: "DELETE" },
  );
  if (!response.ok) {
    throw new Error("PDF-Datei konnte nicht entfernt werden");
  }
}

/**
 * Renames a logical schema. Every version row sharing the source's name
 * is renamed atomically, so the whole version lineage keeps one shared
 * name. This publishes no new version and leaves the form fields
 * untouched. The backend rejects (409) a name already used by a
 * different schema.
 */
export async function renameSchema(
  id: string,
  name: string,
): Promise<FormSchema> {
  const response = await fetch(`${SCHEMA_PATH}/${encodeURIComponent(id)}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name }),
  });
  if (!response.ok) {
    throw await readEnrollmentError(
      response,
      "Formularvorlage konnte nicht umbenannt werden",
      logger,
      "schema_rename_failed",
    );
  }
  return readJSON<FormSchema>(response);
}

export async function deleteSchema(id: string): Promise<void> {
  const response = await fetch(`${SCHEMA_PATH}/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
  if (response.status === 204) return;
  if (!response.ok) {
    throw await readEnrollmentError(
      response,
      "Formularvorlage konnte nicht gelöscht werden",
      logger,
      "schema_delete_failed",
    );
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
