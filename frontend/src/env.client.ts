function required(name: string, value: string | undefined): string {
  if (!value?.trim()) {
    throw new Error(`${name} is not set`);
  }
  return value;
}

function optional(value: string | undefined): string | undefined {
  return value?.trim() ? value : undefined;
}

export const clientEnv = {
  NEXT_PUBLIC_API_URL: required(
    "NEXT_PUBLIC_API_URL",
    process.env.NEXT_PUBLIC_API_URL,
  ),
  NEXT_PUBLIC_TENANT_DOMAIN: required(
    "NEXT_PUBLIC_TENANT_DOMAIN",
    process.env.NEXT_PUBLIC_TENANT_DOMAIN,
  ),
  NEXT_PUBLIC_OPERATOR_HOSTNAME: required(
    "NEXT_PUBLIC_OPERATOR_HOSTNAME",
    process.env.NEXT_PUBLIC_OPERATOR_HOSTNAME,
  ),
  NEXT_PUBLIC_PARENTS_HOSTNAME: required(
    "NEXT_PUBLIC_PARENTS_HOSTNAME",
    process.env.NEXT_PUBLIC_PARENTS_HOSTNAME,
  ),
  NEXT_PUBLIC_SCHOOL_HOSTNAME: required(
    "NEXT_PUBLIC_SCHOOL_HOSTNAME",
    process.env.NEXT_PUBLIC_SCHOOL_HOSTNAME,
  ),
  // Empty in staging and local development: the analytics sends nothing.
  NEXT_PUBLIC_POSTHOG_KEY: optional(process.env.NEXT_PUBLIC_POSTHOG_KEY),
} as const;
