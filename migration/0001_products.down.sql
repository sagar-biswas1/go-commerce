-- Reverses 0001_products.up.sql.
--
-- The indexes go with the table, so dropping the table is enough -- naming them
-- separately would only leave a statement that fails once the table is gone.
DROP TABLE IF EXISTS products;
