ALTER TABLE expenses ADD COLUMN user_number BIGINT;

WITH numbered AS (
    SELECT id, ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY id) AS user_number
    FROM expenses
)
UPDATE expenses AS e
SET user_number = numbered.user_number
FROM numbered
WHERE e.id = numbered.id;

ALTER TABLE expenses ALTER COLUMN user_number SET NOT NULL;
ALTER TABLE expenses ADD CONSTRAINT expenses_user_number_key UNIQUE (user_id, user_number);
