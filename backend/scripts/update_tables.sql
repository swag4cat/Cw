-- Добавляем поле image_base64 к таблице recipes
ALTER TABLE recipes ADD COLUMN IF NOT EXISTS image_base64 TEXT;
