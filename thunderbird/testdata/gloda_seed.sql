CREATE TABLE folderLocations (id INTEGER PRIMARY KEY, folderURI TEXT, name TEXT);
CREATE TABLE messages (
  id INTEGER PRIMARY KEY, folderID INTEGER, messageKey INTEGER,
  date INTEGER, headerMessageID TEXT, deleted INTEGER DEFAULT 0, jsonAttributes TEXT
);
CREATE TABLE messagesText_content (
  docid INTEGER PRIMARY KEY, c0body TEXT, c1subject TEXT,
  c2attachmentNames TEXT, c3author TEXT, c4recipients TEXT
);

INSERT INTO folderLocations VALUES (1, 'imap://alice@example.com/INBOX', 'INBOX');
INSERT INTO folderLocations VALUES (2, 'imap://alice@example.com/Archive', 'Archive');

-- date is microseconds since epoch (PRTime). 1700000000000000 = 2023-11-14.
INSERT INTO messages VALUES (10, 1, 101, 1700000000000000, 'msg-a@example.com', 0, NULL);
INSERT INTO messages VALUES (11, 1, 102, 1700100000000000, 'msg-b@example.com', 0, NULL);
INSERT INTO messages VALUES (12, 2, 103, 1699000000000000, 'msg-c@example.com', 0, NULL);
INSERT INTO messages VALUES (13, 1, 104, 1700200000000000, 'msg-d@example.com', 1, NULL); -- deleted
INSERT INTO messages VALUES (14, NULL, NULL, 1700300000000000, 'ghost@example.com', 0, NULL); -- ghost

INSERT INTO messagesText_content VALUES (10, 'quarterly report body', 'Quarterly report', '', 'bob@example.com', 'alice@example.com');
INSERT INTO messagesText_content VALUES (11, 'lunch tomorrow?', 'Lunch', '', 'carol@example.com', 'alice@example.com');
INSERT INTO messagesText_content VALUES (12, 'old archived note', 'Archive note', '', 'bob@example.com', 'alice@example.com');
INSERT INTO messagesText_content VALUES (13, 'deleted msg', 'Deleted', '', 'x@example.com', 'alice@example.com');
