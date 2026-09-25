# Runtime workload checkpoint-1-v2: measured results

Values are median / worst across three runs. Latency uses nearest-rank percentiles.

| Scenario | p50 ms | p95 ms | Queries/run | Rows/run¹ | Pool wait ms | Lock observations |
|---|---:|---:|---:|---:|---:|---:|
| timetable.not-found | 0.821 / 0.892 | 1.054 / 1.142 | 150.000 / 150.000 | 30.000 / 30.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| organization-tenancy.resolve | 1.855 / 2.197 | 2.333 / 3.146 | 390.000 / 390.000 | 90.000 / 90.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| facilities.update | 1.092 / 1.227 | 1.379 / 1.652 | 210.000 / 210.000 | 120.000 / 120.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| facilities.list | 1.335 / 1.462 | 1.554 / 1.704 | 210.000 / 210.000 | 390.000 / 390.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| school-structure.list | 1.532 / 1.572 | 1.636 / 2.505 | 240.000 / 240.000 | 330.000 / 330.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| people-directory.guardians | 1.147 / 1.158 | 1.316 / 1.418 | 180.000 / 180.000 | 1530.000 / 1530.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| timetable.activities | 1.641 / 1.711 | 1.782 / 1.928 | 240.000 / 240.000 | 690.000 / 690.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| timetable.categories | 0.801 / 0.835 | 0.946 / 0.960 | 150.000 / 150.000 | 360.000 / 360.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| school-calendar.periods | 1.319 / 1.363 | 1.407 / 1.440 | 210.000 / 210.000 | 90.000 / 90.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| care-plan.offerings | 1.015 / 1.087 | 1.182 / 1.251 | 180.000 / 180.000 | 330.000 / 330.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| communication.messages | 2.178 / 3.536 | 2.384 / 3.888 | 150.000 / 150.000 | 30.000 / 30.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| appointments.calendar | 1.748 / 1.990 | 1.852 / 2.383 | 270.000 / 270.000 | 90.000 / 90.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| settings.schema | 1.402 / 1.669 | 1.748 / 1.958 | 150.000 / 150.000 | 30.000 / 30.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| meal-plan.list | 0.922 / 1.058 | 1.132 / 1.268 | 180.000 / 180.000 | 30.000 / 30.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| feedback.list | 0.768 / 0.782 | 0.863 / 0.903 | 150.000 / 150.000 | 30.000 / 30.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| school-membership.staff | 2.788 / 2.842 | 3.017 / 3.087 | 450.000 / 450.000 | 1170.000 / 1170.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| facilities.invalid-id | 0.582 / 0.611 | 0.746 / 0.764 | 120.000 / 120.000 | 30.000 / 30.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| timetable.invalid-id | 0.563 / 0.583 | 0.586 / 0.741 | 120.000 / 120.000 | 30.000 / 30.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| people-directory.invalid-id | 0.562 / 0.578 | 0.594 / 0.883 | 120.000 / 120.000 | 30.000 / 30.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| security.unauthenticated | 0.034 / 0.034 | 0.044 / 0.045 | 0.000 / 0.000 | 0.000 / 0.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| identity-access.login | 9.326 / 9.420 | 11.395 / 12.753 | 1620.000 / 1620.000 | 600.000 / 600.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| identity-access.login-wrong-password | 2.227 / 2.259 | 2.560 / 2.767 | 330.000 / 330.000 | 150.000 / 150.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| identity-access.refresh | 4.716 / 4.786 | 5.073 / 5.198 | 810.000 / 810.000 | 360.000 / 360.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| identity-access.refresh-invalid-token | 0.039 / 0.040 | 0.050 / 0.051 | 0.000 / 0.000 | 0.000 / 0.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| identity-access.mfa-verify | 8.955 / 9.252 | 9.769 / 10.289 | 1560.000 / 1560.000 | 750.000 / 750.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| identity-access.mfa-verify-wrong-code | 3.346 / 3.376 | 3.720 / 3.915 | 540.000 / 540.000 | 210.000 / 210.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| identity-access.passkey-login-options | 1.356 / 1.384 | 1.619 / 2.283 | 270.000 / 270.000 | 90.000 / 90.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| identity-access.passkey-login-options-wrong-origin | 1.133 / 1.164 | 1.429 / 1.757 | 240.000 / 240.000 | 60.000 / 60.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| device-scan.checkin | 7.841 / 9.890 | 10.593 / 11.926 | 1305.000 / 1305.000 | 480.000 / 480.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| device-scan.checkin-invalid-key | 0.213 / 0.216 | 0.240 / 0.242 | 30.000 / 30.000 | 0.000 / 0.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| device-scan.pickup-query | 4.177 / 4.260 | 4.474 / 5.128 | 690.000 / 690.000 | 210.000 / 210.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| device-scan.pickup-query-missing-pin | 0.781 / 0.812 | 0.869 / 1.039 | 150.000 / 150.000 | 60.000 / 60.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| device-scan.status | 2.169 / 2.204 | 2.471 / 2.475 | 450.000 / 450.000 | 120.000 / 120.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| device-scan.status-wrong-pin | 1.405 / 1.481 | 1.686 / 1.711 | 300.000 / 300.000 | 90.000 / 90.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| device-scan.rfid-lookup | 2.588 / 2.623 | 2.886 / 2.905 | 480.000 / 480.000 | 180.000 / 180.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| device-scan.rfid-lookup-malformed-key | 0.036 / 0.036 | 0.048 / 0.083 | 0.000 / 0.000 | 0.000 / 0.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| device-scan.staff-clock | 6.531 / 6.690 | 7.259 / 8.929 | 1050.000 / 1050.000 | 1620.000 / 2415.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| device-scan.staff-clock-wrong-pin | 1.488 / 1.581 | 1.722 / 2.536 | 300.000 / 300.000 | 90.000 / 90.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| people-directory.students | 8.348 / 8.443 | 8.795 / 8.990 | 960.000 / 960.000 | 2820.000 / 2850.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| people-directory.students-invalid-view | 0.589 / 0.601 | 0.807 / 0.808 | 120.000 / 120.000 | 30.000 / 30.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| request-review.queue | 2.198 / 2.223 | 2.386 / 2.536 | 360.000 / 360.000 | 30.000 / 30.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| request-review.queue-invalid-view | 0.579 / 0.591 | 0.810 / 0.828 | 120.000 / 120.000 | 30.000 / 30.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| request-review.care-schedule-decide | 22.065 / 22.335 | 25.239 / 25.679 | 3000.000 / 3000.000 | 1725.000 / 1725.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| request-review.care-schedule-decide-not-found | 0.950 / 1.049 | 1.246 / 1.307 | 180.000 / 180.000 | 30.000 / 30.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| care-plan.withdrawals | 2.201 / 2.342 | 2.494 / 2.603 | 180.000 / 180.000 | 60.000 / 60.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| care-plan.withdrawals-invalid-state | 0.583 / 0.713 | 0.785 / 0.831 | 120.000 / 120.000 | 30.000 / 30.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| student-deletion.withdrawal-impact | 6.811 / 6.827 | 8.571 / 8.998 | 780.000 / 780.000 | 510.000 / 510.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| student-deletion.withdrawal-impact-not-found | 0.786 / 0.814 | 1.118 / 1.154 | 150.000 / 150.000 | 30.000 / 30.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| care-plan.care-end-preview | 4.437 / 4.479 | 4.660 / 5.680 | 600.000 / 600.000 | 930.000 / 930.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| care-plan.care-end-preview-past-day | 0.580 / 0.589 | 0.760 / 0.792 | 120.000 / 120.000 | 30.000 / 30.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| care-plan.care-end | 16.632 / 16.771 | 17.890 / 19.455 | 1920.000 / 1920.000 | 720.000 / 720.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| care-plan.care-end-replan | 16.228 / 16.432 | 18.048 / 20.124 | 1920.000 / 1920.000 | 1020.000 / 1020.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| care-plan.care-end-stale-token | 9.785 / 10.119 | 10.567 / 11.009 | 1170.000 / 1170.000 | 2190.000 / 2190.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| student-deletion.execute | 17.864 / 18.010 | 19.422 / 21.089 | 1920.000 / 1920.000 | 1320.000 / 1320.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| student-deletion.execute-stale-preview | 15.352 / 15.591 | 16.956 / 17.888 | 1650.000 / 1650.000 | 1170.000 / 1170.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| delivery.provider-unavailable | 5.877 / 5.968 | 6.877 / 7.202 | 1020.000 / 1020.000 | 30.000 / 30.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| delivery.render-failure | 4.391 / 4.660 | 4.799 / 6.005 | 960.000 / 960.000 | 30.000 / 30.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| delivery.idle | 2.842 / 2.858 | 3.109 / 3.193 | 600.000 / 600.000 | 0.000 / 0.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| timetable.materialize-create | 3.503 / 3.692 | 3.833 / 3.895 | 570.000 / 570.000 | 30.000 / 30.000 | 0.000 / 0.000 | 0.000 / 0.000 |
| timetable.materialize-existing | 3.024 / 3.042 | 3.263 / 3.401 | 510.000 / 510.000 | 0.000 / 0.000 | 0.000 / 0.000 | 0.000 / 0.000 |

