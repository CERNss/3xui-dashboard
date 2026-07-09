-- Plan-subscriber sync (SyncPlanSubscribers) enumerates ownerships by
-- plan; the existing quota-group index leads on user_id and cannot
-- serve a bare plan_id lookup.
CREATE INDEX client_ownerships_plan_id_idx
  ON client_ownerships (plan_id)
  WHERE plan_id IS NOT NULL;
