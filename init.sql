CREATE TABLE
    numbers (
        id SERIAL PRIMARY KEY,
        number INT NOT NULL,
        is_even BOOLEAN NOT NULL,
        processed_at TIMESTAMP NOT NULL DEFAULT NOW ()
    );

CREATE TABLE
    stats (
        id SERIAL PRIMARY KEY,
        even_count INT NOT NULL DEFAULT 0,
        odd_count INT NOT NULL DEFAULT 0,
        worker_id INT NOT NULL
    );

INSERT INTO
    stats (worker_id, even_count, odd_count)
VALUES
    (1, 0, 0),
    (2, 0, 0);