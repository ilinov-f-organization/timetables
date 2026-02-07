-- +goose Up
-- +goose StatementBegin
CREATE TABLE subgroups
(
    id   INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE teachers
(
    id   INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE locations
(
    id   INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE subjects
(
    id   INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE timetables
(
    id         INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name       TEXT        NOT NULL UNIQUE,
    date_start TIMESTAMPTZ NOT NULL,
    date_end   TIMESTAMPTZ NOT NULL
);

CREATE TABLE lessons
(
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    hash         TEXT    NOT NULL UNIQUE,
    subject_id   INTEGER NOT NULL,
    category     TEXT    NOT NULL,
    day          INTEGER NOT NULL,
    time_start   INTEGER NOT NULL,
    time_end     INTEGER NOT NULL,
    repeat_rule  INTEGER NOT NULL,
    timetable_id INTEGER NOT NULL,

    FOREIGN KEY (subject_id) REFERENCES subjects (id) ON DELETE RESTRICT,
    FOREIGN KEY (timetable_id) REFERENCES timetables (id) ON DELETE CASCADE
);

CREATE TABLE subgroups_assignments
(
    lesson_id   UUID    NOT NULL,
    subgroup_id INTEGER NOT NULL,
    FOREIGN KEY (lesson_id) REFERENCES lessons (id) ON DELETE CASCADE,
    FOREIGN KEY (subgroup_id) REFERENCES subgroups (id) ON DELETE RESTRICT,
    UNIQUE (lesson_id, subgroup_id)
);

CREATE TABLE teacher_location_assignments
(
    lesson_id   UUID    NOT NULL,
    teacher_id  INTEGER NOT NULL,
    location_id INTEGER NOT NULL,
    FOREIGN KEY (lesson_id) REFERENCES lessons (id) ON DELETE CASCADE,
    FOREIGN KEY (teacher_id) REFERENCES teachers (id) ON DELETE RESTRICT,
    FOREIGN KEY (location_id) REFERENCES locations (id) ON DELETE RESTRICT,
    UNIQUE (lesson_id, teacher_id, location_id)
);

CREATE INDEX idx_lessons_hash
    ON lessons (hash);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE teacher_location_assignments;
DROP TABLE subgroups_assignments;
DROP TABLE lessons;
DROP TABLE timetables;
DROP TABLE subjects;
DROP TABLE locations;
DROP TABLE teachers;
DROP TABLE subgroups;
-- +goose StatementEnd
