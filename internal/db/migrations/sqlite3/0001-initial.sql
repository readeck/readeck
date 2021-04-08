CREATE TABLE migration (
    id      integer  PRIMARY KEY,
    name    text     NOT NULL,
    applied datetime NOT NULL
);

CREATE TABLE IF NOT EXISTS user (
    id       integer  PRIMARY KEY AUTOINCREMENT,
    created  datetime NOT NULL,
    updated  datetime NOT NULL,
    username text     UNIQUE NOT NULL,
    email    text     UNIQUE NOT NULL,
    password text     NOT NULL,
    `group`  text     NOT NULL DEFAULT "user",
    settings json     NOT NULL DEFAULT "{}"
);

CREATE TABLE IF NOT EXISTS token (
    id          integer  PRIMARY KEY AUTOINCREMENT,
    uid         text     UNIQUE NOT NULL,
    user_id     integer  NOT NULL,
    created     datetime NOT NULL,
    expires     datetime NULL,
    is_enabled  integer  NOT NULL DEFAULT 1,
    application text	 NOT NULL,
    roles       json     NOT NULL DEFAULT "",

    CONSTRAINT fk_token_user FOREIGN KEY (user_id) REFERENCES user(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS bookmark (
    id          integer  PRIMARY KEY AUTOINCREMENT,
    uid         text     UNIQUE NOT NULL,
    user_id     integer  NOT NULL,
    created     datetime NOT NULL,
    updated     datetime NOT NULL,
    is_marked   integer  NOT NULL DEFAULT 0,
    is_archived integer  NOT NULL DEFAULT 0,
    is_read     integer  NOT NULL DEFAULT 0,
    state       integer  NOT NULL DEFAULT 0,
    url         text     NOT NULL,
    title       text     NOT NULL,
    domain      text     NOT NULL DEFAULT "",
    site        text     NOT NULL DEFAULT "",
    site_name   text     NOT NULL DEFAULT "",
    published   datetime,
    authors     json     NOT NULL DEFAULT "",
    lang        text     NOT NULL DEFAULT "",
    type        text     NOT NULL DEFAULT "",
    description text     NOT NULL DEFAULT "",
    text        text     NOT NULL DEFAULT "",
    embed       text     NOT NULL DEFAULT "",
    file_path   text     NOT NULL DEFAULT "",
    files       json     NOT NULL DEFAULT "",
    errors      json     NOT NULL DEFAULT "",
    labels      json     NOT NULL DEFAULT "",

    CONSTRAINT fk_bookmark_user FOREIGN KEY (user_id) REFERENCES user(id) ON DELETE CASCADE
);

CREATE VIRTUAL TABLE IF NOT EXISTS bookmark_idx USING fts5(
    tokenize='unicode61 remove_diacritics 2',
    content='bookmark',
    content_rowid='id',
    title,
    description,
    text,
    site,
    author,
    label
);

DROP TRIGGER IF EXISTS bookmark_ai;
CREATE TRIGGER bookmark_ai AFTER INSERT ON bookmark BEGIN
    INSERT INTO bookmark_idx (
        rowid, title, description, text, site, author, label
    ) VALUES (
        new.id, new.title, new.description, new.text, new.site_name || ' ' || new.site || ' ' || new.domain, new.authors, new.labels
    );
END;

DROP TRIGGER IF EXISTS bookmark_au;
CREATE TRIGGER bookmark_au AFTER UPDATE ON bookmark BEGIN
    INSERT INTO bookmark_idx(
        bookmark_idx, rowid, title, description, text, site, author, label
    ) VALUES (
        'delete', old.id, old.title, old.description, old.text, old.site, old.authors, old.labels
    );
    INSERT INTO bookmark_idx (
        rowid, title, description, text, site, author, label
    ) VALUES (
        new.id, new.title, new.description, new.text, new.site_name || ' ' || new.site || ' ' || new.domain, new.authors, new.labels
    );
END;

DROP TRIGGER IF EXISTS bookmark_ad;
CREATE TRIGGER IF NOT EXISTS bookmark_ad AFTER DELETE ON bookmark BEGIN
    INSERT INTO bookmark_idx(
        bookmark_idx, rowid, title, description, text, site, author, label
    ) VALUES (
        'delete', old.id, old.title, old.description, old.text, old.site, old.authors, old.labels
    );
END;
