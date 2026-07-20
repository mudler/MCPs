CREATE TABLE properties (card TEXT, name TEXT, value TEXT);
CREATE INDEX properties_card ON properties(card);
CREATE INDEX properties_name ON properties(name);
INSERT INTO properties VALUES ('c1','DisplayName','Bob Jones');
INSERT INTO properties VALUES ('c1','PrimaryEmail','bob@example.com');
INSERT INTO properties VALUES ('c1','NickName','Bobby');
INSERT INTO properties VALUES ('c2','DisplayName','Carol Smith');
INSERT INTO properties VALUES ('c2','PrimaryEmail','carol@example.com');
