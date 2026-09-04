function required(name: string, value: string | undefined): string {
  if (!value?.trim()) {
    throw new Error(`${name} is not set`);
  }
  return value;
}

function optional(value: string | undefined): string | undefined {
  return value?.trim() ? value : undefined;
}

const postHogKey = optional(process.env.NEXT_PUBLIC_POSTHOG_KEY);
const postHogHost = optional(process.env.NEXT_PUBLIC_POSTHOG_HOST);

if (postHogKey && !postHogHost) {
  throw new Error(
    "NEXT_PUBLIC_POSTHOG_HOST is required when NEXT_PUBLIC_POSTHOG_KEY is set",
  );
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
  NEXT_PUBLIC_POSTHOG_KEY: postHogKey,
  NEXT_PUBLIC_POSTHOG_HOST: postHogHost,
} as const;
