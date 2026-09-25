# Runtime workload checkpoint-1-v2: measured results

Values are median / worst across three runs. Latency uses nearest-rank percentiles.

| Scenario | p50 ms | p95 ms | Queries/run | Rows/run¹ | Pool wait ms | Lock observations |
|---|---:|---:|---:|---:|---:|---:|
| timetable.not-found | 0.805 / 0.838 | 1.051 / 1.330 | 150.000 / 150.000 | 30.000 / 30.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| organization-tenancy.resolve | 1.704 / 1.994 | 1.848 / 2.373 | 390.000 / 390.000 | 90.000 / 90.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| facilities.update | 1.019 / 1.096 | 1.210 / 1.297 | 210.000 / 210.000 | 120.000 / 120.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| facilities.list | 1.251 / 1.349 | 1.457 / 1.497 | 210.000 / 210.000 | 390.000 / 390.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| school-structure.list | 1.302 / 1.431 | 1.467 / 1.655 | 240.000 / 240.000 | 330.000 / 330.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| people-directory.guardians | 1.119 / 1.141 | 1.242 / 1.328 | 180.000 / 180.000 | 1530.000 / 1530.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| timetable.activities | 1.656 / 1.697 | 1.844 / 2.199 | 240.000 / 240.000 | 690.000 / 690.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| timetable.categories | 0.772 / 0.776 | 0.852 / 0.887 | 150.000 / 150.000 | 360.000 / 360.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| school-calendar.periods | 1.286 / 1.309 | 1.411 / 1.422 | 210.000 / 210.000 | 90.000 / 90.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| care-plan.offerings | 1.016 / 1.018 | 1.116 / 1.220 | 180.000 / 180.000 | 330.000 / 330.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| communication.messages | 1.978 / 3.474 | 2.205 / 4.236 | 150.000 / 150.000 | 30.000 / 30.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| appointments.calendar | 1.688 / 1.748 | 1.767 / 1.913 | 270.000 / 270.000 | 90.000 / 90.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| settings.schema | 1.360 / 1.379 | 1.656 / 1.857 | 150.000 / 150.000 | 30.000 / 30.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| meal-plan.list | 0.824 / 0.908 | 1.004 / 1.103 | 180.000 / 180.000 | 30.000 / 30.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| feedback.list | 0.650 / 0.738 | 0.857 / 0.928 | 150.000 / 150.000 | 30.000 / 30.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| school-membership.staff | 2.719 / 2.806 | 2.990 / 3.014 | 450.000 / 450.000 | 1170.000 / 1170.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| facilities.invalid-id | 0.499 / 0.548 | 0.694 / 0.838 | 120.000 / 120.000 | 30.000 / 30.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| timetable.invalid-id | 0.490 / 0.569 | 0.667 / 0.730 | 120.000 / 120.000 | 30.000 / 30.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| people-directory.invalid-id | 0.475 / 0.559 | 0.685 / 0.763 | 120.000 / 120.000 | 30.000 / 30.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| security.unauthenticated | 0.016 / 0.027 | 0.022 / 0.084 | 0.000 / 0.000 | 0.000 / 0.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| identity-access.login | 8.421 / 9.357 | 10.277 / 11.128 | 1620.000 / 1620.000 | 600.000 / 600.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| identity-access.login-wrong-password | 2.163 / 2.248 | 2.445 / 3.070 | 330.000 / 330.000 | 150.000 / 150.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| identity-access.refresh | 4.301 / 4.690 | 4.674 / 6.385 | 810.000 / 810.000 | 360.000 / 360.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| identity-access.refresh-invalid-token | 0.020 / 0.020 | 0.029 / 0.029 | 0.000 / 0.000 | 0.000 / 0.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| identity-access.mfa-verify | 8.079 / 8.475 | 8.758 / 9.420 | 1560.000 / 1560.000 | 750.000 / 750.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| identity-access.mfa-verify-wrong-code | 3.009 / 3.262 | 3.491 / 3.548 | 540.000 / 540.000 | 210.000 / 210.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| identity-access.passkey-login-options | 1.209 / 1.319 | 1.319 / 1.453 | 270.000 / 270.000 | 90.000 / 90.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| identity-access.passkey-login-options-wrong-origin | 0.997 / 1.173 | 1.108 / 1.387 | 240.000 / 240.000 | 60.000 / 60.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| device-scan.checkin | 8.750 / 8.961 | 9.965 / 11.198 | 1305.000 / 1305.000 | 480.000 / 480.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| device-scan.checkin-invalid-key | 0.175 / 0.222 | 0.232 / 0.372 | 30.000 / 30.000 | 0.000 / 0.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| device-scan.pickup-query | 3.842 / 4.021 | 4.370 / 5.885 | 690.000 / 690.000 | 210.000 / 210.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| device-scan.pickup-query-missing-pin | 0.654 / 0.790 | 0.893 / 0.952 | 150.000 / 150.000 | 60.000 / 60.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| device-scan.status | 2.002 / 2.222 | 2.300 / 2.510 | 450.000 / 450.000 | 120.000 / 120.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| device-scan.status-wrong-pin | 1.360 / 1.440 | 1.578 / 1.642 | 300.000 / 300.000 | 90.000 / 90.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| device-scan.rfid-lookup | 2.342 / 2.454 | 2.516 / 2.593 | 480.000 / 480.000 | 180.000 / 180.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| device-scan.rfid-lookup-malformed-key | 0.018 / 0.029 | 0.025 / 0.062 | 0.000 / 0.000 | 0.000 / 0.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| device-scan.staff-clock | 6.157 / 6.565 | 6.958 / 7.108 | 1050.000 / 1050.000 | 1620.000 / 2415.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| device-scan.staff-clock-wrong-pin | 1.424 / 1.496 | 1.580 / 1.598 | 300.000 / 300.000 | 90.000 / 90.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| people-directory.students | 7.691 / 8.184 | 8.791 / 10.222 | 960.000 / 960.000 | 2820.000 / 2850.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| people-directory.students-invalid-view | 0.570 / 0.686 | 0.720 / 1.029 | 120.000 / 120.000 | 30.000 / 30.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| request-review.queue | 2.116 / 2.154 | 2.231 / 2.642 | 360.000 / 360.000 | 30.000 / 30.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| request-review.queue-invalid-view | 0.489 / 0.491 | 0.706 / 0.743 | 120.000 / 120.000 | 30.000 / 30.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| request-review.care-schedule-decide | 21.017 / 21.881 | 23.301 / 25.652 | 3000.000 / 3000.000 | 1725.000 / 1725.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| request-review.care-schedule-decide-not-found | 0.806 / 1.045 | 1.129 / 1.285 | 180.000 / 180.000 | 30.000 / 30.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| care-plan.withdrawals | 1.836 / 2.055 | 2.038 / 2.276 | 180.000 / 180.000 | 60.000 / 60.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| care-plan.withdrawals-invalid-state | 0.595 / 0.641 | 0.698 / 0.868 | 120.000 / 120.000 | 30.000 / 30.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| student-deletion.withdrawal-impact | 5.915 / 6.405 | 6.251 / 6.775 | 780.000 / 780.000 | 510.000 / 510.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| student-deletion.withdrawal-impact-not-found | 0.756 / 0.842 | 0.909 / 1.039 | 150.000 / 150.000 | 30.000 / 30.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| care-plan.care-end-preview | 4.427 / 5.038 | 5.491 / 6.060 | 600.000 / 600.000 | 930.000 / 930.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| care-plan.care-end-preview-past-day | 0.577 / 0.624 | 0.837 / 0.843 | 120.000 / 120.000 | 30.000 / 30.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| care-plan.care-end | 15.546 / 15.886 | 17.538 / 19.905 | 1920.000 / 1920.000 | 720.000 / 720.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| care-plan.care-end-replan | 16.052 / 16.311 | 17.485 / 17.672 | 1920.000 / 1920.000 | 1020.000 / 1020.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| care-plan.care-end-stale-token | 9.825 / 11.840 | 12.343 / 12.583 | 1170.000 / 1170.000 | 2190.000 / 2190.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| student-deletion.execute | 17.149 / 17.420 | 18.671 / 19.241 | 1920.000 / 1920.000 | 1320.000 / 1320.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| student-deletion.execute-stale-preview | 14.368 / 14.423 | 14.813 / 15.960 | 1650.000 / 1650.000 | 1170.000 / 1170.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| delivery.provider-unavailable | 5.901 / 6.057 | 6.430 / 7.234 | 1020.000 / 1020.000 | 30.000 / 30.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| delivery.render-failure | 4.160 / 4.359 | 4.729 / 5.783 | 960.000 / 960.000 | 30.000 / 30.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| delivery.idle | 2.632 / 2.729 | 2.971 / 3.009 | 600.000 / 600.000 | 0.000 / 0.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| timetable.materialize-create | 3.413 / 3.480 | 3.739 / 3.743 | 570.000 / 570.000 | 30.000 / 30.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| timetable.materialize-existing | 2.800 / 2.980 | 3.088 / 3.324 | 510.000 / 510.000 | 0.000 / 0.000 | 0.000 / 0.000 | 0.000 / 0.000 |

