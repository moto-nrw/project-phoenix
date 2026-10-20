/** The backend path of the invitation a mailed link names (ADR 0075). */
export function invitationPath(
  params: Record<string, string | string[] | undefined>,
  suffix = "",
): string {
  const schoolId = String(params.schoolId ?? "");
  const token = String(params.token ?? "");
  return `/caterer/invitations/${encodeURIComponent(schoolId)}/${encodeURIComponent(token)}${suffix}`;
}
