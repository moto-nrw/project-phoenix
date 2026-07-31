import { z } from "zod";

/**
 * @param {unknown} value
 * @returns {unknown}
 */
const emptyStringToUndefined = (value) =>
  typeof value === "string" && value.trim() === "" ? undefined : value;

const optionalUrl = z.preprocess(emptyStringToUndefined, z.url().optional());
const optionalString = z.preprocess(
  emptyStringToUndefined,
  z.union([z.string(), z.undefined()]),
);

export const serverEnvSchema = {
  API_URL: z.url(),
  AUTH_JWT_EXPIRY: z.string().min(1),
  AUTH_JWT_REFRESH_EXPIRY: z.string().min(1),
  NODE_ENV: z
    .enum(["development", "test", "production"])
    .default("development"),
  NEXTAUTH_URL: z.url(),
  NEXTAUTH_SECRET: z.string().min(1),
  TENANT_DOMAIN: z.string().min(1),
};

export const clientEnvSchema = {
  NEXT_PUBLIC_API_URL: z.url(),
  NEXT_PUBLIC_LOG_LEVEL: z
    .enum(["debug", "info", "warn", "error"])
    .default("info"),
  NEXT_PUBLIC_TENANT_DOMAIN: z.string().min(1),
  NEXT_PUBLIC_POSTHOG_KEY: optionalString,
  NEXT_PUBLIC_POSTHOG_HOST: optionalUrl,
  NEXT_PUBLIC_OPERATOR_HOSTNAME: z.string().min(1),
  NEXT_PUBLIC_PARENTS_HOSTNAME: z.string().min(1),
  NEXT_PUBLIC_SENTRY_DSN: optionalUrl,
  NEXT_PUBLIC_SENTRY_ENVIRONMENT: optionalString,
};

/**
 * @param {{ NEXT_PUBLIC_POSTHOG_KEY?: string, NEXT_PUBLIC_POSTHOG_HOST?: string }} values
 * @param {z.RefinementCtx} ctx
 */
function requirePostHogHost(values, ctx) {
  if (values.NEXT_PUBLIC_POSTHOG_KEY && !values.NEXT_PUBLIC_POSTHOG_HOST) {
    ctx.addIssue({
      code: "custom",
      path: ["NEXT_PUBLIC_POSTHOG_HOST"],
      message: "Required when NEXT_PUBLIC_POSTHOG_KEY is set",
    });
  }
  if (
    values.NEXT_PUBLIC_POSTHOG_HOST &&
    URL.canParse(values.NEXT_PUBLIC_POSTHOG_HOST) &&
    !["http:", "https:"].includes(
      new URL(values.NEXT_PUBLIC_POSTHOG_HOST).protocol,
    )
  ) {
    ctx.addIssue({
      code: "custom",
      path: ["NEXT_PUBLIC_POSTHOG_HOST"],
      message: "Must use the http or https protocol",
    });
  }
}

const buildEnvSchema = z
  .object({
    API_URL: serverEnvSchema.API_URL,
    TENANT_DOMAIN: serverEnvSchema.TENANT_DOMAIN,
    NEXT_PUBLIC_API_URL: clientEnvSchema.NEXT_PUBLIC_API_URL,
    NEXT_PUBLIC_TENANT_DOMAIN: clientEnvSchema.NEXT_PUBLIC_TENANT_DOMAIN,
    NEXT_PUBLIC_POSTHOG_KEY: clientEnvSchema.NEXT_PUBLIC_POSTHOG_KEY,
    NEXT_PUBLIC_POSTHOG_HOST: clientEnvSchema.NEXT_PUBLIC_POSTHOG_HOST,
    NEXT_PUBLIC_OPERATOR_HOSTNAME:
      clientEnvSchema.NEXT_PUBLIC_OPERATOR_HOSTNAME,
    NEXT_PUBLIC_PARENTS_HOSTNAME: clientEnvSchema.NEXT_PUBLIC_PARENTS_HOSTNAME,
    NEXT_PUBLIC_SENTRY_DSN: clientEnvSchema.NEXT_PUBLIC_SENTRY_DSN,
    NEXT_PUBLIC_SENTRY_ENVIRONMENT:
      clientEnvSchema.NEXT_PUBLIC_SENTRY_ENVIRONMENT,
  })
  .superRefine(requirePostHogHost);

const runtimeEnvSchema = z
  .object({
    ...serverEnvSchema,
    ...clientEnvSchema,
    METRICS_BEARER_TOKEN: z.string().trim().min(1),
  })
  .superRefine(requirePostHogHost);

export function isSkipEnvValidationEnabled() {
  return process.env.SKIP_ENV_VALIDATION === "true";
}

/**
 * @param {z.ZodError} error
 * @returns {string}
 */
function formatEnvIssues(error) {
  return error.issues
    .map((issue) => {
      const key = issue.path.join(".") || "(root)";
      return `${key}: ${issue.message}`;
    })
    .join("; ");
}

/**
 * @template {z.ZodType} T
 * @param {T} schema
 * @param {Record<string, string | undefined> | NodeJS.ProcessEnv} values
 * @param {string} label
 * @returns {z.infer<T>}
 */
function validateEnv(schema, values, label) {
  const result = schema.safeParse(values);
  if (result.success) {
    return result.data;
  }

  throw new Error(
    `${label} validation failed: ${formatEnvIssues(result.error)}`,
  );
}

/**
 * @param {Record<string, string | undefined> | NodeJS.ProcessEnv} [values]
 */
export function validateBuildEnv(values = process.env) {
  return validateEnv(buildEnvSchema, values, "Frontend build env");
}

/**
 * @param {Record<string, string | undefined> | NodeJS.ProcessEnv} [values]
 */
export function validateRuntimeEnv(values = process.env) {
  return validateEnv(runtimeEnvSchema, values, "Frontend runtime env");
}
