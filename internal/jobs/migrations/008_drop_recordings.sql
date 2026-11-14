-- Recordings were removed from the product. The calls table keeps
-- transcript_consent, consent_at, consent_version, and consent_language;
-- only the audio recording state is dropped.
DROP TABLE IF EXISTS call_recordings;
ALTER TABLE calls DROP COLUMN IF EXISTS recording_consent;
