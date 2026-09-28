# Runtime workload checkpoint-1-v1 → checkpoint-1-v2 (bridge): baseline versus candidate

Each cell shows baseline → candidate. Latency uses nearest-rank percentiles.
Latency changes count as material only beyond relative 20.0% and absolute 0.500 ms; any change to another metric is material. The Material column of each table lists only that table's statistic.

## Median of three runs

| Scenario | p50 ms | p95 ms | Queries/run | Rows/run¹ | Pool wait ms | Lock observations | Material |
|---|---:|---:|---:|---:|---:|---:|---|
| timetable.not-found | 0.660 → 0.805 (+21.9%) | 0.889 → 1.051 (+18.3%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| organization-tenancy.resolve | 1.789 → 1.704 (-4.8%) | 2.095 → 1.848 (-11.8%) | 390.000 → 390.000 (+0.0%) | 90.000 → 90.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| facilities.update | 1.201 → 1.019 (-15.2%) | 1.546 → 1.210 (-21.7%) | 210.000 → 210.000 (+0.0%) | 120.000 → 120.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| facilities.list | 1.312 → 1.251 (-4.6%) | 1.473 → 1.457 (-1.1%) | 210.000 → 210.000 (+0.0%) | 390.000 → 390.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| school-structure.list | 1.341 → 1.302 (-2.9%) | 1.539 → 1.467 (-4.7%) | 240.000 → 240.000 (+0.0%) | 330.000 → 330.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| people-directory.guardians | 1.079 → 1.119 (+3.7%) | 1.317 → 1.242 (-5.7%) | 180.000 → 180.000 (+0.0%) | 1530.000 → 1530.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| timetable.activities | 1.583 → 1.656 (+4.6%) | 1.728 → 1.844 (+6.7%) | 240.000 → 240.000 (+0.0%) | 690.000 → 690.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| timetable.categories | 0.683 → 0.772 (+13.0%) | 0.897 → 0.852 (-5.0%) | 150.000 → 150.000 (+0.0%) | 360.000 → 360.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| school-calendar.periods | 1.263 → 1.286 (+1.8%) | 1.411 → 1.411 (+0.0%) | 210.000 → 210.000 (+0.0%) | 90.000 → 90.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| care-plan.offerings | 0.859 → 1.016 (+18.3%) | 1.106 → 1.116 (+0.9%) | 180.000 → 180.000 (+0.0%) | 330.000 → 330.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| communication.messages | 3.526 → 1.978 (-43.9%) | 3.870 → 2.205 (-43.0%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p50_ms, latency_p95_ms |
| appointments.calendar | 1.668 → 1.688 (+1.2%) | 1.802 → 1.767 (-2.0%) | 270.000 → 270.000 (+0.0%) | 90.000 → 90.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| settings.schema | 1.385 → 1.360 (-1.8%) | 1.950 → 1.656 (-15.1%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| meal-plan.list | 0.794 → 0.824 (+3.8%) | 1.000 → 1.004 (+0.5%) | 180.000 → 180.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| feedback.list | 0.640 → 0.650 (+1.5%) | 0.984 → 0.857 (-13.0%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| school-membership.staff | 2.704 → 2.719 (+0.5%) | 2.994 → 2.990 (-0.1%) | 450.000 → 450.000 (+0.0%) | 1170.000 → 1170.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| facilities.invalid-id | 0.478 → 0.499 (+4.3%) | 0.708 → 0.694 (-2.0%) | 120.000 → 120.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| timetable.invalid-id | 0.483 → 0.490 (+1.5%) | 0.719 → 0.667 (-7.3%) | 120.000 → 120.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| people-directory.invalid-id | 0.491 → 0.475 (-3.2%) | 0.620 → 0.685 (+10.6%) | 120.000 → 120.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| security.unauthenticated | 0.019 → 0.016 (-16.1%) | 0.029 → 0.022 (-24.9%) | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | none |
| delivery.provider-unavailable | 5.008 → 5.901 (+17.8%) | 5.674 → 6.430 (+13.3%) | 1020.000 → 1020.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| delivery.render-failure | 4.231 → 4.160 (-1.7%) | 4.728 → 4.729 (+0.0%) | 960.000 → 960.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| delivery.idle | 2.693 → 2.632 (-2.2%) | 2.965 → 2.971 (+0.2%) | 600.000 → 600.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | none |
| timetable.materialize-create | 3.520 → 3.413 (-3.1%) | 3.757 → 3.739 (-0.5%) | 570.000 → 570.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| timetable.materialize-existing | 2.804 → 2.800 (-0.1%) | 3.200 → 3.088 (-3.5%) | 510.000 → 510.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | none |

## Worst of three runs

The worst run is the maximum of each metric across runs, chosen per metric.

| Scenario | p50 ms | p95 ms | Queries/run | Rows/run¹ | Pool wait ms | Lock observations | Material |
|---|---:|---:|---:|---:|---:|---:|---|
| timetable.not-found | 0.871 → 0.838 (-3.8%) | 1.154 → 1.330 (+15.3%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| organization-tenancy.resolve | 2.247 → 1.994 (-11.3%) | 2.409 → 2.373 (-1.5%) | 390.000 → 390.000 (+0.0%) | 90.000 → 90.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| facilities.update | 1.270 → 1.096 (-13.7%) | 1.573 → 1.297 (-17.5%) | 210.000 → 210.000 (+0.0%) | 120.000 → 120.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| facilities.list | 1.555 → 1.349 (-13.2%) | 1.941 → 1.497 (-22.9%) | 210.000 → 210.000 (+0.0%) | 390.000 → 390.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| school-structure.list | 1.377 → 1.431 (+3.9%) | 1.606 → 1.655 (+3.0%) | 240.000 → 240.000 (+0.0%) | 330.000 → 330.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| people-directory.guardians | 1.173 → 1.141 (-2.8%) | 1.393 → 1.328 (-4.7%) | 180.000 → 180.000 (+0.0%) | 1530.000 → 1530.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| timetable.activities | 1.634 → 1.697 (+3.8%) | 1.820 → 2.199 (+20.8%) | 240.000 → 240.000 (+0.0%) | 690.000 → 690.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| timetable.categories | 0.695 → 0.776 (+11.7%) | 0.904 → 0.887 (-2.0%) | 150.000 → 150.000 (+0.0%) | 360.000 → 360.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| school-calendar.periods | 1.264 → 1.309 (+3.5%) | 1.489 → 1.422 (-4.5%) | 210.000 → 210.000 (+0.0%) | 90.000 → 90.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| care-plan.offerings | 0.861 → 1.018 (+18.2%) | 1.120 → 1.220 (+9.0%) | 180.000 → 180.000 (+0.0%) | 330.000 → 330.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| communication.messages | 3.660 → 3.474 (-5.1%) | 4.027 → 4.236 (+5.2%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| appointments.calendar | 1.915 → 1.748 (-8.7%) | 2.115 → 1.913 (-9.6%) | 270.000 → 270.000 (+0.0%) | 90.000 → 90.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| settings.schema | 1.725 → 1.379 (-20.0%) | 2.580 → 1.857 (-28.0%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | latency_p95_ms |
| meal-plan.list | 1.076 → 0.908 (-15.6%) | 1.239 → 1.103 (-10.9%) | 180.000 → 180.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| feedback.list | 0.747 → 0.738 (-1.3%) | 0.995 → 0.928 (-6.7%) | 150.000 → 150.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| school-membership.staff | 2.725 → 2.806 (+3.0%) | 3.211 → 3.014 (-6.1%) | 450.000 → 450.000 (+0.0%) | 1170.000 → 1170.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| facilities.invalid-id | 0.513 → 0.548 (+6.9%) | 0.769 → 0.838 (+8.9%) | 120.000 → 120.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| timetable.invalid-id | 0.675 → 0.569 (-15.7%) | 0.979 → 0.730 (-25.5%) | 120.000 → 120.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| people-directory.invalid-id | 0.556 → 0.559 (+0.7%) | 0.722 → 0.763 (+5.6%) | 120.000 → 120.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| security.unauthenticated | 0.024 → 0.027 (+12.3%) | 0.072 → 0.084 (+16.8%) | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | none |
| delivery.provider-unavailable | 5.120 → 6.057 (+18.3%) | 6.156 → 7.234 (+17.5%) | 1020.000 → 1020.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| delivery.render-failure | 4.620 → 4.359 (-5.6%) | 5.412 → 5.783 (+6.9%) | 960.000 → 960.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| delivery.idle | 2.786 → 2.729 (-2.0%) | 3.005 → 3.009 (+0.2%) | 600.000 → 600.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | none |
| timetable.materialize-create | 3.536 → 3.480 (-1.6%) | 4.095 → 3.743 (-8.6%) | 570.000 → 570.000 (+0.0%) | 30.000 → 30.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | none |
| timetable.materialize-existing | 3.074 → 2.980 (-3.1%) | 3.440 → 3.324 (-3.4%) | 510.000 → 510.000 (+0.0%) | 0.000 → 0.000 | 0.000 → 0.000 | 0.000 → 0.000 | none |

¹ HTTP: driver-reported rows returned or changed, not distinct database rows. Worker: outbox rows claimed or timetable instances created.

## Material changes

- `communication.messages` `latency_p50_ms` median: 3.526 → 1.978
- `communication.messages` `latency_p95_ms` median: 3.870 → 2.205
- `settings.schema` `latency_p95_ms` worst: 2.580 → 1.857

## Classification counts across all scenarios, metrics, and statistics

- unchanged: 889
- within-tolerance: 106
- material: 3
- coverage: 100

## Scenarios only in the candidate workload

Not compared: `identity-access.login`, `identity-access.login-wrong-password`, `identity-access.refresh`, `identity-access.refresh-invalid-token`, `identity-access.mfa-verify`, `identity-access.mfa-verify-wrong-code`, `identity-access.passkey-login-options`, `identity-access.passkey-login-options-wrong-origin`, `device-scan.checkin`, `device-scan.checkin-invalid-key`, `device-scan.pickup-query`, `device-scan.pickup-query-missing-pin`, `device-scan.status`, `device-scan.status-wrong-pin`, `device-scan.rfid-lookup`, `device-scan.rfid-lookup-malformed-key`, `device-scan.staff-clock`, `device-scan.staff-clock-wrong-pin`, `people-directory.students`, `people-directory.students-invalid-view`, `request-review.queue`, `request-review.queue-invalid-view`, `request-review.care-schedule-decide`, `request-review.care-schedule-decide-not-found`, `care-plan.withdrawals`, `care-plan.withdrawals-invalid-state`, `student-deletion.withdrawal-impact`, `student-deletion.withdrawal-impact-not-found`, `care-plan.care-end-preview`, `care-plan.care-end-preview-past-day`, `care-plan.care-end`, `care-plan.care-end-replan`, `care-plan.care-end-stale-token`, `student-deletion.execute`, `student-deletion.execute-stale-preview`.
