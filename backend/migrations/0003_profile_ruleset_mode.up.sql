-- ruleset_mode controls how a profile delivers its rule lists:
--   'passthrough' — rule-providers point at the upstream URL; the
--                   client fetches the lists itself (lean, default).
--   'self_hosted' — rule-providers point at this dashboard's
--                   /sub/ruleset/<key> endpoint; we fetch + cache the
--                   lists and serve them (self-contained; survives the
--                   client not being able to reach the upstream).
ALTER TABLE subscription_profiles
  ADD COLUMN ruleset_mode TEXT NOT NULL DEFAULT 'passthrough';
