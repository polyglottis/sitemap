package sitemap

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gorilla/mux"
)

func TestStatic(t *testing.T) {
	dir, err := ioutil.TempDir("", "sitemap")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	r := NewRouter(mux.NewRouter(), "", dir)
	ts := httptest.NewServer(r)
	defer ts.Close()
	base := "/sitemaps/"
	r.Options.ServerPath = base
	r.Options.Domain = ts.URL

	routes := []string{"/test", "/test/two", "/"}

	for _, route := range routes {
		r.Register(route).HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "%s", r.URL.String())
		})
	}

	r.HandleSitemaps()

	index := new(SitemapIndex)
	mustGetXML(ts.URL+base+"/sitemapindex.xml", index, t)

	if len(index.SitemapRefs) != 1 {
		t.Fatal("Expecting exactly one sitemap")
	}

	loc := index.SitemapRefs[0].Location

	sm := new(Sitemap)
	mustGetXML(loc, sm, t)

	if len(sm.Entries) != len(routes) {
		t.Fatalf("Expecting %d but got %d urls in sitemap", len(routes), len(sm.Entries))
	}

	set := getLocationSet(sm.Entries)
	for _, route := range routes {
		fullRoute := ts.URL + route
		if _, ok := set[fullRoute]; ok {
			// assert route is correctly resolved
			bytes, err := getBytes(fullRoute)
			if err != nil {
				t.Fatal(err)
			}

			actual := string(bytes)
			if actual != route {
				t.Errorf("GET %s replies %s instead of %s", fullRoute, actual, route)
			}
		} else {
			t.Errorf("Route %s is missing in sitemap", fullRoute)
		}
	}
}

func TestDynamic(t *testing.T) {
	dir, err := ioutil.TempDir("", "sitemap")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	r := NewRouter(mux.NewRouter(), "", dir)
	ts := httptest.NewServer(r)
	defer ts.Close()
	r.Options.Domain = ts.URL

	routePattern := "/test/%s"
	route := fmt.Sprintf(routePattern, "{myParam}")
	paramValues := []string{"one", "two", "three"}

	r.RegisterParam(route, func(cb func(...string) error) error {
		for _, value := range paramValues {
			err := cb("myParam", value)
			if err != nil {
				return err
			}
		}
		return nil
	}).HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s\tmyParam=%s", r.URL.String(), mux.Vars(r)["myParam"])
	})

	r.HandleSitemaps()

	index := new(SitemapIndex)
	mustGetXML(ts.URL+"/sitemapindex.xml", index, t)

	if len(index.SitemapRefs) != 1 {
		t.Fatal("Expecting exactly one sitemap")
	}

	loc := index.SitemapRefs[0].Location

	sm := new(Sitemap)
	mustGetXML(loc, sm, t)

	if len(sm.Entries) != len(paramValues) {
		t.Fatalf("Expecting %d but got %d urls in sitemap", len(paramValues), len(sm.Entries))
	}

	set := getLocationSet(sm.Entries)
	for _, value := range paramValues {
		fullRoute := ts.URL + fmt.Sprintf(routePattern, value)
		if _, ok := set[fullRoute]; ok {
			// assert route is correctly resolved
			bytes, err := getBytes(fullRoute)
			if err != nil {
				t.Fatal(err)
			}

			actual := string(bytes)
			expected := fmt.Sprintf(routePattern, value) + "\tmyParam=" + value
			if actual != expected {
				t.Errorf("GET %s replies %s instead of %s", fullRoute, actual, expected)
			}
		} else {
			t.Errorf("Route %s is missing in sitemap", fullRoute)
		}
	}
}

func TestPackageDocumentation(t *testing.T) {
	dir, err := ioutil.TempDir("", "sitemap")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	r := /* sitemap. */ NewRouter(mux.NewRouter(), "http://example.com", dir)

	r.Register("/my/static/route") // .Handler(handler)

	// r.HandleFunc("/my/secret/route", f)

	documents := []struct {
		Category string
		Id       string
	}{{
		Category: "book",
		Id:       "AAA",
	}, {
		Category: "book",
		Id:       "BBB",
	}, {
		Category: "html",
		Id:       "WWW",
	}}

	r.RegisterParam("/documents/{category}/{id:[A-Z]+}", func(cb func(...string) error) error {
		for _, doc := range documents {
			err := cb("category", doc.Category, "id", doc.Id)
			if err != nil {
				return err
			}
		}
		return nil
	}) // .Handler(h)

	handler := r.HandleSitemaps()
	// http.Handle("/", r)

	req, err := http.NewRequest("GET", "http://example.com/sitemapindex.xml", nil)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	t.Logf("%d - %s", w.Code, w.Body.String())

	req, err = http.NewRequest("GET", "http://example.com/sitemap_1.xml", nil)
	if err != nil {
		t.Fatal(err)
	}

	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	t.Logf("%d - %s", w.Code, w.Body.String())
}

func getBytes(addr string) ([]byte, error) {
	res, err := http.Get(addr)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	return ioutil.ReadAll(res.Body)
}

func mustGetXML(addr string, v interface{}, t *testing.T) {
	bytes, err := getBytes(addr)
	if err != nil {
		t.Fatal(err)
	}

	err = xml.Unmarshal(bytes, v)
	if err != nil {
		t.Fatal(err)
	}
}

func getLocationSet(entries []*Entry) map[string]struct{} {
	m := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		m[e.Location] = struct{}{}
	}
	return m
}