¹ HTTP: driver-reported rows returned or changed, not distinct database rows. Worker: outbox rows claimed or timetable instances created. Zero does not assert that every adapter has row instrumentation.

`summary.json` contains every numeric metric's three values, median and worst, stable errors, and nonzero raw counter deltas. Sampled Lock observations are not exact wait durations. Acquisition-statement timing is not used as lock-wait evidence.

## Contention run `contention.student-graph`

16 requests in flight per round against a pool of 12 connections. Median / worst across three runs.

| Metric | Median | Worst |
|---|---:|---:|
| round_wall_p50_ms | 61.216 | 61.677 |
| round_wall_p95_ms | 76.227 | 77.281 |
| pool_wait_count | 120.000 | 120.000 |
| pool_wait_ms | 1238.311 | 1246.234 |
| rounds_with_pool_wait | 30.000 | 30.000 |
| lock_waiting_backend_samples | 2904.000 | 3014.000 |
| lock_max_waiting_backends | 8.000 | 9.000 |
| lock_samples | 2455.000 | 2427.000 |
| lock_max_sample_gap_ms | 3.923 | 5.138 |
| deadlocks | 0.000 | 0.000 |
| transaction_rollbacks | 30.000 | 38.000 |
| transaction_retries | 0.000 | 0.000 |

