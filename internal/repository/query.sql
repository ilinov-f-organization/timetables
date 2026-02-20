-- XLSX IMPORT

-- name: CreateStagingTables :exec
SELECT create_staging_tables();

-- name: UpdateStagingHash :exec
SELECT update_staging_hash();

-- name: FlushStagingToMain :exec
SELECT flush_staging_to_main();

-- name: InsertStagingSubgroups :copyfrom
INSERT INTO subgroups_staging (name) VALUES (@name);

-- name: InsertStagingTeachers :copyfrom
INSERT INTO teachers_staging (name) VALUES (@name);

-- name: InsertStagingLocations :copyfrom
INSERT INTO locations_staging (name) VALUES (@name);

-- name: InsertStagingSubjects :copyfrom
INSERT INTO subjects_staging (name) VALUES (@name);

-- name: InsertStagingTimetables :copyfrom
INSERT INTO timetables_staging (name) VALUES (@name);

-- name: InsertStagingLessons :copyfrom
INSERT INTO lessons_staging (staging_id, subject, category, day, time_start, time_end, repeat_rule, timetable)
VALUES (@staging_id, @subject, @category, @day, @time_start, @time_end, @repeat_rule, @timetable);

-- name: InsertStagingSubgroupsAssignments :copyfrom
INSERT INTO subgroups_assignments_staging (staging_id, subgroup)
VALUES (@staging_id, @subgroup);

-- name: InsertStagingTeacherLocationAssignments :copyfrom
INSERT INTO teacher_location_assignments_staging (staging_id, teacher, location)
VALUES (@staging_id, @teacher, @location);

-- LESSONS

-- name: CreateLesson :one
INSERT INTO lessons (subject_id, category, day, time_start, time_end, repeat_rule, timetable_id, hash)
VALUES (@subject_id, @category, @day, @time_start, @time_end, @repeat_rule, @timetable_id,
        md5(
                (SELECT name FROM subjects WHERE id = @subject_id) || '|' ||
                @category || '|' ||
                @day::TEXT || '|' ||
                @time_start::TEXT || '|' ||
                @time_end::TEXT || '|' ||
                @repeat_rule::TEXT || '|' ||
                (SELECT name FROM timetables WHERE id = timetable_id) || '|' ||
                ''))
RETURNING id;

-- name: CreateSubgroupAssignments :copyfrom
INSERT INTO subgroups_assignments (lesson_id, subgroup_id)
VALUES (@lesson_id, @subgroup_id);

-- name: CreateTeacherLocationAssignments :copyfrom
INSERT INTO teacher_location_assignments (lesson_id, teacher_id, location_id)
VALUES (@lesson_id, @teacher_id, @location_id);

-- name: DeleteLesson :exec
DELETE FROM lessons WHERE id = @id;

-- name: GetLesson :one
SELECT l.id, l.subject_id, l.category, l.day, l.time_start, l.time_end, l.repeat_rule, l.timetable_id,
       s.name AS subject_name, tt.name AS timetable_name, tt.date_start, tt.date_end
FROM lessons l
         JOIN subjects s ON s.id = subject_id
         JOIN timetables tt ON tt.id = timetable_id
         WHERE l.id = @id
ORDER BY tt.date_start, l.day, l.time_start, l.repeat_rule;

-- name: GetLessonSubgroups :many
SELECT *, (SELECT name FROM subgroups WHERE subgroups.id = subgroup_id) AS subgroup_name
FROM subgroups_assignments WHERE lesson_id = @lesson_id
ORDER BY subgroup_name;

-- name: GetLessonAssignments :many
SELECT *, (SELECT name FROM teachers WHERE teachers.id = teacher_id) AS teacher_name,
       (SELECT name FROM locations WHERE locations.id = location_id) AS location_name
FROM teacher_location_assignments WHERE lesson_id = @lesson_id
ORDER BY teacher_name, location_name;

-- name: DeleteLessonAssignments :exec
SELECT delete_lesson_assignments(@lesson_id);

