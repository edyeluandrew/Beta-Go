-- Phase 8: clear dish photos on the menu. URL only until file upload exists.

ALTER TABLE menu_items
    ADD COLUMN IF NOT EXISTS photo_url TEXT NOT NULL DEFAULT '';
