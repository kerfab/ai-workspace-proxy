package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

const (
	peopleContactReadMask      = "names,emailAddresses,organizations,metadata"
	peopleOtherContactReadMask = "names,emailAddresses,metadata"
	peopleSearchPageSizeLimit  = 30
)

func rewritePeopleRequest(method, path string, body []byte, query url.Values) ([]byte, string, string, error) {
	if method != http.MethodGet {
		return body, "", "", fmt.Errorf("People API contact lookup only allows GET requests")
	}
	query.Del("pageToken")
	query.Del("personFields")
	query.Del("requestMask")
	query.Del("mergeSources")
	clampPeoplePageSize(query)

	switch path {
	case "/v1/people:searchContacts":
		query.Set("readMask", peopleContactReadMask)
		query.Set("sources", "READ_SOURCE_TYPE_CONTACT")
	case "/v1/people:searchDirectoryPeople":
		query.Set("readMask", peopleContactReadMask)
		query.Del("sources[]")
		query["sources"] = []string{
			"DIRECTORY_SOURCE_TYPE_DOMAIN_PROFILE",
			"DIRECTORY_SOURCE_TYPE_DOMAIN_CONTACT",
		}
	case "/v1/otherContacts:search":
		query.Set("readMask", peopleOtherContactReadMask)
	default:
		return body, "", "", fmt.Errorf("People API path is not allowed")
	}

	return body, query.Encode(), "", nil
}

func clampPeoplePageSize(query url.Values) {
	pageSize := peopleSearchPageSizeLimit
	if raw := query.Get("pageSize"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err == nil && parsed > 0 && parsed < pageSize {
			pageSize = parsed
		}
	}
	query.Set("pageSize", strconv.Itoa(pageSize))
}
