CREATE TABLE migration (
    id      integer      PRIMARY KEY,
    name    varchar(128) NOT NULL,
    applied timestamptz  NOT NULL
);

CREATE TABLE IF NOT EXISTS "user" (
    id       SERIAL      PRIMARY KEY,
    created  timestamptz NOT NULL,
    updated  timestamptz NOT NULL,
    username text        UNIQUE NOT NULL,
    email    text        UNIQUE NOT NULL,
    password text        NOT NULL
);

CREATE TABLE IF NOT EXISTS token (
    id          SERIAL      PRIMARY KEY,
    uid         text        UNIQUE NOT NULL,
    user_id     integer     NOT NULL,
    created     timestamptz NOT NULL,
    expires     timestamptz NULL,
    is_enabled  boolean     NOT NULL DEFAULT true,
    application text	    NOT NULL,

    CONSTRAINT fk_token_user FOREIGN KEY (user_id) REFERENCES "user"(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS bookmark (
    id          SERIAL      PRIMARY KEY,
    uid         text        UNIQUE NOT NULL,
    user_id     integer     NOT NULL,
    created     timestamptz NOT NULL,
    updated     timestamptz NOT NULL,
    is_marked   boolean     NOT NULL DEFAULT false,
    is_archived boolean     NOT NULL DEFAULT false,
    is_read     boolean     NOT NULL DEFAULT false,
    is_deleted  boolean     NOT NULL DEFAULT false,
    state       integer     NOT NULL DEFAULT 0,
    url         text        NOT NULL,
    title       text        NOT NULL,
    site        text        NOT NULL DEFAULT '',
    site_name   text        NOT NULL DEFAULT '',
    published   timestamptz,
    authors     json        NOT NULL DEFAULT '[]',
    lang        text        NOT NULL DEFAULT '',
    type        text        NOT NULL DEFAULT '',
    description text        NOT NULL DEFAULT '',
    text        text        NOT NULL DEFAULT '',
    embed       text        NOT NULL DEFAULT '',
    file_path   text        NOT NULL DEFAULT '',
    files       json        NOT NULL DEFAULT '[]',
    errors      json        NOT NULL DEFAULT '[]',
    tags        json        NOT NULL DEFAULT '[]',

    CONSTRAINT fk_bookmark_user FOREIGN KEY (user_id) REFERENCES "user"(id) ON DELETE CASCADE
  );
