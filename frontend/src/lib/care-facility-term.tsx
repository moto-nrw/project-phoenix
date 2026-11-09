"use client";

import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { useTranslations } from "next-intl";
import { normalizeCareFacilityTerm } from "~/lib/care-facility-term-value";
import { useTenantSafe } from "~/lib/tenant-context";

export { normalizeCareFacilityTerm } from "~/lib/care-facility-term-value";

interface ScopeValue {
  readonly term: string | undefined;
  readonly setTerm: (term: string | undefined) => void;
}

const CareFacilityTermContext = createContext<ScopeValue | null>(null);

/**
 * A page part whose school is not the tenant in context: the token status
 * page on the parents host, the embedded parent form, the edit form. It holds
 * the term a response named; `useProvideCareFacilityTerm` fills it in once
 * the response arrived.
 */
export function CareFacilityTermScope({
  term,
  children,
}: {
  readonly term?: string;
  readonly children: ReactNode;
}) {
  const [reported, setTerm] = useState<string | undefined>(undefined);
  const value = useMemo<ScopeValue>(
    () => ({ term: reported ?? term, setTerm }),
    [reported, term],
  );
  return (
    <CareFacilityTermContext.Provider value={value}>
      {children}
    </CareFacilityTermContext.Provider>
  );
}

/** Reports the term a loaded response carries to the surrounding scope. */
export function useProvideCareFacilityTerm(term: string | undefined): void {
  const scope = useContext(CareFacilityTermContext);
  const setTerm = scope?.setTerm;
  useEffect(() => {
    if (term !== undefined) setTerm?.(normalizeCareFacilityTerm(term));
  }, [setTerm, term]);
}

/**
 * The term for the texts in view: the scope's school first, then the tenant
 * in context, else „OGS“ (a page that knows no school yet, such as an
 * invalid status link).
 */
export function useCareFacilityTerm(): string {
  const scope = useContext(CareFacilityTermContext);
  const tenant = useTenantSafe()?.tenant;
  if (scope?.term !== undefined) return normalizeCareFacilityTerm(scope.term);
  return normalizeCareFacilityTerm(tenant?.careFacilityTerm);
}

type Translator = ReturnType<typeof useTranslations>;
type Namespace = NonNullable<Parameters<typeof useTranslations>[0]>;
type TranslationValues = Record<string, unknown> | undefined;

/**
 * `useTranslations` with the school's term as the `{careTerm}` value of
 * every message. A value the caller passes keeps its own content; messages
 * without the placeholder ignore it. Each render keeps its own term, so
 * suspended or discarded renders cannot change committed translations.
 */
export function useCareTermTranslations<N extends Namespace>(
  namespace: N,
  /** A term the caller knows better than the scope, such as the school of one loaded record. */
  term?: string,
): ReturnType<typeof useTranslations<N>> {
  const t = useTranslations(namespace);
  const scopeTerm = useCareFacilityTerm();
  const careTerm =
    term === undefined ? scopeTerm : normalizeCareFacilityTerm(term);
  return useMemo(
    () =>
      withCareTerm(
        t as unknown as Translator,
        careTerm,
      ) as unknown as ReturnType<typeof useTranslations<N>>,
    [t, careTerm],
  );
}

function withCareTerm(t: Translator, careTerm: string): Translator {
  const merge = (values: TranslationValues) => ({
    careTerm,
    ...values,
  });
  // The translator's overloads are typed per key; the wrapper forwards the
  // same arguments with one more value, so it keeps the original type.
  const call = t as unknown as (...args: unknown[]) => unknown;
  const source = t as unknown as Record<
    "rich" | "markup" | "raw" | "has",
    (...args: unknown[]) => unknown
  >;
  const wrapped = (
    key: unknown,
    values?: TranslationValues,
    formats?: unknown,
  ) => call(key, merge(values), formats);
  Object.assign(wrapped, {
    rich: (key: unknown, values?: TranslationValues, formats?: unknown) =>
      source.rich(key, merge(values), formats),
    markup: (key: unknown, values?: TranslationValues, formats?: unknown) =>
      source.markup(key, merge(values), formats),
    raw: (key: unknown) => source.raw(key),
    has: (key: unknown) => source.has(key),
  });
  return wrapped as unknown as Translator;
}