¹ HTTP: driver-reported rows returned or changed, not distinct database rows. Worker: outbox rows claimed or timetable instances created. Zero does not assert that every adapter has row instrumentation.

`summary.json` contains every numeric metric's three values, median and worst, stable errors, and nonzero raw counter deltas. Sampled Lock observations are not exact wait durations. Acquisition-statement timing is not used as lock-wait evidence.

## Contention run `contention.student-graph`

16 requests in flight per round against a pool of 12 connections. Median / worst across three runs.

| Metric | Median | Worst |
|---|---:|---:|
| round_wall_p50_ms | 66.278 | 67.262 |
| round_wall_p95_ms | 80.548 | 89.389 |
| pool_wait_count | 120.000 | 120.000 |
| pool_wait_ms | 1322.309 | 1360.897 |
| rounds_with_pool_wait | 30.000 | 30.000 |
| lock_waiting_backend_samples | 3187.000 | 3462.000 |
| lock_max_waiting_backends | 8.000 | 8.000 |
| lock_samples | 2726.000 | 2704.000 |
| lock_max_sample_gap_ms | 5.202 | 5.852 |
| deadlocks | 0.000 | 0.000 |
| transaction_rollbacks | 38.000 | 38.000 |
| transaction_retries | 0.000 | 0.000 |

