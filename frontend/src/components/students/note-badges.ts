/**
 * Kennzeichen einer Notiz, die die Lehrkraft der Klasse in moto schule liest
 * (#4797): die Props des `StatusBadge` an der Notiz, in der Chronik wie bei
 * den dauerhaften Hinweisen.
 */
export const NOTE_SHARED_WITH_SCHOOL_BADGE = {
  compact: true,
  tone: "green",
  label: "Für die Schule freigegeben",
  accessibleLabel:
    "Für die Schule freigegeben: Die Lehrkraft der Klasse liest diese Notiz.",
} as const;