| Operation | Requests per run | p50 ms per run | p95 ms per run | Statuses per run |
|---|---:|---|---|---|
| student-deletion.execute-subject | 30 | 48.199 / 47.074 / 48.866 | 61.114 / 58.524 / 65.291 | 200: 2, 409: 28 / 409: 30 / 409: 30 |
| student-deletion.execute-companion | 30 | 37.926 / 33.824 / 37.712 | 74.023 / 68.142 / 77.032 | 200: 28, 409: 2 / 200: 30 / 200: 30 |
| people-directory.update-subject | 120 | 41.033 / 48.074 / 40.194 | 64.345 / 67.317 / 64.162 | 200: 112, 404: 8 / 200: 120 / 200: 120 |
| people-directory.update-second-companion | 120 | 22.977 / 22.612 / 21.968 | 33.709 / 32.938 / 32.956 | 200: 120 / 200: 120 / 200: 120 |
| student-deletion.preview-companion | 90 | 9.730 / 9.708 / 10.225 | 18.100 / 18.595 / 19.218 | 200: 90 / 200: 90 / 200: 90 |
| people-directory.students | 90 | 14.374 / 14.708 / 14.651 | 23.369 / 22.123 / 23.614 | 200: 90 / 200: 90 / 200: 90 |

## jsonb_to_recordset set sizes

Rows per call of each call site, one value per run. A bounded use keeps its maximum independent of the tenant's size.

| Scenario | Call site | Calls per run | Min rows | Max rows |
|---|---|---|---|---|
| care-plan.care-end-preview | `jsonb_to_recordset(<set>::jsonb) AS removal(tenant_id bigint, enrollment_id bigint, previous_valid_until date)` | [30, 30, 30] | [0, 0, 0] | [0, 0, 0] |
| care-plan.care-end | `jsonb_to_recordset(<set>::jsonb) AS removal(tenant_id bigint, enrollment_id bigint, previous_valid_until date)` | [30, 30, 30] | [0, 0, 0] | [0, 0, 0] |
| care-plan.care-end | `jsonb_to_recordset(<set>::jsonb) AS removal(` | [60, 60, 60] | [0, 0, 0] | [0, 0, 0] |
| care-plan.care-end-replan | `jsonb_to_recordset(<set>::jsonb) AS removal(tenant_id bigint, enrollment_id bigint, previous_valid_until date)` | [30, 30, 30] | [3, 3, 3] | [3, 3, 3] |
| care-plan.care-end-replan | `jsonb_to_recordset(<set>::jsonb) AS removal(` | [60, 60, 60] | [3, 3, 3] | [3, 3, 3] |
| care-plan.care-end-stale-token | `jsonb_to_recordset(<set>::jsonb) AS removal(tenant_id bigint, enrollment_id bigint, previous_valid_until date)` | [30, 30, 30] | [0, 0, 0] | [0, 0, 0] |

## Observed final database state

Measured after the workload, outside request timing. These totals are not per-request rows changed.

| Counter | Final value |
|---|---:|
| care_ended | 210 |
| care_schedule_requests_approved | 105 |
| staff_clock_transitions | 105 |
| students_deleted | 105 |
