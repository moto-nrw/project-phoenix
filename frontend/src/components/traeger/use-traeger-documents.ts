"use client";

// Daten der geteilten Dokumente des Trägers (#4692).

import { useSession } from "next-auth/react";
import { metadataLoadError } from "./metadata-load-error";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { useSWRAuth } from "~/lib/swr";
import { traegerKeys } from "~/lib/swr/portal-keys";
import {
  fetchTraegerDocumentAudiences,
  fetchTraegerDocuments,
} from "~/lib/traeger-documents-api";

/** Alle Dokumente des Trägers mit ihren Freigaben. */
export function useTraegerDocuments() {
  const { data: session } = useSession();
  const response = useSWRAuth(
    session?.user.scope === "org" ? traegerKeys.documents() : null,
    fetchTraegerDocuments,
    { keepPreviousData: false },
  );
  const error = useSwrLoadError(
    response.error,
    "die Liste der Dokumente",
    response.mutate,
  );
  return {
    ...response,
    loadError: metadataLoadError(
      error,
      response.data !== undefined,
      response.error,
    ),
  };
}

/**
 * Die Schulen mit ihren Gruppen, für eine Freigabe. Nur wer freigeben darf,
 * lädt sie; `enabled` ist sonst falsch.
 */
export function useTraegerDocumentAudiences(enabled: boolean) {
  const { data: session } = useSession();
  return useSWRAuth(
    enabled && session?.user.scope === "org"
      ? traegerKeys.documentAudiences()
      : null,
    fetchTraegerDocumentAudiences,
    { keepPreviousData: false },
  );
}
