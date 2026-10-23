-- Run from the private working directory containing candidates.txt.
-- Default is a dry run: only temporary objects, no durable writes.
\if :{?apply}
\else
\set apply false
\endif
BEGIN;
SET LOCAL row_security = off;
SET LOCAL lock_timeout = '5s';
CREATE TEMP TABLE avatar_candidates (avatar text PRIMARY KEY) ON COMMIT DROP;
\copy avatar_candidates FROM 'candidates.txt'
-- Freeze the final reference check against concurrent DB writers. Uploads,
-- API and worker keep running; no concurrent restores or file imports until
-- the queue drains, as required by step 1 of the runbook.
\if :apply
LOCK TABLE auth.accounts, users.profiles IN SHARE MODE;
LOCK TABLE auth.account_avatar_file_cleanup IN SHARE ROW EXCLUSIVE MODE;
\endif
CREATE TEMP VIEW eligible_avatar_orphans AS
SELECT candidate.avatar FROM avatar_candidates AS candidate
-- Deliberately stricter than fs.ValidPath: only known two-level layouts,
-- ASCII upload names, no dot segments, URL escaping, controls or backslashes.
WHERE candidate.avatar ~ '^/uploads/avatars/(global|[1-9][0-9]*)/[A-Za-z0-9_-][A-Za-z0-9_.-]*$'
AND NOT EXISTS (SELECT FROM auth.accounts AS account WHERE account.avatar = candidate.avatar)
AND NOT EXISTS (SELECT FROM users.profiles AS profile WHERE profile.avatar = candidate.avatar)
AND NOT EXISTS (SELECT FROM auth.account_avatar_file_cleanup AS cleanup WHERE cleanup.avatar = candidate.avatar);
SELECT (SELECT count(*) FROM avatar_candidates) AS gefunden,
       (SELECT count(*) FROM eligible_avatar_orphans) AS eintragbar;
\if :apply
CREATE TEMP TABLE enqueued_avatars ON COMMIT DROP AS
WITH inserted AS (
    INSERT INTO auth.account_avatar_file_cleanup (avatar)
    SELECT avatar FROM eligible_avatar_orphans
    ON CONFLICT (avatar) DO NOTHING
    RETURNING avatar
)
SELECT avatar FROM inserted;
-- Keep the exact inserted set private for the count-only follow-up query.
\copy enqueued_avatars TO 'enqueued.txt'
SELECT count(*) AS eingetragen FROM enqueued_avatars;
\endif
COMMIT;
