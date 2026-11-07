"use client";

import { useTranslations } from "next-intl";
import type { SignaturePadLabels } from "~/components/ui/signature-pad";

/**
 * The translated labels of the signature field in the parents portal: the
 * Einverständnis (#4785) and the care contract (#4679) share them.
 */
export function useSignaturePadLabels(): SignaturePadLabels {
  const t = useTranslations("parentDeclaration.signature");
  return {
    label: t("label"),
    hint: t("hint"),
    canvas: t("canvas"),
    clear: t("clear"),
    typeInstead: t("typeInstead"),
    drawInstead: t("drawInstead"),
    typedName: t("typedName"),
    confirm: t("confirm"),
    confirmed: t("confirmed"),
    confirmedTyped: t("confirmedTyped"),
    change: t("change"),
    preview: t("preview"),
    emptyDrawing: t("emptyDrawing"),
    nameTooShort: t("nameTooShort"),
  };
}
