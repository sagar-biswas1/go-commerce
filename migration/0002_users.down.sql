-- Reverses 0002_users.up.sql.
--
-- refresh_tokens references users, so this will refuse to run while that table
-- still exists. That is the point: migrations come down in the reverse order
-- they went up, and the foreign key is what enforces it.
DROP TABLE IF EXISTS users;
