-- Run from the same private directory after a successful apply.
-- Load a connection-local temporary table before opening the read-only query.
CREATE TEMP TABLE enqueued_avatars (avatar text PRIMARY KEY);
\copy enqueued_avatars FROM 'enqueued.txt'
BEGIN READ ONLY;
SET LOCAL row_security = off;
SELECT count(*) AS eingetragen,
       count(cleanup.avatar) AS noch_offen,
       count(*) - count(cleanup.avatar) AS abgearbeitet
FROM enqueued_avatars AS enqueued
LEFT JOIN auth.account_avatar_file_cleanup AS cleanup USING (avatar);
COMMIT;
