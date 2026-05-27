CREATE TABLE IF NOT EXISTS users (
    id            TEXT     PRIMARY KEY,
    nickname      TEXT     NOT NULL UNIQUE,
    email         TEXT     NOT NULL UNIQUE,
    password_hash TEXT     NOT NULL,
    age           INTEGER  NOT NULL CHECK (age > 0 AND age < 150),
    gender        TEXT     NOT NULL,
    first_name    TEXT     NOT NULL,
    last_name     TEXT     NOT NULL,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sessions (
    token      TEXT     PRIMARY KEY,
    user_id    TEXT     NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);

CREATE TABLE IF NOT EXISTS posts (
    id         TEXT     PRIMARY KEY,
    user_id    TEXT     NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title      TEXT     NOT NULL,
    content    TEXT     NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_posts_created ON posts(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_posts_user    ON posts(user_id);

CREATE TABLE IF NOT EXISTS categories (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT    NOT NULL UNIQUE
);

-- Categories are seeded from db.go migrate() so the same code can also
-- rename rows that were inserted by an earlier taxonomy.

CREATE TABLE IF NOT EXISTS post_categories (
    post_id     TEXT    NOT NULL REFERENCES posts(id)      ON DELETE CASCADE,
    category_id INTEGER NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    PRIMARY KEY (post_id, category_id)
);

CREATE TABLE IF NOT EXISTS comments (
    id         TEXT     PRIMARY KEY,
    post_id    TEXT     NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    user_id    TEXT     NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content    TEXT     NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_comments_post    ON comments(post_id, created_at);
CREATE INDEX IF NOT EXISTS idx_comments_user    ON comments(user_id);

CREATE TABLE IF NOT EXISTS messages (
    id          TEXT     PRIMARY KEY,
    sender_id   TEXT     NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    receiver_id TEXT     NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content     TEXT     NOT NULL,
    image_url   TEXT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Two indexes cover the symmetric conversation lookup:
-- "messages between A and B" hits one or the other.
CREATE INDEX IF NOT EXISTS idx_messages_sr ON messages(sender_id, receiver_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_messages_rs ON messages(receiver_id, sender_id, created_at DESC);

-- Reactions are stored in two tables (one per target type) instead of a
-- generic polymorphic table — this lets us keep ON DELETE CASCADE FKs on
-- both sides, which would be impossible with a single table.
--
-- value is constrained to -1 (dislike) or 1 (like); to clear a reaction we
-- delete the row.
CREATE TABLE IF NOT EXISTS post_reactions (
    user_id    TEXT     NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    post_id    TEXT     NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    value      INTEGER  NOT NULL CHECK (value IN (-1, 1)),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, post_id)
);

CREATE INDEX IF NOT EXISTS idx_post_reactions_post ON post_reactions(post_id);

CREATE TABLE IF NOT EXISTS comment_reactions (
    user_id    TEXT     NOT NULL REFERENCES users(id)    ON DELETE CASCADE,
    comment_id TEXT     NOT NULL REFERENCES comments(id) ON DELETE CASCADE,
    value      INTEGER  NOT NULL CHECK (value IN (-1, 1)),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, comment_id)
);

CREATE INDEX IF NOT EXISTS idx_comment_reactions_comment ON comment_reactions(comment_id);

-- chat_reads stores, per user, when they last opened the chat with each
-- peer. Used by GET /api/users to flag conversations with unread messages
-- and by chat.js to render "read" receipts on the sender side.
CREATE TABLE IF NOT EXISTS chat_reads (
    user_id      TEXT     NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    peer_id      TEXT     NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    last_read_at DATETIME NOT NULL,
    PRIMARY KEY (user_id, peer_id)
);
