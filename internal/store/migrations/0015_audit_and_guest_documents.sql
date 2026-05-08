-- +goose Up
-- +goose StatementBegin

CREATE TABLE audit_events (
    id                  uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id     uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    actor_type          text        NOT NULL CHECK (actor_type IN ('staff', 'guest', 'system')),
    actor_user_id       uuid        NULL REFERENCES users(id) ON DELETE SET NULL,
    actor_guest_user_id uuid        NULL REFERENCES guest_users(id) ON DELETE SET NULL,
    action              text        NOT NULL,
    entity_type         text        NOT NULL,
    entity_id           uuid        NULL,
    trip_id             uuid        NULL REFERENCES trips(id) ON DELETE SET NULL,
    trip_guest_id       uuid        NULL REFERENCES trip_guests(id) ON DELETE SET NULL,
    metadata            jsonb       NOT NULL DEFAULT '{}'::jsonb,
    created_at          timestamptz NOT NULL DEFAULT now(),
    CHECK (
        (actor_type = 'staff' AND actor_user_id IS NOT NULL AND actor_guest_user_id IS NULL) OR
        (actor_type = 'guest' AND actor_guest_user_id IS NOT NULL AND actor_user_id IS NULL) OR
        (actor_type = 'system' AND actor_user_id IS NULL AND actor_guest_user_id IS NULL)
    )
);

CREATE INDEX audit_events_org_created_idx
    ON audit_events(organization_id, created_at DESC);
CREATE INDEX audit_events_trip_guest_idx
    ON audit_events(organization_id, trip_guest_id, created_at DESC)
    WHERE trip_guest_id IS NOT NULL;
CREATE INDEX audit_events_entity_idx
    ON audit_events(organization_id, entity_type, entity_id, created_at DESC)
    WHERE entity_id IS NOT NULL;
CREATE INDEX audit_events_action_idx
    ON audit_events(organization_id, action, created_at DESC);
CREATE INDEX audit_events_trip_idx
    ON audit_events(organization_id, trip_id, created_at DESC)
    WHERE trip_id IS NOT NULL;

CREATE OR REPLACE FUNCTION reject_audit_events_mutation()
RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'audit_events is append-only';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER audit_events_no_update
    BEFORE UPDATE ON audit_events
    FOR EACH ROW EXECUTE FUNCTION reject_audit_events_mutation();

CREATE TRIGGER audit_events_no_delete
    BEFORE DELETE ON audit_events
    FOR EACH ROW EXECUTE FUNCTION reject_audit_events_mutation();

CREATE TABLE guest_documents (
    id                         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id            uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    trip_id                    uuid        NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    trip_guest_id              uuid        NOT NULL REFERENCES trip_guests(id) ON DELETE CASCADE,
    uploaded_by_user_id        uuid        NULL REFERENCES users(id) ON DELETE SET NULL,
    uploaded_by_guest_user_id  uuid        NULL REFERENCES guest_users(id) ON DELETE SET NULL,
    category                   text        NOT NULL CHECK (category IN (
        'travel_document',
        'dive_certification',
        'dive_insurance',
        'liability_waiver',
        'medical',
        'other'
    )),
    display_name               text        NOT NULL,
    original_filename          text        NOT NULL,
    content_type               text        NOT NULL CHECK (content_type IN (
        'application/pdf',
        'image/jpeg',
        'image/png',
        'image/heic',
        'image/heif'
    )),
    size_bytes                 bigint      NOT NULL CHECK (size_bytes > 0),
    sha256_hex                 char(64)    NOT NULL,
    storage_key                text        NOT NULL,
    notes                      text        NULL,
    archived_at                timestamptz NULL,
    archived_by_user_id        uuid        NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at                 timestamptz NOT NULL DEFAULT now(),
    updated_at                 timestamptz NOT NULL DEFAULT now(),
    CHECK (
        (uploaded_by_user_id IS NOT NULL AND uploaded_by_guest_user_id IS NULL) OR
        (uploaded_by_user_id IS NULL AND uploaded_by_guest_user_id IS NOT NULL)
    )
);

CREATE INDEX guest_documents_trip_guest_idx
    ON guest_documents(organization_id, trip_guest_id, created_at DESC);
CREATE INDEX guest_documents_active_category_idx
    ON guest_documents(organization_id, trip_guest_id, category)
    WHERE archived_at IS NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS guest_documents;
DROP TRIGGER IF EXISTS audit_events_no_delete ON audit_events;
DROP TRIGGER IF EXISTS audit_events_no_update ON audit_events;
DROP FUNCTION IF EXISTS reject_audit_events_mutation();
DROP TABLE IF EXISTS audit_events;
-- +goose StatementEnd
