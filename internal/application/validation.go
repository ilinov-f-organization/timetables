package application

import "github.com/jackc/pgx/v5/pgtype"

type searchParams struct {
	page     int32
	pageSize int32
	search   pgtype.Text
}

func newSearchParams(page *int32, pageSize *int32, search *string) searchParams {
	params := searchParams{
		page:     1,
		pageSize: 20,
		search: pgtype.Text{
			String: "",
			Valid:  false,
		},
	}
	if page != nil && *page > 0 {
		params.page = *page
	}
	if pageSize != nil && *pageSize > 0 {
		params.pageSize = *pageSize
	}
	if search != nil {
		params.search = pgtype.Text{
			String: *search,
			Valid:  true,
		}
	}
	return params
}
