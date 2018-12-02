ALTER TABLE payment_gateway_events
  ADD COLUMN user_id INTEGER REFERENCES users(id);

CREATE INDEX payment_gateway_events_user_id_idx
  ON payment_gateway_events(user_id)
  WHERE user_id IS NOT NULL;