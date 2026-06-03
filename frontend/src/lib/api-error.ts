export type ApiError = Error & {
  status?: number;
  retryAfterSeconds?: number;
  code?: string;
  // Structured payload mirrored from the backend's ErrResponse.details.
  // Populated for codes that carry follow-up data (e.g. reopen_status_conflict
  // returns the conflicting session id and the existing/requested statuses so
  // the UI doesn't have to look them up in local state).
  details?: Record<string, unknown>;
};
