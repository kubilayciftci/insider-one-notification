CREATE OR REPLACE FUNCTION notify_status_change()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.status IS DISTINCT FROM NEW.status THEN
        PERFORM pg_notify('notification_status', json_build_object(
            'id', NEW.id,
            'status', NEW.status,
            'channel', NEW.channel,
            'updated_at', NEW.updated_at
        )::text);
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_notification_status_change
    AFTER UPDATE ON notifications
    FOR EACH ROW
    EXECUTE FUNCTION notify_status_change();
