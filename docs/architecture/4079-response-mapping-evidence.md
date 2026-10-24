# #4079 response mapping evidence

> **Historical record.** State at `24ed2b1194` (2026-10-18). #2580 and #4069 are closed;
> counts, budgets and open issues below describe that state, not today.
> Final state: [`backend/architecture/WORKPLAN.md`](../../backend/architecture/WORKPLAN.md#endstand).

Immutable base: `a300992867de78719bbf79720f822495e1be95d1`.
This covers the bounded single-key retirement, not the archived full candidate.

## Observed results

- Architecture unit package: PASS, 121.748s.
- Request Review public package: PASS, 0.246s.
- Request Review composition package: PASS, 3.017s, including native queue
  error/filter/access scenarios and lowercase SHA-256 fingerprint encoding.
- Existing focused student request/history/response scenarios: PASS, 1.282s.
- Root `TestFullProductionRouterGolden`: PASS, 1.903s.

Commands, through the repository Go toolchain and the retained private cache:

```sh
go test -C backend -tags=testhooks -parallel 8 -p 2 ./internal/architecture ./modules/requestreview/... -count=1
go test -C backend -tags=testhooks -parallel 8 -p 2 ./modules/peopledirectory/inbound/students ./api -run '^(TestFullProductionRouterGolden|TestAggregatedChangeRequests|TestCareRequest|TestMasterData|TestExcused|TestOffering|TestToOffering|TestPickup)' -count=1
```

## Native-source and normalized golden proof

The moved files `care_responses.go` and `request_responses.go` have identical
function source after removing their former `requestreview.` self-qualification
and replacing the care response's `pickupImpactFingerprint` call with
`securityruntime.Fingerprint`. Both primitives compute lowercase hexadecimal
SHA-256: the former uses `fmt.Sprintf("%x", sha256.Sum256(content))`, the latter
`hex.EncodeToString(sum[:])`. The approval composition keeps its original
fingerprint function unchanged. Its existing encoding test remains unchanged.

`GermanOfferingDiffLabel` is byte-exact after its move; its old definition is
removed. Every response mapper has one implementation; no DTO was copied.
Only mapper call qualifications and imports change in existing queue tests.
All original assertions and expected response values remain intact.

Comparing every `backend/api/testdata/*.golden` file against the immutable base
returns identical bytes. The normalized diff is empty: **zero path renames and
zero substitutions**. No Root or IoT source file changed. There are no route,
transaction, permission, session, error-envelope or response-contract changes.

## Shrink-only measurements

The compose LOC ratchet measured Request Review composition at 1,069 lines,
down from 1,255. Its total measured cap is reduced from 34,483 to 34,297.
No Root field or mutable wiring was added. Policy adds three actual
projection-public → domain/platform public/contract rules, under a tested
reviewed-epoch acceptor. The one retired exact key is listed in
`backend/architecture/http-response-projection-4079.json`; all other #4079
keys stay under their original carrier.

Final architecture, full no-fast affected tests and backend lint are publication
gates, separate from these observed focused results. No benchmark or database
performance claim is made for this package-boundary-only change.


## Development integration

Before publication the single unpublished commit was rebased onto
`63c96ecdee9b4391ac6f012a4a03dd1cee7d8dac` (PR #4315), preserving all of its
quality/ratchet reductions. The sole textual conflict was the compose total:
34,483 minus this slice's 186 lines is 34,297. Rule counts against this base
are 827 → 830; Root composition is 1,652, unchanged by this slice. The focused
results above retain their original provenance; final required gates are rerun
on the rebased tree before publication.
