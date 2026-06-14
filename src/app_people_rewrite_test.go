package main

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestRewritePeopleRequestSanitizesContactSearchQuery(t *testing.T) {
	query := url.Values{}
	query.Set("query", "Frederic")
	query.Set("readMask", "names,emailAddresses,phoneNumbers,birthdays")
	query.Set("pageSize", "500")
	query.Set("pageToken", "next")

	_, rawQuery, _, err := rewritePeopleRequest(http.MethodGet, "/v1/people:searchContacts", nil, query)
	if err != nil {
		t.Fatal(err)
	}
	got, err := url.ParseQuery(rawQuery)
	if err != nil {
		t.Fatal(err)
	}
	if got.Get("readMask") != peopleContactReadMask {
		t.Fatalf("readMask = %q, want %q", got.Get("readMask"), peopleContactReadMask)
	}
	if got.Get("pageSize") != "30" {
		t.Fatalf("pageSize = %q, want 30", got.Get("pageSize"))
	}
	if got.Get("pageToken") != "" {
		t.Fatalf("pageToken was not removed: %q", got.Get("pageToken"))
	}
	if got.Get("sources") != "READ_SOURCE_TYPE_CONTACT" {
		t.Fatalf("sources = %q, want READ_SOURCE_TYPE_CONTACT", got.Get("sources"))
	}
}

func TestRewritePeopleRequestSanitizesDirectorySearchQuery(t *testing.T) {
	query := url.Values{}
	query.Set("query", "Frederic")
	query.Set("pageSize", "5")
	query.Set("mergeSources", "DIRECTORY_MERGE_SOURCE_TYPE_CONTACT")

	_, rawQuery, _, err := rewritePeopleRequest(http.MethodGet, "/v1/people:searchDirectoryPeople", nil, query)
	if err != nil {
		t.Fatal(err)
	}
	got, err := url.ParseQuery(rawQuery)
	if err != nil {
		t.Fatal(err)
	}
	if got.Get("readMask") != peopleContactReadMask {
		t.Fatalf("readMask = %q, want %q", got.Get("readMask"), peopleContactReadMask)
	}
	if got.Get("pageSize") != "5" {
		t.Fatalf("pageSize = %q, want 5", got.Get("pageSize"))
	}
	if got.Get("mergeSources") != "" {
		t.Fatalf("mergeSources was not removed: %q", got.Get("mergeSources"))
	}
	sources := strings.Join(got["sources"], ",")
	if sources != "DIRECTORY_SOURCE_TYPE_DOMAIN_PROFILE,DIRECTORY_SOURCE_TYPE_DOMAIN_CONTACT" {
		t.Fatalf("sources = %q, want both directory sources", sources)
	}
}

func TestRewritePeopleRequestRejectsNonGet(t *testing.T) {
	_, _, _, err := rewritePeopleRequest(http.MethodPost, "/v1/people:searchContacts", nil, url.Values{})
	if err == nil {
		t.Fatal("expected POST People API search rewrite to fail")
	}
}
