package sitemap

import (
	"net/http"
	"os"
)

// sitemapHandler handles the requests to sitemaps.
// It uses the router's read-write lock to ensure only valid sitemaps are served.
// The sitemaps are created automatically on the first request.
type sitemapHandler struct {
	router      *Router
	fileHandler http.Handler
}

// ServeHTTP serves the sitemapindex and the sitemaps from disk.
// It generates the files if they don't exist.
func (sh *sitemapHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	mutex := sh.router.sitemapMutex
	mutex.RLock()
	defer mutex.RUnlock()

	if sh.fileHandler == nil {
		mutex.RUnlock()
		mutex.Lock()

		if sh.fileHandler == nil {
			// check if sitemap index file exists
			_, err := os.Open(sh.router.Options.CachePath + "sitemapindex.xml")
			if err != nil {
				os.MkdirAll(sh.router.Options.CachePath, os.ModeDir|os.ModePerm)
				_, err = sh.router.GenerateSitemaps()
				if err != nil {
					panic(err)
				}
			}
			sh.fileHandler = http.StripPrefix(sh.router.Options.ServerPath,
				http.FileServer(http.Dir(sh.router.Options.CachePath)))
		}

		mutex.Unlock()
		mutex.RLock()
	}
	if sh.fileHandler != nil {
		sh.fileHandler.ServeHTTP(w, r)
	}
}
