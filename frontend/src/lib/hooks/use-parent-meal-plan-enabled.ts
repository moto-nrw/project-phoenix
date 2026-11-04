"use client";

import { type Child } from "~/lib/parent-api";
import { useParentChildren, useParentFeatures } from "./use-parent-resources";

export function useParentMealPlanEnabled(
  enabled: boolean,
  children?: readonly Child[] | null,
): boolean {
  const linked = useParentChildren(enabled && children === undefined);
  const features = useParentFeatures(
    enabled ? (children ?? linked.data) : undefined,
  );
  return Object.values(features.data ?? {}).some(
    (f) => f.meal_plan_enabled === true,
  );
}