| Operation | Requests per run | p50 ms per run | p95 ms per run | Statuses per run |
|---|---:|---|---|---|
| student-deletion.execute-subject | 30 | 48.480 / 45.773 / 45.056 | 68.359 / 60.948 / 57.612 | 200: 2, 409: 28 / 200: 2, 409: 28 / 200: 2, 409: 28 |
| student-deletion.execute-companion | 30 | 38.927 / 40.297 / 58.400 | 74.696 / 72.028 / 74.190 | 200: 28, 409: 2 / 200: 28, 409: 2 / 200: 28, 409: 2 |
| people-directory.update-subject | 120 | 52.272 / 54.223 / 46.575 | 71.926 / 79.287 / 72.554 | 200: 112, 404: 8 / 200: 112, 404: 8 / 200: 112, 404: 8 |
| people-directory.update-second-companion | 120 | 24.261 / 24.866 / 22.799 | 36.178 / 39.991 / 35.178 | 200: 120 / 200: 120 / 200: 120 |
| student-deletion.preview-companion | 90 | 10.333 / 10.364 / 9.671 | 20.027 / 18.045 / 17.214 | 200: 90 / 200: 90 / 200: 90 |
| people-directory.students | 90 | 15.644 / 16.212 / 14.595 | 25.041 / 25.455 / 23.143 | 200: 90 / 200: 90 / 200: 90 |

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
