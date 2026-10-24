-- Human-only export into a private file, never into a ticket or terminal.
-- Administrative role with BYPASSRLS, including all schools and erased rows.
BEGIN READ ONLY;
SET LOCAL row_security = off;
COPY (
    SELECT avatar FROM auth.accounts WHERE avatar LIKE '/uploads/avatars/%'
    UNION
    SELECT avatar FROM users.profiles WHERE avatar LIKE '/uploads/avatars/%'
    UNION
    SELECT avatar FROM auth.account_avatar_file_cleanup
    WHERE avatar LIKE '/uploads/avatars/%'
    ORDER BY avatar
) TO STDOUT;
COMMIT;
