package application

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"
	api "timetables/internal/api"
	"timetables/internal/lib"
	repository "timetables/internal/repository"
	sqlc "timetables/internal/repository/sqlc"

	"github.com/google/uuid"
)

const multipartMaxMemory = 128 * 1024 * 1024

var (
	errFileNotProvided = errors.New("file not provided")
)

var (
	defaultDateStart = time.Unix(0, 0)
	defaultDateEnd   = time.Unix(0, 0)
)

type Server struct {
	repo *repository.Repo
}

func NewServer(repo *repository.Repo) *Server {
	return &Server{
		repo: repo,
	}
}

func (s Server) GetErrors(ctx context.Context, request api.GetErrorsRequestObject) (api.GetErrorsResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (s Server) GetErrorsId(ctx context.Context, request api.GetErrorsIdRequestObject) (api.GetErrorsIdResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (s *Server) PostImport(ctx context.Context, request api.PostImportRequestObject) (api.PostImportResponseObject, error) {
	files, err := unarchive(request)
	if err != nil {
		return nil, err
	}

	parser := lib.NewXlsxParser()

	//if err := loadCashFromDB(ctx, s, parser); err != nil {
	//}

	for fileName, file := range files {
		if strings.Contains(fileName, "ehkzamenov") {
			continue
		}
		err := parser.Parse(file)
		if err != nil {
			slog.Error("error in parsing xlsx", "file", fileName, "err", err.Error())
			continue
		}
	}

	if err := writeCashToDB(ctx, s, parser); err != nil {
		return nil, err
	}

	for _, hash := range parser.LessonsHashes() {
		if len(parser.GetLessonByHash(hash).Subgroups()) > 1 {
			lesson := parser.GetLessonByHash(hash)
			fmt.Printf("%s %s %s ", lesson.Timetable(), lesson.Subject(), lesson.Category())
			for _, sg := range lesson.Subgroups() {
				fmt.Printf("%s ", sg)
			}
			fmt.Printf("\n")
		}
	}

	fmt.Printf("LocationsNames: %d\nSubgroups: %d\nSubjects: %d\nTeachers: %d\nTimetables: %d\nLessons: %d\n", len(parser.LocationsNames()), len(parser.SubgroupsNames()), len(parser.SubjectsNames()), len(parser.TeachersNames()), len(parser.TimetablesNames()), len(parser.LessonsHashes()))
	//TODO implement me
	//panic("implement me")
	return nil, nil
}

func writeCashToDB(ctx context.Context, s *Server, parser *lib.XlsxParser) error {
	tx, err := s.repo.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	qtx := s.repo.WithTx(tx)

	if err := writeTeachersCashToDB(ctx, qtx, parser); err != nil {
		return err
	}
	if err := writeLocationsCashToDB(ctx, qtx, parser); err != nil {
		return err
	}
	if err := writeSubjectsCashToDB(ctx, qtx, parser); err != nil {
		return err
	}
	if err := writeSubgroupsCashToDB(ctx, qtx, parser); err != nil {
		return err
	}
	if err := writeTimetablesCashToDB(ctx, qtx, parser); err != nil {
		return err
	}
	if err := writeLessonsCashToDB(ctx, qtx, parser); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func writeTeachersCashToDB(ctx context.Context, qtx *sqlc.Queries, parser *lib.XlsxParser) error {
	if _, err := qtx.CreateTeachers(ctx, parser.TeachersNames()); err != nil {
		return err
	}
	teachers, err := qtx.GetAllTeachers(ctx)
	if err != nil {
		return err
	}
	for _, teacher := range teachers {
		if err := parser.RewriteCacheTeacher(teacher.Name, teacher.ID); err != nil {
			return err
		}
	}

	return nil
}

func writeLocationsCashToDB(ctx context.Context, qtx *sqlc.Queries, parser *lib.XlsxParser) error {
	if _, err := qtx.CreateLocations(ctx, parser.LocationsNames()); err != nil {
		return err
	}
	locations, err := qtx.GetAllLocations(ctx)
	if err != nil {
		return err
	}
	for _, location := range locations {
		if err := parser.RewriteCacheLocation(location.Name, location.ID); err != nil {
			return err
		}
	}

	return nil
}

func writeSubjectsCashToDB(ctx context.Context, qtx *sqlc.Queries, parser *lib.XlsxParser) error {
	if _, err := qtx.CreateSubjects(ctx, parser.SubjectsNames()); err != nil {
		return err
	}
	subjects, err := qtx.GetAllSubjects(ctx)
	if err != nil {
		return err
	}
	for _, subject := range subjects {
		if err := parser.RewriteCacheSubject(subject.Name, subject.ID); err != nil {
			return err
		}
	}

	return nil
}

func writeSubgroupsCashToDB(ctx context.Context, qtx *sqlc.Queries, parser *lib.XlsxParser) error {
	if _, err := qtx.CreateSubgroups(ctx, parser.SubgroupsNames()); err != nil {
		return err
	}
	subgroups, err := qtx.GetAllSubgroups(ctx)
	if err != nil {
		return err
	}
	for _, subgroup := range subgroups {
		if err := parser.RewriteCacheSubgroup(subgroup.Name, subgroup.ID); err != nil {
			return err
		}
	}

	return nil
}

func writeTimetablesCashToDB(ctx context.Context, qtx *sqlc.Queries, parser *lib.XlsxParser) error {
	createParams := make([]sqlc.CreateTimetablesParams, 0)

	for _, timetableName := range parser.TimetablesNames() {
		timetable := parser.GetOrCacheTimetable(timetableName, nil)
		createParams = append(createParams, sqlc.CreateTimetablesParams{
			Name:      timetableName,
			DateStart: timetable.DateStart(),
			DateEnd:   timetable.DateEnd(),
		})
	}

	if _, err := qtx.CreateTimetables(ctx, createParams); err != nil {
		return err
	}
	timetables, err := qtx.GetAllTimetables(ctx)
	if err != nil {
		return err
	}
	for _, timetable := range timetables {

		if err := parser.RewriteCacheTimetable(timetable.Name,
			lib.NewTimetable(timetable.ID, timetable.DateStart, timetable.DateEnd)); err != nil {
			return err
		}
	}

	return nil
}

func writeLessonsCashToDB(ctx context.Context, qtx *sqlc.Queries, parser *lib.XlsxParser) error {
	createParams := make([]sqlc.CreateLessonsParams, 0)

	for _, hash := range parser.LessonsHashes() {
		lesson := parser.GetLessonByHash(hash)
		createParams = append(createParams, sqlc.CreateLessonsParams{
			Hash:        hash,
			SubjectID:   parser.GetOrCacheSubject(lesson.Subject(), 0),
			Category:    lesson.Category(),
			Day:         lesson.Day(),
			TimeStart:   lesson.TimeStart(),
			TimeEnd:     lesson.TimeEnd(),
			RepeatRule:  lesson.RepeatRule(),
			TimetableID: parser.GetOrCacheTimetable(lesson.Timetable(), nil).Id(),
		})

	}

	_, err := qtx.CreateLessons(ctx, createParams)
	if err != nil {
		return err
	}

	for _, hash := range parser.LessonsHashes() {
		lessonID, err := qtx.GetLessonIDByHash(ctx, hash)
		if err != nil {
			return err
		}
		lesson := parser.GetLessonByHash(hash)
		for _, assignment := range lesson.TeacherLocationAssignments() {
			err := qtx.AssignTeacherLocationToLesson(ctx, sqlc.AssignTeacherLocationToLessonParams{
				LessonID:   lessonID,
				TeacherID:  parser.GetOrCacheTeacher(assignment.Teacher(), 0),
				LocationID: parser.GetOrCacheLocation(assignment.Location(), 0),
			})
			if err != nil {
				return err
			}
		}

		for _, subgroup := range lesson.Subgroups() {
			err := qtx.AssignSubgroupToLesson(ctx, sqlc.AssignSubgroupToLessonParams{
				LessonID:   lessonID,
				SubgroupID: parser.GetOrCacheSubgroup(subgroup, 0),
			})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func unarchive(request api.PostImportRequestObject) (map[string][]byte, error) {
	files := make(map[string][]byte, 0)
	form, err := request.Body.ReadForm(multipartMaxMemory)
	if err != nil {
		return nil, err
	}

	archive := form.File["file"]
	if len(archive) == 0 {
		return nil, errFileNotProvided
	}

	fileHeader := archive[0]
	src, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()

	reader, err := zip.NewReader(src, archive[0].Size)
	if err != nil {
		return nil, err
	}
	for _, file := range reader.File {
		open, err := file.Open()
		if err != nil {
			return nil, err
		}
		bytes, err := io.ReadAll(open)
		if err != nil {
			return nil, err
		}

		files[file.Name] = bytes
	}

	return files, nil
}

func (s Server) PostLessons(ctx context.Context, request api.PostLessonsRequestObject) (api.PostLessonsResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (s Server) GetLessonsLocationsId(ctx context.Context, request api.GetLessonsLocationsIdRequestObject) (api.GetLessonsLocationsIdResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (s Server) GetLessonsSubgroupsId(ctx context.Context, request api.GetLessonsSubgroupsIdRequestObject) (api.GetLessonsSubgroupsIdResponseObject, error) {
	lessons, err := assembleLessons(ctx, &s, &request)
	if err != nil {
		return nil, err
	}
	row, err := s.repo.GetSubgroupById(ctx, int32(request.Id))
	if err != nil {
		return nil, err
	}

	switch *request.Params.Format {
	case api.GetLessonsSubgroupsIdParamsFormatIcs:
		calendar, err := lib.SerializeICS(lessons, row.Name)
		if err != nil {
			return nil, err
		}

		reader := bytes.NewReader(calendar)
		return api.GetLessonsSubgroupsId200TextcalendarResponse{
			Body:          reader,
			ContentLength: int64(len(calendar)),
		}, nil
	default:
		return api.GetLessonsSubgroupsId200JSONResponse(lessons), nil
	}
}

func assembleLessons(ctx context.Context, s *Server, request *api.GetLessonsSubgroupsIdRequestObject) ([]api.Lesson, error) {
	tx, err := s.repo.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	qtx := s.repo.WithTx(tx)

	lessons := make([]api.Lesson, 0)

	rows, err := qtx.GetLessonsBySubgroupId(ctx, int32(request.Id))
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		subgroups, err := getSubgroups(ctx, qtx, row.ID)
		if err != nil {
			return nil, err
		}
		assignments, err := getAssignments(ctx, qtx, row.ID)
		if err != nil {
			return nil, err
		}

		day := int(row.Day)
		repeatRule := int(row.RepeatRule)
		subjectId := int(row.SubjectID)
		timeStart := int(row.TimeStart)
		timeEnd := int(row.TimeEnd)
		timetableId := int(row.TimetableID)

		lesson := api.Lesson{
			Category:   &row.Category,
			Day:        &day,
			Id:         &row.ID,
			RepeatRule: &repeatRule,
			Subgroups:  &subgroups,
			Subject: &api.Subject{
				Id:   &subjectId,
				Name: &row.SubjectName,
			},
			TeacherLocationAssignments: &assignments,
			TimeEnd:                    &timeEnd,
			TimeStart:                  &timeStart,
			Timetable: &api.Timetable{
				EndDate:   &row.TimetableDateEnd,
				Id:        &timetableId,
				Name:      &row.TimetableName,
				StartDate: &row.TimetableDateStart,
			},
		}

		lessons = append(lessons, lesson)
	}

	return lessons, tx.Commit(ctx)
}

func getAssignments(ctx context.Context, qtx *sqlc.Queries, id uuid.UUID) ([]api.TeacherLocationAssignment, error) {
	rows, err := qtx.GetTeacherLocationAssignmentsByLessonId(ctx, id)
	if err != nil {
		return nil, err
	}

	assignments := make([]api.TeacherLocationAssignment, 0)
	for _, row := range rows {
		locationId := int(row.LocationID)
		teacherId := int(row.TeacherID)
		assignments = append(assignments, api.TeacherLocationAssignment{
			Location: &api.Location{
				Id:   &locationId,
				Name: &row.LocationName,
			},
			Teacher: &api.Teacher{
				Id:   &teacherId,
				Name: &row.TeacherName,
			},
		})
	}
	return assignments, nil
}

func getSubgroups(ctx context.Context, qtx *sqlc.Queries, id uuid.UUID) ([]api.Subgroup, error) {
	rows, err := qtx.GetSubgroupsAssignmentByLessonId(ctx, id)
	if err != nil {
		return nil, err
	}
	subgroups := make([]api.Subgroup, 0)

	for _, row := range rows {
		subgroupId := int(row.SubgroupID)
		subgroups = append(subgroups, api.Subgroup{
			Id:   &subgroupId,
			Name: &row.SubgroupName,
		})
	}

	return subgroups, nil
}

func (s Server) GetLessonsSubjectsId(ctx context.Context, request api.GetLessonsSubjectsIdRequestObject) (api.GetLessonsSubjectsIdResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (s Server) GetLessonsTeachersId(ctx context.Context, request api.GetLessonsTeachersIdRequestObject) (api.GetLessonsTeachersIdResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (s Server) DeleteLessonsId(ctx context.Context, request api.DeleteLessonsIdRequestObject) (api.DeleteLessonsIdResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (s Server) GetLessonsId(ctx context.Context, request api.GetLessonsIdRequestObject) (api.GetLessonsIdResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (s Server) PatchLessonsId(ctx context.Context, request api.PatchLessonsIdRequestObject) (api.PatchLessonsIdResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (s Server) GetLocations(ctx context.Context, request api.GetLocationsRequestObject) (api.GetLocationsResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (s Server) PostLocations(ctx context.Context, request api.PostLocationsRequestObject) (api.PostLocationsResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (s Server) DeleteLocationsId(ctx context.Context, request api.DeleteLocationsIdRequestObject) (api.DeleteLocationsIdResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (s Server) GetLocationsId(ctx context.Context, request api.GetLocationsIdRequestObject) (api.GetLocationsIdResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (s Server) PatchLocationsId(ctx context.Context, request api.PatchLocationsIdRequestObject) (api.PatchLocationsIdResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (s Server) GetSubgroups(ctx context.Context, request api.GetSubgroupsRequestObject) (api.GetSubgroupsResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (s Server) PostSubgroups(ctx context.Context, request api.PostSubgroupsRequestObject) (api.PostSubgroupsResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (s Server) DeleteSubgroupsId(ctx context.Context, request api.DeleteSubgroupsIdRequestObject) (api.DeleteSubgroupsIdResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (s Server) GetSubgroupsId(ctx context.Context, request api.GetSubgroupsIdRequestObject) (api.GetSubgroupsIdResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (s Server) PatchSubgroupsId(ctx context.Context, request api.PatchSubgroupsIdRequestObject) (api.PatchSubgroupsIdResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (s Server) GetSubjects(ctx context.Context, request api.GetSubjectsRequestObject) (api.GetSubjectsResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (s Server) PostSubjects(ctx context.Context, request api.PostSubjectsRequestObject) (api.PostSubjectsResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (s Server) DeleteSubjectsId(ctx context.Context, request api.DeleteSubjectsIdRequestObject) (api.DeleteSubjectsIdResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (s Server) GetSubjectsId(ctx context.Context, request api.GetSubjectsIdRequestObject) (api.GetSubjectsIdResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (s Server) PatchSubjectsId(ctx context.Context, request api.PatchSubjectsIdRequestObject) (api.PatchSubjectsIdResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (s Server) GetTeachers(ctx context.Context, request api.GetTeachersRequestObject) (api.GetTeachersResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (s Server) PostTeachers(ctx context.Context, request api.PostTeachersRequestObject) (api.PostTeachersResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (s Server) DeleteTeachersId(ctx context.Context, request api.DeleteTeachersIdRequestObject) (api.DeleteTeachersIdResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (s Server) GetTeachersId(ctx context.Context, request api.GetTeachersIdRequestObject) (api.GetTeachersIdResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (s Server) PatchTeachersId(ctx context.Context, request api.PatchTeachersIdRequestObject) (api.PatchTeachersIdResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (s Server) GetTimetables(ctx context.Context, request api.GetTimetablesRequestObject) (api.GetTimetablesResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (s Server) PostTimetables(ctx context.Context, request api.PostTimetablesRequestObject) (api.PostTimetablesResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (s Server) DeleteTimetablesId(ctx context.Context, request api.DeleteTimetablesIdRequestObject) (api.DeleteTimetablesIdResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (s Server) GetTimetablesId(ctx context.Context, request api.GetTimetablesIdRequestObject) (api.GetTimetablesIdResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (s Server) PatchTimetablesId(ctx context.Context, request api.PatchTimetablesIdRequestObject) (api.PatchTimetablesIdResponseObject, error) {
	//TODO implement me
	panic("implement me")
}
