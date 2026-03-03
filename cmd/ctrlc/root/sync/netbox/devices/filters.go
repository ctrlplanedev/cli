package devices

import (
	"net/url"
	"strconv"
)

const pageSize int32 = 1_000

type deviceFilters struct {
	Query         string
	Site          []string
	Role          []string
	Status        []string
	StatusExclude []string
	Tag           []string
	Tenant        []string
}

func (f deviceFilters) toQuery(offset int32) url.Values {
	q := url.Values{}
	q.Set("limit", strconv.Itoa(int(pageSize)))
	q.Set("offset", strconv.Itoa(int(offset)))

	if f.Query != "" {
		q.Set("q", f.Query)
	}
	for _, v := range f.Site {
		q.Add("site", v)
	}
	for _, v := range f.Role {
		q.Add("role", v)
	}
	for _, v := range f.Status {
		q.Add("status", v)
	}
	for _, v := range f.StatusExclude {
		q.Add("status_n", v)
	}
	for _, v := range f.Tag {
		q.Add("tag", v)
	}
	for _, v := range f.Tenant {
		q.Add("tenant", v)
	}

	return q
}
