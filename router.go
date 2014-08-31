/*
Package sitemap allows dynamic generation of a sitemap.
This package allows to:

	1. Register routes when binding handlers (embedding a github.com/gorilla/mux.Router),
	2. Create a sitemap on request (lazy-creation),
	3. Update the sitemap regularly and thread-safely.

The sitemap is stored (=cached) in XML format on disk, and served directly from there.

Example of use:

1. Create the router:

  	r := sitemap.NewRouter(mux.NewRouter(), "http://example.com", "local/path/to/sitemaps/cache")

2. Static route handler:

		r.Register("/my/static/route").Handler(handler)

... or a secret route (i.e. not appearing in the sitemap):

		r.HandleFunc("/my/secret/route", f)

3. Parameterized route:

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
		}).Handler(h)

4. Handle sitemap requests:

    r.HandleSitemaps()
    http.Handle("/", r)

So that an http GET on (r.Options.ServerPath + "sitemapindex.xml") returns:

		<?xml version="1.0" encoding="UTF-8"?>
		<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://www.sitemaps.org/schemas/sitemap/0.9 http://www.sitemaps.org/schemas/sitemap/0.9/siteindex.xsd">
		  <sitemap>
		    <loc>http://example.com/sitemap_1.xml</loc>
		  </sitemap>
		</sitemapindex>

and (r.Options.ServerPath + "sitemap_1.xml") returns:

		<?xml version="1.0" encoding="UTF-8"?>
		<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://www.sitemaps.org/schemas/sitemap/0.9 http://www.sitemaps.org/schemas/sitemap/0.9/sitemap.xsd">
		  <url>
		    <loc>http://example.com/my/static/route</loc>
		    <priority>0.5</priority>
		  </url>
		  <url>
		    <loc>http://example.com/documents/book/AAA</loc>
		    <priority>0.5</priority>
		  </url>
		  <url>
		    <loc>http://example.com/documents/book/BBB</loc>
		    <priority>0.5</priority>
		  </url>
		  <url>
		    <loc>http://example.com/documents/html/WWW</loc>
		    <priority>0.5</priority>
		  </url>
		</urlset>
*/
package sitemap

import (
	"html"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/mux"
)

// Router has an embedded github.com/gorilla/mux.Router.
// It retains all functionalities of the router (unchanged)
// and has a few extra methods to register the routes which should belong to the sitemap.
//
// See Register(), RegisterParam() and HandleSitemaps().
type Router struct {
	*mux.Router
	sitemapMutex  sync.RWMutex
	staticEntries []*path
	paramEntries  []*paramPath
	Options       *Options
}

// Options is used by Router.
type Options struct {
	CachePath       string  // path of a directory, to store sitemaps on disk
	ServerPath      string  // server path for sitemaps
	DefaultPriority float64 // default priority for sitemap entries
	Domain          string  // domain for entries in the sitemap (multiple domains are not supported)
}

// DefaultOptions is the default options used when calling NewRouter().
var DefaultOptions = &Options{
	ServerPath:      "/",
	DefaultPriority: 0.5,
}

// path represents a static route.
type path struct {
	Priority float64
	Location string
}

// paramPath represents a parameterized route.
type paramPath struct {
	Priority   float64
	Route      *mux.Route
	Enumerator VariableEnumerator
}

// VariableEnumerator calls the callback as many times as there are routes allowed.
//
// The arguments passed to the callback should be as needed by github.com/gorilla/mux.Route.URL().
//
// See the package's main documentation for an example.
type VariableEnumerator func(callback func(pairs ...string) error) error

// NewRouter wraps router into a new Router, ready to register sitemap urls for the given domain.
//
// localPath is the path where to store the sitemaps when created.
//
// Change the routers options if you want more control on the sitemap creation:
//
//     r := NewRouter(router, "example.com", "cache/sitemaps")
//     r.Options.DefaultPriority = 1
//     r.ServerPath = "/sitemaps/" // don't forget the trailing slash!
func NewRouter(router *mux.Router, domain, localPath string) *Router {
	if !strings.HasSuffix(localPath, "/") {
		localPath += "/"
	}
	options := new(Options)
	*options = *DefaultOptions
	options.Domain = domain
	options.CachePath = localPath
	return &Router{
		Router:  router,
		Options: options,
	}
}

// Register creates a static route (no variables in the path) and adds it to the sitemap.
func (r *Router) Register(pattern string) *mux.Route {
	r.staticEntries = append(r.staticEntries, &path{
		Location: pattern,
		Priority: r.Options.DefaultPriority,
	})
	return r.Path(pattern)
}

// RegisterParam creates a route with parameters (=variables) in the path.
// Each time the sitemap is (re-)created, enum is called to get the list of allowed variable values.
//
// See the package's main documentation for an example.
func (r *Router) RegisterParam(pattern string, enum VariableEnumerator) *mux.Route {
	route := r.Path(pattern)
	r.paramEntries = append(r.paramEntries, &paramPath{
		Route:      route,
		Priority:   r.Options.DefaultPriority,
		Enumerator: enum,
	})
	return route
}

func (r *Router) fullLocation(absPath string) string {
	return r.Options.Domain + html.EscapeString(absPath)
}

// GenerateSitemaps creates sitemapindex.xml and as many sitemaps as needed.
// Since there are size restrictions on a sitemap, there may be more than one.
// In this case they are named sitemap_1.xml, sitemap_2.xml, and so on.
//
// All files created is returned (paths relative to r.Options.CachePath).
//
// It is safe to call GenerateSitemaps() even when they are served due to a call to HandleSitemaps().
// A read-write lock takes care of queueing requests until the sitemaps are generated.
func (r *Router) GenerateSitemaps() ([]string, error) {
	r.sitemapMutex.Lock()
	defer r.sitemapMutex.Unlock()

	buffer := NewBuffer(r.Options.Domain, r.Options.CachePath)
	for _, entry := range r.staticEntries {
		buffer.AddEntry(&Entry{
			FileReference: &FileReference{
				Location: r.fullLocation(entry.Location),
			},
			Priority: &entry.Priority,
		})
	}
	for _, entry := range r.paramEntries {
		err := entry.Enumerator(func(pairs ...string) error {
			route, err := entry.Route.URL(pairs...)
			if err != nil {
				return err
			}
			return buffer.AddEntry(&Entry{
				FileReference: &FileReference{
					Location: r.fullLocation(route.String()),
				},
				Priority: &entry.Priority,
			})
		})
		if err != nil {
			return nil, err
		}
	}
	err := buffer.Flush()
	if err != nil {
		return nil, err
	}

	fullLocations := make([]string, len(buffer.Locations))
	for i, loc := range buffer.Locations {
		fullLocations[i] = r.Options.Domain + r.Options.ServerPath + loc
	}

	index := NewSitemapIndex(fullLocations)
	path := "sitemapindex.xml"
	err = index.WriteToFile(r.Options.CachePath + path)
	if err != nil {
		return nil, err
	}
	return append(buffer.Locations, path), nil
}

// HandleSitemaps register routes to serve the sitemap files on the router. The http handler is returned.
//
// All routes registered are:
//     r.Options.ServerPath + "sitemapindex.xml"
//     r.Options.ServerPath + "sitemap_%d.xml" // where %d is a replaced by a positive integer.
func (r *Router) HandleSitemaps() http.Handler {
	sitemapHandler := &sitemapHandler{
		router: r,
	}
	r.Handle(r.Options.ServerPath+"{file:sitemap(index|_\\d+)\\.xml}", sitemapHandler)
	return sitemapHandler
}
