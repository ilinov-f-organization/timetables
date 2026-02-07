-- name: CreateSubgroups :copyfrom
INSERT INTO subgroups (name)
VALUES (sqlc.arg('name'));

-- name: GetAllSubgroups :many
SELECT id, name
FROM subgroups;

-- name: GetPaginatedSubgroups :many
SELECT id, name
FROM subgroups
WHERE (sqlc.arg('name') IS NULL OR name ILIKE '%' || sqlc.arg('name') || '%')
ORDER BY name
LIMIT sqlc.arg('page_size')::INTEGER OFFSET sqlc.arg('page_size')::INTEGER * (sqlc.arg('page')::INTEGER);

-- name: GetOrCreateSubgroupByName :one
INSERT INTO subgroups (name)
VALUES (sqlc.arg('name'))
ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
RETURNING id;

-- name: GetSubgroupById :one
SELECT id, name
FROM subgroups
WHERE id = sqlc.arg('id');

-- name: PatchSubgroupById :one
UPDATE subgroups
SET name = sqlc.arg('name')
WHERE id = sqlc.arg('id')
RETURNING id, name;

-- name: DeleteSubgroupById :one
DELETE
FROM subgroups
WHERE id = sqlc.arg('id')
RETURNING id, name;

-- name: CreateTeachers :copyfrom
INSERT INTO teachers (name)
VALUES (sqlc.arg('name'));

-- name: GetAllTeachers :many
SELECT id, name
FROM teachers;

-- name: GetPaginatedTeachers :many
SELECT id, name
FROM teachers
WHERE (sqlc.arg('name') IS NULL OR name ILIKE '%' || sqlc.arg('name') || '%')
ORDER BY name
LIMIT sqlc.arg('page_size')::INTEGER OFFSET sqlc.arg('page_size')::INTEGER * (sqlc.arg('page')::INTEGER);

-- name: GetOrCreateTeacherByName :one
INSERT INTO teachers (name)
VALUES (sqlc.arg('name'))
ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
RETURNING id;

-- name: GetTeacherById :one
SELECT id, name
FROM teachers
WHERE id = sqlc.arg('id');

-- name: PatchTeacherById :one
UPDATE teachers
SET name = sqlc.arg('name')
WHERE id = sqlc.arg('id')
RETURNING id, name;

-- name: DeleteTeacherById :one
DELETE
FROM teachers
WHERE id = sqlc.arg('id')
RETURNING id, name;

-- name: CreateLocations :copyfrom
INSERT INTO locations (name)
VALUES (sqlc.arg('name'));

-- name: GetAllLocations :many
SELECT id, name
FROM locations;

-- name: GetPaginatedLocations :many
SELECT id, name
FROM locations
WHERE (sqlc.arg('name') IS NULL OR name ILIKE '%' || sqlc.arg('name') || '%')
ORDER BY name
LIMIT sqlc.arg('page_size')::INTEGER OFFSET sqlc.arg('page_size')::INTEGER * (sqlc.arg('page')::INTEGER);

-- name: GetOrCreateLocationByName :one
INSERT INTO locations (name)
VALUES (sqlc.arg('name'))
ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
RETURNING id;

-- name: GetLocationById :one
SELECT id, name
FROM locations
WHERE id = sqlc.arg('id');

-- name: PatchLocationById :one
UPDATE locations
SET name = sqlc.arg('name')
WHERE id = sqlc.arg('id')
RETURNING id, name;

-- name: DeleteLocationById :one
DELETE
FROM locations
WHERE id = sqlc.arg('id')
RETURNING id, name;

-- name: CreateSubjects :copyfrom
INSERT INTO subjects (name)
VALUES (sqlc.arg('name'));

-- name: GetAllSubjects :many
SELECT id, name
FROM subjects;

-- name: GetPaginatedSubjects :many
SELECT id, name
FROM subjects
WHERE (sqlc.arg('name') IS NULL OR name ILIKE '%' || sqlc.arg('name') || '%')
ORDER BY name
LIMIT sqlc.arg('page_size')::INTEGER OFFSET sqlc.arg('page_size')::INTEGER * (sqlc.arg('page')::INTEGER);

-- name: GetOrCreateSubjectByName :one
INSERT INTO subjects (name)
VALUES (sqlc.arg('name'))
ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
RETURNING id;

-- name: GetSubjectById :one
SELECT id, name
FROM subjects
WHERE id = sqlc.arg('id');

-- name: PatchSubjectById :one
UPDATE subjects
SET name = sqlc.arg('name')
WHERE id = sqlc.arg('id')
RETURNING id, name;

-- name: DeleteSubjectById :one
DELETE
FROM subjects
WHERE id = sqlc.arg('id')
RETURNING id, name;

-- name: CreateTimetables :copyfrom
INSERT INTO timetables (name, date_start, date_end)
VALUES (sqlc.arg('name'), sqlc.arg('date_start'), sqlc.arg('date_end'));

-- name: GetAllTimetables :many
SELECT id, name, date_start, date_end
FROM timetables;

