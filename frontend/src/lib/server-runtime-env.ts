export function getMetricsBearerToken(): string {
  const token = process.env.METRICS_BEARER_TOKEN?.trim();
  if (!token) {
    throw new Error("METRICS_BEARER_TOKEN is required");
  }
  return token;
}

export function validateServerRuntimeEnv(): void {
  getMetricsBearerToken();
}
