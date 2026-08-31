"use client";

import { useSession } from "next-auth/react";

import {
  canReviewChangeRequests,
  resolveChangeRequestAccess,
  type ParentRequestReviewAccess,
} from "~/lib/change-request-access";
import { fetchChangeRequestAccess } from "~/lib/change-requests-api";
import { isAdmin } from "~/lib/auth-utils";
import { useShellAuth } from "~/lib/shell-auth-context";
import { useSWRAuth } from "~/lib/swr";

export const CHANGE_REQUEST_ACCESS_SWR_KEY = "change-request-access";

/**
 * Verbindet die langlebigen JWT-Rechte mit dem aktuellen serverseitigen
 * Prüfbereich. Gruppenleitungen und Vertretungen können sich während einer
 * Sitzung ändern; deshalb darf das JWT allein das Anfragen-Modul nicht öffnen.
 */
export function useChangeRequestAccess() {
  const { data: session, status } = useSession();
  const { mode } = useShellAuth();
  const admin = isAdmin(session);
  const needsParentReviewAccess =
    status === "authenticated" &&
    mode === "teacher" &&
    !admin &&
    canReviewChangeRequests(session);

  const { data, error, isLoading, mutate } =
    useSWRAuth<ParentRequestReviewAccess>(
      needsParentReviewAccess ? CHANGE_REQUEST_ACCESS_SWR_KEY : null,
      fetchChangeRequestAccess,
      {
        revalidateOnFocus: true,
        revalidateOnReconnect: true,
        shouldRetryOnError: false,
      },
    );

  const access = resolveChangeRequestAccess(
    session,
    admin ? "admin" : (data ?? "none"),
  );

  return {
    ...access,
    isLoading:
      needsParentReviewAccess && data === undefined && error === undefined
        ? isLoading
        : false,
    error,
    refresh: mutate,
  };
}
