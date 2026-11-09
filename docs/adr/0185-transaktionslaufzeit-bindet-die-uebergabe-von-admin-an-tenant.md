---
status: accepted
---

# Transaktionslaufzeit bindet die Übergabe von Admin an Tenant

Status: Angenommen. Entschieden von Yannick Mark Wenger am 09.11.2026.

[ADR 0161](0161-gleiche-transaktionsrahmen-werden-gebuendelt-admin-geht-nur-einseitig-in-den-tenant.md),
Schritt 4 (#5862), verlangt die Bindung
`NewTenantRuntime:unitOfWork.WithinAdminThenTenant#1` in
`modules/transactionruntime/compose`. Dieser Composition-Etat steht genau auf
seinem gemessenen Wert, und die Bindung braucht eine Anweisung und eine
Kommentarzeile mehr. Der Etat von `modules/transactionruntime/compose` steigt
dafür genau von 137 auf 139 LOC, die Summe `moduleComposeBudgetTotal` um 2.
Kein anderer Etat, Pass-through, Policy-Eintrag oder Composition-Inventarschlüssel
wächst.
