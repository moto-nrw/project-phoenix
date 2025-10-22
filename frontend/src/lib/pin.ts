// lib/pin.ts
// Branded type and helpers for 4-digit PIN values

export type PIN = string & { readonly __brand: 'PIN4' };

export function isPIN(value: string): value is PIN {
  return /^\d{4}$/.test(value);
}

export function asPIN(value: string): PIN {
  if (!isPIN(value)) {
    throw new Error('PIN muss aus genau 4 Ziffern bestehen');
  }
  // Cast through unknown to satisfy eslint rule and enforce brand
  return (value as unknown) as PIN;
}

export function validatePinOrThrow(value: string): PIN {
  return asPIN(value);
}
