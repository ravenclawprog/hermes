CREATE TABLE users (
    id text PRIMARY KEY,
    username text NOT NULL UNIQUE
);

CREATE TABLE workspaces (
    id text PRIMARY KEY,
    owner_id text NOT NULL REFERENCES users (id)
);

CREATE TABLE documents (
    id text PRIMARY KEY,
    workspace_id text NOT NULL REFERENCES workspaces (id),
    title text NOT NULL CHECK (btrim(title) <> ''),
    content text NOT NULL,
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

INSERT INTO users (id, username) VALUES ('demo-user', 'demo');
INSERT INTO workspaces (id, owner_id) VALUES ('demo-workspace', 'demo-user');
INSERT INTO documents (id, workspace_id, title, content)
VALUES ('example', 'demo-workspace', 'Welcome to Hermes',
        E'# Welcome to Hermes\n\nThis is a sample document.\n');
