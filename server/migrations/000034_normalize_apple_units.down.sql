-- Only the allowlist row is reversed. The value conversions are not: after the
-- up migration every row carries the canonical unit, so a reversal cannot tell
-- the rows it converted from the twelve years that were already stored that way
-- and would rescale correct data.
UPDATE metric_allowlist SET display_unit = '' WHERE metric_name = 'time_in_daylight';
