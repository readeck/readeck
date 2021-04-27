CREATE TABLE migration (
    id      integer      PRIMARY KEY,
    name    varchar(128) NOT NULL,
    applied timestamptz  NOT NULL
);

CREATE TABLE IF NOT EXISTS "user" (
    id       SERIAL       PRIMARY KEY,
    created  timestamptz  NOT NULL,
    updated  timestamptz  NOT NULL,
    username varchar(128) UNIQUE NOT NULL,
    email    varchar(128) UNIQUE NOT NULL,
    password varchar(256) NOT NULL,
    "group"  varchar(64)  NOT NULL DEFAULT 'user',
    settings jsonb        NOT NULL DEFAULT '{}',
    seed     integer      NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS token (
    id          SERIAL        PRIMARY KEY,
    uid         varchar(32)   UNIQUE NOT NULL,
    user_id     integer       NOT NULL,
    created     timestamptz   NOT NULL,
    expires     timestamptz   NULL,
    is_enabled  boolean       NOT NULL DEFAULT true,
    application varchar(128)  NOT NULL,
    roles       jsonb         NOT NULL DEFAULT '[]',

    CONSTRAINT fk_token_user FOREIGN KEY (user_id) REFERENCES "user"(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS bookmark (
    id          SERIAL      PRIMARY KEY,
    uid         varchar(32) UNIQUE NOT NULL,
    user_id     integer     NOT NULL,
    created     timestamptz NOT NULL,
    updated     timestamptz NOT NULL,
    is_marked   boolean     NOT NULL DEFAULT false,
    is_archived boolean     NOT NULL DEFAULT false,
    state       integer     NOT NULL DEFAULT 0,
    url         text        NOT NULL,
    domain      text        NOT NULL,
    title       text        NOT NULL,
    site        text        NOT NULL DEFAULT '',
    site_name   text        NOT NULL DEFAULT '',
    published   timestamptz,
    authors     jsonb       NOT NULL DEFAULT '[]',
    lang        varchar(16) NOT NULL DEFAULT '',
    type        varchar(64) NOT NULL DEFAULT '',
    description text        NOT NULL DEFAULT '',
    "text"      text        NOT NULL DEFAULT '',
    word_count  integer     NOT NULL DEFAULT 0,
    embed       text        NOT NULL DEFAULT '',
    file_path   text        NOT NULL DEFAULT '',
    files       jsonb       NOT NULL DEFAULT '[]',
    errors      jsonb       NOT NULL DEFAULT '[]',
    labels      jsonb       NOT NULL DEFAULT '[]',

    CONSTRAINT fk_bookmark_user FOREIGN KEY (user_id) REFERENCES "user"(id) ON DELETE CASCADE
  );


--
-- Search configuration
--
CREATE EXTENSION IF NOT EXISTS unaccent;

CREATE TEXT SEARCH CONFIGURATION ts (COPY = english);

ALTER TEXT SEARCH CONFIGURATION ts
ALTER MAPPING for hword, hword_part, word, host
WITH unaccent, english_stem, french_stem;

CREATE TABLE bookmark_search (
	bookmark_id int4 NOT NULL PRIMARY KEY,
	title       tsvector NULL,
	description tsvector NULL,
	"text"      tsvector NULL,
	site        tsvector NULL,
	author      tsvector NULL,
	"label"     tsvector NULL,

    CONSTRAINT fk_bookmark_search_bookmark FOREIGN KEY (bookmark_id) REFERENCES bookmark(id) ON DELETE CASCADE
);

CREATE INDEX bookmark_search_text_idx ON bookmark_search USING GIN ("text");
CREATE INDEX bookmark_search_title_idx  ON bookmark_search USING GIN (title);
CREATE INDEX bookmark_search_site_idx   ON bookmark_search USING GIN (site);
CREATE INDEX bookmark_search_author_idx ON bookmark_search USING GIN (author);
CREATE INDEX bookmark_search_label_idx  ON bookmark_search USING GIN (label);

CREATE OR REPLACE FUNCTION bookmark_search_update()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
	DELETE FROM bookmark_search WHERE bookmark_id = OLD.id;

	IF tg_op = 'UPDATE' OR tg_op = 'INSERT' THEN
		INSERT INTO bookmark_search (
			bookmark_id, title, description, "text", site, author, "label"
		) VALUES (
			NEW.id,
            setweight(to_tsvector('ts', NEW.title), 'A'),
            to_tsvector('ts', NEW.description),
			to_tsvector('ts', NEW."text"),
            to_tsvector('ts',
                NEW.site_name || ' ' || NEW.domain || ' ' ||
                REGEXP_REPLACE(NEW.site, '^www\.', '') || ' ' ||
                REPLACE(NEW.domain, '.', ' ') ||
                REPLACE(REGEXP_REPLACE(NEW.site, '^www\.', ''), '.', ' ')
            ),
			jsonb_to_tsvector('ts', NEW.authors, '["string"]'),
			setweight(jsonb_to_tsvector('ts', NEW.labels, '["string"]'), 'A')
		);
	END IF;
	RETURN NEW;
END;
$$;

CREATE TRIGGER bookmark_tsu AFTER INSERT OR UPDATE ON bookmark
	FOR EACH ROW EXECUTE PROCEDURE bookmark_search_update();