-- name: PatchLesson :exec
UPDATE lessons SET subject_id = @subject_id,
                    category = @category,
                    day = @day,
                    time_start = @time_start,
                    time_end = @time_end,
                    repeat_rule = @repeat_rule,
                    timetable_id = @timetable_id,
                    hash = calculate_lesson_hash(@id)
WHERE id = @id;

-- name: GetLessonsByTeacherId :many
SELECT l.id, l.subject_id, l.category, l.day, l.time_start, l.time_end, l.repeat_rule, l.timetable_id,
       s.name AS subject_name, tt.name AS timetable_name, tt.date_start, tt.date_end
FROM lessons l
         JOIN subjects s ON s.id = subject_id
         JOIN timetables tt ON tt.id = timetable_id
WHERE l.id IN (SELECT lesson_id FROM teacher_location_assignments WHERE teacher_location_assignments.teacher_id = @teacher_id)
ORDER BY tt.date_start, l.day, l.time_start, l.repeat_rule;

-- name: GetLessonsSubgroupsByTeacherId :many
SELECT *, (SELECT name FROM subgroups WHERE subgroups.id = subgroup_id) AS subgroup_name
FROM subgroups_assignments
WHERE lesson_id IN (SELECT lesson_id FROM teacher_location_assignments WHERE teacher_location_assignments.teacher_id = @teacher_id)
ORDER BY subgroup_name;

-- name: GetLessonAssignmentsByTeacherId :many
SELECT *, (SELECT name FROM teachers WHERE teachers.id = teacher_id) AS teacher_name,
       (SELECT name FROM locations WHERE locations.id = location_id) AS location_name
FROM teacher_location_assignments
WHERE lesson_id IN (SELECT lesson_id FROM teacher_location_assignments WHERE teacher_location_assignments.teacher_id = @teacher_id)
ORDER BY teacher_name, location_name;


-- name: GetLessonsBySubgroupId :many
SELECT l.id, l.subject_id, l.category, l.day, l.time_start, l.time_end, l.repeat_rule, l.timetable_id,
       s.name AS subject_name, tt.name AS timetable_name, tt.date_start, tt.date_end
FROM lessons l
         JOIN subjects s ON s.id = subject_id
         JOIN timetables tt ON tt.id = timetable_id
WHERE l.id IN (SELECT lesson_id FROM subgroups_assignments WHERE subgroups_assignments.subgroup_id = @subgroup_id)
ORDER BY tt.date_start, l.day, l.time_start, l.repeat_rule;

-- name: GetLessonsSubgroupsBySubgroupId :many
SELECT *, (SELECT name FROM subgroups WHERE subgroups.id = subgroup_id) AS subgroup_name
FROM subgroups_assignments
WHERE lesson_id IN (SELECT lesson_id FROM subgroups_assignments WHERE subgroups_assignments.subgroup_id = @subgroup_id)
ORDER BY subgroup_name;

-- name: GetLessonAssignmentsBySubgroupId :many
SELECT *, (SELECT name FROM teachers WHERE teachers.id = teacher_id) AS teacher_name,
       (SELECT name FROM locations WHERE locations.id = location_id) AS location_name
FROM teacher_location_assignments
WHERE lesson_id IN (SELECT lesson_id FROM subgroups_assignments WHERE subgroups_assignments.subgroup_id = @subgroup_id)
ORDER BY teacher_name, location_name;

-- name: GetLessonsByLocationsId :many
SELECT l.id, l.subject_id, l.category, l.day, l.time_start, l.time_end, l.repeat_rule, l.timetable_id,
       s.name AS subject_name, tt.name AS timetable_name, tt.date_start, tt.date_end
FROM lessons l
         JOIN subjects s ON s.id = subject_id
         JOIN timetables tt ON tt.id = timetable_id
WHERE l.id IN (SELECT lesson_id FROM teacher_location_assignments WHERE teacher_location_assignments.location_id = @location_id)
ORDER BY tt.date_start, l.day, l.time_start, l.repeat_rule;