-- name: GetPaginatedTimetables :many
SELECT id, name
FROM timetables
WHERE (sqlc.arg('name') IS NULL OR name ILIKE '%' || sqlc.arg('name') || '%')
ORDER BY name
LIMIT sqlc.arg('page_size')::INTEGER OFFSET sqlc.arg('page_size')::INTEGER * (sqlc.arg('page')::INTEGER);

-- name: GetOrCreateTimetableByName :one
INSERT INTO timetables (name, date_start, date_end)
VALUES (sqlc.arg('name'), sqlc.arg('date_start'), sqlc.arg('date_end'))
ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
RETURNING id;

-- name: GetTimetableById :one
SELECT id, name, date_start, date_end
FROM timetables
WHERE id = sqlc.arg('id');

-- name: PatchTimetableById :one
UPDATE timetables
SET name       = sqlc.arg('name'),
    date_start = sqlc.arg('date_start'),
    date_end   = sqlc.arg('date_end')
WHERE id = sqlc.arg('id')
RETURNING id, name, date_start, date_end;

-- name: DeleteTimetableById :one
DELETE
FROM timetables
WHERE id = sqlc.arg('id')
RETURNING id, name;

-- name: CreateLesson :one
INSERT INTO lessons (hash, subject_id, category, day, time_start, time_end, repeat_rule, timetable_id)
VALUES (sqlc.arg('hash'), sqlc.arg('subject_id'), sqlc.arg('category'), sqlc.arg('day'), sqlc.arg('time_start'),
        sqlc.arg('time_end'), sqlc.arg('repeat_rule'), sqlc.arg('timetable_id'))
RETURNING id;

-- name: CreateLessons :copyfrom
INSERT INTO lessons (hash, subject_id, category, day, time_start, time_end, repeat_rule, timetable_id)
VALUES (sqlc.arg('hash'), sqlc.arg('subject_id'), sqlc.arg('category'), sqlc.arg('day'), sqlc.arg('time_start'),
        sqlc.arg('time_end'), sqlc.arg('repeat_rule'), sqlc.arg('timetable_id'));

-- name: GetAllLessons :many
SELECT *
FROM lessons;

-- name: GetLessonIDByHash :one
SELECT id FROM lessons WHERE hash = sqlc.arg('hash');

-- name: PatchLessonById :one
UPDATE lessons
SET hash         = sqlc.arg('hash'),
    category     = sqlc.arg('category'),
    day          = sqlc.arg('day'),
    time_start   = sqlc.arg('time_start'),
    time_end     = sqlc.arg('time_end'),
    repeat_rule  = sqlc.arg('repeat_rule'),
    timetable_id = sqlc.arg('timetable_id')
WHERE id = sqlc.arg('lesson_id')
RETURNING id;

-- name: AssignSubgroupToLesson :exec
INSERT INTO subgroups_assignments (lesson_id, subgroup_id)
VALUES (sqlc.arg('lesson_id'), sqlc.arg('subgroup_id'))
ON CONFLICT (lesson_id, subgroup_id) DO NOTHING;

-- name: AssignTeacherLocationToLesson :exec
INSERT INTO teacher_location_assignments (lesson_id, teacher_id, location_id)
VALUES (sqlc.arg('lesson_id'), sqlc.arg('teacher_id'),
        sqlc.arg('location_id'))
ON CONFLICT (lesson_id, teacher_id, location_id) DO NOTHING;

-- name: DeleteLessonById :one
DELETE
FROM lessons
WHERE id = sqlc.arg('lesson_id')
RETURNING id;

-- name: GetLessonsBySubgroupId :many
SELECT id,
       subject_id,
       (SELECT name FROM subjects WHERE id = subject_id)           as subject_name,
       category,
       day,
       time_start,
       time_end,
       repeat_rule,
       timetable_id,
       (SELECT name FROM timetables WHERE id = timetable_id)       as timetable_name,
       (SELECT date_start FROM timetables WHERE id = timetable_id) as timetable_date_start,
       (SELECT date_end FROM timetables WHERE id = timetable_id)   as timetable_date_end
FROM lessons
         JOIN subgroups_assignments
              ON subgroups_assignments.lesson_id = id AND subgroups_assignments.subgroup_id = sqlc.arg('subgroup_id');

-- name: GetTeacherLocationAssignmentsByLessonId :many
SELECT teacher_id,
       (SELECT name FROM teachers WHERE id = teacher_id)   AS teacher_name,
       location_id,
       (SELECT name FROM locations WHERE id = location_id) AS location_name
FROM teacher_location_assignments
WHERE lesson_id = sqlc.arg('lesson_id') ORDER BY teacher_name, location_name;

-- name: GetSubgroupsAssignmentByLessonId :many
SELECT subgroup_id, (SELECT name FROM subgroups WHERE id = subgroup_id) AS subgroup_name
FROM subgroups_assignments
WHERE lesson_id = sqlc.arg('lesson_id') ORDER BY subgroup_name;