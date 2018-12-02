CREATE UNIQUE INDEX org_subs_org_id_uniq_idx
  ON org_subs(org_id)
  WHERE deleted_at IS NULL;

CREATE INDEX org_subs_org_id_idx ON org_subs(org_id)