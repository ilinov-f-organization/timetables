package application

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"timetables/internal/api"
	"timetables/internal/lib"
	repository "timetables/internal/repository"
	sqlc "timetables/internal/repository/sqlc"

	uuid "github.com/google/uuid"
)

const multipartMaxMemory = 128 * 1024 * 1024

func (a *Application) GetErrors(ctx context.Context, request api.GetErrorsRequestObject) (api.GetErrorsResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (a *Application) GetErrorsId(ctx context.Context, request api.GetErrorsIdRequestObject) (api.GetErrorsIdResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (a *Application) PostImport(ctx context.Context, request api.PostImportRequestObject) (api.PostImportResponseObject, error) {
	switch ctx.Value(apiRole) {
	case roleUnauthorized:
		return api.PostImport401JSONResponse{
			Code: api.CODEUNAUTHORIZED,
		}, nil
	case roleUser:
		return api.PostImport403JSONResponse{
			Code: api.CODEFORBIDDEN,
		}, nil
	case roleAdmin:
		break
	}

	files, err := unarchive(request)
	if err != nil {
		message := err.Error()
		slog.ErrorContext(ctx, err.Error())
		return api.PostImport400JSONResponse{Code: api.CODEWRONGARCHIVE, Message: &message}, nil
	}
	parser := lib.NewParser()

	parseErrors := make([]api.Error, 0)
	var page int32 = 1
	var totalPages int32 = 1
	pagination := api.Pagination{
		Page:       page,
		TotalPages: totalPages,
	}
	for fileName, file := range files {
		err := parser.ParseLessonsFromBytes(file)
		if err != nil {
			message := err.Error()
			slog.ErrorContext(ctx, err.Error(), "fileName", fileName)
			parseErrors = append(parseErrors, api.Error{Code: api.CODEWRONGXLSX, Message: &message, File: &fileName})
			continue
		}
	}
	if err := insertIntoDB(ctx, a.repo, parser); err != nil {
		slog.ErrorContext(ctx, err.Error())
		message := err.Error()
		return api.PostImport500JSONResponse{
			Code:    api.CODEDBERROR,
			Message: &message,
		}, err
	}

	slog.InfoContext(ctx, "import successful")
	return api.PostImport200JSONResponse{
		Errors:     &parseErrors,
		Pagination: &pagination,
	}, nil
}

func unarchive(request api.PostImportRequestObject) (map[string][]byte, error) {
	files := make(map[string][]byte, 0)

	form, err := request.Body.ReadForm(multipartMaxMemory)
	if err != nil {
		return nil, err
	}

	archive := form.File["file"]
	if len(archive) == 0 {
		return nil, errors.New("file not provided")
	}

	fileHeader := archive[0]
	src, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()

	reader, err := zip.NewReader(src, fileHeader.Size)
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

func insertIntoDB(ctx context.Context, repo *repository.Repo, updater *lib.Parser) error {
	tx, err := repo.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	qtx := repo.WithTx(tx)
	if err := qtx.CreateStagingTables(ctx); err != nil {
		return err
	}

	if _, err := qtx.InsertStagingSubgroups(ctx, updater.Subgroups()); err != nil {
		return err
	}
	if _, err := qtx.InsertStagingTeachers(ctx, updater.Teachers()); err != nil {
		return err
	}
	if _, err := qtx.InsertStagingSubjects(ctx, updater.Subjects()); err != nil {
		return err
	}
	if _, err := qtx.InsertStagingLocations(ctx, updater.Locations()); err != nil {
		return err
	}
	if _, err := qtx.InsertStagingTimetables(ctx, updater.Timetables()); err != nil {
		return err
	}
	if _, err := qtx.InsertStagingLessons(ctx, updater.GetLessonsInsertParams()); err != nil {
		return err
	}
	if _, err := qtx.InsertStagingSubgroupsAssignments(ctx, updater.GetSubgroupsAssignments()); err != nil {
		return err
	}
	if _, err := qtx.InsertStagingTeacherLocationAssignments(ctx, updater.GetTeacherLocationAssignments()); err != nil {
		return err
	}
	if err := qtx.UpdateStagingHash(ctx); err != nil {
		return err
	}
	if err := qtx.FlushStagingToMain(ctx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

func (a *Application) PostLessons(ctx context.Context, request api.PostLessonsRequestObject) (api.PostLessonsResponseObject, error) {
	switch ctx.Value(apiRole) {
	case roleUnauthorized:
		return api.PostLessons401JSONResponse{
			Code: api.CODEUNAUTHORIZED,
		}, nil
	case roleUser:
		return api.PostLessons403JSONResponse{
			Code: api.CODEFORBIDDEN,
		}, nil
	case roleAdmin:
		break
	}

	tx, err := a.repo.Pool.Begin(ctx)
	if err != nil {
		message := err.Error()
		slog.ErrorContext(ctx, message)
		return api.PostLessons500JSONResponse{
			Code:    api.CODEDBERROR,
			Message: &message,
		}, nil
	}
	defer tx.Rollback(ctx)

	qtx := a.repo.WithTx(tx)

	lessonId, err := qtx.CreateLesson(ctx, sqlc.CreateLessonParams{
		SubjectID:   request.Body.Subject.Id,
		Category:    request.Body.Category,
		Day:         request.Body.Day,
		TimeStart:   request.Body.TimeStart,
		TimeEnd:     request.Body.TimeEnd,
		RepeatRule:  request.Body.RepeatRule,
		TimetableID: request.Body.Timetable.Id,
	})
	if err != nil {
		message := err.Error()
		return api.PostLessons400JSONResponse{
			Code:    api.CODEBADREQUEST,
			Message: &message,
		}, nil
	}

	saParams := make([]sqlc.CreateSubgroupAssignmentsParams, len(request.Body.Subgroups))
	for i, subgroup := range request.Body.Subgroups {
		saParams[i] = sqlc.CreateSubgroupAssignmentsParams{
			LessonID:   lessonId,
			SubgroupID: subgroup.Id,
		}
	}

	_, err = qtx.CreateSubgroupAssignments(ctx, saParams)
	if err != nil {
		message := err.Error()
		return api.PostLessons400JSONResponse{
			Code:    api.CODEBADREQUEST,
			Message: &message,
		}, nil
	}

	tlaParams := make([]sqlc.CreateTeacherLocationAssignmentsParams, len(request.Body.TeacherLocationAssignments))
	for i, assignment := range request.Body.TeacherLocationAssignments {
		tlaParams[i] = sqlc.CreateTeacherLocationAssignmentsParams{
			LessonID:   lessonId,
			TeacherID:  assignment.Teacher.Id,
			LocationID: assignment.Location.Id,
		}
	}

	_, err = qtx.CreateTeacherLocationAssignments(ctx, tlaParams)
	if err != nil {
		message := err.Error()
		return api.PostLessons400JSONResponse{
			Code:    api.CODEBADREQUEST,
			Message: &message,
		}, nil
	}

	err = tx.Commit(ctx)
	if err != nil {
		message := err.Error()
		slog.ErrorContext(ctx, message)
		return api.PostLessons500JSONResponse{
			Code:    api.CODEDBERROR,
			Message: &message,
		}, nil
	}

	response := api.PostLessons201JSONResponse(*request.Body)
	response.Id = lessonId
	return response, nil
}

func (a *Application) DeleteLessonsId(ctx context.Context, request api.DeleteLessonsIdRequestObject) (api.DeleteLessonsIdResponseObject, error) {
	switch ctx.Value(apiRole) {
	case roleUnauthorized:
		return api.DeleteLessonsId401JSONResponse{
			Code: api.CODEUNAUTHORIZED,
		}, nil
	case roleUser:
		return api.DeleteLessonsId403JSONResponse{
			Code: api.CODEFORBIDDEN,
		}, nil
	case roleAdmin:
		break
	}

	if err := a.repo.DeleteLesson(ctx, request.Id); err != nil {
		message := err.Error()
		slog.ErrorContext(ctx, message)
		return api.DeleteLessonsId500JSONResponse{
			Code:    api.CODEDBERROR,
			Message: &message,
		}, nil
	}

	return api.DeleteLessonsId200Response{}, nil
}

func (a *Application) GetLessonsId(ctx context.Context, request api.GetLessonsIdRequestObject) (api.GetLessonsIdResponseObject, error) {
	lesson, err := getApiLesson(ctx, a.repo, request.Id)
	if err != nil {
		message := err.Error()
		slog.ErrorContext(ctx, message)
		return api.GetLessonsId500JSONResponse{
			Code:    api.CODEDBERROR,
			Message: &message,
		}, nil
	}

	return api.GetLessonsId200JSONResponse(lesson), nil
}

func getApiLesson(ctx context.Context, repo *repository.Repo, id uuid.UUID) (api.Lesson, error) {
	repoLesson, err := repo.GetLesson(ctx, id)
	if err != nil {
		return api.Lesson{}, err
	}
	repoSubgroups, err := repo.GetLessonSubgroups(ctx, id)
	if err != nil {
		return api.Lesson{}, err
	}
	apiSubgroups := make([]api.Subgroup, len(repoSubgroups))
	for i, subgroup := range repoSubgroups {
		apiSubgroups[i] = api.Subgroup{
			Id:   subgroup.SubgroupID,
			Name: subgroup.SubgroupName,
		}
	}

	repoAssignmnents, err := repo.GetLessonAssignments(ctx, id)
	if err != nil {
		return api.Lesson{}, err
	}
	apiAssignments := make([]api.TeacherLocationAssignment, len(repoAssignmnents))
	for i, assignment := range repoAssignmnents {
		apiAssignments[i] = api.TeacherLocationAssignment{
			Location: api.Location{
				Id:   assignment.LocationID,
				Name: assignment.LocationName,
			},
			Teacher: api.Teacher{
				Id:   assignment.TeacherID,
				Name: assignment.TeacherName,
			},
		}
	}

	return api.Lesson{
		Category:   repoLesson.Category,
		Day:        repoLesson.Day,
		Id:         id,
		RepeatRule: repoLesson.RepeatRule,
		Subgroups:  apiSubgroups,
		Subject: api.Subject{
			Id:   repoLesson.SubjectID,
			Name: repoLesson.SubjectName,
		},
		TeacherLocationAssignments: apiAssignments,
		TimeEnd:                    repoLesson.TimeEnd,
		TimeStart:                  repoLesson.TimeStart,
		Timetable: api.Timetable{
			EndDate:   repoLesson.DateEnd,
			Id:        repoLesson.TimetableID,
			Name:      repoLesson.TimetableName,
			StartDate: repoLesson.DateStart,
		},
	}, nil
}

func (a *Application) PatchLessonsId(ctx context.Context, request api.PatchLessonsIdRequestObject) (api.PatchLessonsIdResponseObject, error) {
	switch ctx.Value(apiRole) {
	case roleUnauthorized:
		return api.PatchLessonsId401JSONResponse{
			Code: api.CODEUNAUTHORIZED,
		}, nil
	case roleUser:
		return api.PatchLessonsId403JSONResponse{
			Code: api.CODEFORBIDDEN,
		}, nil
	case roleAdmin:
		break
	}

	tx, err := a.repo.Pool.Begin(ctx)
	if err != nil {
		message := err.Error()
		slog.ErrorContext(ctx, message)
		return api.PatchLessonsId500JSONResponse{
			Code:    api.CODEDBERROR,
			Message: &message,
		}, nil
	}
	defer tx.Rollback(ctx)

	qtx := a.repo.WithTx(tx)

	err = qtx.DeleteLessonAssignments(ctx, request.Id)
	if err != nil {
		message := err.Error()
		slog.ErrorContext(ctx, message)
		return api.PatchLessonsId500JSONResponse{
			Code:    api.CODEDBERROR,
			Message: &message,
		}, nil
	}

	repoSubgroups := make([]sqlc.CreateSubgroupAssignmentsParams, len(request.Body.Subgroups))
	for i, subgroup := range request.Body.Subgroups {
		repoSubgroups[i] = sqlc.CreateSubgroupAssignmentsParams{
			LessonID:   request.Id,
			SubgroupID: subgroup.Id,
		}
	}

	repoAssignments := make([]sqlc.CreateTeacherLocationAssignmentsParams, len(request.Body.TeacherLocationAssignments))
	for i, assignment := range request.Body.TeacherLocationAssignments {
		repoAssignments[i] = sqlc.CreateTeacherLocationAssignmentsParams{
			LessonID:   request.Id,
			TeacherID:  assignment.Teacher.Id,
			LocationID: assignment.Location.Id,
		}
	}

	if _, err = qtx.CreateSubgroupAssignments(ctx, repoSubgroups); err != nil {
		message := err.Error()
		slog.ErrorContext(ctx, message)
		return api.PatchLessonsId500JSONResponse{
			Code:    api.CODEDBERROR,
			Message: &message,
		}, nil
	}

	if _, err = qtx.CreateTeacherLocationAssignments(ctx, repoAssignments); err != nil {
		message := err.Error()
		slog.ErrorContext(ctx, message)
		return api.PatchLessonsId500JSONResponse{
			Code:    api.CODEDBERROR,
			Message: &message,
		}, nil
	}

	if err = qtx.PatchLesson(ctx, sqlc.PatchLessonParams{
		SubjectID:   request.Body.Subject.Id,
		Category:    request.Body.Category,
		Day:         request.Body.Day,
		TimeStart:   request.Body.TimeStart,
		TimeEnd:     request.Body.TimeEnd,
		RepeatRule:  request.Body.RepeatRule,
		TimetableID: request.Body.Timetable.Id,
		ID:          request.Id,
	}); err != nil {
		message := err.Error()
		slog.ErrorContext(ctx, message)
		return api.PatchLessonsId500JSONResponse{
			Code:    api.CODEDBERROR,
			Message: &message,
		}, nil
	}
	err = tx.Commit(ctx)
	if err != nil {
		message := err.Error()
		slog.ErrorContext(ctx, message)
		return api.PatchLessonsId500JSONResponse{
			Code:    api.CODEDBERROR,
			Message: &message,
		}, nil
	}

	response := api.PatchLessonsId200JSONResponse(*request.Body)
	response.Id = request.Id
	return response, nil
}

func (a *Application) GetLessonsTableId(ctx context.Context, request api.GetLessonsTableIdRequestObject) (api.GetLessonsTableIdResponseObject, error) {
	lessons := make([]api.Lesson, 0)
	var err error = nil
	switch request.Table {
	case api.Teachers:
		lessons, err = getLessonByTeacherId(ctx, a.repo, request)
	case api.Subgroups:
		lessons, err = getLessonBySubgroupId(ctx, a.repo, request)
	case api.Locations:
		lessons, err = getLessonByLocationId(ctx, a.repo, request)
	case api.Subjects:
		lessons, err = getLessonBySubjectId(ctx, a.repo, request)
	default:
		return api.GetLessonsTableId400JSONResponse{
			Code: api.CODEBADREQUEST,
		}, nil
	}

	if err != nil {
		message := err.Error()
		slog.ErrorContext(ctx, message)
		return api.GetLessonsTableId500JSONResponse{
			Code:    api.CODEDBERROR,
			Message: &message,
		}, nil
	}

	if request.Params.Format == nil || *request.Params.Format == api.Json {
		return api.GetLessonsTableId200JSONResponse(lessons), nil
	} else if *request.Params.Format == api.Ics {
		ics, err := lib.SerializeICS(lessons, "KAL")
		if err != nil {
			message := err.Error()
			return api.GetLessonsTableId500JSONResponse{
				Code:    api.CODECANNOTSERIALIZEICS,
				Message: &message,
			}, nil
		}
		reader := bytes.NewReader(ics)
		return api.GetLessonsTableId200TextcalendarResponse{
			Body:          reader,
			ContentLength: reader.Size(),
		}, nil
	}

	return api.GetLessonsTableId400JSONResponse{
		Code: api.CODEBADREQUEST,
	}, nil
}

func getLessonByTeacherId(ctx context.Context, repo *repository.Repo, request api.GetLessonsTableIdRequestObject) ([]api.Lesson, error) {
	lessons := make(map[uuid.UUID]*api.Lesson, 0)
	rowsLessons, err := repo.GetLessonsByTeacherId(ctx, request.Id)
	if err != nil {
		return nil, err
	}
	for _, row := range rowsLessons {
		lesson := api.Lesson{
			Category:   row.Category,
			Day:        row.Day,
			Id:         row.ID,
			RepeatRule: row.RepeatRule,
			Subgroups:  make([]api.Subgroup, 0),
			Subject: api.Subject{
				Id:   row.SubjectID,
				Name: row.SubjectName,
			},
			TeacherLocationAssignments: make([]api.TeacherLocationAssignment, 0),
			TimeEnd:                    row.TimeEnd,
			TimeStart:                  row.TimeStart,
			Timetable: api.Timetable{
				EndDate:   row.DateEnd,
				Id:        row.TimetableID,
				Name:      row.TimetableName,
				StartDate: row.DateStart,
			},
		}
		lessons[row.ID] = &lesson
	}

	rowsSubgroups, err := repo.GetLessonsSubgroupsByTeacherId(ctx, request.Id)
	if err != nil {
		return nil, err
	}
	for _, row := range rowsSubgroups {
		lessons[row.LessonID].Subgroups = append(lessons[row.LessonID].Subgroups, api.Subgroup{
			Id:   row.SubgroupID,
			Name: row.SubgroupName,
		})
	}

	rowsAssignments, err := repo.GetLessonAssignmentsByTeacherId(ctx, request.Id)
	if err != nil {
		return nil, err
	}
	for _, row := range rowsAssignments {
		lessons[row.LessonID].TeacherLocationAssignments = append(lessons[row.LessonID].TeacherLocationAssignments, api.TeacherLocationAssignment{
			Location: api.Location{
				Id:   row.LocationID,
				Name: row.LocationName,
			},
			Teacher: api.Teacher{
				Id:   row.TeacherID,
				Name: row.TeacherName,
			},
		})
	}
	result := make([]api.Lesson, len(lessons))
	i := 0
	for _, lesson := range lessons {
		result[i] = *lesson
		i++
	}
	return result, nil
}

func getLessonBySubgroupId(ctx context.Context, repo *repository.Repo, request api.GetLessonsTableIdRequestObject) ([]api.Lesson, error) {
	lessons := make(map[uuid.UUID]*api.Lesson, 0)
	rowsLessons, err := repo.GetLessonsBySubgroupId(ctx, request.Id)
	if err != nil {
		return nil, err
	}
	for _, row := range rowsLessons {
		lesson := api.Lesson{
			Category:   row.Category,
			Day:        row.Day,
			Id:         row.ID,
			RepeatRule: row.RepeatRule,
			Subgroups:  make([]api.Subgroup, 0),
			Subject: api.Subject{
				Id:   row.SubjectID,
				Name: row.SubjectName,
			},
			TeacherLocationAssignments: make([]api.TeacherLocationAssignment, 0),
			TimeEnd:                    row.TimeEnd,
			TimeStart:                  row.TimeStart,
			Timetable: api.Timetable{
				EndDate:   row.DateEnd,
				Id:        row.TimetableID,
				Name:      row.TimetableName,
				StartDate: row.DateStart,
			},
		}
		lessons[row.ID] = &lesson
	}

	rowsSubgroups, err := repo.GetLessonsSubgroupsBySubgroupId(ctx, request.Id)
	if err != nil {
		return nil, err
	}
	for _, row := range rowsSubgroups {
		lessons[row.LessonID].Subgroups = append(lessons[row.LessonID].Subgroups, api.Subgroup{
			Id:   row.SubgroupID,
			Name: row.SubgroupName,
		})
	}

	rowsAssignments, err := repo.GetLessonAssignmentsBySubgroupId(ctx, request.Id)
	if err != nil {
		return nil, err
	}
	for _, row := range rowsAssignments {
		lessons[row.LessonID].TeacherLocationAssignments = append(lessons[row.LessonID].TeacherLocationAssignments, api.TeacherLocationAssignment{
			Location: api.Location{
				Id:   row.LocationID,
				Name: row.LocationName,
			},
			Teacher: api.Teacher{
				Id:   row.TeacherID,
				Name: row.TeacherName,
			},
		})
	}
	result := make([]api.Lesson, len(lessons))
	i := 0
	for _, lesson := range lessons {
		result[i] = *lesson
		i++
	}
	return result, nil
}

func getLessonByLocationId(ctx context.Context, repo *repository.Repo, request api.GetLessonsTableIdRequestObject) ([]api.Lesson, error) {
	lessons := make(map[uuid.UUID]*api.Lesson, 0)
	rowsLessons, err := repo.GetLessonsByLocationsId(ctx, request.Id)
	if err != nil {
		return nil, err
	}
	for _, row := range rowsLessons {
		lesson := api.Lesson{
			Category:   row.Category,
			Day:        row.Day,
			Id:         row.ID,
			RepeatRule: row.RepeatRule,
			Subgroups:  make([]api.Subgroup, 0),
			Subject: api.Subject{
				Id:   row.SubjectID,
				Name: row.SubjectName,
			},
			TeacherLocationAssignments: make([]api.TeacherLocationAssignment, 0),
			TimeEnd:                    row.TimeEnd,
			TimeStart:                  row.TimeStart,
			Timetable: api.Timetable{
				EndDate:   row.DateEnd,
				Id:        row.TimetableID,
				Name:      row.TimetableName,
				StartDate: row.DateStart,
			},
		}
		lessons[row.ID] = &lesson
	}

	rowsSubgroups, err := repo.GetLessonsSubgroupsByLocationId(ctx, request.Id)
	if err != nil {
		return nil, err
	}
	for _, row := range rowsSubgroups {
		lessons[row.LessonID].Subgroups = append(lessons[row.LessonID].Subgroups, api.Subgroup{
			Id:   row.SubgroupID,
			Name: row.SubgroupName,
		})
	}

	rowsAssignments, err := repo.GetLessonAssignmentsByLocationId(ctx, request.Id)
	if err != nil {
		return nil, err
	}
	for _, row := range rowsAssignments {
		lessons[row.LessonID].TeacherLocationAssignments = append(lessons[row.LessonID].TeacherLocationAssignments, api.TeacherLocationAssignment{
			Location: api.Location{
				Id:   row.LocationID,
				Name: row.LocationName,
			},
			Teacher: api.Teacher{
				Id:   row.TeacherID,
				Name: row.TeacherName,
			},
		})
	}
	result := make([]api.Lesson, len(lessons))
	i := 0
	for _, lesson := range lessons {
		result[i] = *lesson
		i++
	}
	return result, nil
}

func getLessonBySubjectId(ctx context.Context, repo *repository.Repo, request api.GetLessonsTableIdRequestObject) ([]api.Lesson, error) {
	lessons := make(map[uuid.UUID]*api.Lesson, 0)
	rowsLessons, err := repo.GetLessonsBySubjectId(ctx, request.Id)
	if err != nil {
		return nil, err
	}
	for _, row := range rowsLessons {
		lesson := api.Lesson{
			Category:   row.Category,
			Day:        row.Day,
			Id:         row.ID,
			RepeatRule: row.RepeatRule,
			Subgroups:  make([]api.Subgroup, 0),
			Subject: api.Subject{
				Id:   row.SubjectID,
				Name: row.SubjectName,
			},
			TeacherLocationAssignments: make([]api.TeacherLocationAssignment, 0),
			TimeEnd:                    row.TimeEnd,
			TimeStart:                  row.TimeStart,
			Timetable: api.Timetable{
				EndDate:   row.DateEnd,
				Id:        row.TimetableID,
				Name:      row.TimetableName,
				StartDate: row.DateStart,
			},
		}
		lessons[row.ID] = &lesson
	}

	rowsSubgroups, err := repo.GetLessonsSubgroupsBySubjectId(ctx, request.Id)
	if err != nil {
		return nil, err
	}
	for _, row := range rowsSubgroups {
		lessons[row.LessonID].Subgroups = append(lessons[row.LessonID].Subgroups, api.Subgroup{
			Id:   row.SubgroupID,
			Name: row.SubgroupName,
		})
	}

	rowsAssignments, err := repo.GetLessonAssignmentsBySubjectId(ctx, request.Id)
	if err != nil {
		return nil, err
	}
	for _, row := range rowsAssignments {
		lessons[row.LessonID].TeacherLocationAssignments = append(lessons[row.LessonID].TeacherLocationAssignments, api.TeacherLocationAssignment{
			Location: api.Location{
				Id:   row.LocationID,
				Name: row.LocationName,
			},
			Teacher: api.Teacher{
				Id:   row.TeacherID,
				Name: row.TeacherName,
			},
		})
	}
	result := make([]api.Lesson, len(lessons))
	i := 0
	for _, lesson := range lessons {
		result[i] = *lesson
		i++
	}
	return result, nil
}

func (a *Application) GetLocations(ctx context.Context, request api.GetLocationsRequestObject) (api.GetLocationsResponseObject, error) {
	params := newSearchParams(request.Params.Page, request.Params.PageSize, request.Params.Search)
	locations, err := a.repo.GetLocationsOnPage(ctx, sqlc.GetLocationsOnPageParams{
		Name:     params.search,
		PageSize: params.pageSize,
		Page:     params.page,
	})
	if err != nil {
		message := err.Error()
		slog.ErrorContext(ctx, message)
		return api.GetLocations500JSONResponse{
			Code:    api.CODEDBERROR,
			Message: &message,
		}, nil
	}
	amount, err := a.repo.GetLocationsPagesAmount(ctx, params.pageSize)
	if err != nil {
		message := err.Error()
		slog.ErrorContext(ctx, message)
		return api.GetLocations500JSONResponse{
			Code:    api.CODEDBERROR,
			Message: &message,
		}, nil
	}

	response := api.ListLocations{
		Locations: make([]api.Location, len(locations)),
		Pagination: api.Pagination{
			Page:       params.page,
			TotalPages: amount,
		},
	}
	for i, location := range locations {
		response.Locations[i] = api.Location{
			Id:   location.ID,
			Name: location.Name,
		}
	}

	return api.GetLocations200JSONResponse(response), nil
}

func (a *Application) PostLocations(ctx context.Context, request api.PostLocationsRequestObject) (api.PostLocationsResponseObject, error) {
	switch ctx.Value(apiRole) {
	case roleUnauthorized:
		return api.PostLocations401JSONResponse{
			Code: api.CODEUNAUTHORIZED,
		}, nil
	case roleUser:
		return api.PostLocations403JSONResponse{
			Code: api.CODEFORBIDDEN,
		}, nil
	case roleAdmin:
		break
	}

	location, err := a.repo.CreateLocation(ctx, request.Body.Name)
	if err != nil {
		message := err.Error()
		slog.ErrorContext(ctx, message)
		return api.PostLocations500JSONResponse{
			Code:    api.CODEDBERROR,
			Message: &message,
		}, nil
	}

	return api.PostLocations201JSONResponse{
		Id:   location.ID,
		Name: location.Name,
	}, nil
}

func (a *Application) DeleteLocationsId(ctx context.Context, request api.DeleteLocationsIdRequestObject) (api.DeleteLocationsIdResponseObject, error) {
	switch ctx.Value(apiRole) {
	case roleUnauthorized:
		return api.DeleteLocationsId401JSONResponse{
			Code: api.CODEUNAUTHORIZED,
		}, nil
	case roleUser:
		return api.DeleteLocationsId403JSONResponse{
			Code: api.CODEFORBIDDEN,
		}, nil
	case roleAdmin:
		break
	}

	_, err := a.repo.DeleteLocationById(ctx, request.Id)
	if err != nil {
		message := err.Error()
		slog.ErrorContext(ctx, message)
		return api.DeleteLocationsId500JSONResponse{
			Code:    api.CODEDBERROR,
			Message: &message,
		}, nil
	}

	return api.DeleteLocationsId200Response{}, nil
}

func (a *Application) GetLocationsId(ctx context.Context, request api.GetLocationsIdRequestObject) (api.GetLocationsIdResponseObject, error) {
	location, err := a.repo.GetLocationById(ctx, request.Id)
	if err != nil {
		message := err.Error()
		slog.ErrorContext(ctx, message)
		return api.GetLocationsId500JSONResponse{
			Code:    api.CODEDBERROR,
			Message: &message,
		}, nil
	}

	return api.GetLocationsId200JSONResponse{
		Id:   location.ID,
		Name: location.Name,
	}, nil
}

func (a *Application) PatchLocationsId(ctx context.Context, request api.PatchLocationsIdRequestObject) (api.PatchLocationsIdResponseObject, error) {
	switch ctx.Value(apiRole) {
	case roleUnauthorized:
		return api.PatchLocationsId401JSONResponse{
			Code: api.CODEUNAUTHORIZED,
		}, nil
	case roleUser:
		return api.PatchLocationsId403JSONResponse{
			Code: api.CODEFORBIDDEN,
		}, nil
	case roleAdmin:
		break
	}

	location, err := a.repo.PatchLocationById(ctx, sqlc.PatchLocationByIdParams{
		Name: request.Body.Name,
		ID:   request.Id,
	})
	if err != nil {
		message := err.Error()
		slog.ErrorContext(ctx, message)
		return api.PatchLocationsId500JSONResponse{
			Code:    api.CODEDBERROR,
			Message: &message,
		}, nil
	}

	return api.PatchLocationsId200JSONResponse{
		Id:   location.ID,
		Name: location.Name,
	}, nil
}

func (a *Application) GetSubgroups(ctx context.Context, request api.GetSubgroupsRequestObject) (api.GetSubgroupsResponseObject, error) {
	params := newSearchParams(request.Params.Page, request.Params.PageSize, request.Params.Search)
	subgroups, err := a.repo.GetSubgroupsOnPage(ctx, sqlc.GetSubgroupsOnPageParams{
		Name:     params.search,
		PageSize: params.pageSize,
		Page:     params.page,
	})
	if err != nil {
		message := err.Error()
		slog.ErrorContext(ctx, message)
		return api.GetSubgroups500JSONResponse{
			Code:    api.CODEDBERROR,
			Message: &message,
		}, nil
	}
	amount, err := a.repo.GetSubgroupsPagesAmount(ctx, params.pageSize)
	if err != nil {
		message := err.Error()
		slog.ErrorContext(ctx, message)
		return api.GetSubgroups500JSONResponse{
			Code:    api.CODEDBERROR,
			Message: &message,
		}, nil
	}

	response := api.ListSubgroups{
		Subgroups: make([]api.Subgroup, len(subgroups)),
		Pagination: api.Pagination{
			Page:       params.page,
			TotalPages: amount,
		},
	}
	for i, subgroup := range subgroups {
		response.Subgroups[i] = api.Subgroup{
			Id:   subgroup.ID,
			Name: subgroup.Name,
		}
	}

	return api.GetSubgroups200JSONResponse(response), nil

}

func (a *Application) PostSubgroups(ctx context.Context, request api.PostSubgroupsRequestObject) (api.PostSubgroupsResponseObject, error) {
	switch ctx.Value(apiRole) {
	case roleUnauthorized:
		return api.PostSubgroups401JSONResponse{
			Code: api.CODEUNAUTHORIZED,
		}, nil
	case roleUser:
		return api.PostSubgroups403JSONResponse{
			Code: api.CODEFORBIDDEN,
		}, nil
	case roleAdmin:
		break
	}

	subgroup, err := a.repo.CreateSubgroup(ctx, request.Body.Name)
	if err != nil {
		message := err.Error()
		slog.ErrorContext(ctx, message)
		return api.PostSubgroups500JSONResponse{
			Code:    api.CODEDBERROR,
			Message: &message,
		}, nil
	}

	return api.PostSubgroups201JSONResponse{
		Id:   subgroup.ID,
		Name: subgroup.Name,
	}, nil

}

func (a *Application) DeleteSubgroupsId(ctx context.Context, request api.DeleteSubgroupsIdRequestObject) (api.DeleteSubgroupsIdResponseObject, error) {
	switch ctx.Value(apiRole) {
	case roleUnauthorized:
		return api.DeleteSubgroupsId401JSONResponse{
			Code: api.CODEUNAUTHORIZED,
		}, nil
	case roleUser:
		return api.DeleteSubgroupsId403JSONResponse{
			Code: api.CODEFORBIDDEN,
		}, nil
	case roleAdmin:
		break
	}

	_, err := a.repo.DeleteSubgroupById(ctx, request.Id)
	if err != nil {
		message := err.Error()
		slog.ErrorContext(ctx, message)
		return api.DeleteSubgroupsId500JSONResponse{
			Code:    api.CODEDBERROR,
			Message: &message,
		}, nil
	}

	return api.DeleteSubgroupsId200Response{}, nil

}

func (a *Application) GetSubgroupsId(ctx context.Context, request api.GetSubgroupsIdRequestObject) (api.GetSubgroupsIdResponseObject, error) {
	subgroup, err := a.repo.GetSubgroupById(ctx, request.Id)
	if err != nil {
		message := err.Error()
		slog.ErrorContext(ctx, message)
		return api.GetSubgroupsId500JSONResponse{
			Code:    api.CODEDBERROR,
			Message: &message,
		}, nil
	}

	return api.GetSubgroupsId200JSONResponse{
		Id:   subgroup.ID,
		Name: subgroup.Name,
	}, nil
}

func (a *Application) PatchSubgroupsId(ctx context.Context, request api.PatchSubgroupsIdRequestObject) (api.PatchSubgroupsIdResponseObject, error) {
	switch ctx.Value(apiRole) {
	case roleUnauthorized:
		return api.PatchSubgroupsId401JSONResponse{
			Code: api.CODEUNAUTHORIZED,
		}, nil
	case roleUser:
		return api.PatchSubgroupsId403JSONResponse{
			Code: api.CODEFORBIDDEN,
		}, nil
	case roleAdmin:
		break
	}

	subgroup, err := a.repo.PatchSubgroupById(ctx, sqlc.PatchSubgroupByIdParams{
		Name: request.Body.Name,
		ID:   request.Id,
	})
	if err != nil {
		message := err.Error()
		slog.ErrorContext(ctx, message)
		return api.PatchSubgroupsId500JSONResponse{
			Code:    api.CODEDBERROR,
			Message: &message,
		}, nil
	}

	return api.PatchSubgroupsId200JSONResponse{
		Id:   subgroup.ID,
		Name: subgroup.Name,
	}, nil

}

func (a *Application) GetSubjects(ctx context.Context, request api.GetSubjectsRequestObject) (api.GetSubjectsResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (a *Application) PostSubjects(ctx context.Context, request api.PostSubjectsRequestObject) (api.PostSubjectsResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (a *Application) DeleteSubjectsId(ctx context.Context, request api.DeleteSubjectsIdRequestObject) (api.DeleteSubjectsIdResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (a *Application) GetSubjectsId(ctx context.Context, request api.GetSubjectsIdRequestObject) (api.GetSubjectsIdResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (a *Application) PatchSubjectsId(ctx context.Context, request api.PatchSubjectsIdRequestObject) (api.PatchSubjectsIdResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (a *Application) GetTeachers(ctx context.Context, request api.GetTeachersRequestObject) (api.GetTeachersResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (a *Application) PostTeachers(ctx context.Context, request api.PostTeachersRequestObject) (api.PostTeachersResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (a *Application) DeleteTeachersId(ctx context.Context, request api.DeleteTeachersIdRequestObject) (api.DeleteTeachersIdResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (a *Application) GetTeachersId(ctx context.Context, request api.GetTeachersIdRequestObject) (api.GetTeachersIdResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (a *Application) PatchTeachersId(ctx context.Context, request api.PatchTeachersIdRequestObject) (api.PatchTeachersIdResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (a *Application) GetTimetables(ctx context.Context, request api.GetTimetablesRequestObject) (api.GetTimetablesResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (a *Application) PostTimetables(ctx context.Context, request api.PostTimetablesRequestObject) (api.PostTimetablesResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (a *Application) DeleteTimetablesId(ctx context.Context, request api.DeleteTimetablesIdRequestObject) (api.DeleteTimetablesIdResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (a *Application) GetTimetablesId(ctx context.Context, request api.GetTimetablesIdRequestObject) (api.GetTimetablesIdResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (a *Application) PatchTimetablesId(ctx context.Context, request api.PatchTimetablesIdRequestObject) (api.PatchTimetablesIdResponseObject, error) {
	//TODO implement me
	panic("implement me")
}
