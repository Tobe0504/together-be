-- Who currently has a live screen share, if anyone.
--
-- Distinct from room_participants.is_primary, which is about holding the
-- stage (presenter rights) — someone can hold the stage without having
-- started sharing yet. This column answers "is a screen live right now,
-- and whose", which is what decides whether anyone else may start one.
--
-- Nullable: no row means nobody is presenting.
ALTER TABLE rooms ADD COLUMN primary_presenter_id TEXT;
