CREATE TABLE cal_events (
  cal_id TEXT, id TEXT, title TEXT, event_start INTEGER, event_end INTEGER,
  event_start_tz TEXT, event_end_tz TEXT, flags INTEGER
);
CREATE TABLE cal_properties (cal_id TEXT, item_id TEXT, key TEXT, value TEXT);
-- 1700000000000000 microseconds = 2023-11-14T22:13:20Z
INSERT INTO cal_events VALUES ('cal-uuid-1','e1','Standup',1700000000000000,1700003600000000,'floating','floating',0);
INSERT INTO cal_properties VALUES ('cal-uuid-1','e1','LOCATION','Room A');
INSERT INTO cal_events VALUES ('cal-uuid-1','e2','Review',1700100000000000,1700103600000000,'floating','floating',0);