-- name: GetLessonsSubgroupsByLocationId :many
SELECT *, (SELECT name FROM subgroups WHERE subgroups.id = subgroup_id) AS subgroup_name
FROM subgroups_assignments
WHERE lesson_id IN (SELECT lesson_id FROM teacher_location_assignments WHERE teacher_location_assignments.location_id = @location_id)
ORDER BY subgroup_name;

-- name: GetLessonAssignmentsByLocationId :many
SELECT *, (SELECT name FROM teachers WHERE teachers.id = teacher_id) AS teacher_name,
       (SELECT name FROM locations WHERE locations.id = location_id) AS location_name
FROM teacher_location_assignments
WHERE lesson_id IN (SELECT lesson_id FROM teacher_location_assignments WHERE teacher_location_assignments.location_id = @location_id)
ORDER BY teacher_name, location_name;

-- name: GetLessonsBySubjectId :many
SELECT l.id, l.subject_id, l.category, l.day, l.time_start, l.time_end, l.repeat_rule, l.timetable_id,
       s.name AS subject_name, tt.name AS timetable_name, tt.date_start, tt.date_end
FROM lessons l
         JOIN subjects s ON s.id = subject_id
         JOIN timetables tt ON tt.id = timetable_id
WHERE subject_id = @subject_id
ORDER BY tt.date_start, l.day, l.time_start, l.repeat_rule;

-- name: GetLessonsSubgroupsBySubjectId :many
SELECT *, (SELECT name FROM subgroups WHERE subgroups.id = subgroup_id) AS subgroup_name
FROM subgroups_assignments
WHERE lesson_id IN (SELECT lessons.id FROM lessons WHERE subject_id = @subject_id)
ORDER BY subgroup_name;

-- name: GetLessonAssignmentsBySubjectId :many
SELECT *, (SELECT name FROM teachers WHERE teachers.id = teacher_id) AS teacher_name,
       (SELECT name FROM locations WHERE locations.id = location_id) AS location_name
FROM teacher_location_assignments
WHERE lesson_id IN (SELECT lessons.id FROM lessons WHERE subject_id = @subject_id)
ORDER BY teacher_name, location_name;

-- LOCATIONS

-- name: GetLocationsOnPage :many
SELECT id, name FROM locations
WHERE (sqlc.narg(name)::TEXT IS NULL OR name ILIKE '%' || sqlc.narg(name)::TEXT || '%')
ORDER BY name
LIMIT sqlc.arg(page_size)::INTEGER
    OFFSET sqlc.arg(page_size)::INTEGER * (sqlc.arg(page)::INTEGER - 1);

-- name: GetLocationsPagesAmount :one
SELECT CEILING(COUNT(*) / (@page_size::INT)::FLOAT)::INT FROM locations;

-- name: CreateLocation :one
INSERT INTO locations (name)
VALUES (@name) RETURNING id, name;

-- name: GetLocationById :one
SELECT *
FROM locations
WHERE id = @id;

-- name: PatchLocationById :one
UPDATE locations
SET name = @name
WHERE id = @id RETURNING id, name;

-- name: DeleteLocationById :one
DELETE
FROM locations
WHERE id = @id RETURNING id, name;

-- SUBGROUPS

-- name: GetSubgroupsOnPage :many
SELECT id, name FROM subgroups
WHERE (sqlc.narg(name)::TEXT IS NULL OR name ILIKE '%' || sqlc.narg(name)::TEXT || '%')
ORDER BY name
LIMIT sqlc.arg(page_size)::INTEGER
    OFFSET sqlc.arg(page_size)::INTEGER * (sqlc.arg(page)::INTEGER - 1);

-- name: GetSubgroupsPagesAmount :one
SELECT CEILING(COUNT(*) / (@page_size::INT)::FLOAT)::INT FROM subgroups;

-- name: CreateSubgroup :one
INSERT INTO subgroups (name)
VALUES (@name) RETURNING id, name;

-- name: GetSubgroupById :one
SELECT *
FROM subgroups
WHERE id = @id;

-- name: PatchSubgroupById :one
UPDATE subgroups
SET name = @name
WHERE id = @id RETURNING id, name;

-- name: DeleteSubgroupById :one
DELETE
FROM subgroups
WHERE id = @id RETURNING id, name;