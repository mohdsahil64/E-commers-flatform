-- Allow a chat message in a support ticket to carry one image (proof photo
-- sent mid-conversation). Empty string = text-only message.
ALTER TABLE support_ticket_messages
    ADD COLUMN IF NOT EXISTS image_key TEXT NOT NULL DEFAULT '';
